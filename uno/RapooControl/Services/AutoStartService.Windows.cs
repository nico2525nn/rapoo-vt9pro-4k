#if WINDOWS
using System.Text.RegularExpressions;
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

    private static readonly HashSet<string> LegacyExecutables = new(StringComparer.OrdinalIgnoreCase)
    {
        "RapooControl.exe",
        "RapooControlUno.exe",
        "RapooVT9Tray.exe",
        "rapoo-vt9pro-4k.exe",
        "rapoo-control.exe",
    };

    private static partial bool IsEnabledCore()
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

    private static partial bool IsStartHiddenCore()
    {
        var command = ReadRunValue(RunValueName);
        return command is not null &&
            command.Contains("--hidden", StringComparison.OrdinalIgnoreCase);
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

    private static partial void MigrateLegacyCore()
    {
        try
        {
            var legacy = new List<(string Name, string Command)>();
            foreach (var name in LegacyRunValueNames)
            {
                var command = ReadRunValue(name);
                if (command is not null && CommandBelongsToRapoo(command))
                {
                    legacy.Add((name, command));
                }
            }

            if (legacy.Count == 0)
            {
                return;
            }

            if (ReadRunValue(RunValueName) is null)
            {
                var hidden = legacy.Any(item =>
                    item.Command.Contains("--hidden", StringComparison.OrdinalIgnoreCase));
                if (!SetEnabledCore(true, hidden))
                {
                    return;
                }
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
        return LegacyExecutables.Contains(fileName) ||
            command.Contains("--app=wails", StringComparison.OrdinalIgnoreCase) ||
            command.Contains("--app=winui", StringComparison.OrdinalIgnoreCase) ||
            command.Contains("--app=uno", StringComparison.OrdinalIgnoreCase);
    }
}
#endif
