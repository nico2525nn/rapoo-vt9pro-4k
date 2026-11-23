using RapooControl.Models;
using RapooControl.Services;

namespace RapooControl.ViewModels;

/// <summary>Shell status and the DPI page view model.</summary>
public sealed class MainViewModel : ObservableObject
{
    private readonly IRapooService _service;
    private readonly SynchronizationContext? _uiContext = SynchronizationContext.Current;
    private DeviceStatus _status;
    private Task? _initialization;
    private bool _hasError;
    private string _errorMessage = string.Empty;

    public MainViewModel(IRapooService service)
    {
        _service = service;
        _service.StatusChanged += OnStatusChanged;

        _status = new DeviceStatus(
            DeviceName: "Rapoo VT9 Pro 4K",
            Connection: ConnectionKind.Disconnected,
            MouseOnline: false,
            BatteryPercent: -1,
            Charging: false,
            Sleeping: false,
            ActiveDpi: 0,
            PollingRateHz: 0,
            Firmware: "—",
            UpdatedAt: DateTimeOffset.Now,
            IsMock: service.IsMock);

        Dpi = new DpiPageViewModel(service);
    }

    public DpiPageViewModel Dpi { get; }

    public RealRapooService? DeviceService => _service as RealRapooService;

    public string DeviceName => LocalizationService.Instance.T(_status.DeviceName);

    public string FirmwareText => IsConnected ? _status.Firmware : "—";

    public int BatteryPercent => _status.BatteryPercent;

    public bool IsCharging => _status.Charging;

    public bool IsDeviceConnected => IsConnected;

    public bool HasBattery => _status.BatteryPercent >= 0;

    public bool IsSleeping => IsConnected && _status.Sleeping;

    public string Mode => _status.Mode;

    public string BatteryText => _status.BatteryPercent < 0
        ? LocalizationService.Instance.T("未取得")
        : _status.Charging
            ? $"{_status.BatteryPercent}% · {LocalizationService.Instance.T("充電中")}"
            : $"{_status.BatteryPercent}%";

    public string TrayTooltip => $"{DeviceName} — {BatteryText}";

    private bool IsConnected => _status.Connected || _status.Connection != ConnectionKind.Disconnected;

    internal int DeviceRevision { get; private set; }

    public bool IsDemoMode => _status.DemoMode;

    public bool IsPassive => _status.Mode == "passive_only";

    public bool CanConfigureDevice => IsConnected && _status.MouseOnline && !IsPassive;

    public string ConnectionMessage => _status switch
    {
        _ when !IsConnected => LocalizationService.Instance.T("マウスが接続されていません。レシーバーまたは USB ケーブルを確認してください。"),
        { Sleeping: true } => LocalizationService.Instance.T("マウスがスリープ中です。マウスを動かして復帰させてください。"),
        { MouseOnline: false } => LocalizationService.Instance.T("マウスからの応答を待っています。マウスを動かしてください。"),
        { IsMock: true } => LocalizationService.Instance.T("モックデータを表示しています。"),
        _ => string.Empty,
    };

    public bool HasConnectionMessage => ConnectionMessage.Length > 0;

    public string SettingsNotice => DeviceService is null
        ? LocalizationService.Instance.T("このページの設定は実機またはデモモードで利用できます。")
        : IsPassive
            ? LocalizationService.Instance.T("0-TX モード中です。変更するには「設定」で通常に戻してください。")
            : ConnectionMessage;

    public string DpiNotice => DeviceService is null ? ConnectionMessage : SettingsNotice;

    public bool HasDpiNotice => DpiNotice.Length > 0;

    public bool HasError => _hasError;

    public string ErrorMessage => _errorMessage;

    public Task InitializeAsync()
    {
        if (_initialization is null || _initialization.IsFaulted || _initialization.IsCanceled)
        {
            _initialization = InitializeCoreAsync();
        }

        return _initialization;
    }

    private async Task InitializeCoreAsync()
    {
        await _service.StartAsync();

        var status = await _service.GetStatusAsync();
        ApplyStatus(status);
    }

    public async Task RefreshAsync()
    {
        try
        {
            ApplyStatus(await _service.GetStatusAsync());
            if (DeviceService is not null && CanConfigureDevice)
            {
                await Dpi.LoadAsync();
            }
        }
        catch (Exception ex)
        {
            SetError(ex.Message);
        }
    }

    public void RefreshLocalization()
    {
        OnPropertyChanged(nameof(DeviceName));
        OnPropertyChanged(nameof(BatteryText));
        OnPropertyChanged(nameof(TrayTooltip));
        OnPropertyChanged(nameof(ConnectionMessage));
        OnPropertyChanged(nameof(HasConnectionMessage));
        OnPropertyChanged(nameof(SettingsNotice));
        OnPropertyChanged(nameof(DpiNotice));
        OnPropertyChanged(nameof(HasDpiNotice));
        Dpi.RefreshLocalization();
    }

    public void SetError(string message)
    {
        _hasError = true;
        _errorMessage = LocalizationService.Instance.Error(message);
        OnPropertyChanged(nameof(HasError));
        OnPropertyChanged(nameof(ErrorMessage));
    }

    private void OnStatusChanged(object? sender, DeviceStatus status)
    {
        if (_uiContext is null || SynchronizationContext.Current == _uiContext)
        {
            ApplyStatus(status);
            return;
        }

        _uiContext.Post(_ => ApplyStatus(status), null);
    }

    private void ApplyStatus(DeviceStatus status)
    {
        var deviceChanged = _status.DemoMode != status.DemoMode || _status.Connection != status.Connection ||
            _status.DeviceName != status.DeviceName;
        _status = status;
        if (deviceChanged)
        {
            DeviceRevision++;
            Dpi.InvalidateConfiguration();
            OnPropertyChanged(nameof(DeviceRevision));
        }
        Dpi.DeviceWritable = CanConfigureDevice;

        OnPropertyChanged(nameof(DeviceName));
        OnPropertyChanged(nameof(FirmwareText));
        OnPropertyChanged(nameof(BatteryPercent));
        OnPropertyChanged(nameof(IsCharging));
        OnPropertyChanged(nameof(IsDeviceConnected));
        OnPropertyChanged(nameof(HasBattery));
        OnPropertyChanged(nameof(IsSleeping));
        OnPropertyChanged(nameof(Mode));
        OnPropertyChanged(nameof(BatteryText));
        OnPropertyChanged(nameof(TrayTooltip));
        OnPropertyChanged(nameof(ConnectionMessage));
        OnPropertyChanged(nameof(HasConnectionMessage));
        OnPropertyChanged(nameof(DpiNotice));
        OnPropertyChanged(nameof(HasDpiNotice));
        OnPropertyChanged(nameof(IsDemoMode));
        OnPropertyChanged(nameof(IsPassive));
        OnPropertyChanged(nameof(CanConfigureDevice));
    }
}
