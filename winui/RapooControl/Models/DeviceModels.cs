namespace RapooControl.Models;

/// <summary>Hardware limits shared by the UI and any service implementation.</summary>
public static class RapooLimits
{
    public const int MinDpi = 50;
    public const int MaxDpi = 26000;
    public const int DpiStep = 50;
    public const int MaxDpiStages = 7;

    /// <summary>Polling rates accepted by the VT9 Pro 4K family.</summary>
    public static readonly int[] PollingRates = [125, 250, 500, 1000, 2000, 4000];
}

/// <summary>How the mouse is currently attached.</summary>
public enum ConnectionKind
{
    Disconnected,
    Wired,
    Wireless4K,
}

/// <summary>
/// Immutable device summary rendered by the shell header and the overview page.
/// This intentionally mirrors the fields the Wails UI already exposes.
/// </summary>
public sealed record DeviceStatus(
    string DeviceName,
    ConnectionKind Connection,
    bool MouseOnline,
    int BatteryPercent,
    bool Charging,
    int ActiveDpi,
    int PollingRateHz,
    string Firmware,
    DateTimeOffset UpdatedAt,
    bool IsMock = true,
    bool Connected = false,
    bool DemoMode = false,
    string Mode = "normal",
    int MaxRateHz = 4000)
{
    public string ConnectionLabel => !MouseOnline
        ? "スリープ中"
        : Connection switch
        {
            ConnectionKind.Wired => "USB-C 有線",
            ConnectionKind.Wireless4K => "4K ワイヤレス",
            _ => "未接続",
        };

    public string BatteryText => BatteryPercent < 0 ? "--%" : $"{BatteryPercent}%";

    public string ChargingLabel => !MouseOnline
        ? "オフライン"
        : Charging
            ? "充電中"
            : "バッテリー駆動";

    public string ActiveDpiText => ActiveDpi > 0 ? $"{ActiveDpi:N0} DPI" : "—";

    public string PollingRateText => PollingRateHz > 0 ? $"{PollingRateHz:N0} Hz" : "—";
}

/// <summary>One onboard DPI stage (1-based, matching the Rapoo protocol).</summary>
public sealed record DpiStage(int Index, int X, int Y)
{
    public string Label => $"Stage {Index}";
}

/// <summary>Full DPI configuration: 1..7 stages plus the active stage.</summary>
public sealed record DpiConfiguration(int StageCount, int CurrentStage, IReadOnlyList<DpiStage> Stages);
