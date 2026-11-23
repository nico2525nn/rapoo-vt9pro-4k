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
    [Test]
    public async Task DemoModeReply_IsAcceptedAcrossTheDemoDisplayNameChange()
    {
        Assume.That(!OperatingSystem.IsWindows(), "The fake sidecar uses a POSIX script.");
        var script = Path.Combine(Path.GetTempPath(), $"rapoo-demo-test-{Guid.NewGuid():N}.py");
        File.WriteAllText(script,
            """
            #!/usr/bin/env python3
            import json, sys

            def state(demo, name):
                return {
                    "connected": True, "demoMode": demo, "deviceName": name,
                    "productId": 4625, "connection": "wireless_4k",
                    "hasBattery": True, "percentage": 82, "mouseOnline": True,
                    "sleeping": False, "maxRateHz": 4000
                }

            def snapshot(demo):
                return {
                    "model": {"name": "Rapoo VT9 Pro 4K", "productId": 4625,
                              "connection": "wireless_4k", "maxRateHz": 4000},
                    "demoMode": demo,
                    "battery": {"percentage": 82, "charging": False, "online": True},
                    "sensor": {"pollingRateHz": 1000},
                    "dpi": {"currentStage": 1, "stageCount": 1,
                            "stages": [{"index": 1, "x": 800, "y": 800}],
                            "allStages": [{"index": 1, "x": 800, "y": 800}]}
                }

            print(json.dumps({"event": "ready", "data": {"version": "1"}}), flush=True)
            for line in sys.stdin:
                request = json.loads(line)
                request_id = request["id"]
                method = request["method"]
                if method == "state":
                    result = state(False, "Rapoo VT9 Pro 4K")
                elif method == "demo":
                    print(json.dumps({"event": "state",
                                      "data": state(True, "Rapoo VT9 Pro 4K (デモ)")}), flush=True)
                    result = snapshot(True)
                else:
                    result = snapshot(False)
                print(json.dumps({"id": request_id, "result": result}), flush=True)
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
            await service.StartAsync();
            var snapshot = await service.SetDemoModeAsync(true).WaitAsync(TimeSpan.FromSeconds(5));
            Assert.That(snapshot.DemoMode, Is.True);
        }
        finally
        {
            await service.DisposeAsync();
            Environment.SetEnvironmentVariable("RAPOO_BRIDGE_PATH", previous);
            File.Delete(script);
        }
    }

}
