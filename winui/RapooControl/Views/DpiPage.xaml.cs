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

    private async void ApplyButton_Click(object sender, RoutedEventArgs e)
    {
        await ViewModel.Dpi.ApplyAsync();
    }
}
