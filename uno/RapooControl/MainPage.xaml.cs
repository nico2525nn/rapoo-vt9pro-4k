using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using RapooControl.Services;
using RapooControl.ViewModels;
using RapooControl.Views;

namespace RapooControl;

public sealed partial class MainPage : Page
{
    private IConfigPageActions? _activeActions;
    private bool _runningAction;
    private bool CanRecoverBridge => ViewModel.HasError && ViewModel.DeviceService is { IsRunning: false };

    public MainViewModel ViewModel => App.Instance.ViewModel;

    public UIElement TitleBarDragRegion => WindowDragArea;

    public MainPage()
    {
        InitializeComponent();
        ViewModel.PropertyChanged += (_, args) =>
        {
            if (args.PropertyName == nameof(MainViewModel.HasError))
            {
                UpdateActionButtons();
            }
        };
        Loaded += (_, _) =>
        {
            LocalizationService.Instance.Apply(this);
            if (NavView.SelectedItem is null)
            {
                NavView.SelectedItem = NavView.MenuItems[0];
            }
            NavigateTo("overview");
            UpdateActionButtons();
        };
    }

    private void NavView_SelectionChanged(NavigationView sender, NavigationViewSelectionChangedEventArgs args)
    {
        if (args.SelectedItem is NavigationViewItem { Tag: string tag })
        {
            NavigateTo(tag);
        }
    }

    private void NavigateTo(string tag)
    {
        var type = tag switch
        {
            "dpi" => typeof(DpiPage),
            "sensor" => typeof(SensorPage),
            "buttons" => typeof(ButtonsPage),
            "profiles" => typeof(ProfilesPage),
            "settings" => typeof(SettingsPage),
            _ => typeof(OverviewPage),
        };
        if (ContentFrame.CurrentSourcePageType != type)
        {
            ContentFrame.Navigate(type);
        }
    }

    private void ContentFrame_Navigated(object sender, NavigationEventArgs e)
    {
        if (_activeActions is not null)
        {
            _activeActions.ActionsChanged -= PageActionsChanged;
        }
        if (e.Content is FrameworkElement page)
        {
            LocalizationService.Instance.Apply(page);
            // Uno realizes the page's visual tree after the Navigated event.
            DispatcherQueue.TryEnqueue(() => LocalizationService.Instance.Apply(page));
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
        ReloadButton.IsEnabled = !_runningAction && (CanRecoverBridge || _activeActions?.CanReload == true);
        ApplyButton.IsEnabled = !_runningAction && _activeActions?.CanApply == true;
    }

    private async void Reload_Click(object sender, RoutedEventArgs e)
    {
        if (CanRecoverBridge)
        {
            await RunActionAsync(async () =>
            {
                await ViewModel.RefreshAsync();
                if (_activeActions is { CanReload: true } recoveredPage)
                {
                    await recoveredPage.ReloadAsync();
                }
            });
        }
        else if (_activeActions is { CanReload: true } page)
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
        catch (Exception ex)
        {
            ViewModel.SetError(ex.Message);
        }
        finally
        {
            _runningAction = false;
            UpdateActionButtons();
        }
    }
}
