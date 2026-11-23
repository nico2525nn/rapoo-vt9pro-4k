namespace RapooControl.Services;

/// <summary>
/// Platform auto-start entry for the Uno shell. The operating-system entry is
/// authoritative: the preference file never claims an auto-start that the OS
/// did not accept.
/// </summary>
public static partial class AutoStartService
{
    public static bool SupportsTray => OperatingSystem.IsWindows();

    public static bool SupportsStartHidden => OperatingSystem.IsWindows();

    public static bool IsEnabled() => IsEnabledCore();

    /// <summary>Reads the hidden-start flag from the OS entry, if any.</summary>
    public static bool IsStartHidden() => IsStartHiddenCore();

    public static bool SetEnabled(bool enabled, bool startHidden) =>
        SetEnabledCore(enabled, startHidden);

    /// <summary>
    /// Moves the startup entry from the replaced WinUI/Wails apps to the Uno
    /// shell. The legacy entry is removed only after the Uno entry was written
    /// successfully, and its hidden preference is preserved.
    /// </summary>
    public static void MigrateLegacy() => MigrateLegacyCore();

    /// <summary>
    /// Escapes an executable path for a desktop-entry Exec value. The Exec
    /// command line is escaped first, then the whole value is escaped for the
    /// desktop-entry string parser. This is deliberately two layers.
    /// </summary>
    internal static string QuoteDesktopExec(string executable)
    {
        var commandLine = "\"" + executable
            .Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\"", "\\\"", StringComparison.Ordinal)
            .Replace("`", "\\`", StringComparison.Ordinal)
            .Replace("$", "\\$", StringComparison.Ordinal)
            .Replace("%", "%%", StringComparison.Ordinal) + "\"";

        return commandLine
            .Replace("\\", "\\\\", StringComparison.Ordinal)
            .Replace("\n", "\\n", StringComparison.Ordinal)
            .Replace("\r", "\\r", StringComparison.Ordinal)
            .Replace("\t", "\\t", StringComparison.Ordinal);
    }

    private static partial bool IsEnabledCore();

    private static partial bool IsStartHiddenCore();

    private static partial bool SetEnabledCore(bool enabled, bool startHidden);

    private static partial void MigrateLegacyCore();
}
