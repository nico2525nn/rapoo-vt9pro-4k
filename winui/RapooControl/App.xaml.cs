using System.Runtime.InteropServices;
using Microsoft.UI.Dispatching;
using Microsoft.UI.Xaml;
using Windows.Graphics;
using WinRT.Interop;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl;

public partial class App : Application
{
    [DllImport("user32.dll")]
    private static extern bool ShowWindow(IntPtr hWnd, int command);

    private const int SwShowNoActivate = 4;

    private const string InstanceMutexName = @"Local\RapooControlWinUI";
    private const string ShowEventName = @"Local\RapooControlWinUI.Show";

    private readonly Mutex _instanceMutex;
    private readonly EventWaitHandle _showEvent;

    public static App Instance => (App)Current;

    public IRapooService RapooService { get; }

    public MainViewModel ViewModel { get; }

    public Window? MainWindow { get; private set; }

    public App()
    {
        _instanceMutex = new Mutex(true, InstanceMutexName, out var createdNew);
        if (!createdNew)
        {
            try
            {
                EventWaitHandle.OpenExisting(ShowEventName).Set();
            }
            catch
            {
                // The first instance is shutting down.
            }
            Environment.Exit(0);
        }

        _showEvent = new EventWaitHandle(false, EventResetMode.AutoReset, ShowEventName);
        ThreadPool.RegisterWaitForSingleObject(
            _showEvent,
            (_, _) =>
            {
                if (MainWindow is RapooControl.MainWindow window)
                {
                    window.DispatcherQueue.TryEnqueue(window.ShowFromTray);
                }
            },
            null,
            Timeout.Infinite,
            executeOnlyOnce: false);

        InitializeComponent();
        LocalizationService.Instance.Initialize();

        var queue = DispatcherQueue.GetForCurrentThread();
        if (queue is not null)
        {
            SynchronizationContext.SetSynchronizationContext(new DispatcherQueueSynchronizationContext(queue));
        }

        var useMock = Environment.GetEnvironmentVariable("RAPOO_USE_MOCK") == "1";
        RapooService = useMock ? new MockRapooService() : new RealRapooService();
        ViewModel = new MainViewModel(RapooService);

        AppDomain.CurrentDomain.ProcessExit += (_, _) => StopService();
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        MainWindow = new MainWindow();
        if (Environment.GetEnvironmentVariable("RAPOO_HEADLESS_TEST") == "1" && MainWindow is not null)
        {
            MainWindow.AppWindow.Move(new PointInt32(-2000, -2000));
            ShowWindow(WindowNative.GetWindowHandle(MainWindow), SwShowNoActivate);
        }
        else
        {
            MainWindow?.Activate();
        }
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
            await ViewModel.InitializeAsync();
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine(ex);
            ViewModel.SetError(LocalizationService.Instance.Format("サービスの初期化に失敗しました: {0}: {1}", ex.GetType().Name, ex.Message));
        }
    }
}
