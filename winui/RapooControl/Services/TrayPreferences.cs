using System.Text.Json;
using Microsoft.Win32;

namespace RapooControl.Services;

/// <summary>Close-to-tray and Windows-startup preferences for the WinUI shell.</summary>
public sealed class TrayPreferences
{
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string RunValueName = "RapooControl";
    private const int CurrentVersion = 2;

    private readonly string _prefsPath;
    private bool _loaded;

    private TrayPreferences()
    {
        var dir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "rapoo-vt9pro-4k");
        Directory.CreateDirectory(dir);
        _prefsPath = Path.Combine(dir, "winui-tray-prefs.json");
    }

    public static TrayPreferences Current { get; } = new();

    public bool CloseToTray { get; private set; }

    public bool AutoStart { get; private set; }

    public void Load()
    {
        if (_loaded)
        {
            return;
        }
        _loaded = true;
        var migrated = false;
        try
        {
            if (File.Exists(_prefsPath))
            {
                using var document = JsonDocument.Parse(File.ReadAllText(_prefsPath));
                var root = document.RootElement;
                var version = root.TryGetProperty("version", out var versionElement)
                    ? versionElement.GetInt32()
                    : 0;

                // Version 1 defaulted close-to-tray to true. That default
                // stranded windows when the notification icon was hidden, so
                // require an explicit opt-in after the upgrade.
                if (version >= CurrentVersion &&
                    root.TryGetProperty("closeToTray", out var close))
                {
                    CloseToTray = close.GetBoolean();
                }
                else
                {
                    migrated = true;
                }
            }
        }
        catch
        {
            // Use defaults.
        }

        AutoStart = IsAutoStartEnabled();
        if (migrated)
        {
            Save();
        }
    }

    public void SetCloseToTray(bool enabled)
    {
        CloseToTray = enabled;
        Save();
    }

    public void SetAutoStart(bool enabled)
    {
        AutoStart = enabled;
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: true);
            if (key is null)
            {
                return;
            }
            if (enabled)
            {
                var executable = Environment.ProcessPath;
                if (!string.IsNullOrWhiteSpace(executable))
                {
                    key.SetValue(RunValueName, $"\"{executable}\"");
                }
            }
            else
            {
                key.DeleteValue(RunValueName, throwOnMissingValue: false);
            }
        }
        catch
        {
            AutoStart = IsAutoStartEnabled();
        }
        Save();
    }

    private bool IsAutoStartEnabled()
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath);
            return key?.GetValue(RunValueName) is string;
        }
        catch
        {
            return false;
        }
    }

    private void Save()
    {
        try
        {
            File.WriteAllText(
                _prefsPath,
                JsonSerializer.Serialize(new
                {
                    version = CurrentVersion,
                    closeToTray = CloseToTray,
                    autoStart = AutoStart,
                }, new JsonSerializerOptions { WriteIndented = true }));
        }
        catch
        {
            // The setting still applies for this session.
        }
    }
}
