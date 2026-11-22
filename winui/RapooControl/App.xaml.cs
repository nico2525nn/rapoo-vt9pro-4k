using Microsoft.UI.Xaml;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl;

public partial class App : Application
{
    public static App Instance => (App)Current;

    public IRapooService RapooService { get; }

    public MainViewModel ViewModel { get; }

    public Window? MainWindow { get; private set; }

    public App()
    {
        InitializeComponent();

        // The prototype deliberately uses a mock service. Replace this single
        // construction with the Go bridge implementation when it becomes available.
        RapooService = new MockRapooService();
        ViewModel = new MainViewModel(RapooService);
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        MainWindow = new MainWindow();
        MainWindow.Activate();
        InitializeService();
    }

    private async void InitializeService()
    {
        try
        {
            await ViewModel.InitializeAsync();
        }
        catch (Exception ex)
        {
            ViewModel.SetError($"サービスの初期化に失敗しました: {ex.Message}");
        }
    }
}
