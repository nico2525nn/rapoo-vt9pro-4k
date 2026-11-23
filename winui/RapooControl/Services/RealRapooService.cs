using RapooControl.Models;

namespace RapooControl.Services;

/// <summary>
/// Real device service backed by the Go sidecar. HID ownership and all protocol
/// work stay in Go; this class only maps the bridge JSON to the UI models.
/// </summary>
public sealed partial class RealRapooService : IRapooService, IAsyncDisposable
{
    private readonly RapooBridgeClient _client = new();
    private BridgeState? _lastState;
    private BridgeSnapshot? _lastSnapshot;

    public RealRapooService()
    {
        _client.StateChanged += (_, state) =>
        {
            if (_lastState is { } previous &&
                (previous.Connected != state.Connected || previous.DemoMode != state.DemoMode ||
                 previous.Connection != state.Connection || previous.ProductId != state.ProductId))
            {
                _lastSnapshot = null;
            }
            _lastState = state;
            StatusChanged?.Invoke(this, MapStatus(state));
        };
        _client.Faulted += (_, message) =>
        {
            Faulted?.Invoke(this, message);
        };
    }

    public string Name => "RapooBridge";

    public bool IsMock => false;

    public event EventHandler<DeviceStatus>? StatusChanged;

    public event EventHandler<string>? Faulted;

    public async Task StartAsync(CancellationToken cancellationToken = default)
    {
        await _client.StartAsync(cancellationToken).ConfigureAwait(false);
        _lastState = await _client.GetStateAsync(cancellationToken).ConfigureAwait(false);
        StatusChanged?.Invoke(this, MapStatus(_lastState));

        try
        {
            if (Environment.GetEnvironmentVariable("RAPOO_DEMO") == "1")
            {
                await _client.SetDemoModeAsync(true, cancellationToken).ConfigureAwait(false);
            }

            _lastSnapshot = await ReadSnapshotWithRetryAsync(cancellationToken).ConfigureAwait(false);
            if (_lastState is not null)
            {
                StatusChanged?.Invoke(this, MapStatus(_lastState));
            }
        }
        catch (Exception ex) when (ex is RapooBridgeException or InvalidOperationException)
        {
            // The mouse may be asleep or disconnected. The shell still renders.
        }
    }

    public Task StopAsync() => _client.StopAsync();

    public async Task<DeviceStatus> GetStatusAsync(CancellationToken cancellationToken = default)
    {
        _lastState = await _client.GetStateAsync(cancellationToken).ConfigureAwait(false);
        return MapStatus(_lastState);
    }

    public async Task<DpiConfiguration> GetDpiConfigurationAsync(CancellationToken cancellationToken = default)
    {
        var snapshot = _lastSnapshot ?? await ReadSnapshotWithRetryAsync(cancellationToken).ConfigureAwait(false);
        return snapshot.Dpi is { } dpi
            ? MapDpiConfiguration(dpi)
            : throw new InvalidOperationException("DPI設定を読み取れませんでした。");
    }

    internal static DpiConfiguration MapDpiConfiguration(BridgeDpiConfig dpi) =>
        new(
            dpi.StageCount,
            dpi.CurrentStage,
            (dpi.AllStages ?? dpi.Stages ?? [])
                .OrderBy(stage => stage.Index)
                .Select(stage => new DpiStage(stage.Index, stage.X, stage.Y))
                .ToArray());

    public async Task<DeviceStatus> ApplyDpiConfigurationAsync(
        DpiConfiguration configuration,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(configuration);

        var stages = configuration.Stages
            .OrderBy(stage => stage.Index)
            .Select(stage => new BridgeDpiStage
            {
                Index = stage.Index,
                X = stage.X,
                Y = stage.Y,
                Enabled = true,
            })
            .ToArray();

        if (stages.Length < configuration.StageCount)
        {
            throw new ArgumentException(
                $"Expected {configuration.StageCount} DPI stages, but received {stages.Length}.",
                nameof(configuration));
        }

        var snapshot = await _client.SetDpiConfigAsync(
            configuration.StageCount,
            configuration.CurrentStage,
            stages,
            cancellationToken).ConfigureAwait(false);

        _lastSnapshot = snapshot;
        return PublishStatus(snapshot);
    }

    public async Task<DeviceStatus> SetPollingRateAsync(
        int hertz,
        CancellationToken cancellationToken = default)
    {
        var snapshot = await _client.SetPollingRateAsync(hertz, cancellationToken).ConfigureAwait(false);
        _lastSnapshot = snapshot;
        return PublishStatus(snapshot);
    }

