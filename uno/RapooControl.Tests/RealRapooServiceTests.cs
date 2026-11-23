using RapooControl.Services;

namespace RapooControl.Tests;

[TestFixture]
public sealed class RealRapooServiceTests
{
    [Test]
    public async Task OlderStateReply_DoesNotOverrideANewerEvent()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX script.");
        var script = Path.Combine(Path.GetTempPath(), $"rapoo-state-test-{Guid.NewGuid():N}.py");
        File.WriteAllText(script,
            """
            #!/usr/bin/env python3
            import json, sys

            print(json.dumps({"event": "ready", "data": {"version": "1"}}), flush=True)
            for line in sys.stdin:
                request = json.loads(line)
                request_id = request["id"]
                print(json.dumps({
                    "event": "state",
                    "data": {
                        "connected": False,
                        "demoMode": False,
                        "deviceName": "Fake",
                        "hasBattery": False,
                        "percentage": -1,
                        "mouseOnline": False,
                        "sleeping": False,
                        "updatedAt": "2026-01-01T00:00:00Z"
                    }
                }), flush=True)
                print(json.dumps({
                    "id": request_id,
                    "result": {
                        "connected": True,
                        "demoMode": False,
                        "deviceName": "Fake",
                        "hasBattery": False,
                        "percentage": -1,
                        "mouseOnline": True,
                        "sleeping": False,
                        "updatedAt": "2026-01-01T00:00:00Z"
                    }
                }), flush=True)
            """);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(script,
                UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute);
        }

        var previous = Environment.GetEnvironmentVariable("RAPOO_BRIDGE_PATH");
        Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", script);
        var service = new RealRapooService();
        try
        {
            var notifications = new List<bool>();
            service.StatusChanged += (_, status) => notifications.Add(status.Connected);
            await service.StartAsync();
            var status = await service.GetStatusAsync().WaitAsync(TimeSpan.FromSeconds(5));
            Assert.Multiple(() =>
            {
                Assert.That(status.Connected, Is.False);
                var firstDisconnect = notifications.IndexOf(false);
                Assert.That(firstDisconnect, Is.GreaterThanOrEqualTo(0));
                Assert.That(notifications.Skip(firstDisconnect).Any(connected => connected), Is.False);
            });
        }
        finally
        {
            await service.DisposeAsync();
            Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", previous);
            File.Delete(script);
        }
    }
}
