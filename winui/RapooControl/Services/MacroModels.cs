using System.Text.Json.Serialization;

namespace RapooControl.Services;

/// <summary>A saved macro: one global shortcut that plays an ordered key sequence.</summary>
public sealed class MacroDefinition
{
    public string Id { get; set; } = Guid.NewGuid().ToString("N");
    public string Name { get; set; } = string.Empty;
    public bool Enabled { get; set; } = true;
    public MacroTrigger Trigger { get; set; } = new();
    public List<MacroStep> Steps { get; set; } = [];
    public DateTimeOffset CreatedAt { get; set; } = DateTimeOffset.Now;
    public DateTimeOffset UpdatedAt { get; set; } = DateTimeOffset.Now;
}

/// <summary>The global shortcut that starts a macro. VirtualKey is a Windows VK code.</summary>
public sealed class MacroTrigger
{
    public bool Ctrl { get; set; }
    public bool Shift { get; set; }
    public bool Alt { get; set; }
    public bool Win { get; set; }
    public int VirtualKey { get; set; }
    public string KeyName { get; set; } = string.Empty;

    public string Display => BuildDisplay(Ctrl, Shift, Alt, Win, KeyName, VirtualKey);

    internal static string BuildDisplay(bool ctrl, bool shift, bool alt, bool win, string? name, int vk)
    {
        var parts = new List<string>(5);
        if (ctrl) parts.Add("Ctrl");
        if (shift) parts.Add("Shift");
        if (alt) parts.Add("Alt");
        if (win) parts.Add("Win");
        parts.Add(string.IsNullOrWhiteSpace(name) ? $"VK {vk:X2}" : name);
        return string.Join(" + ", parts);
    }
}

/// <summary>One step in a macro. Kind is "key" or "delay".</summary>
public sealed class MacroStep
{
    public string Kind { get; set; } = "key";
    public bool Ctrl { get; set; }
    public bool Shift { get; set; }
    public bool Alt { get; set; }
    public bool Win { get; set; }
    public int VirtualKey { get; set; }
    public string KeyName { get; set; } = string.Empty;
    public int DelayMs { get; set; } = 30;

    public string Display => Kind == "delay"
        ? $"{Math.Clamp(DelayMs, 0, 10_000)} ms 待機"
        : MacroTrigger.BuildDisplay(Ctrl, Shift, Alt, Win, KeyName, VirtualKey);
}

public sealed class MacroRegistrationError
{
    public required string MacroId { get; init; }
    public required string Message { get; init; }
}

internal sealed class MacroFile
{
    [JsonPropertyName("macros")] public List<MacroDefinition> Macros { get; set; } = [];
}
