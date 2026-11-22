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
    private bool _deviceWritable;
    private bool _loaded;
    private string _applyStatus = string.Empty;

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
                OnPropertyChanged(nameof(CanEdit));
            }
        }
    }

    public bool DeviceWritable
    {
        set
        {
            if (SetProperty(ref _deviceWritable, value))
            {
                OnPropertyChanged(nameof(CanEdit));
            }
        }
    }

    public bool CanEdit => _loaded && _deviceWritable && !_isBusy;

    public bool HasConfiguration => _loaded;

    public string ApplyStatus => _applyStatus;

    public async Task LoadAsync()
    {
        try
        {
            var configuration = await _service.GetDpiConfigurationAsync();
            var status = await _service.GetStatusAsync();
            LoadConfiguration(configuration, status.PollingRateHz);
        }
        catch (Exception ex)
        {
            ReportLoadError(ex.Message);
        }
    }

    internal void ReportLoadError(string message)
    {
        _applyStatus = message;
        OnPropertyChanged(nameof(ApplyStatus));
    }

    internal void InvalidateConfiguration()
    {
        if (SetProperty(ref _loaded, false, nameof(HasConfiguration)))
        {
            OnPropertyChanged(nameof(CanEdit));
        }
    }

    internal void LoadConfiguration(DpiConfiguration configuration, int pollingRateHz)
    {
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
        SetPollingRate(pollingRateHz);

        SetProperty(ref _loaded, true, nameof(HasConfiguration));
        _applyStatus = string.Empty;
        OnPropertyChanged(nameof(CanEdit));
        OnPropertyChanged(nameof(ApplyStatus));
    }

    public async Task ApplyAsync()
    {
        if (!CanEdit)
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

            await _service.ApplyDpiConfigurationAsync(configuration);
            await _service.SetPollingRateAsync(_pollingRateHz);

            _applyStatus = "設定を適用しました。";
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
