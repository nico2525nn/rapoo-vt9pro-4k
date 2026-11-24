using System.Collections.ObjectModel;
using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class ProfilesPage : Page, IConfigPageActions
{
    private bool _loaded;
    private MainViewModel ViewModel => App.Instance.ViewModel;
    public PageFeedback Feedback { get; } = new();
    public ObservableCollection<BridgePreset> Presets { get; } = [];

    public bool CanReload => !Feedback.IsBusy && ViewModel.DeviceService is not null;
    public bool CanApply => CanReload && _loaded && ViewModel.CanConfigureDevice && ProfilesList.SelectedItem is BridgePreset;
    public event EventHandler? ActionsChanged;

    public ProfilesPage()
    {
        InitializeComponent();
        LocalizationService.Instance.Apply(this);
        Feedback.PropertyChanged += (_, e) =>
        {
            if (e.PropertyName == nameof(PageFeedback.IsBusy))
            {
                UpdateControls();
            }
        };
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        ViewModel.PropertyChanged += DeviceStatusChanged;
        UpdateControls();
        if (_loaded || ViewModel.DeviceService is not { } service)
        {
            return;
        }
        await Feedback.RunAsync(() => ReadProfilesAsync(service));
    }

    public Task ReloadAsync() => CanReload && ViewModel.DeviceService is { } service
        ? Feedback.RunAsync(() => ReadProfilesAsync(service), PageFeedback.Reloaded)
        : Task.CompletedTask;

    private async Task ReadProfilesAsync(RealRapooService service)
    {
        await ViewModel.InitializeAsync();
        var selectedId = (ProfilesList.SelectedItem as BridgePreset)?.Id;
        var presets = await service.ListPresetsAsync();
        Presets.Clear();
        foreach (var preset in presets)
        {
            if (!preset.BuiltIn)
            {
                Presets.Add(preset);
            }
        }
        ProfilesList.SelectedItem = Presets.FirstOrDefault(preset => preset.Id == selectedId);
        _loaded = true;
    }

    private void Page_Unloaded(object sender, RoutedEventArgs e) => ViewModel.PropertyChanged -= DeviceStatusChanged;

    private void DeviceStatusChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainViewModel.CanConfigureDevice))
        {
            UpdateControls();
        }
    }

    private void PresetSelectionChanged(object sender, SelectionChangedEventArgs e)
    {
        if (!Feedback.IsBusy)
        {
            Feedback.Message = string.Empty;
        }
        UpdateControls();
    }

    private async void Save_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is not { } service || !ViewModel.CanConfigureDevice)
        {
            return;
        }

        var name = new TextBox { Header = "名前" };
        var description = new TextBox { Header = "説明（任意）" };
        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = "プロファイルを保存",
            Content = new StackPanel { Spacing = 8, Children = { name, description } },
            PrimaryButtonText = "保存",
            IsPrimaryButtonEnabled = false,
            CloseButtonText = "キャンセル",
        };
        name.TextChanged += (_, _) => dialog.IsPrimaryButtonEnabled = !string.IsNullOrWhiteSpace(name.Text);
        LocalizationService.Instance.ApplyDialog(dialog);
        if (await dialog.ShowAsync() != ContentDialogResult.Primary || !ViewModel.CanConfigureDevice)
        {
            return;
        }

        await Feedback.RunAsync(async () =>
        {
            var preset = await service.SavePresetAsync(name.Text.Trim(), description.Text.Trim());
            Presets.Insert(0, preset);
            ProfilesList.SelectedItem = preset;
        }, "プロファイルを保存しました。");
    }

    public Task ApplyAsync()
    {
        if (!CanApply || ViewModel.DeviceService is not { } service ||
            ProfilesList.SelectedItem is not BridgePreset preset)
        {
            return Task.CompletedTask;
        }

        var generation = ViewModel.DeviceGeneration;
        return Feedback.RunAsync(async () =>
        {
            await service.ApplyPresetAsync(preset.Id, generation);
            ProfilesList.SelectedItem = null;
        }, PageFeedback.Applied);
    }

    private async void Delete_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is not { } service || ProfilesList.SelectedItem is not BridgePreset preset)
        {
            return;
        }

        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = "プロファイルを削除しますか？",
            Content = preset.Name,
            PrimaryButtonText = "削除",
            CloseButtonText = "キャンセル",
            DefaultButton = ContentDialogButton.Close,
        };
        LocalizationService.Instance.ApplyDialog(dialog);
        if (await dialog.ShowAsync() != ContentDialogResult.Primary)
        {
            return;
        }
        await Feedback.RunAsync(async () =>
        {
            await service.DeletePresetAsync(preset.Id);
            Presets.Remove(preset);
        }, "プロファイルを削除しました。");
    }

    private void UpdateControls()
    {
        var idle = _loaded && !Feedback.IsBusy;
        var selected = ProfilesList.SelectedItem is BridgePreset;
        SaveButton.IsEnabled = idle && ViewModel.CanConfigureDevice;
        DeleteButton.IsEnabled = idle && selected;
        ProfilesList.IsEnabled = idle;
        EmptyNotice.Visibility = _loaded && Presets.Count == 0 ? Visibility.Visible : Visibility.Collapsed;
        DeviceNotice.Text = ViewModel.SettingsNotice;
        DeviceNotice.Visibility = DeviceNotice.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
        ActionsChanged?.Invoke(this, EventArgs.Empty);
    }
}
