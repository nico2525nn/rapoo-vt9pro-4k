using Microsoft.UI.Dispatching;
using Microsoft.UI.Xaml;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl;

public partial class App : Application
{
    public static App Instance => (App)Current;

    public IRapooService RapooService { get; }

    public MacroService Macros { get; }

    public MainViewModel ViewModel { get; }

    public Window? MainWindow { get; private set; }

    public App()
    {
        InitializeComponent();

        var queue = DispatcherQueue.GetForCurrentThread();
        if (queue is not null)
        {
            SynchronizationContext.SetSynchronizationContext(new DispatcherQueueSynchronizationContext(queue));
        }

        var useMock = Environment.GetEnvironmentVariable("RAPOO_USE_MOCK") == "1";
        RapooService = useMock ? new MockRapooService() : new RealRapooService();
        Macros = new MacroService();
        ViewModel = new MainViewModel(RapooService);

        AppDomain.CurrentDomain.ProcessExit += (_, _) =>
        {
            Macros.Dispose();
            StopService();
        };
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        MainWindow = new MainWindow();
        MainWindow.Activate();
        InitializeService();
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

    private async void InitializeService()
    {
        try
        {
            await Macros.StartAsync();
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine(ex);
            ViewModel.SetError($"マクロの初期化に失敗しました: {ex.GetType().Name}: {ex.Message}");
        }

        try
        {
            await ViewModel.InitializeAsync();
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine(ex);
            ViewModel.SetError($"サービスの初期化に失敗しました: {ex.GetType().Name}: {ex.Message}");
        }
    }
}
