using RapooControl.Models;
using RapooControl.Services;

namespace RapooControl.ViewModels;

/// <summary>Shell state: header metrics plus the DPI page view model.</summary>
public sealed class MainViewModel : ObservableObject
{
    private readonly IRapooService _service;
    private DeviceStatus _status;
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
        Dpi.PropertyChanged += (_, args) =>
        {
            if (args.PropertyName is nameof(DpiPageViewModel.StageCount) or nameof(DpiPageViewModel.CurrentStage))
            {
                OnPropertyChanged(nameof(DpiSummary));
            }
        };
    }

    public DpiPageViewModel Dpi { get; }

    public string DpiSummary => $"使用 Stage {Dpi.CurrentStage} / {Dpi.StageCount} ステージ";

    public bool IsMock => _service.IsMock;

    public string MockLabel => IsMock ? "MOCK" : "LIVE";

    public string ServiceName => _service.Name;

    public string DeviceName => _status.DeviceName;

    public string ConnectionLabel => _status.ConnectionLabel;

    public string BatteryText => _status.BatteryText;

    public double BatteryPercent => Math.Clamp(_status.BatteryPercent, 0, 100);

    public string ChargingLabel => _status.ChargingLabel;

    public string ActiveDpiText => _status.ActiveDpiText;

    public string PollingRateText => _status.PollingRateText;

    public string FirmwareText => _status.Firmware;

    public string UpdatedText => $"最終更新 {_status.UpdatedAt.ToLocalTime():HH:mm:ss}";

    public string ConnectionDetail => $"{ConnectionLabel} · {PollingRateText}";

    public string StatusLine => $"{ConnectionLabel} · {ActiveDpiText} · {PollingRateText} · {BatteryText}";

    public bool HasError => _hasError;

    public string ErrorMessage => _errorMessage;

    public async Task InitializeAsync()
    {
        var status = await _service.GetStatusAsync();
        ApplyStatus(status);

        await _service.StartAsync();
        await Dpi.LoadAsync();
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
        ApplyStatus(status);
    }

    private void ApplyStatus(DeviceStatus status)
    {
        _status = status;

        OnPropertyChanged(nameof(DeviceName));
        OnPropertyChanged(nameof(ConnectionLabel));
        OnPropertyChanged(nameof(ConnectionDetail));
        OnPropertyChanged(nameof(StatusLine));
        OnPropertyChanged(nameof(BatteryText));
        OnPropertyChanged(nameof(BatteryPercent));
        OnPropertyChanged(nameof(ChargingLabel));
        OnPropertyChanged(nameof(ActiveDpiText));
        OnPropertyChanged(nameof(PollingRateText));
        OnPropertyChanged(nameof(FirmwareText));
        OnPropertyChanged(nameof(UpdatedText));
    }
}
