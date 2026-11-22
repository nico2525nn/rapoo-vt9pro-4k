using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.ViewModels;
using RapooControl.Views;

namespace RapooControl;

public sealed partial class MainWindow : Window
{
    public MainViewModel ViewModel { get; }

    public MainWindow()
    {
        ViewModel = App.Instance.ViewModel;
        InitializeComponent();

        ExtendsContentIntoTitleBar = true;
        SetTitleBar(WindowDragArea);

        AppWindow.Resize(new Windows.Graphics.SizeInt32(1180, 800));

        NavView.SelectedItem = NavView.MenuItems[0];
        NavigateTo("overview");
    }

    private void NavView_SelectionChanged(NavigationView sender, NavigationViewSelectionChangedEventArgs args)
    {
        if (args.SelectedItemContainer?.Tag is string tag)
        {
            NavigateTo(tag);
        }
    }

    private void NavigateTo(string tag)
    {
        var pageType = tag switch
        {
            "overview" => typeof(OverviewPage),
            "dpi" => typeof(DpiPage),
            "sensor" => typeof(SensorPage),
            "buttons" => typeof(ButtonsPage),
            "settings" => typeof(SettingsPage),
            _ => null,
        };

        if (pageType is null || ContentFrame.CurrentSourcePageType == pageType)
        {
            return;
        }

        ContentFrame.Navigate(pageType);
    }
}
