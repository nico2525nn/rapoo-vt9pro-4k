using System.Text.RegularExpressions;

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

    private static readonly Regex LegacyExecutablePattern = new(
        @"^(?:rapoo[-_ ]?control|rapoovt9tray|rapoo-vt9pro-4k|vt9tray)[-_a-z0-9.]*\.exe$",
        RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);

    private static readonly Regex LegacyAppFlag = new(
        @"(?:^|\s)--app=(?:winui|wails|uno)(?=\s|$)",
        RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);

    /// <summary>
    /// True only for executable names in the Rapoo app family. The standalone
    /// tray writes its path without an --app marker, so the name match is
    /// sufficient; an app marker is accepted only for the same family.
    /// </summary>
    internal static bool IsLegacyRapooCommand(string command)
    {
        if (!TrySplitCommandLine(command, out var path, out var arguments))
        {
            return false;
        }

        var separator = Math.Max(path.LastIndexOf('\\'), path.LastIndexOf('/'));
        var fileName = separator >= 0 ? path[(separator + 1)..] : path;
        if (!LegacyExecutablePattern.IsMatch(fileName))
        {
            return false;
        }

        return !arguments.Contains("--app=", StringComparison.OrdinalIgnoreCase) ||
            LegacyAppFlag.IsMatch(arguments);
    }

    /// <summary>
    /// GLib's desktop-entry parser cannot reliably launch an executable whose
    /// path contains a literal percent field code. Refuse to write an entry
    /// that would be reported as enabled but never start.
    /// </summary>
    internal static bool IsSupportedDesktopExecutable(string? path) =>
        !string.IsNullOrWhiteSpace(path) && !path.Contains('%');

    /// <summary>True for the exact --hidden or --tray argument, never a path substring.</summary>
    internal static bool HasHiddenFlag(string command) =>
        TrySplitCommandLine(command, out _, out var arguments) &&
        Regex.IsMatch(
            arguments,
            @"(?:^|\s)--(?:hidden|tray)(?=\s|$)",
            RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);

    private static bool TrySplitCommandLine(
        string command,
        out string executable,
        out string arguments)
    {
        var match = Regex.Match(command, "^\\s*(?:\"([^\"]+)\"|(\\S+))(.*)$");
        if (!match.Success)
        {
            executable = string.Empty;
            arguments = string.Empty;
            return false;
        }

        executable = match.Groups[1].Success ? match.Groups[1].Value : match.Groups[2].Value;
        arguments = match.Groups[3].Value;
        return executable.Length > 0;
    }

    private static partial bool IsEnabledCore();

    private static partial bool IsStartHiddenCore();

    private static partial bool SetEnabledCore(bool enabled, bool startHidden);

    private static partial void MigrateLegacyCore();
}
