using System.Text.Json;
using System.Text.Json.Nodes;

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
        : this(AppPaths.DataDirectory)
    {
    }

    internal DesktopPreferences(string directory)
    {
        Directory.CreateDirectory(directory);
        _path = Path.Combine(directory, "uno-prefs.json");
    }

    public static DesktopPreferences Current { get; } = new();

    public bool CloseToTray { get; private set; }

    public bool AutoStart { get; private set; }

    public bool StartHidden { get; private set; }

    /// <summary>Raised after a preference was changed or reloaded.</summary>
    public event EventHandler? Changed;

    /// <summary>
    /// Imports the WinUI tray opt-in once. Version 1 defaulted close-to-tray
    /// to true, so only an explicit version 2+ value is imported.
    /// </summary>
    public void MigrateLegacy()
    {
        if (File.Exists(_path))
        {
            return;
        }

        try
        {
            var legacyPath = Path.Combine(Path.GetDirectoryName(_path)!, "winui-tray-prefs.json");
            if (!File.Exists(legacyPath))
            {
                return;
            }

            using var document = JsonDocument.Parse(File.ReadAllText(legacyPath));
            var root = document.RootElement;
            var version = root.TryGetProperty("version", out var versionElement)
                ? versionElement.GetInt32()
                : 0;
            if (version >= 2 &&
                root.TryGetProperty("closeToTray", out var closeElement))
            {
                CloseToTray = closeElement.GetBoolean();
            }
            if (root.TryGetProperty("startHidden", out var hiddenElement))
            {
                StartHidden = hiddenElement.GetBoolean();
            }

            _ = Save();
        }
        catch
        {
            // A malformed legacy file must not block startup.
        }
    }

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

    /// <summary>In-memory defaults for the Windows release gate; never saved.</summary>
    internal void UseSmokeDefaults()
    {
        CloseToTray = false;
        AutoStart = false;
        StartHidden = false;
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
            Changed?.Invoke(this, EventArgs.Empty);
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

        // The OS entry is authoritative. Keep the state read back from it and
        // report the save failure separately so the UI never shows "off" while
        // the OS still starts the app.
        AutoStart = AutoStartService.IsEnabled();
        StartHidden = AutoStart ? AutoStartService.IsStartHidden() : previousHidden;
        Changed?.Invoke(this, EventArgs.Empty);
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
            Changed?.Invoke(this, EventArgs.Empty);
            return false;
        }

        StartHidden = enabled;
        if (Save())
        {
            Changed?.Invoke(this, EventArgs.Empty);
            return true;
        }

        StartHidden = AutoStart ? AutoStartService.IsStartHidden() : previous;
        Changed?.Invoke(this, EventArgs.Empty);
        return false;
    }

    private bool Save()
    {
        try
        {
            var temporary = _path + ".tmp";
            var payload = new JsonObject
            {
                ["closeToTray"] = CloseToTray,
                ["autoStart"] = AutoStart,
                ["startHidden"] = StartHidden,
            };
            File.WriteAllText(temporary, payload.ToJsonString(JsonOptions));
            File.Move(temporary, _path, overwrite: true);
            return true;
        }
        catch
        {
            return false;
        }
    }
}
