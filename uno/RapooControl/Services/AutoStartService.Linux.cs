#if LINUX
namespace RapooControl.Services;

public static partial class AutoStartService
{
    private static partial bool IsEnabledCore() => File.Exists(LinuxAutoStartPath());

    private static partial bool IsStartHiddenCore() => false;

    private static partial bool SetEnabledCore(bool enabled, bool startHidden)
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

    private static partial void MigrateLegacyCore()
    {
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
}
#endif
