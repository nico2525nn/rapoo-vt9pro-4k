#if WINDOWS
using System.Drawing;
using System.Runtime.InteropServices;
using H.NotifyIcon;
using Microsoft.UI;
using Microsoft.UI.Dispatching;
using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.ViewModels;
using WinRT.Interop;

namespace RapooControl.Services;

/// <summary>Windows notification-area, close-to-tray, and window integration.</summary>
public sealed partial class DesktopIntegration
{
    public static DesktopIntegration Current { get; } = new();

    private Window? _window;
    private DispatcherQueue? _dispatcher;
    private MainViewModel? _viewModel;
    private TaskbarIcon? _tray;
    private MenuFlyoutItem? _batteryItem;
    private MenuFlyoutItem? _openItem;
    private MenuFlyoutItem? _reloadItem;
    private MenuFlyoutItem? _quitItem;
    private ToggleMenuFlyoutItem? _normalItem;
    private ToggleMenuFlyoutItem? _lowItem;
    private ToggleMenuFlyoutItem? _passiveItem;
    private ToggleMenuFlyoutItem? _demoItem;
    private ToggleMenuFlyoutItem? _autoStartItem;
    private bool _quitting;
    private bool _shutdown;
    private bool _trayHintShown;
    private TrayIconSpec? _lastSpec;

    public bool IsTrayReady => _tray?.IsCreated == true;

    public void Initialize(Window window, MainViewModel viewModel)
    {
        _window = window;
        _dispatcher = window.DispatcherQueue;
        _viewModel = viewModel;

        var loc = LocalizationService.Instance;
        var menu = new MenuFlyout();
        _batteryItem = new MenuFlyoutItem { IsEnabled = false, Text = "Battery" };
        menu.Items.Add(_batteryItem);
        menu.Items.Add(new MenuFlyoutSeparator());

        _openItem = new MenuFlyoutItem
        {
            Text = loc.T("Rapoo Control を開く"),
            Command = new RelayCommand(ShowWindow),
        };
        _reloadItem = new MenuFlyoutItem
        {
            Text = loc.T("再読み込み"),
            Command = new RelayCommand(() => _ = _viewModel.RefreshAsync()),
        };
        menu.Items.Add(_openItem);
        menu.Items.Add(_reloadItem);
        menu.Items.Add(new MenuFlyoutSeparator());

        _normalItem = new ToggleMenuFlyoutItem
        {
            Text = loc.T("通常モード"),
            Command = new RelayCommand(() => _ = SetModeAsync("normal")),
        };
        _lowItem = new ToggleMenuFlyoutItem
        {
            Text = loc.T("低頻度モード"),
            Command = new RelayCommand(() => _ = SetModeAsync("low_freq")),
        };
        _passiveItem = new ToggleMenuFlyoutItem
        {
            Text = loc.T("0-TX (完全受動)"),
            Command = new RelayCommand(() => _ = SetModeAsync("passive_only")),
        };
        _demoItem = new ToggleMenuFlyoutItem
        {
            Text = loc.T("デモモード"),
            Command = new RelayCommand(() => _ = ToggleDemoAsync()),
        };
        menu.Items.Add(_normalItem);
        menu.Items.Add(_lowItem);
        menu.Items.Add(_passiveItem);
        menu.Items.Add(_demoItem);
        menu.Items.Add(new MenuFlyoutSeparator());

        _autoStartItem = new ToggleMenuFlyoutItem
        {
            Text = loc.T("Windows スタートアップ"),
            Command = new RelayCommand(() =>
            {
                var next = !DesktopPreferences.Current.AutoStart;
                DesktopPreferences.Current.SetAutoStart(next);
                UpdateTray(force: true);
            }),
        };
        _quitItem = new MenuFlyoutItem
        {
            Text = loc.T("終了"),
            Command = new RelayCommand(Quit),
        };
        menu.Items.Add(_autoStartItem);
        menu.Items.Add(_quitItem);

        try
        {
            _tray = new TaskbarIcon
            {
                ToolTipText = "Rapoo VT9 Pro 4K",
                ContextFlyout = menu,
                ContextMenuMode = ContextMenuMode.PopupMenu,
                NoLeftClickDelay = true,
                LeftClickCommand = new RelayCommand(ShowWindow),
            };
            UpdateTray(force: true);
            _tray.ForceCreate(enablesEfficiencyMode: false);
        }
        catch (Exception ex)
        {
            // Explorer may reject registration during startup. Keep the app
            // usable: the window is shown and close-to-tray stays disabled.
            System.Diagnostics.Debug.WriteLine($"tray create failed: {ex}");
            try
            {
                _tray?.Dispose();
            }
            catch
            {
            }
            _tray = null;
        }

        viewModel.PropertyChanged += OnViewModelChanged;
        DesktopPreferences.Current.Changed += OnPreferencesChanged;
        window.AppWindow.Closing += OnClosing;
        _ = PromoteAsync();
    }

