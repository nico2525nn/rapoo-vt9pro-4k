using System.Text.Json.Serialization;

namespace RapooControl.Services;

/// <summary>
/// Source-generated JSON metadata for the sidecar protocol. Trimming removes
/// reflection metadata, so every wire type must be registered here instead of
/// relying on <c>JsonSerializer</c> reflection.
/// </summary>
[JsonSourceGenerationOptions(PropertyNameCaseInsensitive = true)]
[JsonSerializable(typeof(BridgeState))]
[JsonSerializable(typeof(BridgeSnapshot))]
[JsonSerializable(typeof(BridgeCatalog))]
[JsonSerializable(typeof(BridgePreset))]
[JsonSerializable(typeof(List<BridgePreset>))]
[JsonSerializable(typeof(BridgeButtonAction))]
[JsonSerializable(typeof(string))]
internal sealed partial class BridgeJsonContext : JsonSerializerContext;
