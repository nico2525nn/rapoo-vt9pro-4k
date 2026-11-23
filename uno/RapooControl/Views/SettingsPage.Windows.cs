#if WINDOWS
using Microsoft.UI.Xaml;
using RapooControl.Services;
using Windows.Storage.Pickers;
using WinRT.Interop;

namespace RapooControl.Views;

public sealed partial class SettingsPage
{
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
                Feedback.Message = LocalizationService.Instance.T("RapooGameDev.exe または FirmwareExe.exe を選択してください。");
                return;
            }

            await Feedback.RunAsync(
                () => service.OpenFirmwareUpdaterAsync(file.Path),
                "公式ソフトを起動しました。");
        }
        catch (Exception ex)
        {
            Feedback.Message = ex.Message;
        }
    }
}
#endif
