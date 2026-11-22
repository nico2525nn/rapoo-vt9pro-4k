using RapooControl.Models;
using RapooControl.Services;

namespace RapooControl.ViewModels;

/// <summary>Shell status and the DPI page view model.</summary>
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
    }

    public DpiPageViewModel Dpi { get; }

    public string DeviceName => _status.DeviceName;

    public string FirmwareText => _status.Firmware;

    public string StatusLine
    {
        get
        {
            if (_status.Connection == ConnectionKind.Disconnected)
            {
                return "未接続";
            }

            var connection = !_status.MouseOnline
                ? "スリープ"
                : _status.Connection == ConnectionKind.Wired ? "USB-C" : "4K";
            return $"{connection} · {_status.ActiveDpiText} · {_status.PollingRateText} · {_status.BatteryText}";
        }
    }

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
        OnPropertyChanged(nameof(FirmwareText));
        OnPropertyChanged(nameof(StatusLine));
    }
}
