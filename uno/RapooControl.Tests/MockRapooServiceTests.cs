using RapooControl.Models;
using RapooControl.Services;

namespace RapooControl.Tests;

[TestFixture]
public sealed class MockRapooServiceTests
{
    [Test]
    public async Task StartAndStatus_ExposeUsableDefaults()
    {
        var service = new MockRapooService();

        await service.StartAsync();
        var status = await service.GetStatusAsync();

        Assert.Multiple(() =>
        {
            Assert.That(service.IsMock, Is.True);
            Assert.That(status.Connected, Is.False);
            Assert.That(status.Connection, Is.EqualTo(ConnectionKind.Wireless4K));
            Assert.That(status.BatteryPercent, Is.EqualTo(78));
            Assert.That(status.PollingRateHz, Is.EqualTo(4000));
        });
    }

    [Test]
    public async Task ApplyDpiConfiguration_NormalizesAndPublishesStatus()
    {
        var service = new MockRapooService();
        var changed = 0;
        service.StatusChanged += (_, _) => changed++;

        var configuration = new DpiConfiguration(
            StageCount: 2,
            CurrentStage: 2,
            Stages:
            [
                new DpiStage(1, 1234, 1234),
                new DpiStage(2, 26050, 26050),
            ]);

        var status = await service.ApplyDpiConfigurationAsync(configuration);
        var stored = await service.GetDpiConfigurationAsync();

        Assert.Multiple(() =>
        {
            Assert.That(changed, Is.EqualTo(1));
            Assert.That(status.ActiveDpi, Is.EqualTo(26000));
            Assert.That(stored.StageCount, Is.EqualTo(2));
            Assert.That(stored.CurrentStage, Is.EqualTo(2));
            Assert.That(stored.Stages[0].X, Is.EqualTo(1250));
            Assert.That(stored.Stages[1].X, Is.EqualTo(26000));
        });
    }

    [Test]
    public void SetPollingRate_RejectsUnsupportedValues()
    {
        var service = new MockRapooService();

        Assert.ThrowsAsync<ArgumentOutOfRangeException>(
            () => service.SetPollingRateAsync(1234));
    }
}
