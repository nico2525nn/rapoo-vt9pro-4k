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
        UpdateSnapshotAsync(_ => _client.GetSnapshotAsync());

    public Task<BridgeSnapshot> ApplySensorAsync(IReadOnlyDictionary<string, object> patch) =>
        UpdateSnapshotAsync(session => _client.ApplySensorAsync(patch, default, session));

    public Task<BridgeSnapshot> SetButtonAsync(int buttonId, BridgeButtonAction action) =>
        UpdateSnapshotAsync(session => _client.SetButtonAsync(buttonId, action, default, session));

    public Task<BridgeSnapshot> ResetButtonsAsync() =>
        UpdateSnapshotAsync(session => _client.ResetButtonsAsync(default, session));

    public Task<BridgeSnapshot> FactoryResetAsync() =>
        UpdateSnapshotAsync(session => _client.FactoryResetAsync(default, session));

    public Task<List<BridgePreset>> ListPresetsAsync() => _client.ListPresetsAsync();

    public Task<BridgePreset> SavePresetAsync(string name, string description) =>
        _client.SavePresetAsync(string.Empty, name, description);

    public Task<BridgeSnapshot> ApplyPresetAsync(string id) =>
        UpdateSnapshotAsync(session => _client.ApplyPresetAsync(id, default, session));

    public Task DeletePresetAsync(string id) => _client.DeletePresetAsync(id);

    public Task<string> OpenFirmwareUpdaterAsync(string? path = null) => _client.OpenFirmwareUpdaterAsync(path);

    private async Task<BridgeSnapshot> UpdateSnapshotAsync(Func<long, Task<BridgeSnapshot>> request)
    {
        var epoch = CurrentEpoch;
        var sessionId = CurrentSession();
        var snapshot = await request(sessionId).ConfigureAwait(false);
        PublishSnapshot(snapshot, epoch, sessionId);
        return snapshot;
    }
}
