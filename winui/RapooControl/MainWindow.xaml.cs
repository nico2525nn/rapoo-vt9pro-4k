using System.Runtime.InteropServices;
using System.Windows.Input;
using Microsoft.UI;
using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Navigation;
using RapooControl.ViewModels;
using RapooControl.Views;
using RapooControl.Services;
using WinRT.Interop;

namespace RapooControl;

public sealed partial class MainWindow : Window
{
    [DllImport("user32.dll")]
    private static extern bool ShowWindow(IntPtr hWnd, int command);

    private const int SwShowNoActivate = 4;

    private IConfigPageActions? _activeActions;
    private bool _runningAction;
    private bool _quitting;
    private bool _trayReady;
    private bool _trayHintShown;

    private MainViewModel ViewModel => App.Instance.ViewModel;

    public ICommand ShowWindowCommand { get; }

    public MainWindow()
    {
        InitializeComponent();
        LocalizationService.Instance.Apply(Content as DependencyObject);
        if (Content is FrameworkElement root)
        {
            root.Loaded += (_, _) => LocalizationService.Instance.Apply(root);
        }

        TrayPreferences.Current.Load();
        ShowWindowCommand = new RelayCommand(ShowFromTray);
        TrayIcon.LeftClickCommand = ShowWindowCommand;
        TrayOpenItem.Command = ShowWindowCommand;
        TrayReloadItem.Command = new RelayCommand(() => _ = ReloadFromTrayAsync());
        TrayNormalItem.Command = new RelayCommand(() => _ = SetPollingModeAsync("normal"));
        TrayLowItem.Command = new RelayCommand(() => _ = SetPollingModeAsync("low_freq"));
        TrayPassiveItem.Command = new RelayCommand(() => _ = SetPollingModeAsync("passive_only"));
        TrayDemoItem.Command = new RelayCommand(() => _ = ToggleDemoAsync());
        TrayAutoStartItem.Command = new RelayCommand(() =>
        {
            TrayPreferences.Current.SetAutoStart(TrayAutoStartItem.IsChecked);
            TrayAutoStartItem.IsChecked = TrayPreferences.Current.AutoStart;
        });
        TrayQuitItem.Command = new RelayCommand(QuitFromTray);
        AppWindow.Closing += OnClosing;
        ViewModel.PropertyChanged += (_, _) => UpdateTray();
        try
        {
            TrayIcon.ForceCreate();
            _trayReady = TrayIcon.IsCreated;
            LogTray($"created={_trayReady}");
        }
        catch (Exception ex)
        {
            LogTray($"create failed: {ex}");
        }

        ExtendsContentIntoTitleBar = true;
        SetTitleBar(WindowDragArea);

        AppWindow.Resize(new Windows.Graphics.SizeInt32(1180, 800));

        NavView.SelectedItem = NavView.MenuItems[0];
        NavigateTo("overview");
        UpdateTray();
    }

    private void OnClosing(AppWindow sender, AppWindowClosingEventArgs args)
    {
        if (_quitting || !_trayReady || !TrayPreferences.Current.CloseToTray)
        {
            return;
        }

        args.Cancel = true;
        AppWindow.Hide();
        if (!_trayHintShown)
        {
            _trayHintShown = true;
            try
            {
                TrayIcon.ShowNotification(
                    "Rapoo Control",
                    LocalizationService.Instance.T("通知領域で実行中です。トレイアイコンをクリックすると再表示します。"),
                    H.NotifyIcon.Core.NotificationIcon.Info);
            }
            catch (Exception ex)
            {
                LogTray($"notification failed: {ex.Message}");
            }
        }
        LogTray("hidden to tray");
    }

    public void ShowFromTray()
    {
        if (Environment.GetEnvironmentVariable("RAPOO_HEADLESS_TEST") == "1")
        {
            // Headless verification only counts processes; never steal focus.
            return;
        }

        AppWindow.Show();
        Activate();
    }

    private void UpdateTray()
    {
        var percent = ViewModel.BatteryPercent;
        TrayIconSource.Text = percent >= 0 ? percent.ToString() : "--";
        TrayIconSource.Foreground = new SolidColorBrush(
            percent >= 0 && percent <= 20 && !ViewModel.IsCharging ? Colors.OrangeRed : Colors.White);
        var tooltip = ViewModel.TrayTooltip;
        TrayIcon.ToolTipText = tooltip.Length > 63 ? tooltip[..60] + "..." : tooltip;
    }

