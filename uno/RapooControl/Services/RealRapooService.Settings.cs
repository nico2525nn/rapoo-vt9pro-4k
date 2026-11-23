namespace RapooControl.Services;

public sealed partial class RealRapooService
{
    internal BridgeSnapshot? CachedSnapshot
    {
        get
        {
            lock (_stateGate)
            {
                return _snapshotEpoch == _deviceEpoch ? _lastSnapshot : null;
            }
        }
    }

    public Task<BridgeState> SetLanguageAsync(string language) => _client.SetLanguageAsync(language);

    public Task<BridgeCatalog> GetCatalogAsync() => _client.GetCatalogAsync();

    public Task<BridgeSnapshot> RefreshSnapshotAsync() =>
        UpdateSnapshotAsync(() => _client.GetSnapshotAsync());

    public Task<BridgeSnapshot> ApplySensorAsync(IReadOnlyDictionary<string, object> patch) =>
        UpdateSnapshotAsync(() => _client.ApplySensorAsync(patch));

    public Task<BridgeSnapshot> SetButtonAsync(int buttonId, BridgeButtonAction action) =>
        UpdateSnapshotAsync(() => _client.SetButtonAsync(buttonId, action));

    public Task<BridgeSnapshot> ResetButtonsAsync() =>
        UpdateSnapshotAsync(() => _client.ResetButtonsAsync());

    public Task<BridgeSnapshot> FactoryResetAsync() =>
        UpdateSnapshotAsync(() => _client.FactoryResetAsync());

    public Task<List<BridgePreset>> ListPresetsAsync() => _client.ListPresetsAsync();

    public Task<BridgePreset> SavePresetAsync(string name, string description) =>
        _client.SavePresetAsync(string.Empty, name, description);

    public Task<BridgeSnapshot> ApplyPresetAsync(string id) =>
        UpdateSnapshotAsync(() => _client.ApplyPresetAsync(id));

    public Task DeletePresetAsync(string id) => _client.DeletePresetAsync(id);

    public Task<string> OpenFirmwareUpdaterAsync(string? path = null) => _client.OpenFirmwareUpdaterAsync(path);

    private async Task<BridgeSnapshot> UpdateSnapshotAsync(Func<Task<BridgeSnapshot>> request)
    {
        var epoch = CurrentEpoch;
        var snapshot = await request().ConfigureAwait(false);
        PublishSnapshot(snapshot, epoch);
        return snapshot;
    }
}
