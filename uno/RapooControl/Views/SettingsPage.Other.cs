#if !WINDOWS
using Microsoft.UI.Xaml;

namespace RapooControl.Views;

public sealed partial class SettingsPage
{
    private void BrowseUpdater_Click(object sender, RoutedEventArgs e)
    {
        // Firmware updates are a Windows-only capability.
    }
}
#endif
