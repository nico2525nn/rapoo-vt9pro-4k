#if WINDOWS
using Microsoft.Win32;
using System.Text.RegularExpressions;
#endif

namespace RapooControl.Services;

/// <summary>
/// Platform auto-start entry for the Uno shell. The operating-system entry is
/// authoritative: the preference file never claims an auto-start that the OS
/// did not accept.
/// </summary>
public static class AutoStartService
{
#if WINDOWS
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string RunValueName = "RapooControlUno";
    private static readonly string[] LegacyRunValueNames =
    [
        "RapooControlWinUI",
        "RapooControl",
        "RapooVT9Tray",
    ];
#endif

    public static bool SupportsTray => OperatingSystem.IsWindows();

    public static bool SupportsStartHidden => OperatingSystem.IsWindows();

    public static bool IsEnabled()
    {
#if WINDOWS
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath);
            return key?.GetValue(RunValueName) is string;
        }
        catch
        {
            return false;
        }
#else
        return File.Exists(LinuxAutoStartPath());
#endif
    }

    /// <summary>Reads the hidden-start flag from the OS entry, if any.</summary>
    public static bool IsStartHidden()
    {
#if WINDOWS
        var command = ReadRunValue(RunValueName);
        return command is not null &&
            command.Contains("--hidden", StringComparison.OrdinalIgnoreCase);
#else
        return false;
#endif
    }

    public static bool SetEnabled(bool enabled, bool startHidden)
    {
#if WINDOWS
        return SetWindows(enabled, startHidden);
#else
        return SetLinux(enabled);
#endif
    }

    /// <summary>
    /// Moves the startup entry from the replaced WinUI/Wails apps to the Uno
    /// shell. The legacy entry is removed only after the Uno entry was written
    /// successfully, and its hidden preference is preserved.
    /// </summary>
    public static void MigrateLegacy()
    {
#if WINDOWS
        try
        {
            if (ReadRunValue(RunValueName) is not null)
            {
                return;
            }

            foreach (var legacyName in LegacyRunValueNames)
            {
                var legacy = ReadRunValue(legacyName);
                if (legacy is null || !CommandBelongsToRapoo(legacy))
                {
                    continue;
                }

                var hidden = legacy.Contains("--hidden", StringComparison.OrdinalIgnoreCase);
                if (!SetWindows(true, hidden))
                {
                    return;
                }

                using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: true);
                key?.DeleteValue(legacyName, throwOnMissingValue: false);
                return;
            }
        }
        catch
        {
            // Keep the legacy entry when migration is not possible.
        }
#endif
    }

#if WINDOWS
    private static bool SetWindows(bool enabled, bool startHidden)
    {
        try
        {
            using var key = Registry.CurrentUser.CreateSubKey(RunKeyPath, writable: true);
            if (key is null)
            {
                return false;
            }
            if (!enabled)
            {
                key.DeleteValue(RunValueName, throwOnMissingValue: false);
                return true;
            }

            var executable = Environment.ProcessPath;
            if (string.IsNullOrWhiteSpace(executable))
            {
                return false;
            }
            var command = startHidden
                ? $"\"{executable}\" --app=uno --hidden"
                : $"\"{executable}\" --app=uno";
            key.SetValue(RunValueName, command);
            return string.Equals(ReadRunValue(RunValueName), command, StringComparison.Ordinal);
        }
        catch
        {
            return false;
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

    private static bool CommandBelongsToRapoo(string command)
    {
        // Only touch entries that run a Rapoo Control executable or carry one
        // of the old app markers. Never delete an unrelated Run value.
        if (!command.Contains("rapoo", StringComparison.OrdinalIgnoreCase) &&
            !command.Contains("--app=winui", StringComparison.OrdinalIgnoreCase) &&
            !command.Contains("--app=wails", StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }

        var match = Regex.Match(command, "^\\s*\"([^\"]+)\"");
        if (!match.Success)
        {
            match = Regex.Match(command, "^\\s*(\\S+)");
        }
        if (!match.Success)
        {
            return false;
        }

        var fileName = Path.GetFileName(match.Groups[1].Value);
        return fileName.Contains("rapoo", StringComparison.OrdinalIgnoreCase);
    }
#else
    private static bool SetLinux(bool enabled)
    {
        try
        {
            var path = LinuxAutoStartPath();
            if (!enabled)
            {
                if (File.Exists(path))
                {
                    File.Delete(path);
                }
                return !File.Exists(path);
            }

            var executable = Environment.ProcessPath;
            if (string.IsNullOrWhiteSpace(executable))
            {
                return false;
            }
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            var contents =
                "[Desktop Entry]\n" +
                "Type=Application\n" +
                "Name=Rapoo Control\n" +
                $"Exec={QuoteDesktopExec(executable)} --app=uno\n" +
                "Terminal=false\n" +
                "X-GNOME-Autostart-enabled=true\n";
            File.WriteAllText(path, contents);
            return File.Exists(path);
        }
        catch
        {
            return false;
        }
    }
#endif

    /// <summary>
    /// Resolves the XDG config directory without creating it. The previous
    /// implementation used SpecialFolder.ApplicationData, whose answer can
    /// change once the directory exists.
    /// </summary>
    private static string LinuxConfigHome()
    {
        var xdg = Environment.GetEnvironmentVariable("XDG_CONFIG_HOME");
        if (!string.IsNullOrWhiteSpace(xdg) && Path.IsPathRooted(xdg))
        {
            return xdg;
        }

        var home = Environment.GetEnvironmentVariable("HOME");
        if (string.IsNullOrWhiteSpace(home))
        {
            home = Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
        }
        return Path.Combine(home, ".config");
    }

    private static string LinuxAutoStartPath() =>
        Path.Combine(LinuxConfigHome(), "autostart", "rapoo-control.desktop");

    /// <summary>Escapes an executable path for a desktop-entry Exec value.</summary>
    internal static string QuoteDesktopExec(string executable)
    {
        var escaped = executable
            .Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\"", "\\\"", StringComparison.Ordinal)
            .Replace("`", "\\`", StringComparison.Ordinal)
            .Replace("$", "\\$", StringComparison.Ordinal)
            .Replace("%", "%%", StringComparison.Ordinal);
        return $"\"{escaped}\"";
    }
}
