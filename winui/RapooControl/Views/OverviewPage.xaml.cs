using Microsoft.UI.Xaml.Controls;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class OverviewPage : Page
{
    public MainViewModel ViewModel { get; }

    public OverviewPage()
    {
        ViewModel = App.Instance.ViewModel;
        InitializeComponent();
    }
}
