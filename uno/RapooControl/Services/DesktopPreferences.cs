using System.Text.Json;

namespace RapooControl.Services;

/// <summary>
/// Cross-platform shell preferences. Auto-start is always read back from the
/// operating system so the UI cannot report a state that was never applied.
/// </summary>
public sealed class DesktopPreferences
{
    private static readonly JsonSerializerOptions JsonOptions = new() { WriteIndented = true };

    private readonly string _path;

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

    /// <summary>Raised after a preference was changed or reloaded.</summary>
    public event EventHandler? Changed;

    public void Load()
    {
        try
        {
            if (File.Exists(_path))
            {
                using var document = JsonDocument.Parse(File.ReadAllText(_path));
                var root = document.RootElement;
                CloseToTray = root.TryGetProperty("closeToTray", out var close) && close.GetBoolean();
                StartHidden = root.TryGetProperty("startHidden", out var hidden) && hidden.GetBoolean();
            }
        }
        catch
        {
            // Use defaults.
        }

        AutoStart = AutoStartService.IsEnabled();
        if (AutoStart)
        {
            StartHidden = AutoStartService.IsStartHidden();
        }
        Changed?.Invoke(this, EventArgs.Empty);
    }

    public bool SetCloseToTray(bool enabled)
    {
        var previous = CloseToTray;
        CloseToTray = enabled;
        if (Save())
        {
            Changed?.Invoke(this, EventArgs.Empty);
            return true;
        }
        CloseToTray = previous;
        return false;
    }

    public bool SetAutoStart(bool enabled)
    {
        var previousAuto = AutoStart;
        var previousHidden = StartHidden;
        if (!AutoStartService.SetEnabled(enabled, StartHidden))
        {
            AutoStart = AutoStartService.IsEnabled();
            StartHidden = AutoStart ? AutoStartService.IsStartHidden() : previousHidden;
            return false;
        }

        AutoStart = AutoStartService.IsEnabled();
        if (AutoStart)
        {
            StartHidden = AutoStartService.IsStartHidden();
        }
        if (Save())
        {
            Changed?.Invoke(this, EventArgs.Empty);
            return true;
        }

        AutoStart = previousAuto;
        StartHidden = previousHidden;
        return false;
    }

    public bool SetStartHidden(bool enabled)
    {
        if (!AutoStartService.SupportsStartHidden)
        {
            return false;
        }

        var previous = StartHidden;
        if (AutoStart && !AutoStartService.SetEnabled(true, enabled))
        {
            StartHidden = AutoStartService.IsStartHidden();
            return false;
        }

        StartHidden = enabled;
        if (Save())
        {
            Changed?.Invoke(this, EventArgs.Empty);
            return true;
        }

        StartHidden = previous;
        return false;
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
            }, JsonOptions));
            File.Move(temporary, _path, overwrite: true);
            return true;
        }
        catch
        {
            return false;
        }
    }
}
