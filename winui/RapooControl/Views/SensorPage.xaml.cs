using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed partial class SensorPage : Page, IConfigPageActions
{
    public sealed record Option(int Value, string Label);

    private BridgeSensor? _sensor;
    private int _deviceRevision;
    private bool _debounceUsesMilliseconds;
    private List<BridgeDebounceStep>? _debounceSteps;
    private MainViewModel ViewModel => App.Instance.ViewModel;
    public PageFeedback Feedback { get; } = new();

    private bool HasConfiguration => _sensor is not null && _deviceRevision == ViewModel.DeviceRevision;
    private bool IsDirty => HasConfiguration && GetPatch(_sensor!).Count > 0;
    public bool CanReload => !Feedback.IsBusy && ViewModel.DeviceService is not null && ViewModel.CanConfigureDevice;
    public bool CanApply => CanReload && IsDirty;
    public event EventHandler? ActionsChanged;

    public SensorPage()
    {
        InitializeComponent();
        Feedback.PropertyChanged += (_, e) =>
        {
            if (e.PropertyName == nameof(PageFeedback.IsBusy))
            {
                UpdateControls();
            }
        };
        foreach (var comboBox in new[] { LiftOff, PressDebounce, ReleaseDebounce })
        {
            comboBox.SelectionChanged += (_, _) => DraftChanged();
        }
        foreach (var numberBox in new[] { Angle, SleepMinutes })
        {
            numberBox.ValueChanged += (_, _) => DraftChanged();
        }
        foreach (var toggle in new[] { MotionSync, Ripple, Correction, GlassMode, LedLowPower })
        {
            toggle.Toggled += (_, _) => DraftChanged();
        }
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        ViewModel.PropertyChanged += DeviceStatusChanged;
        UpdateControls();
        if (IsDirty || ViewModel.DeviceService is not { } service)
        {
            return;
        }

        await Feedback.RunAsync(async () =>
        {
            await ViewModel.InitializeAsync();
            if (service.CachedSnapshot is { Sensor: not null } snapshot)
            {
                await LoadDebounceStepsAsync(service);
                LoadSnapshot(snapshot);
            }
            else if (ViewModel.CanConfigureDevice)
            {
                await LoadAsync(service);
            }
        });
    }

    private void Page_Unloaded(object sender, RoutedEventArgs e) => ViewModel.PropertyChanged -= DeviceStatusChanged;

    private void DeviceStatusChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainViewModel.CanConfigureDevice))
        {
            UpdateControls();
        }
    }

    public Task ReloadAsync() => CanReload && ViewModel.DeviceService is { } service
        ? Feedback.RunAsync(() => LoadAsync(service), PageFeedback.Reloaded)
        : Task.CompletedTask;

    private async Task LoadAsync(RealRapooService service)
    {
        await LoadDebounceStepsAsync(service);
        LoadSnapshot(await service.RefreshSnapshotAsync());
    }

    private async Task LoadDebounceStepsAsync(RealRapooService service)
    {
        _debounceSteps ??= (await service.GetCatalogAsync()).DebounceSteps
            ?? throw new InvalidOperationException("デバウンスの選択肢を読み取れませんでした。");
    }

    private void LoadSnapshot(BridgeSnapshot snapshot)
    {
        var sensor = snapshot.Sensor ?? throw new InvalidOperationException("センサー設定を読み取れませんでした。");
        _debounceUsesMilliseconds = snapshot.Model?.ProtocolGen == 2;
        var debounce = _debounceUsesMilliseconds
            ? Enumerable.Range(0, 33).Select(ms => new Option(ms, $"{ms} ms")).ToArray()
            : _debounceSteps!.Select(step => new Option(step.Index, step.Label)).ToArray();
        var liftOff = Enumerable.Range(0, 11).Select(level => new Option(level, $"{1 + level / 10.0:F1} mm")).ToArray();
        Select(LiftOff, liftOff, sensor.LiftOffLevel);
        Select(PressDebounce, debounce, _debounceUsesMilliseconds ? sensor.PressDebounceMs : sensor.PressDebounceStep);
        Select(ReleaseDebounce, debounce, _debounceUsesMilliseconds ? sensor.ReleaseDebounceMs : sensor.ReleaseDebounceStep);
        Angle.Value = sensor.SensorAngleDeg;
        SleepMinutes.Value = sensor.SleepMinutes;
        MotionSync.IsOn = sensor.MotionSync;
        Ripple.IsOn = sensor.RippleControl;
        Correction.IsOn = sensor.LinearCorrection;
        GlassMode.IsOn = sensor.GlassMode;
        LedLowPower.IsOn = sensor.LedLowPowerMode != 0;
        _sensor = sensor;
        _deviceRevision = ViewModel.DeviceRevision;
        UpdateControls();
    }

    private static void Select(ComboBox control, IReadOnlyList<Option> options, int value)
    {
        control.ItemsSource = options;
        control.SelectedItem = options.FirstOrDefault(option => option.Value == value);
    }

    public async Task ApplyAsync()
    {
        if (!CanReload || !HasConfiguration || ViewModel.DeviceService is not { } service)
        {
            return;
        }

        var patch = GetPatch(_sensor!);
        if (patch.Count == 0)
        {
            Feedback.Message = PageFeedback.NoChanges;
            return;
        }

        await Feedback.RunAsync(async () => LoadSnapshot(await service.ApplySensorAsync(patch)), PageFeedback.Applied);
    }

    private Dictionary<string, object> GetPatch(BridgeSensor sensor)
    {
        var patch = new Dictionary<string, object>();
        AddSelection("liftOffLevel", LiftOff, sensor.LiftOffLevel);
        AddSelection("pressDebounceStep", PressDebounce, _debounceUsesMilliseconds ? sensor.PressDebounceMs : sensor.PressDebounceStep);
        AddSelection("releaseDebounceStep", ReleaseDebounce, _debounceUsesMilliseconds ? sensor.ReleaseDebounceMs : sensor.ReleaseDebounceStep);
        Change("sensorAngleDeg", double.IsNaN(Angle.Value) ? sensor.SensorAngleDeg : (int)Math.Round(Angle.Value), sensor.SensorAngleDeg);
        Change("sleepMinutes", double.IsNaN(SleepMinutes.Value) ? sensor.SleepMinutes : (int)Math.Round(SleepMinutes.Value), sensor.SleepMinutes);
        Change("motionSync", MotionSync.IsOn, sensor.MotionSync);
        Change("rippleControl", Ripple.IsOn, sensor.RippleControl);
        Change("linearCorrection", Correction.IsOn, sensor.LinearCorrection);
        Change("glassMode", GlassMode.IsOn, sensor.GlassMode);
        if (LedLowPower.IsOn != (sensor.LedLowPowerMode != 0))
        {
            patch["ledLowPowerMode"] = LedLowPower.IsOn ? 1 : 0;
        }

        return patch;

        void Change<T>(string key, T next, T current) where T : struct
        {
            if (!EqualityComparer<T>.Default.Equals(next, current))
            {
                patch[key] = next;
            }
        }

        void AddSelection(string key, ComboBox control, int current)
        {
            if (control.SelectedItem is Option option)
            {
                Change(key, option.Value, current);
            }
        }
    }

    private void DraftChanged()
    {
        if (!Feedback.IsBusy)
        {
            Feedback.Message = string.Empty;
            ActionsChanged?.Invoke(this, EventArgs.Empty);
        }
    }

    private void UpdateControls()
    {
        Editor.Visibility = HasConfiguration ? Visibility.Visible : Visibility.Collapsed;
        PowerCard.Visibility = Editor.Visibility;
        Editor.IsEnabled = PowerEditor.IsEnabled = HasConfiguration && CanReload;
        DeviceNotice.Text = ViewModel.SettingsNotice;
        DeviceNotice.Visibility = DeviceNotice.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
        ActionsChanged?.Invoke(this, EventArgs.Empty);
    }
}