    public Task<BridgeSnapshot> SetDemoModeAsync(
        bool enabled,
        CancellationToken cancellationToken = default) =>
        UpdateSnapshotAsync(_client.SetDemoModeAsync(enabled, cancellationToken));

    public async Task<DeviceStatus> SetPollingModeAsync(
        string mode,
        CancellationToken cancellationToken = default)
    {
        _lastState = await _client.SetPollingModeAsync(mode, cancellationToken).ConfigureAwait(false);
        var status = MapStatus(_lastState);
        StatusChanged?.Invoke(this, status);
        return status;
    }

    public async ValueTask DisposeAsync()
    {
        await _client.DisposeAsync().ConfigureAwait(false);
    }

    private async Task<BridgeSnapshot> ReadSnapshotWithRetryAsync(CancellationToken cancellationToken)
    {
        var delay = TimeSpan.FromMilliseconds(120);
        Exception? last = null;

        for (var attempt = 0; attempt < 6; attempt++)
        {
            try
            {
                var snapshot = await _client.GetSnapshotAsync(cancellationToken).ConfigureAwait(false);
                if (snapshot.Dpi is not null || snapshot.Sensor is not null)
                {
                    return snapshot;
                }
                last = new RapooBridgeException("デバイス設定がまだ準備できていません。");
            }
            catch (Exception ex) when (ex is RapooBridgeException or InvalidOperationException)
            {
                last = ex;
            }

            await Task.Delay(delay, cancellationToken).ConfigureAwait(false);
            delay = TimeSpan.FromMilliseconds(Math.Min(delay.TotalMilliseconds * 1.5, 800));
        }

        throw last ?? new RapooBridgeException("デバイス設定を読み取れませんでした。");
    }

    private DeviceStatus PublishStatus(BridgeSnapshot snapshot)
    {
        var state = _lastState ?? new BridgeState
        {
            Connected = true,
            Connection = snapshot.Model?.Connection,
            DemoMode = snapshot.DemoMode,
            DeviceName = snapshot.Model?.Name,
            MaxRateHz = snapshot.Model?.MaxRateHz ?? 4000,
            HasBattery = snapshot.Battery is not null,
            Percentage = snapshot.Battery?.Percentage ?? -1,
            Charging = snapshot.Battery?.Charging ?? false,
            MouseOnline = snapshot.Battery?.Online ?? false,
            ActiveDpi = snapshot.Dpi?.Stages?.FirstOrDefault(stage => stage.Index == snapshot.Dpi.CurrentStage)?.X ?? 0,
            PollingRateHz = snapshot.Sensor?.PollingRateHz ?? 0,
            UpdatedAt = DateTimeOffset.Now,
        };

        var status = MapStatus(state);
        StatusChanged?.Invoke(this, status);
        return status;
    }

    private DeviceStatus MapStatus(BridgeState state)
    {
        var connection = state.Connection switch
        {
            "wired" => ConnectionKind.Wired,
            "wireless_4k" => ConnectionKind.Wireless4K,
            _ => ConnectionKind.Disconnected,
        };
        if (!state.Connected)
        {
            connection = ConnectionKind.Disconnected;
        }

        var firmware = _lastSnapshot?.Firmware;
        var firmwareText = firmware is null
            ? "—"
            : string.Join(
                " / ",
                new[] { firmware.MouseString, firmware.DongleString }
                    .Where(value => !string.IsNullOrWhiteSpace(value)));

        return new DeviceStatus(
            DeviceName: state.DeviceName ?? "Rapoo VT9 Pro 4K",
            Connection: connection,
            MouseOnline: state.MouseOnline,
            BatteryPercent: state.HasBattery ? state.Percentage : -1,
            Charging: state.Charging,
            Sleeping: state.Sleeping,
            ActiveDpi: state.ActiveDpi,
            PollingRateHz: state.PollingRateHz,
            Firmware: string.IsNullOrWhiteSpace(firmwareText) ? "—" : firmwareText,
            UpdatedAt: state.UpdatedAt == default ? DateTimeOffset.Now : state.UpdatedAt,
            IsMock: false,
            Connected: state.Connected,
            DemoMode: state.DemoMode,
            Mode: state.Mode ?? "normal",
            MaxRateHz: state.MaxRateHz);
    }
}
