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

    [DllImport("user32.dll")]
    private static extern bool AllowSetForegroundWindow(int processId);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern IntPtr FindWindow(string? className, string? windowName);

    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);

    private const int SwHide = 0;
    private const int SwShowNoActivate = 4;
    private const int AsfwAny = -1;

    private const string InstanceMutexName = @"Local\RapooControlWinUI";
    private const string ShowEventName = @"Local\RapooControlWinUI.Show";

    private readonly Mutex _instanceMutex;
    private readonly EventWaitHandle _showEvent;
    private int _showRequested;

    public static App Instance => (App)Current;

    public IRapooService RapooService { get; }

    public MainViewModel ViewModel { get; }

    public Window? MainWindow { get; private set; }

    public App()
    {
        // Manual reset keeps an early request signaled until the first
        // instance is ready. The losing process must not register a waiter,
        // or it could consume its own request before the winner sees it.
        _showEvent = new EventWaitHandle(false, EventResetMode.ManualReset, ShowEventName);
        _instanceMutex = new Mutex(true, InstanceMutexName, out var createdNew);
        if (!createdNew)
        {
            // A hidden startup or a second Windows-startup launch must not
            // pull the first instance into the foreground.
            if (!StartsHidden())
            {
                var firstWindow = FindWindow(null, "Rapoo Control");
                if (firstWindow != IntPtr.Zero)
                {
                    _ = GetWindowThreadProcessId(firstWindow, out var firstPid);
                    AllowSetForegroundWindow((int)firstPid);
                }
                else
                {
                    AllowSetForegroundWindow(AsfwAny);
                }
                _showEvent.Set();
            }
            Environment.Exit(0);
        }

        ThreadPool.RegisterWaitForSingleObject(
            _showEvent,
            (_, _) =>
            {
                _showEvent.Reset();
                if (MainWindow is RapooControl.MainWindow window)
                {
                    window.DispatcherQueue.TryEnqueue(window.ShowFromTray);
                }
                else
                {
                    Interlocked.Exchange(ref _showRequested, 1);
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
        if (MainWindow is RapooControl.MainWindow mainWindow &&
            Interlocked.Exchange(ref _showRequested, 0) == 1)
        {
            mainWindow.DispatcherQueue.TryEnqueue(mainWindow.ShowFromTray);
        }
        if (Environment.GetEnvironmentVariable("RAPOO_HEADLESS_TEST") == "1" && MainWindow is not null)
        {
            MainWindow.AppWindow.Move(new PointInt32(-2000, -2000));
            ShowWindow(WindowNative.GetWindowHandle(MainWindow), SwShowNoActivate);
        }
        else if (StartsHidden() && MainWindow is RapooControl.MainWindow { IsTrayReady: true } hiddenWindow)
        {
            hiddenWindow.AppWindow.Hide();
            ShowWindow(WindowNative.GetWindowHandle(hiddenWindow), SwHide);
        }
        else
        {
            MainWindow?.Activate();
        }
        InitializeService();
    }

    private static bool StartsHidden()
    {
        if (Environment.GetEnvironmentVariable("RAPOO_START_HIDDEN") == "1")
        {
            return true;
        }
        foreach (var argument in Environment.GetCommandLineArgs())
        {
            if (string.Equals(argument, "--hidden", StringComparison.OrdinalIgnoreCase) ||
                string.Equals(argument, "--tray", StringComparison.OrdinalIgnoreCase))
            {
                return true;
            }
        }
        return false;
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
