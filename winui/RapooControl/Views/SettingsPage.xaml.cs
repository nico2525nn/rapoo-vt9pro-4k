using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;
using Windows.Storage.Pickers;
using WinRT.Interop;

namespace RapooControl.Views;

public sealed partial class SettingsPage : Page, IConfigPageActions
{
    private bool _languageReady;
    private bool _trayPrefsReady;
    private BridgeSnapshot? _lastSnapshot;

    private MainViewModel ViewModel => App.Instance.ViewModel;
    public PageFeedback Feedback { get; } = new();

    public bool CanReload => !Feedback.IsBusy && ViewModel.DeviceService is not null && ViewModel.CanConfigureDevice;
    public bool CanApply => false;
    public event EventHandler? ActionsChanged;

    public SettingsPage()
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
        LanguageCombo.SelectedIndex = LocalizationService.Instance.IsJapanese ? 1 : 0;
        LanguageCombo.DropDownClosed += LanguageCombo_DropDownClosed;
        _languageReady = true;
        TrayPreferences.Current.Load();
        CloseToTraySwitch.IsOn = TrayPreferences.Current.CloseToTray;
        AutoStartSwitch.IsOn = TrayPreferences.Current.AutoStart;
        _trayPrefsReady = true;
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        LocalizationService.Instance.Apply(this);
        LanguageCombo.SelectedIndex = LocalizationService.Instance.IsJapanese ? 1 : 0;
        TrayPreferences.Current.Load();
        CloseToTraySwitch.IsOn = TrayPreferences.Current.CloseToTray;
        AutoStartSwitch.IsOn = TrayPreferences.Current.AutoStart;
        ViewModel.PropertyChanged += DeviceStatusChanged;
        UpdateControls();
        if (ViewModel.DeviceService is not { } service)
        {
            return;
        }
        await Feedback.RunAsync(async () =>
        {
            await ViewModel.InitializeAsync();
            if (service.CachedSnapshot is { } snapshot)
            {
                LoadFirmware(snapshot);
            }
            else if (ViewModel.CanConfigureDevice)
            {
                LoadFirmware(await service.RefreshSnapshotAsync());
            }
            else
            {
                MouseFirmware.Text = DongleFirmware.Text = "—";
            }
        });
    }

    private void Page_Unloaded(object sender, RoutedEventArgs e) => ViewModel.PropertyChanged -= DeviceStatusChanged;

    private void CloseToTray_Toggled(object sender, RoutedEventArgs e)
    {
        if (!_trayPrefsReady)
        {
            return;
        }
        TrayPreferences.Current.SetCloseToTray(CloseToTraySwitch.IsOn);
    }

    private void AutoStart_Toggled(object sender, RoutedEventArgs e)
    {
        if (!_trayPrefsReady)
        {
            return;
        }
        TrayPreferences.Current.SetAutoStart(AutoStartSwitch.IsOn);
    }

    private void LanguageCombo_SelectionChanged(object sender, SelectionChangedEventArgs e) =>
        ApplyLanguageSelection();

    private void LanguageCombo_DropDownClosed(object? sender, object e) =>
        ApplyLanguageSelection();

    private async void ApplyLanguageSelection()
    {
        if (!_languageReady || LanguageCombo.SelectedItem is not ComboBoxItem { Tag: string language })
        {
            return;
        }
        try
        {
            var logDir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "rapoo-vt9pro-4k");
            Directory.CreateDirectory(logDir);
            File.AppendAllText(
                Path.Combine(logDir, "winui-language.log"),
                $"{DateTimeOffset.Now:O} selected={language}{Environment.NewLine}");
        }
        catch
        {
            // Diagnostics only.
        }

        var localization = LocalizationService.Instance;
        if (localization.Language == language)
        {
            return;
        }
        localization.SetLanguage(language);
        if (ViewModel.DeviceService is { } service)
        {
            try
            {
                await service.SetLanguageAsync(language);
            }
            catch
            {
                // The UI language still applies; the sidecar will pick it up next launch.
            }
        }
        localization.Apply(App.Instance.MainWindow?.Content);
        ViewModel.RefreshLocalization();
        UpdateControls();
        if (_lastSnapshot is { } snapshot)
        {
            LoadFirmware(snapshot);
        }
        Feedback.Message = language == "ja" ? "言語を日本語に変更しました。" : "Language changed to English.";
    }

    private void DeviceStatusChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainViewModel.CanConfigureDevice))
        {
            UpdateControls();
        }
    }

    private void LoadFirmware(BridgeSnapshot snapshot)
    {
        _lastSnapshot = snapshot;
        MouseFirmware.Text = FirmwareText(snapshot.Firmware?.MouseString);
        DongleFirmware.Text = FirmwareText(snapshot.Firmware?.DongleString);
    }

    private static string FirmwareText(string? value) => string.IsNullOrWhiteSpace(value) || value == "unknown" ? "—" : value;

    public Task ReloadAsync() => CanReload && ViewModel.DeviceService is { } service
        ? Feedback.RunAsync(async () => LoadFirmware(await service.RefreshSnapshotAsync()), PageFeedback.Reloaded)
        : Task.CompletedTask;

    public Task ApplyAsync() => Task.CompletedTask;

    private async void PollingMode_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is { } service)
        {
            var restore = ViewModel.IsPassive;
            await Feedback.RunAsync(
                () => service.SetPollingModeAsync(restore ? "normal" : "passive_only"),
                restore ? "通常モードに戻しました。" : "0-TX モードに切り替えました。");
        }
    }

    private async void DemoMode_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is { } service)
        {
            var enable = !ViewModel.IsDemoMode;
            await Feedback.RunAsync(async () =>
            {
                MouseFirmware.Text = DongleFirmware.Text = "—";
                var snapshot = await service.SetDemoModeAsync(enable);
                LoadFirmware(snapshot);
            }, enable ? "デモモードを開始しました。" : "デモモードを終了しました。");
        }
    }

    private async void OpenUpdater_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is { } service)
        {
            await Feedback.RunAsync(() => service.OpenFirmwareUpdaterAsync(), "公式ソフトを起動しました。");
        }
    }

    private async void BrowseUpdater_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is not { } service || App.Instance.MainWindow is not { } window)
        {
            return;
        }

        try
        {
            var picker = new FileOpenPicker { FileTypeFilter = { ".exe" } };
            InitializeWithWindow.Initialize(picker, WindowNative.GetWindowHandle(window));
            var file = await picker.PickSingleFileAsync();
            if (file is null)
            {
                return;
            }

            if (!Path.IsPathFullyQualified(file.Path) ||
                !(string.Equals(file.Name, "RapooGameDev.exe", StringComparison.OrdinalIgnoreCase) ||
                  string.Equals(file.Name, "FirmwareExe.exe", StringComparison.OrdinalIgnoreCase)))
            {
                Feedback.Message = "RapooGameDev.exe または FirmwareExe.exe を選択してください。";
                return;
            }

            await Feedback.RunAsync(() => service.OpenFirmwareUpdaterAsync(file.Path), "公式ソフトを起動しました。");
        }
        catch (Exception ex)
        {
            Feedback.Message = ex.Message;
        }
    }

    private async void FactoryReset_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is not { } service || !ViewModel.CanConfigureDevice)
        {
            return;
        }

        var confirmation = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = "マウスを初期化しますか？",
            Content = "DPI、センサー、ボタンなど、マウス本体の設定が工場出荷時の状態に戻ります。",
            PrimaryButtonText = "初期化",
            CloseButtonText = "キャンセル",
            DefaultButton = ContentDialogButton.Close,
        };
        LocalizationService.Instance.ApplyDialog(confirmation);
        if (await confirmation.ShowAsync() != ContentDialogResult.Primary || !ViewModel.CanConfigureDevice)
        {
            return;
        }

        await Feedback.RunAsync(async () =>
        {
            var snapshot = await service.FactoryResetAsync();
            LoadFirmware(snapshot);
        }, "工場出荷時設定に戻しました。");
    }

    private void UpdateControls()
    {
        var idle = ViewModel.DeviceService is not null && !Feedback.IsBusy;
        ModeButton.IsEnabled = idle;
        ModeButton.Content = LocalizationService.Instance.T(ViewModel.IsPassive ? "通常に戻す" : "0-TX に切り替え");
        DemoButton.IsEnabled = idle;
        DemoButton.Content = LocalizationService.Instance.T(ViewModel.IsDemoMode ? "終了" : "開始");
        UpdaterButton.IsEnabled = BrowseButton.IsEnabled = idle;
        ResetButton.IsEnabled = idle && ViewModel.CanConfigureDevice;
        DeviceNotice.Text = ViewModel.DeviceService is null ? ViewModel.SettingsNotice : ViewModel.ConnectionMessage;
        DeviceNotice.Visibility = DeviceNotice.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
        ActionsChanged?.Invoke(this, EventArgs.Empty);
    }
}
