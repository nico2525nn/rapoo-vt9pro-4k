using RapooControl.Models;

namespace RapooControl.Services;

/// <summary>
/// Real device service backed by the Go sidecar. HID ownership and all protocol
/// work stay in Go; this class only maps the bridge JSON to the UI models.
///
/// Every state/snapshot is tied to a device epoch. A reply that started before
/// a disconnect, demo-mode switch, or device replacement is discarded instead
/// of being published against the new device.
/// </summary>
public sealed partial class RealRapooService : IRapooService
{
    private readonly RapooBridgeClient _client = new();
    private readonly SemaphoreSlim _startGate = new(1, 1);
    private readonly object _stateGate = new();
    private BridgeState? _lastState;
    private BridgeSnapshot? _lastSnapshot;
    private int _deviceEpoch;
    private int _snapshotEpoch;
    private long _snapshotSessionId;
    private long _lastStateSequence;
    private long _lastStateRevision;
    private long _activeSessionId;

    public RealRapooService()
    {
        _client.StateChanged += (_, state) =>
            PublishState(state.SessionId, state.State, state.Sequence);
        _client.Faulted += (_, fault) =>
        {
            if (fault.SessionId == _client.CurrentSessionId)
            {
                PublishFault(fault.Message, fault.Sequence);
            }
        };
    }

    public bool IsMock => false;

    public bool IsRunning => _client.IsRunning;

    public event EventHandler<DeviceStatus>? StatusChanged;

    public event EventHandler<string>? Faulted;

