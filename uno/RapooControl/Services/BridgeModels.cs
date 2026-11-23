using System.Text.Json.Serialization;

namespace RapooControl.Services;

// Wire models for the Go sidecar. Property names mirror the Go JSON tags so the
// protocol stays readable from either side.

public sealed class BridgeState
{
    [JsonPropertyName("connected")] public bool Connected { get; set; }
    [JsonPropertyName("demoMode")] public bool DemoMode { get; set; }
    [JsonPropertyName("deviceName")] public string? DeviceName { get; set; }
    [JsonPropertyName("connection")] public string? Connection { get; set; }
    [JsonPropertyName("productId")] public int ProductId { get; set; }
    [JsonPropertyName("maxRateHz")] public int MaxRateHz { get; set; }
    [JsonPropertyName("hasBattery")] public bool HasBattery { get; set; }
    [JsonPropertyName("percentage")] public int Percentage { get; set; }
    [JsonPropertyName("charging")] public bool Charging { get; set; }
    [JsonPropertyName("mouseOnline")] public bool MouseOnline { get; set; }
    [JsonPropertyName("sleeping")] public bool Sleeping { get; set; }
    [JsonPropertyName("activeDpi")] public int ActiveDpi { get; set; }
    [JsonPropertyName("activeStage")] public int ActiveStage { get; set; }
    [JsonPropertyName("stageCount")] public int StageCount { get; set; }
    [JsonPropertyName("pollingRateHz")] public int PollingRateHz { get; set; }
    [JsonPropertyName("mode")] public string? Mode { get; set; }
    [JsonPropertyName("permissionDenied")] public bool PermissionDenied { get; set; }
    [JsonPropertyName("platform")] public string? Platform { get; set; }
    [JsonPropertyName("updatedAt")] public DateTimeOffset UpdatedAt { get; set; }
    [JsonPropertyName("lastError")] public string? LastError { get; set; }
}

public sealed class BridgeSnapshot
{
    [JsonPropertyName("model")] public BridgeModel? Model { get; set; }
    [JsonPropertyName("battery")] public BridgeBattery? Battery { get; set; }
    [JsonPropertyName("firmware")] public BridgeFirmware? Firmware { get; set; }
    [JsonPropertyName("dpi")] public BridgeDpiConfig? Dpi { get; set; }
    [JsonPropertyName("sensor")] public BridgeSensor? Sensor { get; set; }
    [JsonPropertyName("buttons")] public List<BridgeButtonMapping>? Buttons { get; set; }
    [JsonPropertyName("demoMode")] public bool DemoMode { get; set; }
}

public sealed class BridgeModel
{
    [JsonPropertyName("name")] public string? Name { get; set; }
    [JsonPropertyName("connection")] public string? Connection { get; set; }
    [JsonPropertyName("productId")] public int ProductId { get; set; }
    [JsonPropertyName("protocolGen")] public int ProtocolGen { get; set; }
    [JsonPropertyName("maxRateHz")] public int MaxRateHz { get; set; }
}

public sealed class BridgeBattery
{
    [JsonPropertyName("percentage")] public int Percentage { get; set; }
    [JsonPropertyName("charging")] public bool Charging { get; set; }
    [JsonPropertyName("online")] public bool Online { get; set; }
    [JsonPropertyName("sleeping")] public bool Sleeping { get; set; }
    [JsonPropertyName("updatedAt")] public DateTimeOffset UpdatedAt { get; set; }
}

public sealed class BridgeFirmware
{
    [JsonPropertyName("mouseString")] public string? MouseString { get; set; }
    [JsonPropertyName("dongleString")] public string? DongleString { get; set; }
}

public sealed class BridgeDpiConfig
{
    [JsonPropertyName("currentStage")] public int CurrentStage { get; set; }
    [JsonPropertyName("stageCount")] public int StageCount { get; set; }
    [JsonPropertyName("independentXY")] public bool IndependentXY { get; set; }
    [JsonPropertyName("stages")] public List<BridgeDpiStage>? Stages { get; set; }
    [JsonPropertyName("allStages")] public List<BridgeDpiStage>? AllStages { get; set; }
}

public sealed class BridgeDpiStage
{
    [JsonPropertyName("index")] public int Index { get; set; }
    [JsonPropertyName("x")] public int X { get; set; }
    [JsonPropertyName("y")] public int Y { get; set; }
    [JsonPropertyName("enabled")] public bool Enabled { get; set; } = true;
}

