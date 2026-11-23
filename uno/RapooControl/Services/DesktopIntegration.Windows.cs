#if WINDOWS
using System.Drawing;
using H.NotifyIcon;
using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.ViewModels;

namespace RapooControl.Services;

/// <summary>Windows notification-area, close-to-tray, and window integration.</summary>
public sealed partial class DesktopIntegration
{
    public static DesktopIntegration Current { get; } = new();

    private Window? _window;
    private MainViewModel? _viewModel;
    private TaskbarIcon? _tray;
    private MenuFlyoutItem? _batteryItem;
    private ToggleMenuFlyoutItem? _normalItem;
    private ToggleMenuFlyoutItem? _lowItem;
    private ToggleMenuFlyoutItem? _passiveItem;
    private ToggleMenuFlyoutItem? _demoItem;
    private ToggleMenuFlyoutItem? _autoStartItem;
    private bool _quitting;

    public bool IsTrayReady => _tray?.IsCreated == true;

    public void Initialize(Window window, MainViewModel viewModel)
    {
        _window = window;
        _viewModel = viewModel;

        var menu = new MenuFlyout();
        _batteryItem = new MenuFlyoutItem { IsEnabled = false, Text = "Battery" };
        menu.Items.Add(_batteryItem);
        menu.Items.Add(new MenuFlyoutSeparator());

        var open = new MenuFlyoutItem { Text = "Open Rapoo Control", Command = new RelayCommand(ShowWindow) };
        var reload = new MenuFlyoutItem { Text = "Reload", Command = new RelayCommand(() => _ = viewModel.RefreshAsync()) };
        menu.Items.Add(open);
        menu.Items.Add(reload);
        menu.Items.Add(new MenuFlyoutSeparator());

        _normalItem = new ToggleMenuFlyoutItem { Text = "Normal polling", Command = new RelayCommand(() => _ = SetModeAsync("normal")) };
        _lowItem = new ToggleMenuFlyoutItem { Text = "Low frequency", Command = new RelayCommand(() => _ = SetModeAsync("low_freq")) };
        _passiveItem = new ToggleMenuFlyoutItem { Text = "0-TX (passive)", Command = new RelayCommand(() => _ = SetModeAsync("passive_only")) };
        _demoItem = new ToggleMenuFlyoutItem { Text = "Demo mode", Command = new RelayCommand(() => _ = ToggleDemoAsync()) };
        menu.Items.Add(_normalItem);
        menu.Items.Add(_lowItem);
        menu.Items.Add(_passiveItem);
        menu.Items.Add(_demoItem);
        menu.Items.Add(new MenuFlyoutSeparator());

        _autoStartItem = new ToggleMenuFlyoutItem
        {
            Text = "Start with Windows",
            Command = new RelayCommand(() =>
            {
                var next = !DesktopPreferences.Current.AutoStart;
                DesktopPreferences.Current.SetAutoStart(next);
                UpdateTray();
            }),
        };
        menu.Items.Add(_autoStartItem);
        menu.Items.Add(new MenuFlyoutItem { Text = "Quit", Command = new RelayCommand(Quit) });

        _tray = new TaskbarIcon
        {
            ToolTipText = "Rapoo VT9 Pro 4K",
            ContextFlyout = menu,
            ContextMenuMode = ContextMenuMode.PopupMenu,
            NoLeftClickDelay = true,
            LeftClickCommand = new RelayCommand(ShowWindow),
        };
        UpdateTray();
        _tray.ForceCreate();
        viewModel.PropertyChanged += (_, _) => UpdateTray();
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
            _window.AppWindow.Show();
        }
        catch
        {
        }
        _window.Activate();
    }

    public void Shutdown()
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
    }

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

    private void UpdateTray()
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
        _tray.Icon = TrayIconRenderer.Render(spec);
        var tooltip = _viewModel.TrayTooltip;
        _tray.ToolTipText = tooltip.Length > 63 ? tooltip[..60] + "..." : tooltip;

        var loc = LocalizationService.Instance;
        if (_batteryItem is not null)
        {
            _batteryItem.Text = $"{loc.T("バッテリー")}: {_viewModel.BatteryText}";
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
            }
        }
        UpdateTray();
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
            }
        }
        UpdateTray();
    }

    private static async Task PromoteAsync()
    {
        await Task.Delay(TimeSpan.FromSeconds(2));
        _ = TrayVisibility.Promote(Environment.ProcessPath);
    }
}
#endif
