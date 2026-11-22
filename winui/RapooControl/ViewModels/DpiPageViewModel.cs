using System.Collections.ObjectModel;
using RapooControl.Models;
using RapooControl.Services;

namespace RapooControl.ViewModels;

/// <summary>Editable DPI stages and polling rate for the DPI page.</summary>
public sealed class DpiPageViewModel : ObservableObject
{
    private static readonly int[] Rates = RapooLimits.PollingRates;

    private readonly IRapooService _service;
    private int _stageCount = 4;
    private int _currentStage = 2;
    private int _pollingRateHz = 4000;
    private bool _isBusy;
    private string _applyStatus = "変更は「設定を適用」でモックサービスに反映されます。";

    public DpiPageViewModel(IRapooService service)
    {
        _service = service;

        // Seven slots exist on the device; only StageCount of them are visible.
        for (var i = 1; i <= RapooLimits.MaxDpiStages; i++)
        {
            var dpi = i == 1 ? 800 : 800 * (int)Math.Pow(2, i - 1);
            var clamped = Math.Min(dpi, RapooLimits.MaxDpi);
            Stages.Add(new DpiStageViewModel(i, clamped, clamped));
        }

        UpdateVisibility();
        UpdateCurrentStageOptions();
    }

    public ObservableCollection<DpiStageViewModel> Stages { get; } = [];

    public IReadOnlyList<int> StageCountOptions { get; } = Enumerable.Range(1, RapooLimits.MaxDpiStages).ToArray();

    public IReadOnlyList<string> PollingRateOptions { get; } =
        Rates.Select(rate => $"{rate:N0} Hz").ToArray();

    public IReadOnlyList<int> CurrentStageOptions { get; private set; } = [];

    public int StageCount => _stageCount;

    /// <summary>ComboBox.SelectedIndex binding (0-based).</summary>
    public int StageCountIndex
    {
        get => Math.Clamp(_stageCount - 1, 0, RapooLimits.MaxDpiStages - 1);
        set
        {
            if (value >= 0)
            {
                SetStageCount(value + 1);
            }
        }
    }

    public int CurrentStage => _currentStage;

    public int CurrentStageIndex
    {
        get => Math.Clamp(_currentStage - 1, 0, Math.Max(0, _stageCount - 1));
        set
        {
            if (value < 0)
            {
                return;
            }

            var stage = Math.Clamp(value + 1, 1, _stageCount);
            if (SetProperty(ref _currentStage, stage))
            {
                OnPropertyChanged(nameof(CurrentStageIndex));
            }
        }
    }

    public int PollingRateHz => _pollingRateHz;

    public int PollingRateIndex
    {
        get
        {
            var index = Array.IndexOf(Rates, _pollingRateHz);
            return index >= 0 ? index : Rates.Length - 1;
        }
        set
        {
            if (value >= 0 && value < Rates.Length && SetProperty(ref _pollingRateHz, Rates[value]))
            {
                OnPropertyChanged(nameof(PollingRateIndex));
            }
        }
    }

    public bool IsBusy
    {
        get => _isBusy;
        private set
        {
            if (SetProperty(ref _isBusy, value))
            {
                OnPropertyChanged(nameof(CanApply));
            }
        }
    }

    public bool CanApply => !_isBusy;

    public string ApplyStatus => _applyStatus;

    public async Task LoadAsync()
    {
        var configuration = await _service.GetDpiConfigurationAsync();
        var status = await _service.GetStatusAsync();

        for (var index = 1; index <= RapooLimits.MaxDpiStages; index++)
        {
            var configured = configuration.Stages.FirstOrDefault(stage => stage.Index == index);
            var x = configured?.X ?? Stages[index - 1].X;
            var y = configured?.Y ?? Stages[index - 1].Y;
            Stages[index - 1].X = x;
            Stages[index - 1].Y = y;
        }

        SetStageCount(configuration.StageCount);
        SetCurrentStage(configuration.CurrentStage);
        SetPollingRate(status.PollingRateHz);

        _applyStatus = $"モックから読み込みました（{configuration.StageCount} ステージ / {status.PollingRateHz:N0} Hz）。";
        OnPropertyChanged(nameof(ApplyStatus));
    }

    public async Task ApplyAsync()
    {
        if (_isBusy)
        {
            return;
        }

        IsBusy = true;
        try
        {
            var stages = Stages
                .Where(stage => stage.Index <= _stageCount)
                .Select(stage => new DpiStage(stage.Index, (int)stage.X, (int)stage.Y))
                .ToArray();

            var configuration = new DpiConfiguration(_stageCount, _currentStage, stages);

            var dpiStatus = await _service.ApplyDpiConfigurationAsync(configuration);
            var status = await _service.SetPollingRateAsync(_pollingRateHz);

            _applyStatus = $"モックに適用しました（使用 Stage {_currentStage}: {dpiStatus.ActiveDpiText} / {status.PollingRateText}）。実機には送信していません。";
        }
        catch (Exception ex)
        {
            _applyStatus = $"適用に失敗しました: {ex.Message}";
        }
        finally
        {
            IsBusy = false;
            OnPropertyChanged(nameof(ApplyStatus));
        }
    }

    private void SetStageCount(int count)
    {
        var clamped = Math.Clamp(count, 1, RapooLimits.MaxDpiStages);
        if (_stageCount == clamped)
        {
            OnPropertyChanged(nameof(StageCountIndex));
            return;
        }

        _stageCount = clamped;
        OnPropertyChanged(nameof(StageCount));
        OnPropertyChanged(nameof(StageCountIndex));

        if (_currentStage > clamped)
        {
            _currentStage = clamped;
            OnPropertyChanged(nameof(CurrentStage));
            OnPropertyChanged(nameof(CurrentStageIndex));
        }

        UpdateVisibility();
        UpdateCurrentStageOptions();
    }

    private void SetCurrentStage(int stage)
    {
        var clamped = Math.Clamp(stage, 1, _stageCount);
        _currentStage = clamped;
        OnPropertyChanged(nameof(CurrentStage));
        OnPropertyChanged(nameof(CurrentStageIndex));
    }

    private void SetPollingRate(int hertz)
    {
        var index = Array.IndexOf(Rates, hertz);
        if (index < 0)
        {
            return;
        }

        _pollingRateHz = hertz;
        OnPropertyChanged(nameof(PollingRateHz));
        OnPropertyChanged(nameof(PollingRateIndex));
    }

    private void UpdateVisibility()
    {
        foreach (var stage in Stages)
        {
            stage.IsVisible = stage.Index <= _stageCount;
        }
    }

    private void UpdateCurrentStageOptions()
    {
        CurrentStageOptions = Enumerable.Range(1, _stageCount).ToArray();
        OnPropertyChanged(nameof(CurrentStageOptions));
    }
}
