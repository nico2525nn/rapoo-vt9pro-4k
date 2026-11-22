using RapooControl.Models;

namespace RapooControl.Services;

/// <summary>
/// Minimal contract between the WinUI shell and the device engine.
///
/// The prototype ships with <see cref="MockRapooService"/>. A later implementation
/// can forward these calls to the existing Go engine (local HTTP/JSON or a named
/// pipe) without changing the pages or view models.
/// </summary>
public interface IRapooService
{
    /// <summary>Human-readable implementation name shown in the settings page.</summary>
    string Name { get; }

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
        CancellationToken cancellationToken = default);

    Task<DeviceStatus> SetPollingRateAsync(int hertz, CancellationToken cancellationToken = default);
}
