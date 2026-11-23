#if WINDOWS
using System.Drawing;
using System.Runtime.InteropServices;
using H.NotifyIcon;
using Microsoft.UI;
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
    private TrayIconSpec? _lastSpec;

    public bool IsTrayReady => _tray?.IsCreated == true;

    public void Initialize(Window window, MainViewModel viewModel)
    {
        _window = window;
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

        _tray = new TaskbarIcon
        {
            ToolTipText = "Rapoo VT9 Pro 4K",
            ContextFlyout = menu,
            ContextMenuMode = ContextMenuMode.PopupMenu,
            NoLeftClickDelay = true,
            LeftClickCommand = new RelayCommand(ShowWindow),
        };
        UpdateTray(force: true);
        try
        {
            _tray.ForceCreate();
        }
        catch (Exception ex)
        {
            // Explorer may reject registration during startup. Keep the app
            // usable: the window is shown and close-to-tray stays disabled.
            System.Diagnostics.Debug.WriteLine($"tray create failed: {ex}");
            try
            {
                _tray.Dispose();
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
                SetForegroundWindow(handle);
            }
        }
        catch
        {
        }
    }

    public void Shutdown()
    {
        _quitting = true;
        if (_viewModel is not null)
        {
            _viewModel.PropertyChanged -= OnViewModelChanged;
        }
        DesktopPreferences.Current.Changed -= OnPreferencesChanged;
        if (_window is not null)
        {
            _window.AppWindow.Closing -= OnClosing;
        }
        try
        {
            _tray?.Dispose();
        }
        catch
        {
        }
        _tray = null;
        _lastSpec = null;
    }

    private void OnViewModelChanged(object? sender, System.ComponentModel.PropertyChangedEventArgs e) =>
        UpdateTray();

    private void OnPreferencesChanged(object? sender, EventArgs e) => UpdateTray(force: true);

    private void OnClosing(AppWindow sender, AppWindowClosingEventArgs args)
    {
        if (_quitting || !DesktopPreferences.Current.CloseToTray || !IsTrayReady)
        {
            return;
        }
        args.Cancel = true;
        sender.Hide();
    }

    private void Quit()
    {
        _quitting = true;
        try
        {
            _tray?.Dispose();
        }
        catch
        {
        }
        _tray = null;
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
        if (force || spec != _lastSpec)
        {
            _lastSpec = spec;
            _tray.Icon = TrayIconRenderer.Render(spec);
        }

        var tooltip = _viewModel.TrayTooltip;
        _tray.ToolTipText = tooltip.Length > 63 ? tooltip[..60] + "..." : tooltip;

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

    [DllImport("user32.dll")]
    private static extern bool SetForegroundWindow(IntPtr hWnd);
}
#endif
