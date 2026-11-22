using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class DpiPage : Page
{
    public MainViewModel ViewModel { get; }

    public DpiPage()
    {
        ViewModel = App.Instance.ViewModel;
        InitializeComponent();
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        try
        {
            await ViewModel.InitializeAsync();
            if (ViewModel.DeviceService is { } service && ViewModel.CanConfigureDevice && !ViewModel.Dpi.HasConfiguration)
            {
                ViewModel.UpdateDpi(await service.RefreshSnapshotAsync());
            }
        }
        catch (Exception ex)
        {
            ViewModel.Dpi.ReportLoadError(ex.Message);
        }
    }

    private async void ApplyButton_Click(object sender, RoutedEventArgs e)
    {
        await ViewModel.Dpi.ApplyAsync();
    }
}
