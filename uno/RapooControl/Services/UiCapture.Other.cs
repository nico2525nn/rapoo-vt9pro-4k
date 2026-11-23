#if !WINDOWS
using Microsoft.UI.Xaml;

namespace RapooControl.Services;

internal static class UiCapture
{
    public static Task CaptureAsync(FrameworkElement element) => Task.CompletedTask;
}
#endif
