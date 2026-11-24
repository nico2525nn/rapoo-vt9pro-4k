using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class SettingsPage : Page, IConfigPageActions
{
    private bool _languageReady;
    private bool _trayPrefsReady;
    private BridgeSnapshot? _lastSnapshot;
    private int _firmwareRevision = -1;

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
        DesktopPreferences.Current.Load();
        ApplyPreferencesToControls();
        DesktopPreferences.Current.Changed += OnPreferencesChanged;
    }

    private void ApplyPreferencesToControls()
    {
        _trayPrefsReady = false;
        CloseToTraySwitch.IsOn = DesktopPreferences.Current.CloseToTray;
        AutoStartSwitch.IsOn = DesktopPreferences.Current.AutoStart;
        StartHiddenSwitch.IsOn = DesktopPreferences.Current.StartHidden;
        _trayPrefsReady = true;
    }

    private void OnPreferencesChanged(object? sender, EventArgs e) => ApplyPreferencesToControls();

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        LocalizationService.Instance.Apply(this);
        // Firmware support is a Windows capability, independent of the tray.
        FirmwareUpdateRow.Visibility = OperatingSystem.IsWindows()
            ? Visibility.Visible
            : Visibility.Collapsed;
        CloseToTrayRow.Visibility = AutoStartService.SupportsTray
            ? Visibility.Visible
            : Visibility.Collapsed;
        StartHiddenRow.Visibility = AutoStartService.SupportsStartHidden
            ? Visibility.Visible
            : Visibility.Collapsed;
        LanguageCombo.SelectedIndex = LocalizationService.Instance.IsJapanese ? 1 : 0;
        DesktopPreferences.Current.Load();
        ApplyPreferencesToControls();
        ViewModel.PropertyChanged += DeviceStatusChanged;
        UpdateControls();
        if (ViewModel.DeviceService is not { } service)
        {
            return;
        }
        if (OperatingSystem.IsLinux())
        {
            _ = LoadUdevAsync(service);
        }
        await Feedback.RunAsync(async () =>
        {
            await ViewModel.InitializeAsync();
            var revision = ViewModel.DeviceRevision;
            if (service.CachedSnapshot is { } snapshot)
            {
                LoadFirmware(snapshot, revision);
            }
            else if (ViewModel.CanConfigureDevice)
            {
                var refreshed = await service.RefreshSnapshotAsync();
                if (revision != ViewModel.DeviceRevision)
                {
                    ClearFirmware();
                    return;
                }
                LoadFirmware(refreshed, revision);
            }
            else
            {
                ClearFirmware();
            }
        });
    }

    private void Page_Unloaded(object sender, RoutedEventArgs e) => ViewModel.PropertyChanged -= DeviceStatusChanged;

    private async Task LoadUdevAsync(RealRapooService service)
    {
        try
        {
            var catalog = await service.GetCatalogAsync();
            if (!string.IsNullOrWhiteSpace(catalog.LinuxUdevInstallCmd))
            {
                UdevCommandBox.Text = catalog.LinuxUdevInstallCmd;
                UdevCard.Visibility = Visibility.Visible;
            }
        }
        catch
        {
            // The udev card stays hidden when the catalog is unavailable.
        }
    }

    private void CopyUdev_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            var package = new Windows.ApplicationModel.DataTransfer.DataPackage();
            package.SetText(UdevCommandBox.Text);
            Windows.ApplicationModel.DataTransfer.Clipboard.SetContent(package);
            UdevFeedback.Text = LocalizationService.Instance.T("udev 設定コマンドをコピーしました。");
            UdevFeedback.Visibility = Visibility.Visible;
        }
        catch
        {
            UdevFeedback.Text = UdevCommandBox.Text;
            UdevFeedback.Visibility = Visibility.Visible;
        }
    }

    private void CloseToTray_Toggled(object sender, RoutedEventArgs e)
    {
        if (!_trayPrefsReady)
        {
            return;
        }
        if (!DesktopPreferences.Current.SetCloseToTray(CloseToTraySwitch.IsOn))
        {
            ApplyPreferencesToControls();
            Feedback.Message = LocalizationService.Instance.T("トレイ設定を保存できませんでした。");
        }
    }

    private void AutoStart_Toggled(object sender, RoutedEventArgs e)
    {
        if (!_trayPrefsReady)
        {
            return;
        }
        if (!DesktopPreferences.Current.SetAutoStart(AutoStartSwitch.IsOn))
        {
            ApplyPreferencesToControls();
            Feedback.Message = LocalizationService.Instance.T("スタートアップ設定を保存できませんでした。");
        }
    }

    private void StartHidden_Toggled(object sender, RoutedEventArgs e)
    {
        if (!_trayPrefsReady)
        {
            return;
        }
        if (!DesktopPreferences.Current.SetStartHidden(StartHiddenSwitch.IsOn))
        {
            ApplyPreferencesToControls();
            Feedback.Message = LocalizationService.Instance.T("スタートアップ設定を保存できませんでした。");
        }
    }

    private void LanguageCombo_SelectionChanged(object sender, SelectionChangedEventArgs e) =>
        ApplyLanguageSelection();

    private void LanguageCombo_DropDownClosed(object? sender, object e) =>
        ApplyLanguageSelection();

    private void ApplyLanguageSelection()
    {
        if (!_languageReady || LanguageCombo.SelectedItem is not ComboBoxItem { Tag: string language })
        {
            return;
        }
        var localization = LocalizationService.Instance;
        if (localization.Language == language)
        {
            return;
        }
        var saved = localization.SetLanguage(language);
        UdevFeedback.Visibility = Visibility.Collapsed;
        localization.Apply(App.Instance.MainWindow?.Content);
        ViewModel.RefreshLocalization();
        UpdateControls();
        if (_lastSnapshot is { } snapshot && _firmwareRevision == ViewModel.DeviceRevision)
        {
            LoadFirmware(snapshot, _firmwareRevision);
        }
        else
        {
            ClearFirmware();
        }
        Feedback.Message = saved
            ? language == "ja" ? "言語を日本語に変更しました。" : "Language changed to English."
            : localization.T("言語設定を保存できませんでした。");
        if (ViewModel.DeviceService is { } service)
        {
            _ = SyncSidecarLanguageAsync(service, language);
        }
    }

    private static async Task SyncSidecarLanguageAsync(RealRapooService service, string language)
    {
        try
        {
            await service.SetLanguageAsync(language).WaitAsync(TimeSpan.FromSeconds(2));
        }
        catch
        {
            // The UI language still applies; the sidecar will pick it up next launch.
        }
    }

    private void DeviceStatusChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainViewModel.DeviceRevision) &&
            _firmwareRevision != ViewModel.DeviceRevision)
        {
            ClearFirmware();
        }
        if (e.PropertyName is nameof(MainViewModel.CanConfigureDevice) or nameof(MainViewModel.DeviceRevision))
        {
            UpdateControls();
        }
    }

    private void LoadFirmware(BridgeSnapshot snapshot, int revision)
    {
        _lastSnapshot = snapshot;
        _firmwareRevision = revision;
        MouseFirmware.Text = FirmwareText(snapshot.Firmware?.MouseString);
        DongleFirmware.Text = FirmwareText(snapshot.Firmware?.DongleString);
    }

    private void ClearFirmware()
    {
        _lastSnapshot = null;
        _firmwareRevision = -1;
        MouseFirmware.Text = DongleFirmware.Text = "—";
    }

    private static string FirmwareText(string? value) => string.IsNullOrWhiteSpace(value) || value == "unknown" ? "—" : value;

    public Task ReloadAsync() => CanReload && ViewModel.DeviceService is { } service
        ? Feedback.RunAsync(async () =>
        {
            var revision = ViewModel.DeviceRevision;
            var snapshot = await service.RefreshSnapshotAsync();
            if (revision != ViewModel.DeviceRevision)
            {
                ClearFirmware();
                return;
            }
            LoadFirmware(snapshot, revision);
        }, PageFeedback.Reloaded)
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
                LoadFirmware(snapshot, ViewModel.DeviceRevision);
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
        var generation = ViewModel.DeviceGeneration;
        if (await confirmation.ShowAsync() != ContentDialogResult.Primary ||
            !ViewModel.CanConfigureDevice)
        {
            return;
        }

        await Feedback.RunAsync(async () =>
        {
            var snapshot = await service.FactoryResetAsync(generation);
            LoadFirmware(snapshot, ViewModel.DeviceRevision);
        }, "工場出荷時設定に戻しました。");
    }

    private void UpdateControls()
    {
        var idle = ViewModel.DeviceService is not null && !Feedback.IsBusy;
        var loc = LocalizationService.Instance;
        var startupLabel = loc.T(OperatingSystem.IsWindows() ? "Windows スタートアップ" : "サインイン時に起動");
        AutoStartLabel.Text = startupLabel;
        Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(AutoStartSwitch, startupLabel);
        ModeButton.IsEnabled = idle;
        ModeButton.Content = loc.T(ViewModel.IsPassive ? "通常に戻す" : "0-TX に切り替え");
        DemoButton.IsEnabled = idle;
        DemoButton.Content = LocalizationService.Instance.T(ViewModel.IsDemoMode ? "終了" : "開始");
        UpdaterButton.IsEnabled = BrowseButton.IsEnabled = idle;
        ResetButton.IsEnabled = idle && ViewModel.CanConfigureDevice;
        DeviceNotice.Text = ViewModel.DeviceService is null ? ViewModel.SettingsNotice : ViewModel.ConnectionMessage;
        DeviceNotice.Visibility = DeviceNotice.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
        ActionsChanged?.Invoke(this, EventArgs.Empty);
    }
}
