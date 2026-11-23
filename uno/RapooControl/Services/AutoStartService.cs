#if WINDOWS
using Microsoft.Win32;
#endif

namespace RapooControl.Services;

/// <summary>Platform auto-start entry for the Uno shell.</summary>
public static class AutoStartService
{
    private const string RunKeyPath = @"Software\Microsoft\Windows\CurrentVersion\Run";
    private const string RunValueName = "RapooControlUno";

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

    public static bool SetEnabled(bool enabled, bool startHidden)
    {
#if WINDOWS
        return SetWindows(enabled, startHidden);
#else
        return SetLinux(enabled);
#endif
    }

#if WINDOWS
    private static bool SetWindows(bool enabled, bool startHidden)
    {
        try
        {
            using var key = Registry.CurrentUser.OpenSubKey(RunKeyPath, writable: true);
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
            return true;
        }
        catch
        {
            return false;
        }
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
                return true;
            }

            var executable = Environment.ProcessPath;
            if (string.IsNullOrWhiteSpace(executable))
            {
                return false;
            }
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllText(path,
                "[Desktop Entry]\n" +
                "Type=Application\n" +
                "Name=Rapoo Control\n" +
                $"Exec=\"{executable}\" --app=uno\n" +
                "X-GNOME-Autostart-enabled=true\n");
            return true;
        }
        catch
        {
            return false;
        }
    }
#endif

    private static string LinuxAutoStartPath()
    {
        var config = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        if (string.IsNullOrWhiteSpace(config))
        {
            config = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".config");
        }
        return Path.Combine(config, "autostart", "rapoo-control.desktop");
    }
}
