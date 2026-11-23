using System.Text.Json;

namespace RapooControl.Services;

/// <summary>
/// Cross-platform shell preferences. Platform-specific auto-start is applied by
/// the desktop head; this class owns the persisted values.
/// </summary>
public sealed class DesktopPreferences
{
    private readonly string _path;
    private bool _loaded;

    private DesktopPreferences()
    {
        var dir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "rapoo-vt9pro-4k");
        Directory.CreateDirectory(dir);
        _path = Path.Combine(dir, "uno-prefs.json");
    }

    public static DesktopPreferences Current { get; } = new();

    public bool CloseToTray { get; private set; }

    public bool AutoStart { get; private set; }

    public bool StartHidden { get; private set; }

    public void Load()
    {
        if (_loaded)
        {
            return;
        }
        _loaded = true;
        AutoStart = AutoStartService.IsEnabled();
        try
        {
            if (File.Exists(_path))
            {
                using var document = JsonDocument.Parse(File.ReadAllText(_path));
                var root = document.RootElement;
                CloseToTray = root.TryGetProperty("closeToTray", out var close) && close.GetBoolean();
                AutoStart = root.TryGetProperty("autoStart", out var auto) && auto.GetBoolean();
                StartHidden = root.TryGetProperty("startHidden", out var hidden) && hidden.GetBoolean();
            }
        }
        catch
        {
            // Use defaults.
        }
    }

    public void SetCloseToTray(bool enabled)
    {
        CloseToTray = enabled;
        Save();
    }

    public bool SetAutoStart(bool enabled)
    {
        AutoStart = enabled;
        var applied = AutoStartService.SetEnabled(enabled, StartHidden);
        Save();
        return applied;
    }

    public bool SetStartHidden(bool enabled)
    {
        StartHidden = enabled;
        var applied = !AutoStart || AutoStartService.SetEnabled(true, enabled);
        Save();
        return applied;
    }

    private bool Save()
    {
        try
        {
            var temporary = _path + ".tmp";
            File.WriteAllText(temporary, JsonSerializer.Serialize(new
            {
                closeToTray = CloseToTray,
                autoStart = AutoStart,
                startHidden = StartHidden,
            }, new JsonSerializerOptions { WriteIndented = true }));
            File.Move(temporary, _path, overwrite: true);
            return true;
        }
        catch
        {
            return false;
        }
    }
}
