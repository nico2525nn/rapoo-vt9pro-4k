#if LINUX
using Microsoft.UI.Xaml;
using RapooControl.ViewModels;

namespace RapooControl.Services;

/// <summary>Skia desktop head. A system tray is not available yet.</summary>
public sealed partial class DesktopIntegration
{
    public static DesktopIntegration Current { get; } = new();

    private Window? _window;

    public bool IsTrayReady => false;

    public void Initialize(Window window, MainViewModel viewModel)
    {
        _window = window;
    }

    public void HideWindow()
    {
    }

    public void ShowWindow()
    {
        _window?.Activate();
    }

    public void Shutdown()
    {
    }
}
#endif
