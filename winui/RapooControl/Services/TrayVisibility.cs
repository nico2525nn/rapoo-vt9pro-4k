using Microsoft.Win32;

namespace RapooControl.Services;

/// <summary>
/// Windows 11 hides newly registered notification icons by default. Promote the
/// current executable so the battery indicator is visible in the taskbar.
/// </summary>
public static class TrayVisibility
{
    private const string SettingsKey = @"Control Panel\NotifyIconSettings";

    public static bool Promote(string? executablePath)
    {
        if (string.IsNullOrWhiteSpace(executablePath))
        {
            return false;
        }

        var promoted = false;
        try
        {
            using var root = Registry.CurrentUser.OpenSubKey(SettingsKey, writable: true);
            if (root is null)
            {
                return false;
            }

            foreach (var name in root.GetSubKeyNames())
            {
                using var item = root.OpenSubKey(name, writable: true);
                if (item?.GetValue("ExecutablePath") is not string path)
                {
                    continue;
                }
                if (!string.Equals(
                        Path.GetFullPath(path),
                        Path.GetFullPath(executablePath),
                        StringComparison.OrdinalIgnoreCase))
                {
                    continue;
                }

                // Only set the default when Windows has no preference yet.
                // If the user explicitly hid the icon (0), keep that choice.
                if (item.GetValue("IsPromoted") is null)
                {
                    item.SetValue("IsPromoted", 1, RegistryValueKind.DWord);
                    promoted = true;
                }
            }
        }
        catch
        {
            // Best effort; the icon remains available in the overflow.
        }

        return promoted;
    }
}
