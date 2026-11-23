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
