#if LINUX
namespace RapooControl.Services;

public static partial class AutoStartService
{
    private static partial bool IsEnabledCore()
    {
        var userEntry = LinuxAutoStartPath();
        if (File.Exists(userEntry))
        {
            return IsEntryEnabled(userEntry);
        }

        foreach (var systemEntry in SystemAutoStartPaths())
        {
            if (File.Exists(systemEntry))
            {
                return IsEntryEnabled(systemEntry);
            }
        }
        return false;
    }

    private static partial bool IsStartHiddenCore() => false;

    private static partial bool SetEnabledCore(bool enabled, bool startHidden)
    {
        try
        {
            var path = LinuxAutoStartPath();
            var executable = Environment.ProcessPath;
            if (!enabled)
            {
                if (SystemAutoStartPaths().Any(File.Exists))
                {
                    // A hidden user override shadows the system entry; deleting
                    // the override would silently re-enable the system one.
                    if (string.IsNullOrWhiteSpace(executable) ||
                        !IsSupportedDesktopExecutable(executable))
                    {
                        return false;
                    }
                    WriteUserEntry(path, executable, hidden: true);
                    return !IsEntryEnabled(path);
                }

                if (File.Exists(path))
                {
                    File.Delete(path);
                }
                return !IsEnabledCore();
            }

            if (string.IsNullOrWhiteSpace(executable) ||
                !IsSupportedDesktopExecutable(executable))
            {
                return false;
            }

            WriteUserEntry(path, executable, hidden: false);
            return IsEntryEnabled(path);
        }
        catch
        {
            return false;
        }
    }

    private static partial void MigrateLegacyCore()
    {
    }

    private static void WriteUserEntry(string path, string executable, bool hidden)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        var contents =
            "[Desktop Entry]\n" +
            "Type=Application\n" +
            "Name=Rapoo Control\n" +
            $"Exec={QuoteDesktopExec(executable)} --app=uno\n" +
            "Terminal=false\n" +
            $"Hidden={(hidden ? "true" : "false")}\n" +
            $"X-GNOME-Autostart-enabled={(hidden ? "false" : "true")}\n";

        var temporary = path + ".tmp";
        File.WriteAllText(temporary, contents);
        File.Move(temporary, path, overwrite: true);
    }

    /// <summary>
    /// Resolves the XDG config directory without requiring it to exist, so the
    /// path stays stable before and after the autostart folder is created.
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

    private static IEnumerable<string> SystemAutoStartPaths()
    {
        var dirs = Environment.GetEnvironmentVariable("XDG_CONFIG_DIRS");
        var roots = string.IsNullOrWhiteSpace(dirs)
            ? new[] { "/etc/xdg" }
            : dirs.Split(':', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);

        foreach (var root in roots.Where(Path.IsPathRooted))
        {
            yield return Path.Combine(root, "autostart", "rapoo-control.desktop");
        }
    }

    private static bool IsEntryEnabled(string path)
    {
        try
        {
            if (!File.Exists(path))
            {
                return false;
            }

            var hidden = false;
            var disabled = false;
            foreach (var line in File.ReadLines(path))
            {
                var separator = line.IndexOf('=');
                if (separator <= 0)
                {
                    continue;
                }

                var key = line[..separator].Trim();
                var value = line[(separator + 1)..].Trim();
                if (string.Equals(key, "Hidden", StringComparison.OrdinalIgnoreCase) &&
                    string.Equals(value, "true", StringComparison.OrdinalIgnoreCase))
                {
                    hidden = true;
                }
                if (string.Equals(key, "X-GNOME-Autostart-enabled", StringComparison.OrdinalIgnoreCase) &&
                    string.Equals(value, "false", StringComparison.OrdinalIgnoreCase))
                {
                    disabled = true;
                }
            }

            return !hidden && !disabled;
        }
        catch
        {
            return false;
        }
    }
}
#endif