    public void HideWindow()
    {
        try
        {
            _window?.AppWindow.Hide();
        }
        catch
        {
        }
    }

    public void ShowWindow()
    {
        if (_window is null)
        {
            return;
        }
        try
        {
            var presenter = _window.AppWindow.Presenter as OverlappedPresenter;
            if (presenter?.State == OverlappedPresenterState.Minimized)
            {
                presenter.Restore();
            }
            _window.AppWindow.Show();
        }
        catch
        {
        }

        try
        {
            _window.Activate();
            var handle = WindowNative.GetWindowHandle(_window);
            if (handle != IntPtr.Zero)
            {
                ForceForeground(handle);
            }
        }
        catch
        {
        }
    }

    public void Shutdown()
    {
        if (_shutdown)
        {
            return;
        }
        _shutdown = true;
        _quitting = true;
        if (_viewModel is not null)
        {
            _viewModel.PropertyChanged -= OnViewModelChanged;
        }
        DesktopPreferences.Current.Changed -= OnPreferencesChanged;

        var onUiThread = _dispatcher?.HasThreadAccess == true;
        if (onUiThread && _window is not null)
        {
            try
            {
                _window.AppWindow.Closing -= OnClosing;
            }
            catch (Exception ex)
            {
                System.Diagnostics.Debug.WriteLine($"detach closing failed: {ex.Message}");
            }
        }

        var tray = _tray;
        _tray = null;
        _lastSpec = null;
        if (tray is not null)
        {
            if (onUiThread)
            {
                try
                {
                    tray.Dispose();
                }
                catch (Exception ex)
                {
                    System.Diagnostics.Debug.WriteLine($"tray dispose failed: {ex.Message}");
                }
            }
            // On the ProcessExit thread the OS reclaims the icon; touching the
            // WinRT window/dispatcher there can throw RPC_E_WRONG_THREAD.
        }
    }

    private void OnViewModelChanged(object? sender, System.ComponentModel.PropertyChangedEventArgs e) =>
        RunOnUiThread(() => UpdateTray());

    private void OnPreferencesChanged(object? sender, EventArgs e) =>
        RunOnUiThread(() => UpdateTray(force: true));

    private void RunOnUiThread(Action action)
    {
        if (_dispatcher is null || _dispatcher.HasThreadAccess)
        {
            action();
            return;
        }
        _dispatcher.TryEnqueue(() => action());
    }

