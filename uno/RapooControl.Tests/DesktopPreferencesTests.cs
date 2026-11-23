using RapooControl.Services;

namespace RapooControl.Tests;

[TestFixture]
public sealed class DesktopPreferencesTests
{
    [Test]
    public void MigrateLegacy_ImportsOnlyTheVersionedWinUiTrayOptIn()
    {
        var directory = Path.Combine(Path.GetTempPath(), $"rapoo-prefs-{Guid.NewGuid():N}");
        try
        {
            Directory.CreateDirectory(directory);
            File.WriteAllText(
                Path.Combine(directory, "winui-tray-prefs.json"),
                """
                {"version":3,"closeToTray":true,"startHidden":true}
                """);

            var preferences = new DesktopPreferences(directory);
            preferences.MigrateLegacy();
            preferences.Load();

            Assert.Multiple(() =>
            {
                Assert.That(preferences.CloseToTray, Is.True);
                Assert.That(preferences.StartHidden, Is.True);
                Assert.That(File.Exists(Path.Combine(directory, "uno-prefs.json")), Is.True);
            });
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }
}
