#if WINDOWS
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
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

            if (!await ConfirmUpdaterPublisherAsync(file.Path))
            {
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

    /// <summary>
    /// A signed updater must carry a Rapoo certificate. An unsigned file is
    /// shown as an explicit confirmation instead of being silently accepted;
    /// the official tool has historically shipped without Authenticode.
    /// </summary>
    private async Task<bool> ConfirmUpdaterPublisherAsync(string path)
    {
        try
        {
#pragma warning disable SYSLIB0057 // No X509CertificateLoader replacement exists for a signed PE file.
            using var certificate = X509Certificate.CreateFromSignedFile(path);
#pragma warning restore SYSLIB0057
            if (certificate.Subject.Contains("Rapoo", StringComparison.OrdinalIgnoreCase))
            {
                return true;
            }
            Feedback.Message = LocalizationService.Instance.T(
                "この実行ファイルは Rapoo の署名がありません。公式ソフトを選択してください。");
            return false;
        }
        catch (CryptographicException)
        {
            var confirmation = new ContentDialog
            {
                XamlRoot = XamlRoot,
                Title = "デジタル署名がありません",
                Content = "この実行ファイルにはデジタル署名がありません。Rapoo 公式サイトから取得したファイルであることを確認してください。",
                PrimaryButtonText = "起動する",
                CloseButtonText = "キャンセル",
                DefaultButton = ContentDialogButton.Close,
            };
            LocalizationService.Instance.ApplyDialog(confirmation);
            return await confirmation.ShowAsync() == ContentDialogResult.Primary;
        }
    }
}
#endif
