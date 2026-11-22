using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using RapooControl.Views;

namespace RapooControl;

public sealed partial class MainWindow : Window
{
    private IConfigPageActions? _activeActions;
    private bool _runningAction;

    public MainWindow()
    {
        InitializeComponent();

        ExtendsContentIntoTitleBar = true;
        SetTitleBar(WindowDragArea);

        AppWindow.Resize(new Windows.Graphics.SizeInt32(1180, 800));

        NavView.SelectedItem = NavView.MenuItems[0];
        NavigateTo("overview");
    }

    private void ContentFrame_Navigated(object sender, NavigationEventArgs e)
    {
        if (_activeActions is not null)
        {
            _activeActions.ActionsChanged -= PageActionsChanged;
        }

        _activeActions = ContentFrame.Content as IConfigPageActions;
        if (_activeActions is not null)
        {
            _activeActions.ActionsChanged += PageActionsChanged;
        }
        UpdateActionButtons();
    }

    private void PageActionsChanged(object? sender, EventArgs e) => UpdateActionButtons();

    private void UpdateActionButtons()
    {
        ReloadButton.IsEnabled = !_runningAction && _activeActions?.CanReload == true;
        ApplyButton.IsEnabled = !_runningAction && _activeActions?.CanApply == true;
    }

    private async void Reload_Click(object sender, RoutedEventArgs e)
    {
        if (_activeActions is { CanReload: true } page)
        {
            await RunActionAsync(page.ReloadAsync);
        }
    }

    private async void Apply_Click(object sender, RoutedEventArgs e)
    {
        if (_activeActions is { CanApply: true } page)
        {
            await RunActionAsync(page.ApplyAsync);
        }
    }

    private async Task RunActionAsync(Func<Task> action)
    {
        if (_runningAction)
        {
            return;
        }

        _runningAction = true;
        UpdateActionButtons();
        try
        {
            await action();
        }
        finally
        {
            _runningAction = false;
            UpdateActionButtons();
        }
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
            "profiles" => typeof(ProfilesPage),
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
