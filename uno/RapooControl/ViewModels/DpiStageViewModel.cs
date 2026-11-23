using RapooControl.Models;
using RapooControl.Services;

namespace RapooControl.ViewModels;

/// <summary>One editable DPI stage row.</summary>
public sealed class DpiStageViewModel : ObservableObject
{
    private double _x;
    private double _y;
    private bool _isVisible = true;
    private bool _isCurrent;

    public DpiStageViewModel(int index, double x, double y)
    {
        Index = index;
        _x = Normalize(x, 800);
        _y = Normalize(y, 800);
    }

    public int Index { get; }

    public string Label => LocalizationService.Instance.IsJapanese ? $"ステージ {Index}" : $"Stage {Index}";

    public string XLabel => $"{Label} X DPI";

    public string YLabel => $"{Label} Y DPI";

    internal void RefreshLocalization()
    {
        OnPropertyChanged(nameof(Label));
        OnPropertyChanged(nameof(XLabel));
        OnPropertyChanged(nameof(YLabel));
    }

    public double X
    {
        get => _x;
        set
        {
            var normalized = Normalize(value, _x);
            SetProperty(ref _x, normalized);
        }
    }

    public double Y
    {
        get => _y;
        set
        {
            var normalized = Normalize(value, _y);
            SetProperty(ref _y, normalized);
        }
    }

    public bool IsVisible
    {
        get => _isVisible;
        set => SetProperty(ref _isVisible, value);
    }

    public bool IsCurrent
    {
        get => _isCurrent;
        set => SetProperty(ref _isCurrent, value);
    }

    private static double Normalize(double value, double fallback)
    {
        if (double.IsNaN(value) || double.IsInfinity(value))
        {
            return fallback;
        }

        var clamped = Math.Clamp(value, RapooLimits.MinDpi, RapooLimits.MaxDpi);
        return Math.Round(clamped / RapooLimits.DpiStep, MidpointRounding.AwayFromZero) * RapooLimits.DpiStep;
    }
}
