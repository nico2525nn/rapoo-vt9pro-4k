namespace RapooControl.Services;

public sealed partial class RealRapooService
{
    internal BridgeSnapshot? CachedSnapshot
    {
        get
        {
            lock (_stateGate)
            {
                return _snapshotEpoch == _deviceEpoch &&
                       _snapshotSessionId == _activeSessionId &&
                       _lastState is { Connected: true } &&
                       _client.IsSessionCurrent(_activeSessionId) &&
                       (_lastSnapshot is not { Generation: > 0 } snapshot ||
                        _lastState is not { Generation: > 0 } state ||
                        snapshot.Generation == state.Generation)
                    ? _lastSnapshot
                    : null;
            }
        }
    }

    public Task<BridgeState> SetLanguageAsync(string language) => _client.SetLanguageAsync(language);

    public Task<BridgeCatalog> GetCatalogAsync() => _client.GetCatalogAsync();

    public Task<BridgeSnapshot> RefreshSnapshotAsync() =>
        UpdateSnapshotAsync((_, _) => _client.RefreshSnapshotAsync());

    public Task<BridgeSnapshot> ApplySensorAsync(
        IReadOnlyDictionary<string, object> patch,
        long expectedGeneration = 0,
        long expectedSession = 0) =>
        UpdateSnapshotAsync((session, generation) =>
            _client.ApplySensorAsync(
                patch,
                default,
                PinnedSession(expectedSession, session),
                PinnedGeneration(expectedGeneration, generation)));

    public Task<BridgeSnapshot> SetButtonAsync(
        int buttonId,
        BridgeButtonAction action,
        long expectedGeneration = 0,
        long expectedSession = 0) =>
        UpdateSnapshotAsync((session, generation) =>
            _client.SetButtonAsync(
                buttonId,
                action,
                default,
                PinnedSession(expectedSession, session),
                PinnedGeneration(expectedGeneration, generation)));

    public Task<BridgeSnapshot> ResetButtonsAsync(
        long expectedGeneration = 0,
        long expectedSession = 0) =>
        UpdateSnapshotAsync((session, generation) =>
            _client.ResetButtonsAsync(
                default,
                PinnedSession(expectedSession, session),
                PinnedGeneration(expectedGeneration, generation)));

    public Task<BridgeSnapshot> FactoryResetAsync() => FactoryResetAsync(0, 0);

    public Task<BridgeSnapshot> FactoryResetAsync(
        long expectedGeneration,
        long expectedSession = 0) =>
        UpdateSnapshotAsync((session, generation) =>
            _client.FactoryResetAsync(
                default,
                PinnedSession(expectedSession, session),
                PinnedGeneration(expectedGeneration, generation)));

    public Task<List<BridgePreset>> ListPresetsAsync() => _client.ListPresetsAsync();

    public Task<BridgePreset> SavePresetAsync(string name, string description) =>
        _client.SavePresetAsync(string.Empty, name, description);

    public Task<BridgeSnapshot> ApplyPresetAsync(
        string id,
        long expectedGeneration = 0,
        long expectedSession = 0) =>
        UpdateSnapshotAsync((session, generation) =>
            _client.ApplyPresetAsync(
                id,
                default,
                PinnedSession(expectedSession, session),
                PinnedGeneration(expectedGeneration, generation)));

    public Task DeletePresetAsync(string id) => _client.DeletePresetAsync(id);

    public Task<string> OpenFirmwareUpdaterAsync(string? path = null) => _client.OpenFirmwareUpdaterAsync(path);

    private static long PinnedGeneration(long expectedGeneration, long currentGeneration) =>
        expectedGeneration != 0 ? expectedGeneration : currentGeneration;

    private static long PinnedSession(long expectedSession, long currentSession) =>
        expectedSession != 0 ? expectedSession : currentSession;


    private async Task<BridgeSnapshot> UpdateSnapshotAsync(
        Func<long, long, Task<BridgeSnapshot>> request)
    {
        var epoch = CurrentEpoch;
        var sessionId = CurrentSession();
        var generation = CurrentGeneration();
        var snapshot = await request(sessionId, generation).ConfigureAwait(false);
        PublishSnapshot(snapshot, epoch, sessionId);
        return snapshot;
    }
}
