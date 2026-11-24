using System.Runtime.InteropServices;
using Microsoft.UI.Windowing;
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

    private readonly bool _startHidden;
    private bool _closing;

    public App()
    {
        InitializeComponent();
        var smoke = Environment.GetEnvironmentVariable("RAPOO_SMOKE") == "1";
        LocalizationService.Instance.Initialize();
        if (!smoke)
        {
            AutoStartService.MigrateLegacy();
            DesktopPreferences.Current.MigrateLegacy();
        }
        DesktopPreferences.Current.Load();
        if (smoke)
        {
            DesktopPreferences.Current.UseSmokeDefaults();
        }

        var useMock = Environment.GetEnvironmentVariable("RAPOO_USE_MOCK") == "1";
        RapooService = useMock ? new MockRapooService() : new RealRapooService();
        ViewModel = new MainViewModel(RapooService);

        _startHidden = ShouldStartHidden();
        try
        {
            if (!SingleInstance.Current.TryAcquire(requestShow: !_startHidden))
            {
                Environment.Exit(0);
            }
        }
        catch (IOException ex)
        {
            ShowStartupError(ex.Message);
            Environment.Exit(1);
        }

        AppDomain.CurrentDomain.ProcessExit += (_, _) => Shutdown();
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        MainWindow = new Window
        {
            Title = "Rapoo Control",
        };
        var page = new MainPage();
        MainWindow.Content = page;
        MainWindow.Activated += OnFirstWindowActivation;
        MainWindow.Closed += (_, _) =>
        {
            _closing = true;
            SingleInstance.Current.BeginShutdown();
        };
#if WINDOWS
        MainWindow.ExtendsContentIntoTitleBar = true;
        MainWindow.SetTitleBar(page.TitleBarDragRegion);
#endif
        ViewModel.AttachDispatcher(action =>
            MainWindow.DispatcherQueue.TryEnqueue(() => action()));
        SingleInstance.Current.SetShowHandler(
            () => !_closing &&
                  MainWindow is { } window &&
                  window.DispatcherQueue.TryEnqueue(ShowWindow));
        DesktopIntegration.Current.Initialize(MainWindow, ViewModel);
        MainWindow.AppWindow.Closing += (_, _) =>
        {
            if (!DesktopPreferences.Current.CloseToTray ||
                !DesktopIntegration.Current.IsTrayReady)
            {
                _closing = true;
                SingleInstance.Current.BeginShutdown();
            }
        };
        if (_startHidden && DesktopIntegration.Current.IsTrayReady)
        {
            DesktopIntegration.Current.HideWindow();
        }
        else
        {
            MainWindow.Activate();
        }
        InitializeService();
    }

    private void OnFirstWindowActivation(object sender, WindowActivatedEventArgs args)
    {
        if (args.WindowActivationState == WindowActivationState.Deactivated || MainWindow is null)
        {
            return;
        }

        MainWindow.Activated -= OnFirstWindowActivation;
        var window = MainWindow.AppWindow;
        const int width = 1180;
        const int height = 800;
        window.Resize(new SizeInt32(width, height));
#if WINDOWS
        var display = DisplayArea.GetFromWindowId(window.Id, DisplayAreaFallback.Nearest);
        if (display is not null)
        {
            var area = display.WorkArea;
            window.Move(new PointInt32(
                area.X + Math.Max(0, (area.Width - width) / 2),
                area.Y + Math.Max(0, (area.Height - height) / 2)));
        }
#endif
    }

    private static bool ShouldStartHidden()
    {
        if (string.Equals(
            Environment.GetEnvironmentVariable("RAPOO_START_HIDDEN"),
            "1",
            StringComparison.OrdinalIgnoreCase))
        {
            return true;
        }

        return Environment.GetCommandLineArgs().Any(argument =>
            string.Equals(argument, "--hidden", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(argument, "--tray", StringComparison.OrdinalIgnoreCase));
    }

    internal void ShowWindow()
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
                "サービスの初期化に失敗しました: {0}: {1}",
                ex.GetType().Name,
                LocalizationService.Instance.Error(ex.Message)));
        }
    }

    private void Shutdown()
    {
        // Stop accepting launches before the sidecar is torn down, then
        // release the instance lock only after the sidecar has stopped.
        SingleInstance.Current.BeginShutdown();
        var stopped = StopService();
        DesktopIntegration.Current.Shutdown();
        if (stopped)
        {
            SingleInstance.Current.Dispose();
        }
        else
        {
            // Keep the ownership lock until the process exits; the forced stop
            // in the bridge client should make this path extremely rare.
            System.Diagnostics.Debug.WriteLine(
                "service did not stop; retaining the instance lock until process exit");
        }
    }

    private static bool StopService()
    {
        if (Current is not App app)
        {
            return true;
        }

        try
        {
            app.RapooService.DisposeAsync().AsTask().GetAwaiter().GetResult();
            return true;
        }
        catch (Exception ex)
        {
            // Never delay process exit; keep a diagnostic trail.
            System.Diagnostics.Debug.WriteLine($"service shutdown failed: {ex}");
            return false;
        }
    }

    private static void ShowStartupError(string message)
    {
#if WINDOWS
        MessageBoxW(IntPtr.Zero, message, "Rapoo Control", 0x00000010);
#else
        Console.Error.WriteLine(message);
#endif
    }

#if WINDOWS
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern int MessageBoxW(IntPtr hWnd, string text, string caption, uint type);
#endif
}
