using System.Text.Json;
using Microsoft.Win32;

namespace RapooControl.Services;

/// <summary>Close-to-tray and Windows-startup preferences for the WinUI shell.</summary>
public sealed class TrayPreferences
{
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string RunValueName = "RapooControlWinUI";
    private const string LegacyRunValueName = "RapooControl";
    private const int CloseToTrayVersion = 2;
    private const int CurrentVersion = 3;

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

    public bool StartHidden { get; private set; }

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
                if (version >= CloseToTrayVersion &&
                    root.TryGetProperty("closeToTray", out var close))
                {
                    CloseToTray = close.GetBoolean();
                }
                else if (version < CloseToTrayVersion)
                {
                    migrated = true;
                }

                if (root.TryGetProperty("startHidden", out var startHidden))
                {
                    StartHidden = startHidden.GetBoolean();
                }
                if (version < CurrentVersion)
                {
                    migrated = true;
                }
            }
        }
        catch
        {
            // Use defaults.
        }

        MigrateLegacyAutoStart();
        AutoStart = IsAutoStartEnabled();
        if (migrated)
        {
            _ = Save();
        }
    }

    public void SetCloseToTray(bool enabled)
    {
        CloseToTray = enabled;
        _ = Save();
    }

    public bool SetAutoStart(bool enabled)
    {
        try
        {
            WriteAutoStart(enabled);
            AutoStart = enabled;
            return Save();
        }
        catch
        {
            AutoStart = IsAutoStartEnabled();
            return false;
        }
    }

    public bool SetStartHidden(bool enabled)
    {
        StartHidden = enabled;
        var saved = Save();
        if (!AutoStart)
        {
            return saved;
        }

        try
        {
            WriteAutoStart(true);
            return saved;
        }
        catch
        {
            return false;
        }
    }

    private void WriteAutoStart(bool enabled)
    {
        using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: true)
            ?? throw new InvalidOperationException("The Windows startup registry key is unavailable.");
        if (!enabled)
        {
            key.DeleteValue(RunValueName, throwOnMissingValue: false);
            DeleteLegacyIfOwned(key);
            return;
        }

        var executable = Environment.ProcessPath;
        if (string.IsNullOrWhiteSpace(executable))
        {
            throw new InvalidOperationException("The executable path is unavailable.");
        }
        var command = StartHidden
            ? $"\"{executable}\" --app=winui --hidden"
            : $"\"{executable}\" --app=winui";
        key.SetValue(RunValueName, command);
    }

    private void MigrateLegacyAutoStart()
    {
        try
        {
            if (ReadRunValue(RunValueName) is not null)
            {
                return;
            }
            var legacy = ReadRunValue(LegacyRunValueName);
            var executable = Environment.ProcessPath;
            if (legacy is null || string.IsNullOrWhiteSpace(executable) ||
                !CommandOwnedByWinUi(legacy, executable))
            {
                return;
            }

            WriteAutoStart(true);
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: true);
            key?.DeleteValue(LegacyRunValueName, throwOnMissingValue: false);
        }
        catch
        {
            // Keep the old value if migration is not possible.
        }
    }

    private static string? ReadRunValue(string name)
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath);
            return key?.GetValue(name) as string;
        }
        catch
        {
            return null;
        }
    }

    private static bool CommandOwnedByWinUi(string command, string executable)
    {
        if (command.Contains("--app=wails", StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }
        if (command.Contains("--app=winui", StringComparison.OrdinalIgnoreCase) ||
            CommandMatchesExecutable(command, executable))
        {
            return true;
        }

        var candidate = ExtractExecutable(command);
        if (string.IsNullOrWhiteSpace(candidate))
        {
            return false;
        }
        var name = Path.GetFileName(candidate).ToLowerInvariant();
        return name.Contains("winui") || name == "rapoocontrol.exe";
    }

    private static bool CommandMatchesExecutable(string command, string executable)
    {
        var candidate = ExtractExecutable(command);
        if (string.IsNullOrWhiteSpace(candidate))
        {
            return false;
        }
        try
        {
            return string.Equals(
                Path.GetFullPath(candidate),
                Path.GetFullPath(executable),
                StringComparison.OrdinalIgnoreCase);
        }
        catch
        {
            return false;
        }
    }

    private static string? ExtractExecutable(string command)
    {
        command = command.Trim();
        if (command.Length == 0)
        {
            return null;
        }
        if (command[0] == '"')
        {
            var end = command.IndexOf('"', 1);
            return end > 1 ? command[1..end] : null;
        }
        var space = command.IndexOfAny([' ', '\t']);
        return space > 0 ? command[..space] : command;
    }

    private static void DeleteLegacyIfOwned(RegistryKey key)
    {
        var executable = Environment.ProcessPath;
        if (string.IsNullOrWhiteSpace(executable) ||
            key.GetValue(LegacyRunValueName) is not string legacy)
        {
            return;
        }
        if (CommandOwnedByWinUi(legacy, executable))
        {
            key.DeleteValue(LegacyRunValueName, throwOnMissingValue: false);
        }
    }

    private bool IsAutoStartEnabled() => ReadRunValue(RunValueName) is not null;

    private bool Save()
    {
        try
        {
            var temporary = _prefsPath + ".tmp";
            File.WriteAllText(
                temporary,
                JsonSerializer.Serialize(new
                {
                    version = CurrentVersion,
                    closeToTray = CloseToTray,
                    autoStart = AutoStart,
                    startHidden = StartHidden,
                }, new JsonSerializerOptions { WriteIndented = true }));
            File.Move(temporary, _prefsPath, overwrite: true);
            return true;
        }
        catch
        {
            return false;
        }
    }
}