    public async Task StartAsync(CancellationToken cancellationToken = default)
    {
        await _startGate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            await _client.StartAsync(cancellationToken).ConfigureAwait(false);
            var sessionId = _client.CurrentSessionId;
            lock (_stateGate)
            {
                if (sessionId > _activeSessionId)
                {
                    // Reset only after the new session exists. Never move the
                    // active id backwards when a delayed start resumes.
                    _activeSessionId = sessionId;
                    _lastStateSequence = 0;
                    _lastStateRevision = 0;
                    _lastSnapshot = null;
                    _snapshotEpoch = 0;
                    _snapshotSessionId = 0;
                }
            }

            var initial = await _client.GetStateWithSequenceAsync(cancellationToken).ConfigureAwait(false);
            PublishState(initial.SessionId, initial.State, initial.Sequence);
            var epoch = CurrentEpoch;

            try
            {
                if (Environment.GetEnvironmentVariable("RAPOO_DEMO") == "1")
                {
                    await _client.SetDemoModeAsync(true, cancellationToken).ConfigureAwait(false);
                }

                var snapshot = await ReadSnapshotWithRetryAsync(cancellationToken).ConfigureAwait(false);
                PublishSnapshot(snapshot, epoch, sessionId);
            }
            catch (Exception ex) when (ex is RapooBridgeException or InvalidOperationException)
            {
                // The mouse may be asleep or disconnected. The shell still renders.
            }
        }
        finally
        {
            _startGate.Release();
        }
    }

    public Task StopAsync() => _client.StopAsync();

    public async Task<DeviceStatus> GetStatusAsync(CancellationToken cancellationToken = default)
    {
        var state = await _client.GetStateWithSequenceAsync(cancellationToken).ConfigureAwait(false);
        return PublishState(state.SessionId, state.State, state.Sequence);
    }

    public async Task<DpiConfiguration> GetDpiConfigurationAsync(CancellationToken cancellationToken = default)
    {
        var epoch = CurrentEpoch;
        var sessionId = CurrentSession();
        var snapshot = CachedSnapshot ??
            await ReadSnapshotWithRetryAsync(cancellationToken).ConfigureAwait(false);
        EnsureEpoch(epoch);
        if (sessionId != CurrentSession() || !_client.IsSessionCurrent(sessionId))
        {
            throw new InvalidOperationException(
                LocalizationService.Instance.T("接続先が変わりました。設定を再読み込みしてください。"));
        }
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
        var epoch = CurrentEpoch;
        var sessionId = CurrentSession();

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

        return PublishSnapshot(snapshot, epoch, sessionId);
    }

    public async Task<DeviceStatus> SetPollingRateAsync(
        int hertz,
        CancellationToken cancellationToken = default)
    {
        var epoch = CurrentEpoch;
        var sessionId = CurrentSession();
        var snapshot = await _client.SetPollingRateAsync(hertz, cancellationToken).ConfigureAwait(false);
        return PublishSnapshot(snapshot, epoch, sessionId);
    }

    public async Task<BridgeSnapshot> SetDemoModeAsync(
        bool enabled,
        CancellationToken cancellationToken = default)
    {
        var epoch = CurrentEpoch;
        var sessionId = CurrentSession();
        var snapshot = await _client.SetDemoModeAsync(enabled, cancellationToken).ConfigureAwait(false);
        try
        {
            PublishSnapshot(snapshot, epoch, sessionId);
        }
        catch (InvalidOperationException)
        {
            // Demo mode legitimately changes the display name. Accept the reply
            // only for the same product in the new demo state; the identity and
            // epoch check happen in one critical section.
            PublishSnapshotForDemoChange(snapshot, sessionId);
        }
        return snapshot;
    }

    public async Task<DeviceStatus> SetPollingModeAsync(
        string mode,
        CancellationToken cancellationToken = default)
    {
        var state = await _client.SetPollingModeWithSequenceAsync(mode, cancellationToken).ConfigureAwait(false);
        return PublishState(state.SessionId, state.State, state.Sequence);
    }

    public async ValueTask DisposeAsync() =>
        await _client.DisposeAsync().ConfigureAwait(false);

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

            if (attempt < 5)
            {
                await Task.Delay(delay, cancellationToken).ConfigureAwait(false);
                delay = TimeSpan.FromMilliseconds(Math.Min(delay.TotalMilliseconds * 1.5, 800));
            }
        }

        throw last ?? new RapooBridgeException("デバイス設定を読み取れませんでした。");
    }

    private DeviceStatus PublishState(long sessionId, BridgeState state, long sequence)
    {
        lock (_stateGate)
        {
            if (sessionId != _activeSessionId ||
                !_client.IsSessionCurrent(sessionId))
            {
                return MapStatus(_lastState ?? state, _lastSnapshot);
            }
            if (state.Revision > 0)
            {
                if (state.Revision < _lastStateRevision)
                {
                    return MapStatus(_lastState ?? state, _lastSnapshot);
                }
                if (state.Revision == _lastStateRevision &&
                    sequence > 0 && sequence <= _lastStateSequence)
                {
                    return MapStatus(_lastState ?? state, _lastSnapshot);
                }
                _lastStateRevision = state.Revision;
            }
            else if (sequence > 0 && sequence <= _lastStateSequence)
            {
                return MapStatus(_lastState ?? state, _lastSnapshot);
            }

            if (sequence > 0)
            {
                _lastStateSequence = sequence;
            }
            var status = ApplyStateLocked(state);
            StatusChanged?.Invoke(this, status);
            return status;
        }
    }

    private DeviceStatus ApplyStateLocked(BridgeState state)
    {
        if (_lastState is { } previous && DeviceChanged(previous, state))
        {
            _deviceEpoch++;
            _lastSnapshot = null;
            _snapshotEpoch = 0;
            _snapshotSessionId = 0;
        }
        _lastState = state;
        return MapStatus(state, _lastSnapshot);
    }

    private DeviceStatus PublishSnapshot(BridgeSnapshot snapshot, int epoch, long sessionId)
    {
        lock (_stateGate)
        {
            if (sessionId != _activeSessionId || !_client.IsSessionCurrent(sessionId))
            {
                throw new InvalidOperationException(
                    LocalizationService.Instance.T("接続先が変わりました。設定を再読み込みしてください。"));
            }
            return PublishSnapshotLocked(snapshot, epoch, sessionId);
        }
    }

    private DeviceStatus PublishSnapshotLocked(BridgeSnapshot snapshot, int epoch, long sessionId)
    {
        EnsureEpochLocked(epoch);
        _lastSnapshot = snapshot;
        _snapshotEpoch = epoch;
        _snapshotSessionId = sessionId;
        var state = _lastState ?? CreateState(snapshot);
        _lastState = state;
        var status = MapStatus(state, snapshot);
        StatusChanged?.Invoke(this, status);
        return status;
    }

    private DeviceStatus PublishSnapshotForDemoChange(BridgeSnapshot snapshot, long sessionId)
    {
        lock (_stateGate)
        {
            if (sessionId != _activeSessionId ||
                !_client.IsSessionCurrent(sessionId) ||
                !MatchesCurrentDeviceLocked(snapshot))
            {
                throw new InvalidOperationException(
                    LocalizationService.Instance.T("接続先が変わりました。設定を再読み込みしてください。"));
            }
            return PublishSnapshotLocked(snapshot, _deviceEpoch, sessionId);
        }
    }

    private void PublishFault(string message, long sequence)
    {
        DeviceStatus status;
        lock (_stateGate)
        {
            if (sequence > 0 && sequence < _lastStateSequence)
            {
                return;
            }
            if (sequence > 0)
            {
                _lastStateSequence = sequence;
            }
            _deviceEpoch++;
            _lastSnapshot = null;
            _snapshotEpoch = 0;
            _snapshotSessionId = 0;
            _lastState = new BridgeState
            {
                Connected = false,
                LastError = message,
                UpdatedAt = DateTimeOffset.Now,
            };
            status = MapStatus(_lastState, null);
            StatusChanged?.Invoke(this, status);
        }

        Faulted?.Invoke(this, message);
    }

    private long CurrentSession()
    {
        lock (_stateGate)
        {
            return _activeSessionId;
        }
    }

    private int CurrentEpoch
    {
        get
        {
            lock (_stateGate)
            {
                return _deviceEpoch;
            }
        }
    }

    private void EnsureEpoch(int epoch)
    {
        lock (_stateGate)
        {
            EnsureEpochLocked(epoch);
        }
    }

    private void EnsureEpochLocked(int epoch)
    {
        if (epoch != _deviceEpoch)
        {
            throw new InvalidOperationException(
                LocalizationService.Instance.T("接続先が変わりました。設定を再読み込みしてください。"));
        }
    }

    private bool MatchesCurrentDeviceLocked(BridgeSnapshot snapshot)
    {
        if (_lastState is not { } state || state.DemoMode != snapshot.DemoMode)
        {
            return false;
        }

        var productId = snapshot.Model?.ProductId ?? 0;
        return productId > 0
            ? state.ProductId == productId
            : state.DeviceName == snapshot.Model?.Name;
    }

    private static bool DeviceChanged(BridgeState previous, BridgeState current) =>
        previous.Connected != current.Connected ||
        previous.DemoMode != current.DemoMode ||
        previous.Connection != current.Connection ||
        previous.ProductId != current.ProductId ||
        previous.DeviceName != current.DeviceName;

    private static BridgeState CreateState(BridgeSnapshot snapshot) =>
        new()
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
            ActiveDpi = snapshot.Dpi?.Stages?
                .FirstOrDefault(stage => stage.Index == snapshot.Dpi.CurrentStage)?.X ?? 0,
            PollingRateHz = snapshot.Sensor?.PollingRateHz ?? 0,
            UpdatedAt = DateTimeOffset.Now,
        };

    private static DeviceStatus MapStatus(BridgeState state, BridgeSnapshot? snapshot)
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

        var firmware = snapshot?.Firmware;
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
            MaxRateHz: state.MaxRateHz,
            PermissionDenied: state.PermissionDenied,
            LastError: state.LastError);
    }
}
