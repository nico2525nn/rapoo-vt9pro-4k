using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Tests;

[TestFixture]
public sealed class DpiPageViewModelTests
{
    [Test]
    public async Task LoadAsync_PopulatesStagesAndClearsDirtyState()
    {
        var service = new MockRapooService();
        var viewModel = new DpiPageViewModel(service);

        await viewModel.LoadAsync();

        Assert.Multiple(() =>
        {
            Assert.That(viewModel.HasConfiguration, Is.True);
            Assert.That(viewModel.StageCount, Is.EqualTo(4));
            Assert.That(viewModel.CurrentStage, Is.EqualTo(2));
            Assert.That(viewModel.IsDirty, Is.False);
            Assert.That(viewModel.Stages[0].IsVisible, Is.True);
            Assert.That(viewModel.Stages[3].IsVisible, Is.True);
            Assert.That(viewModel.Stages[4].IsVisible, Is.False);
        });
    }

    [Test]
    public async Task ChangingStage_MarksDirtyAndAppliesToService()
    {
        var service = new MockRapooService();
        var viewModel = new DpiPageViewModel(service);
        await viewModel.LoadAsync();

        viewModel.Stages[0].X = 900;
        Assert.That(viewModel.IsDirty, Is.True);

        await viewModel.ApplyAsync();
        var stored = await service.GetDpiConfigurationAsync();

        Assert.Multiple(() =>
        {
            Assert.That(viewModel.IsDirty, Is.False);
            Assert.That(stored.Stages[0].X, Is.EqualTo(900));
        });
    }

    [Test]
    public async Task DeviceMaximum_ClampsPollingRateOptions()
    {
        var service = new MockRapooService();
        var viewModel = new DpiPageViewModel(service);
        await viewModel.LoadAsync();

        viewModel.SetMaxRate(1000);

        Assert.Multiple(() =>
        {
            Assert.That(viewModel.PollingRateOptions, Does.Contain("1,000 Hz"));
            Assert.That(viewModel.PollingRateOptions, Does.Not.Contain("4,000 Hz"));
            Assert.That(viewModel.PollingRateHz, Is.EqualTo(1000));
        });
    }
}
