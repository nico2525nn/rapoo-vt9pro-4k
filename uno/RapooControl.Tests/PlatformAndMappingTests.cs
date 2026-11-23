using System.Diagnostics;
using RapooControl.Services;

namespace RapooControl.Tests;

[TestFixture]
public sealed class PlatformAndMappingTests
{
    [Test]
    public void QuoteDesktopExec_EscapesDesktopEntryMetacharacters()
    {
        var quoted = AutoStartService.QuoteDesktopExec("/opt/Rapoo $HOME\\100%");

        Assert.That(quoted, Is.EqualTo("""
            "/opt/Rapoo \\$HOME\\\\100%%"
            """));
    }

    [TestCase(@"C:\Apps\vt9tray.exe", true)]
    [TestCase(@"C:\Apps\vt9tray-v1.3.0-windows-amd64.exe", true)]
    [TestCase(@"C:\Apps\RapooControl.exe", true)]
    [TestCase(@"C:\Apps\RapooControl.exe --app=winui", true)]
    [TestCase(@"C:\Apps\rapoo-control-winui-v1.3.0-windows-amd64.exe --app=winui --hidden", true)]
    [TestCase(@"C:\Apps\rapoo-vt9pro-4k.exe --app=wails", true)]
    [TestCase(@"C:\Apps\notepad.exe", false)]
    [TestCase(@"C:\Apps\RapooControl.exe --app=other", false)]
    [TestCase(@"""C:\Apps\RapooControl.exe"" --app=winui", true)]
    [TestCase(@"""C:\Apps\--app=winui\RapooControl.exe"" --app=other", false)]
    [TestCase("\"C:\\Apps\\RapooControl.exe\" --app=\"winui\"", true)]
    public void IsLegacyRapooCommand_MatchesOnlyTheAppFamily(string command, bool expected)
    {
        Assert.That(AutoStartService.IsLegacyRapooCommand(command), Is.EqualTo(expected));
    }

    [TestCase(@"""C:\Apps\RapooControl.exe"" --hidden", true)]
    [TestCase(@"""C:\Apps\RapooControl.exe"" --tray", true)]
    [TestCase(@"""C:\Apps\--hidden\RapooControl.exe""", false)]
    [TestCase(@"""C:\Apps\RapooControl.exe"" --hiddenx", false)]
    [TestCase("\"C:\\Apps\\RapooControl.exe\" \"--hidden\"", true)]
    [TestCase("\"C:\\Apps\\RapooControl.exe\" \"--tray\"", true)]
    public void HasHiddenFlag_MatchesOnlyWholeArguments(string command, bool expected)
    {
        Assert.That(AutoStartService.HasHiddenFlag(command), Is.EqualTo(expected));
    }

    [Test]
    public void QuoteDesktopExec_IsAcceptedByDesktopFileValidate()
    {
        Assume.That(OperatingSystem.IsLinux(), "desktop-file-validate is Linux-only.");
        const string validator = "/usr/bin/desktop-file-validate";
        Assume.That(File.Exists(validator), "desktop-file-validate is not installed.");

        var path = Path.Combine(Path.GetTempPath(), $"rapoo-entry-{Guid.NewGuid():N}.desktop");
        try
        {
            File.WriteAllText(path,
                "[Desktop Entry]\n" +
                "Type=Application\n" +
                "Name=Rapoo Control\n" +
                $"Exec={AutoStartService.QuoteDesktopExec("/opt/Rapoo $HOME\\100%")} --app=uno\n" +
                "Terminal=false\n" +
                "Hidden=false\n" +
                "X-GNOME-Autostart-enabled=true\n");

            using var process = Process.Start(new ProcessStartInfo
            {
                FileName = validator,
                ArgumentList = { path },
                UseShellExecute = false,
                RedirectStandardError = true,
                RedirectStandardOutput = true,
            });
            Assert.That(process, Is.Not.Null);
            process!.WaitForExit();
            var error = process.StandardError.ReadToEnd();
            Assert.That(process.ExitCode, Is.EqualTo(0), error);
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Test]
    public async Task BridgeStartStop_IsIdempotent()
    {
        var client = new RapooBridgeClient();
        await client.DisposeAsync();
        await client.DisposeAsync();
        await client.StopAsync();
    }

    [Test]
    public void MapDpiConfiguration_PrefersAllStagesAndSortsByIndex()
    {
        var dpi = new BridgeDpiConfig
        {
            StageCount = 3,
            CurrentStage = 2,
            Stages =
            [
                new BridgeDpiStage { Index = 1, X = 800, Y = 800 },
            ],
            AllStages =
            [
                new BridgeDpiStage { Index = 2, X = 1600, Y = 1600 },
                new BridgeDpiStage { Index = 1, X = 400, Y = 400 },
                new BridgeDpiStage { Index = 3, X = 3200, Y = 3200 },
            ],
        };

        var mapped = RealRapooService.MapDpiConfiguration(dpi);

        Assert.Multiple(() =>
        {
            Assert.That(mapped.StageCount, Is.EqualTo(3));
            Assert.That(mapped.CurrentStage, Is.EqualTo(2));
            Assert.That(mapped.Stages.Select(stage => stage.Index), Is.EqualTo(new[] { 1, 2, 3 }));
            Assert.That(mapped.Stages[0].X, Is.EqualTo(400));
        });
    }
}
