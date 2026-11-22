using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class SettingsPage : Page
{
    public MainViewModel ViewModel { get; }

    public SettingsPage()
    {
        ViewModel = App.Instance.ViewModel;
        InitializeComponent();
    }

    private async void RefreshButton_Click(object sender, RoutedEventArgs e)
    {
        RefreshStatusText.Text = "再読み込み中…";

        try
        {
            await ViewModel.InitializeAsync();
            RefreshStatusText.Text = "再読み込みしました。";
        }
        catch (Exception ex)
        {
            RefreshStatusText.Text = $"再読み込みに失敗しました: {ex.Message}";
        }
    }
}
