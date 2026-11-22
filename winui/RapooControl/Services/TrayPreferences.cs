using System.Text.Json;
using Microsoft.Win32;

namespace RapooControl.Services;

/// <summary>Close-to-tray and Windows-startup preferences for the WinUI shell.</summary>
public sealed class TrayPreferences
{
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string RunValueName = "RapooControl";

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

    public bool CloseToTray { get; private set; } = true;

    public bool AutoStart { get; private set; }

    public void Load()
    {
        if (_loaded)
        {
            return;
        }
        _loaded = true;
        try
        {
            if (File.Exists(_prefsPath))
            {
                using var document = JsonDocument.Parse(File.ReadAllText(_prefsPath));
                if (document.RootElement.TryGetProperty("closeToTray", out var close))
                {
                    CloseToTray = close.GetBoolean();
                }
            }
        }
        catch
        {
            // Use defaults.
        }

        AutoStart = IsAutoStartEnabled();
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
