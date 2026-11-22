using System.Collections.ObjectModel;
using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class ProfilesPage : Page
{
    private bool _loaded;
    private MainViewModel ViewModel => App.Instance.ViewModel;
    public PageFeedback Feedback { get; } = new();
    public ObservableCollection<BridgePreset> Presets { get; } = [];

    public ProfilesPage()
    {
        InitializeComponent();
        Feedback.PropertyChanged += (_, _) => UpdateControls();
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        ViewModel.PropertyChanged += DeviceStatusChanged;
        UpdateControls();
        if (ViewModel.DeviceService is not { } service)
        {
            return;
        }
        await Feedback.RunAsync(async () =>
        {
            await ViewModel.InitializeAsync();
            var presets = await service.ListPresetsAsync();
            Presets.Clear();
            foreach (var preset in presets)
            {
                if (!preset.BuiltIn)
                {
                    Presets.Add(preset);
                }
            }
            _loaded = true;
        });
    }

    private void Page_Unloaded(object sender, RoutedEventArgs e) => ViewModel.PropertyChanged -= DeviceStatusChanged;

    private void DeviceStatusChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainViewModel.CanConfigureDevice))
        {
            UpdateControls();
        }
    }

    private void PresetSelectionChanged(object sender, SelectionChangedEventArgs e) => UpdateControls();

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

    private async void Apply_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is { } service && ProfilesList.SelectedItem is BridgePreset preset &&
            ViewModel.CanConfigureDevice)
        {
            await Feedback.RunAsync(async () =>
            {
                ViewModel.UpdateDpi(await service.ApplyPresetAsync(preset.Id));
            }, "プロファイルを適用しました。");
        }
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
        ApplyButton.IsEnabled = SaveButton.IsEnabled && selected;
        DeleteButton.IsEnabled = idle && selected;
        EmptyNotice.Visibility = _loaded && Presets.Count == 0 ? Visibility.Visible : Visibility.Collapsed;
        DeviceNotice.Text = ViewModel.SettingsNotice;
        DeviceNotice.Visibility = DeviceNotice.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
    }
}
