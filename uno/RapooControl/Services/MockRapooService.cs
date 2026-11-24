using RapooControl.Models;

namespace RapooControl.Services;

/// <summary>
/// In-memory implementation used by the WinUI 3 prototype. Values never leave the
/// process: there is no HID access, no firmware flashing, and no mouse changes.
/// </summary>
public sealed class MockRapooService : IRapooService
{
    private readonly object _gate = new();
    private DpiConfiguration _dpi;
    private DeviceStatus _status;

    public MockRapooService()
    {
        _dpi = new DpiConfiguration(
            StageCount: 4,
            CurrentStage: 2,
            Stages: new[]
            {
                new DpiStage(1, 800, 800),
                new DpiStage(2, 1600, 1600),
                new DpiStage(3, 3200, 3200),
                new DpiStage(4, 6400, 6400),
            });

        _status = new DeviceStatus(
            DeviceName: "Rapoo VT9 Pro 4K",
            Connection: ConnectionKind.Wireless4K,
            MouseOnline: true,
            BatteryPercent: 78,
            Charging: false,
            Sleeping: false,
            ActiveDpi: 1600,
            PollingRateHz: 4000,
            Firmware: "1.25 / 1.08",
            UpdatedAt: DateTimeOffset.Now,
            IsMock: true);
    }

    public bool IsMock => true;

    public event EventHandler<DeviceStatus>? StatusChanged;

    public Task StartAsync(CancellationToken cancellationToken = default)
    {
        return Task.CompletedTask;
    }

    public Task StopAsync() => Task.CompletedTask;

    public ValueTask DisposeAsync() => ValueTask.CompletedTask;

    public Task<DeviceStatus> GetStatusAsync(CancellationToken cancellationToken = default)
    {
        var status = GetStatus();
        RaiseStatusChanged(status);
        return Task.FromResult(status);
    }

    public Task<DeviceStatus> RefreshAsync(CancellationToken cancellationToken = default) =>
        GetStatusAsync(cancellationToken);

    public Task<DpiConfiguration> GetDpiConfigurationAsync(CancellationToken cancellationToken = default)
    {
        lock (_gate)
        {
            return Task.FromResult(_dpi);
        }
    }

    public Task<DeviceStatus> ApplyDpiConfigurationAsync(
        DpiConfiguration configuration,
        CancellationToken cancellationToken = default,
        long expectedGeneration = 0,
        long expectedSession = 0)
    {
        DeviceStatus status;

        lock (_gate)
        {
            var stageCount = Math.Clamp(configuration.StageCount, 1, RapooLimits.MaxDpiStages);
            var currentStage = Math.Clamp(configuration.CurrentStage, 1, stageCount);

            var stages = configuration.Stages
                .OrderBy(stage => stage.Index)
                .Take(stageCount)
                .Select((stage, i) => new DpiStage(i + 1, NormalizeDpi(stage.X), NormalizeDpi(stage.Y)))
                .ToArray();

            if (stages.Length < stageCount)
            {
                throw new ArgumentException(
                    $"Expected {stageCount} DPI stages, but received {stages.Length}.",
                    nameof(configuration));
            }

            _dpi = new DpiConfiguration(stageCount, currentStage, stages);

            var active = stages[currentStage - 1];
            _status = _status with
            {
                ActiveDpi = active.X,
                UpdatedAt = DateTimeOffset.Now,
            };

            status = _status;
        }

        RaiseStatusChanged(status);
        return Task.FromResult(status);
    }

    public Task<DeviceStatus> SetPollingRateAsync(
        int hertz,
        CancellationToken cancellationToken = default,
        long expectedGeneration = 0,
        long expectedSession = 0)
    {
        if (!RapooLimits.PollingRates.Contains(hertz))
        {
            throw new ArgumentOutOfRangeException(
                nameof(hertz),
                hertz,
                $"Unsupported polling rate. Expected one of: {string.Join(", ", RapooLimits.PollingRates)} Hz.");
        }

        DeviceStatus status;

        lock (_gate)
        {
            _status = _status with
            {
                PollingRateHz = hertz,
                UpdatedAt = DateTimeOffset.Now,
            };
            status = _status;
        }

        RaiseStatusChanged(status);
        return Task.FromResult(status);
    }

    private DeviceStatus GetStatus()
    {
        lock (_gate)
        {
            return _status;
        }
    }

    private void RaiseStatusChanged(DeviceStatus status)
    {
        StatusChanged?.Invoke(this, status);
    }

    /// <summary>Match the PAW3395 hardware: 50..26000, snapped to 50 DPI steps.</summary>
    private static int NormalizeDpi(int value)
    {
        var clamped = Math.Clamp(value, RapooLimits.MinDpi, RapooLimits.MaxDpi);
        return (int)(Math.Round(clamped / (double)RapooLimits.DpiStep, MidpointRounding.AwayFromZero) * RapooLimits.DpiStep);
    }
}
