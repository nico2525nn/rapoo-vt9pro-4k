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

    public App()
    {
        InitializeComponent();
        LocalizationService.Instance.Initialize();
        DesktopPreferences.Current.Load();

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

        AutoStartService.MigrateLegacy();

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
#if WINDOWS
        MainWindow.ExtendsContentIntoTitleBar = true;
        MainWindow.SetTitleBar(page.TitleBarDragRegion);
#endif
        ViewModel.AttachDispatcher(action =>
            MainWindow.DispatcherQueue.TryEnqueue(() => action()));
        SingleInstance.Current.SetShowHandler(
            () => MainWindow?.DispatcherQueue.TryEnqueue(ShowWindow));
        DesktopIntegration.Current.Initialize(MainWindow, ViewModel);
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

    private static bool ShouldStartHidden() => Environment.GetCommandLineArgs()
        .Any(argument => argument is "--hidden" or "--tray");

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
                "サービスの初期化に失敗しました: {0}: {1}", ex.GetType().Name, ex.Message));
        }
    }

    private void Shutdown()
    {
        // Release the single-instance lock only after the sidecar has stopped,
        // so a successor never starts a second HID owner.
        StopService();
        DesktopIntegration.Current.Shutdown();
        SingleInstance.Current.Dispose();
    }

    private static void StopService()
    {
        if (Current is not App app)
        {
            return;
        }

        try
        {
            app.RapooService.DisposeAsync().AsTask().GetAwaiter().GetResult();
        }
        catch
        {
            // Shutdown is best effort; never delay process exit.
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
