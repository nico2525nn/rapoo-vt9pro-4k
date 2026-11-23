using Microsoft.UI.Xaml;
using Windows.Graphics;
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
        LocalizationService.Instance.Initialize();

        var useMock = Environment.GetEnvironmentVariable("RAPOO_USE_MOCK") == "1";
        RapooService = useMock ? new MockRapooService() : new RealRapooService();
        ViewModel = new MainViewModel(RapooService);

        if (!SingleInstance.Current.TryAcquire())
        {
            Environment.Exit(0);
        }

        AppDomain.CurrentDomain.ProcessExit += (_, _) =>
        {
            DesktopIntegration.Current.Shutdown();
            SingleInstance.Current.Dispose();
            StopService();
        };
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        MainWindow = new Window
        {
            Title = "Rapoo Control",
        };
        MainWindow.Content = new MainPage();
        try
        {
            MainWindow.AppWindow.Resize(new SizeInt32(1180, 800));
        }
        catch
        {
            // The Skia head can use the platform default window size.
        }
        SingleInstance.Current.ShowRequested += (_, _) =>
            MainWindow?.DispatcherQueue.TryEnqueue(ShowWindow);
        if (SingleInstance.Current.ConsumePendingShow())
        {
            MainWindow.DispatcherQueue.TryEnqueue(ShowWindow);
        }
        DesktopIntegration.Current.Initialize(MainWindow, ViewModel);
        var startHidden = Environment.GetCommandLineArgs()
            .Any(argument => argument is "--hidden" or "--tray");
        if (startHidden && DesktopIntegration.Current.IsTrayReady)
        {
            DesktopIntegration.Current.HideWindow();
        }
        else
        {
            MainWindow.Activate();
        }
        InitializeService();
    }

    private void ShowWindow()
    {
        DesktopIntegration.Current.ShowWindow();
    }

    private async void InitializeService()
    {
        try
        {
            await ViewModel.InitializeAsync();
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine(ex);
            ViewModel.SetError(LocalizationService.Instance.Format(
                "サービスの初期化に失敗しました: {0}: {1}", ex.GetType().Name, ex.Message));
        }
    }

    private static void StopService()
    {
        if (Current is App app && app.RapooService is IAsyncDisposable disposable)
        {
            try
            {
                disposable.DisposeAsync().AsTask().GetAwaiter().GetResult();
            }
            catch
            {
                // Shutdown best effort.
            }
        }
    }
}
