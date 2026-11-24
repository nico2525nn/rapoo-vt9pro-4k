using RapooControl.Models;

namespace RapooControl.Services;

/// <summary>
/// Contract between the UI and the device engine. The real implementation
/// forwards to the Go sidecar; the mock implementation keeps the UI usable
/// without hardware.
/// </summary>
public interface IRapooService : IAsyncDisposable
{
    /// <summary>True when no real hardware is involved.</summary>
    bool IsMock { get; }

    /// <summary>Raised when the device summary changes (polling, DPI, battery, ...).</summary>
    event EventHandler<DeviceStatus>? StatusChanged;

    Task StartAsync(CancellationToken cancellationToken = default);

    Task StopAsync();

    Task<DeviceStatus> GetStatusAsync(CancellationToken cancellationToken = default);

    Task<DpiConfiguration> GetDpiConfigurationAsync(CancellationToken cancellationToken = default);

    Task<DeviceStatus> ApplyDpiConfigurationAsync(
        DpiConfiguration configuration,
        CancellationToken cancellationToken = default,
        long expectedGeneration = 0,
        long expectedSession = 0);

    Task<DeviceStatus> SetPollingRateAsync(
        int hertz,
        CancellationToken cancellationToken = default,
        long expectedGeneration = 0,
        long expectedSession = 0);
}