public sealed class BridgeSensor
{
    [JsonPropertyName("pollingRateHz")] public int PollingRateHz { get; set; }
    [JsonPropertyName("liftOffLevel")] public int LiftOffLevel { get; set; }
    [JsonPropertyName("liftOffDistanceMm")] public double LiftOffDistanceMm { get; set; }
    [JsonPropertyName("motionSync")] public bool MotionSync { get; set; }
    [JsonPropertyName("pressDebounceStep")] public int PressDebounceStep { get; set; }
    [JsonPropertyName("pressDebounceMs")] public int PressDebounceMs { get; set; }
    [JsonPropertyName("pressDebounceLabel")] public string? PressDebounceLabel { get; set; }
    [JsonPropertyName("releaseDebounceStep")] public int ReleaseDebounceStep { get; set; }
    [JsonPropertyName("releaseDebounceMs")] public int ReleaseDebounceMs { get; set; }
    [JsonPropertyName("releaseDebounceLabel")] public string? ReleaseDebounceLabel { get; set; }
    [JsonPropertyName("sleepMinutes")] public int SleepMinutes { get; set; }
    [JsonPropertyName("rippleControl")] public bool RippleControl { get; set; }
    [JsonPropertyName("linearCorrection")] public bool LinearCorrection { get; set; }
    [JsonPropertyName("sensorAngleDeg")] public int SensorAngleDeg { get; set; }
    [JsonPropertyName("glassMode")] public bool GlassMode { get; set; }
    [JsonPropertyName("ledLowPowerMode")] public int LedLowPowerMode { get; set; }
}

public sealed class BridgeButtonMapping
{
    [JsonPropertyName("slot")] public BridgeButtonSlot? Slot { get; set; }
    [JsonPropertyName("action")] public BridgeButtonAction? Action { get; set; }
}

public sealed class BridgeButtonSlot
{
    [JsonPropertyName("id")] public int Id { get; set; }
    [JsonPropertyName("nameJa")] public string? NameJa { get; set; }
    [JsonPropertyName("nameEn")] public string? NameEn { get; set; }
    [JsonPropertyName("view")] public string? View { get; set; }
    [JsonPropertyName("defaultAction")] public string? DefaultAction { get; set; }
    [JsonPropertyName("clickable")] public bool Clickable { get; set; }
}

public sealed class BridgeButtonAction
{
    [JsonPropertyName("type")] public string? Type { get; set; }
    [JsonPropertyName("labelJa")] public string? LabelJa { get; set; }
    [JsonPropertyName("category")] public string? Category { get; set; }
    [JsonPropertyName("keyCode")] public int KeyCode { get; set; }
    [JsonPropertyName("keyName")] public string? KeyName { get; set; }
    [JsonPropertyName("modifiers")] public BridgeKeyModifiers? Modifiers { get; set; }
}

public sealed class BridgeKeyModifiers
{
    [JsonPropertyName("ctrl")] public bool Ctrl { get; set; }
    [JsonPropertyName("shift")] public bool Shift { get; set; }
    [JsonPropertyName("alt")] public bool Alt { get; set; }
    [JsonPropertyName("meta")] public bool Meta { get; set; }
}

public sealed class BridgeKey
{
    [JsonPropertyName("code")] public int Code { get; set; }
    [JsonPropertyName("name")] public string Name { get; set; } = string.Empty;
}

public sealed class BridgeDebounceStep
{
    [JsonPropertyName("index")] public int Index { get; set; }
    [JsonPropertyName("label")] public string Label { get; set; } = string.Empty;
}

public sealed class BridgeCatalog
{
    [JsonPropertyName("platform")] public string? Platform { get; set; }
    [JsonPropertyName("pollingRates")] public List<int>? PollingRates { get; set; }
    [JsonPropertyName("buttons")] public List<BridgeButtonSlot>? Buttons { get; set; }
    [JsonPropertyName("supportedKeys")] public List<BridgeKey>? SupportedKeys { get; set; }
    [JsonPropertyName("debounceSteps")] public List<BridgeDebounceStep>? DebounceSteps { get; set; }
    [JsonPropertyName("linuxUdevRule")] public string? LinuxUdevRule { get; set; }
    [JsonPropertyName("linuxUdevInstallCmd")] public string? LinuxUdevInstallCmd { get; set; }
}

public sealed class BridgePreset
{
    [JsonPropertyName("id")] public string Id { get; set; } = string.Empty;
    [JsonPropertyName("name")] public string Name { get; set; } = string.Empty;
    [JsonPropertyName("description")] public string? Description { get; set; }
    [JsonPropertyName("builtIn")] public bool BuiltIn { get; set; }
    [JsonPropertyName("createdAt")] public DateTimeOffset CreatedAt { get; set; }
    [JsonPropertyName("updatedAt")] public DateTimeOffset UpdatedAt { get; set; }
    [JsonPropertyName("pollingHz")] public int PollingHz { get; set; }
}
