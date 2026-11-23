#if !WINDOWS && !LINUX
namespace RapooControl.Services;

/// <summary>Fallback for the platform-neutral target; auto-start is unsupported.</summary>
public static partial class AutoStartService
{
    private static partial bool IsEnabledCore() => false;

    private static partial bool IsStartHiddenCore() => false;

    private static partial bool SetEnabledCore(bool enabled, bool startHidden) => false;

    private static partial void MigrateLegacyCore()
    {
    }
}
#endif