    private void TrayMenu_Opening(object sender, object e)
    {
        var loc = LocalizationService.Instance;
        TrayBatteryItem.Text = $"{loc.T("バッテリー")}: {ViewModel.BatteryText}";
        TrayOpenItem.Text = loc.T("開く");
        TrayReloadItem.Text = loc.T("再読み込み");
        TrayNormalItem.Text = loc.T("通常モード");
        TrayLowItem.Text = loc.T("低頻度モード");
        TrayPassiveItem.Text = loc.T("0-TX (完全受動)");
        TrayDemoItem.Text = loc.T("デモモード");
        TrayAutoStartItem.Text = loc.T("Windows スタートアップ");
        TrayQuitItem.Text = loc.T("終了");
        TrayNormalItem.IsChecked = ViewModel.Mode == "normal";
        TrayLowItem.IsChecked = ViewModel.Mode == "low_freq";
        TrayPassiveItem.IsChecked = ViewModel.Mode == "passive_only";
        TrayDemoItem.IsChecked = ViewModel.IsDemoMode;
        TrayAutoStartItem.IsChecked = TrayPreferences.Current.AutoStart;
        UpdateTray();
    }

    private async Task ReloadFromTrayAsync()
    {
        await ViewModel.RefreshAsync();
        UpdateTray();
    }

    private async Task SetPollingModeAsync(string mode)
    {
        if (ViewModel.DeviceService is { } service)
        {
            try
            {
                await service.SetPollingModeAsync(mode);
            }
            catch (Exception ex)
            {
                LogTray($"polling mode failed: {ex.Message}");
            }
        }
        TrayMenu_Opening(TrayMenu, EventArgs.Empty);
    }

    private async Task ToggleDemoAsync()
    {
        if (ViewModel.DeviceService is { } service)
        {
            try
            {
                await service.SetDemoModeAsync(!ViewModel.IsDemoMode);
            }
            catch (Exception ex)
            {
                LogTray($"demo mode failed: {ex.Message}");
            }
        }
        TrayMenu_Opening(TrayMenu, EventArgs.Empty);
    }

    private void QuitFromTray()
    {
        _quitting = true;
        try
        {
            TrayIcon.Dispose();
        }
        catch
        {
            // Best effort.
        }
        Close();
    }

    private static void LogTray(string message)
    {
        try
        {
            var dir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "rapoo-vt9pro-4k");
            Directory.CreateDirectory(dir);
            File.AppendAllText(
                Path.Combine(dir, "winui-tray.log"),
                $"{DateTimeOffset.Now:O} {message}{Environment.NewLine}");
        }
        catch
        {
            // Diagnostics must never break the app.
        }
    }

    private void ContentFrame_Navigated(object sender, NavigationEventArgs e)
    {
        if (_activeActions is not null)
        {
            _activeActions.ActionsChanged -= PageActionsChanged;
        }

        LocalizationService.Instance.Apply(e.Content as DependencyObject);
        _activeActions = ContentFrame.Content as IConfigPageActions;
        if (_activeActions is not null)
        {
            _activeActions.ActionsChanged += PageActionsChanged;
        }
        UpdateActionButtons();
    }

    private void PageActionsChanged(object? sender, EventArgs e) => UpdateActionButtons();

    private void UpdateActionButtons()
    {
        ReloadButton.IsEnabled = !_runningAction && _activeActions?.CanReload == true;
        ApplyButton.IsEnabled = !_runningAction && _activeActions?.CanApply == true;
    }

    private async void Reload_Click(object sender, RoutedEventArgs e)
    {
        if (_activeActions is { CanReload: true } page)
        {
            await RunActionAsync(page.ReloadAsync);
        }
    }

    private async void Apply_Click(object sender, RoutedEventArgs e)
    {
        if (_activeActions is { CanApply: true } page)
        {
            await RunActionAsync(page.ApplyAsync);
        }
    }

    private async Task RunActionAsync(Func<Task> action)
    {
        if (_runningAction)
        {
            return;
        }

        _runningAction = true;
        UpdateActionButtons();
        try
        {
            await action();
        }
        finally
        {
            _runningAction = false;
            UpdateActionButtons();
        }
    }

    private void NavView_SelectionChanged(NavigationView sender, NavigationViewSelectionChangedEventArgs args)
    {
        if (args.SelectedItemContainer?.Tag is string tag)
        {
            NavigateTo(tag);
        }
    }

    private void NavigateTo(string tag)
    {
        var pageType = tag switch
        {
            "overview" => typeof(OverviewPage),
            "dpi" => typeof(DpiPage),
            "sensor" => typeof(SensorPage),
            "buttons" => typeof(ButtonsPage),
            "profiles" => typeof(ProfilesPage),
            "settings" => typeof(SettingsPage),
            _ => null,
        };

        if (pageType is null || ContentFrame.CurrentSourcePageType == pageType)
        {
            return;
        }

        ContentFrame.Navigate(pageType);
    }
}
