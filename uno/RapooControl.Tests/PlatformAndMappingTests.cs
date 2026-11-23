using RapooControl.Services;

namespace RapooControl.Tests;

[TestFixture]
public sealed class PlatformAndMappingTests
{
    [Test]
    public void QuoteDesktopExec_EscapesDesktopEntryMetacharacters()
    {
        var quoted = AutoStartService.QuoteDesktopExec("/opt/Rapoo $HOME/100% `x` \\\"test\\\"");

        Assert.That(quoted, Is.EqualTo("\"/opt/Rapoo \\$HOME/100%% \\`x\\` \\\\\\\"test\\\\\\\"\""));
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