    private void OnClosing(AppWindow sender, AppWindowClosingEventArgs args)
    {
        if (_quitting || !DesktopPreferences.Current.CloseToTray || !IsTrayReady)
        {
            // Normal close: release the tray while the UI dispatcher is alive.
            Shutdown();
            return;
        }
        args.Cancel = true;
        try
        {
            sender.Hide();
        }
        catch (Exception ex)
        {
            // Hiding failed (Explorer restart, lost registration). Let the
            // close proceed so the user is not left with an invisible window.
            System.Diagnostics.Debug.WriteLine($"tray hide failed: {ex.Message}");
            args.Cancel = false;
            Shutdown();
            return;
        }
        if (_trayHintShown)
        {
            return;
        }

        _trayHintShown = true;
        try
        {
            _tray?.ShowNotification(
                "Rapoo Control",
                LocalizationService.Instance.T("通知領域で実行中です。トレイアイコンをクリックすると再表示します。"),
                H.NotifyIcon.Core.NotificationIcon.Info);
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"tray notification failed: {ex.Message}");
        }
    }

    private void Quit()
    {
        _quitting = true;
        var tray = _tray;
        _tray = null;
        _lastSpec = null;
        try
        {
            tray?.Dispose();
        }
        catch
        {
        }
        _window?.Close();
    }

    private void UpdateTray(bool force = false)
    {
        if (_tray is null || _viewModel is null)
        {
            return;
        }

        var spec = new TrayIconSpec(
            Connected: _viewModel.IsDeviceConnected,
            HasBattery: _viewModel.HasBattery,
            Percentage: _viewModel.BatteryPercent,
            Charging: _viewModel.IsCharging,
            Sleeping: _viewModel.IsSleeping);
        try
        {
            if (force || spec != _lastSpec)
            {
                _tray.Icon = TrayIconRenderer.Render(spec);
                _lastSpec = spec;
            }
            var tooltip = _viewModel.TrayTooltip;
            _tray.ToolTipText = tooltip.Length > 63 ? tooltip[..60] + "..." : tooltip;
        }
        catch (Exception ex)
        {
            // A dispatched callback must never take the UI down; the next
            // successful update retries because _lastSpec was not advanced.
            System.Diagnostics.Debug.WriteLine($"tray update failed: {ex.Message}");
            return;
        }

        var loc = LocalizationService.Instance;
        if (_batteryItem is not null)
        {
            _batteryItem.Text = $"{loc.T("バッテリー")}: {_viewModel.BatteryText}";
        }
        if (_openItem is not null)
        {
            _openItem.Text = loc.T("Rapoo Control を開く");
        }
        if (_reloadItem is not null)
        {
            _reloadItem.Text = loc.T("再読み込み");
        }
        if (_normalItem is not null)
        {
            _normalItem.Text = loc.T("通常モード");
            _normalItem.IsChecked = _viewModel.Mode == "normal";
        }
        if (_lowItem is not null)
        {
            _lowItem.Text = loc.T("低頻度モード");
            _lowItem.IsChecked = _viewModel.Mode == "low_freq";
        }
        if (_passiveItem is not null)
        {
            _passiveItem.Text = loc.T("0-TX (完全受動)");
            _passiveItem.IsChecked = _viewModel.Mode == "passive_only";
        }
        if (_demoItem is not null)
        {
            _demoItem.Text = loc.T("デモモード");
            _demoItem.IsChecked = _viewModel.IsDemoMode;
        }
        if (_autoStartItem is not null)
        {
            _autoStartItem.Text = loc.T("Windows スタートアップ");
            _autoStartItem.IsChecked = DesktopPreferences.Current.AutoStart;
        }
        if (_quitItem is not null)
        {
            _quitItem.Text = loc.T("終了");
        }
    }

    private async Task SetModeAsync(string mode)
    {
        if (_viewModel?.DeviceService is { } service)
        {
            try
            {
                await service.SetPollingModeAsync(mode);
            }
            catch (Exception ex)
            {
                System.Diagnostics.Debug.WriteLine($"tray mode: {ex}");
                _viewModel.SetError(ex.Message);
            }
        }
        UpdateTray(force: true);
    }

    private async Task ToggleDemoAsync()
    {
        if (_viewModel?.DeviceService is { } service)
        {
            try
            {
                await service.SetDemoModeAsync(!_viewModel.IsDemoMode);
            }
            catch (Exception ex)
            {
                System.Diagnostics.Debug.WriteLine($"tray demo: {ex}");
                _viewModel.SetError(ex.Message);
            }
        }
        UpdateTray(force: true);
    }

    private static async Task PromoteAsync()
    {
        await Task.Delay(TimeSpan.FromSeconds(2));
        _ = TrayVisibility.Promote(Environment.ProcessPath);
    }

    private static void ForceForeground(IntPtr hwnd)
    {
        var foreground = GetForegroundWindow();
        var foregroundThread = GetWindowThreadProcessId(foreground, out _);
        var currentThread = GetCurrentThreadId();
        var attached = foregroundThread != 0 &&
            foregroundThread != currentThread &&
            AttachThreadInput(currentThread, foregroundThread, true);
        try
        {
            BringWindowToTop(hwnd);
            SetForegroundWindow(hwnd);
        }
        finally
        {
            if (attached)
            {
                AttachThreadInput(currentThread, foregroundThread, false);
            }
        }
    }

    [DllImport("user32.dll")]
    private static extern bool SetForegroundWindow(IntPtr hWnd);

    [DllImport("user32.dll")]
    private static extern IntPtr GetForegroundWindow();

    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);

    [DllImport("kernel32.dll")]
    private static extern uint GetCurrentThreadId();

    [DllImport("user32.dll")]
    private static extern bool AttachThreadInput(uint idAttach, uint idAttachTo, bool attach);

    [DllImport("user32.dll")]
    private static extern bool BringWindowToTop(IntPtr hWnd);
}
#endif
