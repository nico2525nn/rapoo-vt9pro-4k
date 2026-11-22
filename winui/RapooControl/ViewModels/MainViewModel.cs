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
            ActiveDpi: 0,
            PollingRateHz: 0,
            Firmware: "—",
            UpdatedAt: DateTimeOffset.Now,
            IsMock: service.IsMock);

        Dpi = new DpiPageViewModel(service);
    }

    public DpiPageViewModel Dpi { get; }

    public RealRapooService? DeviceService => _service as RealRapooService;

    public string DeviceName => _status.DeviceName;

    public string FirmwareText => IsConnected ? _status.Firmware : "—";

    private bool IsConnected => _status.Connected || _status.Connection != ConnectionKind.Disconnected;

    public string StatusLine
    {
        get
        {
            if (!IsConnected)
            {
                return "未接続";
            }

            if (!_status.MouseOnline)
            {
                return "スリープ";
            }

            var connection = _status.DemoMode ? "デモ" : _status.IsMock ? "モック" :
                _status.Connection == ConnectionKind.Wired ? "USB-C" : "4K";
            return $"{connection} · {_status.ActiveDpiText} · {_status.PollingRateText} · {_status.BatteryText}";
        }
    }

    public bool IsDemoMode => _status.DemoMode;

    public bool IsPassive => _status.Mode == "passive_only";

    public bool CanConfigureDevice => IsConnected && _status.MouseOnline && !IsPassive;

    public string ConnectionMessage => _status switch
    {
        _ when !IsConnected => "マウスが接続されていません。レシーバーまたは USB ケーブルを確認してください。",
        { MouseOnline: false } => "マウスがスリープ中です。マウスを動かして復帰させてください。",
        { IsMock: true } => "モックデータを表示しています。",
        _ => string.Empty,
    };

    public bool HasConnectionMessage => ConnectionMessage.Length > 0;

    public string SettingsNotice => DeviceService is null
        ? "このページの設定は実機またはデモモードで利用できます。"
        : IsPassive ? "0-TX モード中です。変更するには「設定」で通常に戻してください。" : ConnectionMessage;

    public string DpiNotice => DeviceService is null ? ConnectionMessage : SettingsNotice;

    public bool HasDpiNotice => DpiNotice.Length > 0;

    public bool HasError => _hasError;

    public string ErrorMessage => _errorMessage;

    public Task InitializeAsync()
    {
        if (_initialization is { IsCompletedSuccessfully: true })
        {
            return _initialization;
        }

        return _initialization = InitializeCoreAsync();
    }

    private async Task InitializeCoreAsync()
    {
        await _service.StartAsync();

        var status = await _service.GetStatusAsync();
        ApplyStatus(status);

        if (CanConfigureDevice)
        {
            await Dpi.LoadAsync();
        }
    }

    public void UpdateDpi(BridgeSnapshot snapshot)
    {
        if (snapshot.Dpi is { } dpi)
        {
            Dpi.LoadConfiguration(RealRapooService.MapDpiConfiguration(dpi), snapshot.Sensor?.PollingRateHz ?? _status.PollingRateHz);
        }
    }

    public void SetError(string message)
    {
        _hasError = true;
        _errorMessage = message;
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
        if (_status.DemoMode != status.DemoMode || _status.Connection != status.Connection)
        {
            Dpi.InvalidateConfiguration();
        }
        _status = status;

        OnPropertyChanged(nameof(DeviceName));
        OnPropertyChanged(nameof(FirmwareText));
        OnPropertyChanged(nameof(StatusLine));
        OnPropertyChanged(nameof(ConnectionMessage));
        OnPropertyChanged(nameof(HasConnectionMessage));
        OnPropertyChanged(nameof(DpiNotice));
        OnPropertyChanged(nameof(HasDpiNotice));
        OnPropertyChanged(nameof(IsDemoMode));
        OnPropertyChanged(nameof(IsPassive));
        OnPropertyChanged(nameof(CanConfigureDevice));
        Dpi.DeviceWritable = CanConfigureDevice;
    }
}
