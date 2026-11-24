namespace RapooControl.Services;

/// <summary>
/// One data root for the app and the sidecar. The Windows release gate sets
/// RAPOO_DATA_DIR so it never touches the real profile.
/// </summary>
internal static class AppPaths
{
    internal static string DataDirectory
    {
        get
        {
            var overrideDir = Environment.GetEnvironmentVariable("RAPOO_DATA_DIR");
            if (!string.IsNullOrWhiteSpace(overrideDir))
            {
                return Path.Combine(overrideDir, "rapoo-vt9pro-4k");
            }
            return Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "rapoo-vt9pro-4k");
        }
    }
}
