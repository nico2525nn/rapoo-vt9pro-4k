using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class DpiPage : Page, IConfigPageActions
{
    public MainViewModel ViewModel { get; } = App.Instance.ViewModel;
    public PageFeedback Feedback { get; } = new();

    public bool CanReload => !Feedback.IsBusy && ViewModel.CanConfigureDevice;
    public bool CanApply => ViewModel.Dpi.CanEdit && ViewModel.Dpi.IsDirty;
    public event EventHandler? ActionsChanged;

    public DpiPage()
    {
        InitializeComponent();
        LocalizationService.Instance.Apply(this);
        Feedback.PropertyChanged += (_, e) =>
        {
            if (e.PropertyName == nameof(PageFeedback.IsBusy))
            {
                ViewModel.Dpi.IsBusy = Feedback.IsBusy;
                ActionsChanged?.Invoke(this, EventArgs.Empty);
            }
        };
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        LocalizationService.Instance.Apply(this);
        ViewModel.Dpi.RefreshLocalization();
        ViewModel.PropertyChanged += StateChanged;
        ViewModel.Dpi.PropertyChanged += StateChanged;
        if (ViewModel.Dpi.IsDirty)
        {
            ActionsChanged?.Invoke(this, EventArgs.Empty);
            return;
        }

        await Feedback.RunAsync(async () =>
        {
            await ViewModel.InitializeAsync();
            if (ViewModel.CanConfigureDevice || ViewModel.DeviceService?.CachedSnapshot?.Dpi is not null)
            {
                await ReadAsync(refresh: false);
            }
        });
    }

    private void Page_Unloaded(object sender, RoutedEventArgs e)
    {
        ViewModel.PropertyChanged -= StateChanged;
        ViewModel.Dpi.PropertyChanged -= StateChanged;
    }

    private void StateChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(DpiPageViewModel.IsDirty) && ViewModel.Dpi.IsDirty && !Feedback.IsBusy)
        {
            Feedback.Message = string.Empty;
        }

        if (e.PropertyName is nameof(DpiPageViewModel.IsDirty) or nameof(DpiPageViewModel.CanEdit) or nameof(MainViewModel.CanConfigureDevice))
        {
            ActionsChanged?.Invoke(this, EventArgs.Empty);
        }
    }

    public Task ReloadAsync() => CanReload
        ? Feedback.RunAsync(() => ReadAsync(refresh: true), PageFeedback.Reloaded)
        : Task.CompletedTask;

    public Task ApplyAsync() => CanApply
        ? Feedback.RunAsync(ViewModel.Dpi.ApplyAsync, PageFeedback.Applied)
        : Task.CompletedTask;

    private async Task ReadAsync(bool refresh)
    {
        if (ViewModel.DeviceService is not { } service)
        {
            await ViewModel.Dpi.LoadAsync();
            return;
        }

        var snapshot = (refresh ? null : service.CachedSnapshot) ?? await service.RefreshSnapshotAsync();
        ViewModel.Dpi.SetMaxRate(snapshot.Model?.MaxRateHz ?? 4000);
        var dpi = snapshot.Dpi ?? throw new InvalidOperationException("DPI設定を読み取れませんでした。");
        var rate = snapshot.Sensor?.PollingRateHz ?? (await service.GetStatusAsync()).PollingRateHz;
        ViewModel.Dpi.LoadConfiguration(RealRapooService.MapDpiConfiguration(dpi), rate);
    }
}
