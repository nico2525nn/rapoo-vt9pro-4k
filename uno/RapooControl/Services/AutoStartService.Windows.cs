#if WINDOWS
using Microsoft.Win32;

namespace RapooControl.Services;

public static partial class AutoStartService
{
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string RunValueName = "RapooControlUno";

    private static readonly string[] LegacyRunValueNames =
    [
        "RapooControlWinUI",
        "RapooControlWails",
        "RapooControl",
        "RapooVT9Tray",
    ];

    private static partial bool IsEnabledCore()
    {
        try
        {
            return IsCommandUsable(ReadRunValue(RunValueName));
        }
        catch
        {
            return false;
        }
    }

    private static partial bool IsStartHiddenCore()
    {
        var command = ReadRunValue(RunValueName);
        return command is not null && HasHiddenFlag(command);
    }

    private static partial bool SetEnabledCore(bool enabled, bool startHidden)
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
            if (string.IsNullOrWhiteSpace(executable) || !File.Exists(executable))
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

    private static partial void MigrateLegacyCore()
    {
        try
        {
            var legacy = new List<(string Name, string Command)>();
            foreach (var name in LegacyRunValueNames)
            {
                var command = ReadRunValue(name);
                if (command is not null && IsLegacyRapooCommand(command))
                {
                    legacy.Add((name, command));
                }
            }

            if (legacy.Count == 0)
            {
                return;
            }

            if (!IsCommandUsable(ReadRunValue(RunValueName)))
            {
                var hidden = legacy.Any(item => HasHiddenFlag(item.Command));
                if (!SetEnabledCore(true, hidden))
                {
                    return;
                }
            }
            // Only remove the legacy entries once the Uno value points at an
            // executable that still exists; a stale path must not win.
            if (!IsCommandUsable(ReadRunValue(RunValueName)))
            {
                return;
            }

            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: true);
            if (key is null)
            {
                return;
            }
            foreach (var (name, _) in legacy)
            {
                key.DeleteValue(name, throwOnMissingValue: false);
            }
        }
        catch
        {
            // Keep legacy entries when migration is not possible; retry next launch.
        }
    }

    private static bool IsCommandUsable(string? command)
    {
        if (string.IsNullOrWhiteSpace(command))
        {
            return false;
        }
        var text = command.Trim();
        string? executable;
        if (text.StartsWith('"'))
        {
            var end = text.IndexOf('"', 1);
            if (end <= 1)
            {
                return false;
            }
            executable = text[1..end];
        }
        else
        {
            var end = text.IndexOf(' ');
            executable = end > 0 ? text[..end] : text;
        }
        return !string.IsNullOrWhiteSpace(executable) && File.Exists(executable);
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


}
#endif
