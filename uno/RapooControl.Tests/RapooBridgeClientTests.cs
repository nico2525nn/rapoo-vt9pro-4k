using System.Diagnostics;
using System.Text.Json;
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

    [Test]
    public async Task DpiConfig_SendsStageArrayAndReadsSnapshot()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX shell.");
        var capture = Path.Combine(Path.GetTempPath(), $"rapoo-bridge-request-{Guid.NewGuid():N}.json");
        var script = CreateScript(
            """
            #!/bin/sh
            printf '%s\n' '{"event":"ready","data":{"version":"1"}}'
            while IFS= read -r line; do
              printf '%s\n' "$line" > '__CAPTURE__'
              printf '%s\n' '{"id":1,"result":{"model":{"name":"Fake"},"dpi":{"currentStage":1,"stageCount":1,"independentXY":false,"stages":[{"index":1,"x":450,"y":400}]}}}'
            done
            """.Replace("__CAPTURE__", capture));
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        await using var client = new RapooBridgeClient();
        try
        {
            await client.StartAsync();
            var snapshot = await client.SetDpiConfigAsync(
                1,
                1,
                new[] { new BridgeDpiStage { Index = 1, X = 450, Y = 400 } },
                cancellationToken: default,
                sessionId: client.CurrentSessionId);

            Assert.That(snapshot.Dpi?.Stages, Has.Count.EqualTo(1));
            Assert.That(snapshot.Dpi!.Stages![0].X, Is.EqualTo(450));

            using var request = JsonDocument.Parse(File.ReadAllText(capture));
            var root = request.RootElement;
            Assert.That(root.GetProperty("method").GetString(), Is.EqualTo("dpiConfig"));
            var stages = root.GetProperty("params").GetProperty("config").GetProperty("stages");
            Assert.That(stages.ValueKind, Is.EqualTo(JsonValueKind.Array));
            Assert.That(stages[0].GetProperty("x").GetInt32(), Is.EqualTo(450));
            Assert.That(stages[0].GetProperty("y").GetInt32(), Is.EqualTo(400));
        }
        finally
        {
            File.Delete(script);
            File.Delete(capture);
        }
    }

    [Test]
    public async Task Requests_SerializeAllParamsWithoutReflection()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses Python.");
        var capture = Path.Combine(Path.GetTempPath(), $"rapoo-bridge-requests-{Guid.NewGuid():N}.jsonl");
        var script = CreateScript(
            """
            #!/usr/bin/env python3
            import json, sys
            print(json.dumps({"event": "ready", "data": {"version": "1"}}), flush=True)
            for line in sys.stdin:
                request = json.loads(line)
                with open('__CAPTURE__', 'a') as log:
                    log.write(line)
                result = "launched" if request["method"] == "openUpdater" else {}
                print(json.dumps({"id": request["id"], "result": result}), flush=True)
            """.Replace("__CAPTURE__", capture));
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(script,
                UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);

        await using var client = new RapooBridgeClient();
        try
        {
            await client.StartAsync();
            Assert.That(await client.OpenFirmwareUpdaterAsync(), Is.EqualTo("launched"));
            _ = await client.SetLanguageAsync("ja");
            _ = await client.ApplySensorAsync(
                new Dictionary<string, object>
                {
                    ["motionSync"] = false,
                    ["sleepMinutes"] = 10,
                },
                cancellationToken: default,
                sessionId: client.CurrentSessionId);
            _ = await client.SetButtonAsync(
                5,
                new BridgeButtonAction { Type = "left_click", LabelJa = "左クリック" },
                cancellationToken: default,
                sessionId: client.CurrentSessionId);

            var requests = File.ReadAllLines(capture)
                .Select(line => JsonDocument.Parse(line))
                .ToArray();
            try
            {
                Assert.That(requests, Has.Length.EqualTo(4));
                Assert.That(requests[0].RootElement.GetProperty("method").GetString(), Is.EqualTo("openUpdater"));
                Assert.That(requests[0].RootElement.GetProperty("params").GetProperty("path").GetString(), Is.Empty);
                Assert.That(requests[1].RootElement.GetProperty("params").GetProperty("language").GetString(), Is.EqualTo("ja"));

                var patch = requests[2].RootElement.GetProperty("params").GetProperty("patch");
                Assert.That(patch.GetProperty("motionSync").GetBoolean(), Is.False);
                Assert.That(patch.GetProperty("sleepMinutes").GetInt32(), Is.EqualTo(10));

                var button = requests[3].RootElement.GetProperty("params");
                Assert.That(button.GetProperty("buttonId").GetInt32(), Is.EqualTo(5));
                Assert.That(button.GetProperty("action").GetProperty("type").GetString(), Is.EqualTo("left_click"));
            }
            finally
            {
                foreach (var request in requests)
                {
                    request.Dispose();
                }
            }
        }
        finally
        {
            File.Delete(script);
            File.Delete(capture);
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
