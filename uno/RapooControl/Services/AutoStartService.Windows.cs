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
        "RapooControl",
        "RapooVT9Tray",
    ];

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
                if (!SetEnabledCore(true, hidden))
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
}
#endif
