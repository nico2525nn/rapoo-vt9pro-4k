using System.Diagnostics;
using RapooControl.Services;

namespace RapooControl.Tests;

[TestFixture]
public sealed class RapooBridgeClientTests
{
    private string? _previousOverride;

    [SetUp]
    public void SetUp()
    {
        _previousOverride = Environment.GetEnvironmentVariable("RAPOO_BRIDGE_PATH");
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", null);
    }

    [TearDown]
    public void TearDown()
    {
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", _previousOverride);
    }

    [Test]
    public async Task ObjectError_CompletesTheRequestInsteadOfHanging()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var script = CreateScript(
            """
            #!/bin/sh
            printf '%s\n' '{"event":"ready","data":{"version":"1"}}'
            while IFS= read -r line; do
              printf '%s\n' '{"id":1,"error":{"code":42,"message":"boom"}}'
            done
            """);
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        await using var client = new RapooBridgeClient();
        try
        {
            await client.StartAsync();
            Assert.ThrowsAsync<RapooBridgeException>(
                () => client.GetStateAsync().WaitAsync(TimeSpan.FromSeconds(3)));
        }
        finally
        {
            File.Delete(script);
        }
    }

    [Test]
    public async Task EarlyExit_FaultsReadinessInsteadOfWaitingForTimeout()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var script = CreateScript(
            """
            #!/bin/sh
            exit 3
            """);
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        await using var client = new RapooBridgeClient();
        try
        {
            var started = Stopwatch.StartNew();
            Assert.ThrowsAsync<InvalidOperationException>(
                () => client.StartAsync().WaitAsync(TimeSpan.FromSeconds(3)));
            Assert.That(started.Elapsed, Is.LessThan(TimeSpan.FromSeconds(3)));
        }
        finally
        {
            File.Delete(script);
        }
    }

    [Test]
    public async Task StopAndRestart_StartsAFreshSession()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var script = CreateScript(
            """
            #!/bin/sh
            printf '%s\n' '{"event":"ready","data":{"version":"1"}}'
            while IFS= read -r line; do
              printf '%s\n' '{"id":1,"result":{"connected":false,"demoMode":false,"deviceName":"Fake","maxRateHz":4000}}'
            done
            """);
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        await using var client = new RapooBridgeClient();
        try
        {
            await client.StartAsync();
            Assert.That(client.IsRunning, Is.True);
            await client.StopAsync();
            Assert.That(client.IsRunning, Is.False);
            await client.StartAsync();
            Assert.That(client.IsRunning, Is.True);
        }
        finally
        {
            File.Delete(script);
        }
    }


    [Test]
    public async Task VoidReplyWithoutResult_CompletesDeletePreset()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var script = CreateScript(
            """
            #!/bin/sh
            printf '%s\n' '{"event":"ready","data":{"version":"1"}}'
            while IFS= read -r line; do
              printf '%s\n' '{"id":1}'
            done
            """);
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        await using var client = new RapooBridgeClient();
        try
        {
            await client.StartAsync();
            await client.DeletePresetAsync("x").WaitAsync(TimeSpan.FromSeconds(3));
        }
        finally
        {
            File.Delete(script);
        }
    }

    [Test]
    public async Task ConcurrentStartAndDispose_DoesNotLeaveABridgeRunning()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var script = CreateScript(
            """
            #!/bin/sh
            sleep 0.4
            printf '%s\n' '{"event":"ready","data":{"version":"1"}}'
            while true; do sleep 1; done
            """);
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        var client = new RapooBridgeClient();
        try
        {
            var start = client.StartAsync();
            await Task.Delay(50);
            var dispose = client.DisposeAsync().AsTask();
            try
            {
                await Task.WhenAll(start, dispose).WaitAsync(TimeSpan.FromSeconds(9));
            }
            catch (Exception ex) when (ex is ObjectDisposedException or OperationCanceledException)
            {
                // Disposal won the race; that is the expected outcome.
            }
            Assert.That(client.IsRunning, Is.False);
        }
        finally
        {
            await client.DisposeAsync();
            File.Delete(script);
        }
    }

    [Test]
    public async Task ConcurrentStopAndDispose_DoNotDeadlock()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var script = CreateScript(
            """
            #!/bin/sh
            printf '%s\n' '{"event":"ready","data":{"version":"1"}}'
            while true; do sleep 1; done
            """);
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        var client = new RapooBridgeClient();
        try
        {
            await client.StartAsync();
            var stop = client.StopAsync();
            var dispose = client.DisposeAsync().AsTask();
            await Task.WhenAll(stop, dispose).WaitAsync(TimeSpan.FromSeconds(9));
        }
        finally
        {
            await client.DisposeAsync();
            File.Delete(script);
        }
    }


    [Test]
    public async Task StateCallback_CanReadIsRunningWithoutDeadlock()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var script = CreateScript(
            """
            #!/bin/sh
            printf '%s\n' '{"event":"ready","data":{"version":"1"}}'
            printf '%s\n' '{"event":"state","data":{"connected":true,"demoMode":false,"deviceName":"Fake","hasBattery":false,"percentage":-1,"mouseOnline":true}}'
            while IFS= read -r line; do
              printf '%s\n' '{"id":1,"result":{"connected":true,"demoMode":false,"deviceName":"Fake","maxRateHz":4000}}'
            done
            """);
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        await using var client = new RapooBridgeClient();
        try
        {
            var observed = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
            client.StateChanged += (_, _) => observed.TrySetResult(client.IsRunning);
            await client.StartAsync();
            Assert.That(await observed.Task.WaitAsync(TimeSpan.FromSeconds(3)), Is.True);
        }
        finally
        {
            File.Delete(script);
        }
    }


    [Test]
    public async Task StateCallback_CanCallGetStateAndRestart()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX script.");
        var script = CreateScript(
            """
            #!/usr/bin/env python3
            import json, sys
            print(json.dumps({"event": "ready", "data": {"version": "1"}}), flush=True)
            print(json.dumps({"event": "state", "data": {"connected": True, "demoMode": False,
                "deviceName": "Fake", "hasBattery": False, "percentage": -1,
                "mouseOnline": True}}), flush=True)
            for line in sys.stdin:
                request = json.loads(line)
                print(json.dumps({"id": request["id"], "result": {"connected": True,
                    "demoMode": False, "deviceName": "Fake", "maxRateHz": 4000}}), flush=True)
            """);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(script,
                UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        var client = new RapooBridgeClient();
        try
        {
            var restarted = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
            client.StateChanged += (_, _) => _ = Task.Run(async () =>
            {
                try
                {
                    await client.GetStateAsync().WaitAsync(TimeSpan.FromSeconds(2));
                    await client.StopAsync().WaitAsync(TimeSpan.FromSeconds(5));
                    await client.StartAsync().WaitAsync(TimeSpan.FromSeconds(5));
                    restarted.TrySetResult(client.IsRunning);
                }
                catch (Exception ex)
                {
                    restarted.TrySetException(ex);
                }
            });
            await client.StartAsync();
            Assert.That(await restarted.Task.WaitAsync(TimeSpan.FromSeconds(8)), Is.True);
        }
        finally
        {
            await client.DisposeAsync();
            File.Delete(script);
        }
    }

    private static string CreateScript(string contents)
    {
        var path = Path.Combine(Path.GetTempPath(), $"rapoo-bridge-test-{Guid.NewGuid():N}.sh");
        File.WriteAllText(path, contents);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(path,
                UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }
        return path;
    }
}
