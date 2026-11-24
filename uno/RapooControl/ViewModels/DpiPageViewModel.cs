using System.Collections.ObjectModel;
using RapooControl.Models;
using RapooControl.Services;

namespace RapooControl.ViewModels;

/// <summary>Editable DPI stages and polling rate for the DPI page.</summary>
public sealed class DpiPageViewModel : ObservableObject
{
    private static readonly int[] Rates = RapooLimits.PollingRates;

    private readonly IRapooService _service;
    private readonly Func<long>? _currentGeneration;
    private int _stageCount = 4;
    private int _currentStage = 2;
    private int _pollingRateHz = 4000;
    private bool _isBusy;
    private bool _deviceWritable;
    private int _savedPollingRate;
    private DpiConfiguration? _savedConfiguration;

    public DpiPageViewModel(IRapooService service, Func<long>? currentGeneration = null)
    {
        _service = service;
        _currentGeneration = currentGeneration;

        // Seven slots exist on the device; only StageCount of them are visible.
        for (var i = 1; i <= RapooLimits.MaxDpiStages; i++)
        {
            var dpi = DefaultDpi(i);
            var stage = new DpiStageViewModel(i, dpi, dpi);
            stage.PropertyChanged += (_, e) =>
            {
                if (e.PropertyName is nameof(DpiStageViewModel.X) or nameof(DpiStageViewModel.Y))
                {
                    OnPropertyChanged(nameof(IsDirty));
                }
            };
            Stages.Add(stage);
        }

        UpdateVisibility();
        UpdateCurrentStageOptions();
    }

    public ObservableCollection<DpiStageViewModel> Stages { get; } = [];

    public IReadOnlyList<int> StageCountOptions { get; } = Enumerable.Range(1, RapooLimits.MaxDpiStages).ToArray();

    private int _maxRateHz = 4000;

    private int[] CurrentRates => Rates.Where(rate => rate <= _maxRateHz).ToArray();

    public IReadOnlyList<string> PollingRateOptions =>
        CurrentRates.Select(rate => $"{rate:N0} Hz").ToArray();

    internal void SetMaxRate(int maxRateHz)
    {
        var next = Rates.Where(rate => rate <= maxRateHz).DefaultIfEmpty(1000).Max();
        if (_maxRateHz == next)
        {
            return;
        }
        _maxRateHz = next;
        OnPropertyChanged(nameof(PollingRateOptions));
        OnPropertyChanged(nameof(PollingRateIndex));
    }

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
                OnPropertyChanged(nameof(IsDirty));
            }
        }
    }

    public int PollingRateHz
    {
        get
        {
            var rates = CurrentRates;
            return rates.Contains(_pollingRateHz) ? _pollingRateHz : rates[^1];
        }
    }

    public int PollingRateIndex
    {
        get
        {
            var rates = CurrentRates;
            var index = Array.IndexOf(rates, PollingRateHz);
            return index >= 0 ? index : Math.Max(0, rates.Length - 1);
        }
        set
        {
            var rates = CurrentRates;
            if (value >= 0 && value < rates.Length && SetProperty(ref _pollingRateHz, rates[value]))
            {
                OnPropertyChanged(nameof(PollingRateIndex));
                OnPropertyChanged(nameof(IsDirty));
            }
        }
    }

    public bool IsBusy
    {
        get => _isBusy;
        set
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

    public bool CanEdit => HasConfiguration && _deviceWritable && !_isBusy;

    public bool HasConfiguration => _savedConfiguration is not null;

    public bool IsDirty => _savedConfiguration is { } saved &&
        (_pollingRateHz != _savedPollingRate || HasDpiChanges(saved, CreateDraft()));

    public async Task LoadAsync()
    {
        var configuration = await _service.GetDpiConfigurationAsync();
        var status = await _service.GetStatusAsync();
        LoadConfiguration(configuration, status.PollingRateHz);
    }

    internal void RefreshLocalization()
    {
        foreach (var stage in Stages)
        {
            stage.RefreshLocalization();
        }
    }

    internal void InvalidateConfiguration()
    {
        if (_savedConfiguration is not null)
        {
            _savedConfiguration = null;
            OnPropertyChanged(nameof(HasConfiguration));
            OnPropertyChanged(nameof(CanEdit));
            OnPropertyChanged(nameof(IsDirty));
        }
    }

    internal void LoadConfiguration(DpiConfiguration configuration, int pollingRateHz)
    {
        for (var index = 1; index <= RapooLimits.MaxDpiStages; index++)
        {
            var configured = configuration.Stages.FirstOrDefault(stage => stage.Index == index);
            var x = configured?.X ?? DefaultDpi(index);
            var y = configured?.Y ?? x;
            Stages[index - 1].X = x;
            Stages[index - 1].Y = y;
        }

        SetStageCount(configuration.StageCount);
        SetCurrentStage(configuration.CurrentStage);
        SetPollingRate(pollingRateHz);

        _savedConfiguration = CreateDraft();
        _savedPollingRate = _pollingRateHz;
        OnPropertyChanged(nameof(HasConfiguration));
        OnPropertyChanged(nameof(CanEdit));
        OnPropertyChanged(nameof(IsDirty));
    }

    public async Task ApplyAsync()
    {
        if (_savedConfiguration is not { } saved)
        {
            return;
        }

        var configuration = CreateDraft();
        // Pin both writes in this apply to the same device generation.
        var generation = _currentGeneration?.Invoke() ?? 0;
        if (HasDpiChanges(saved, configuration))
        {
            await _service.ApplyDpiConfigurationAsync(
                configuration,
                expectedGeneration: generation);
            if (!ReferenceEquals(_savedConfiguration, saved))
            {
                throw new InvalidOperationException("接続先が変わりました。設定を再読み込みしてください。");
            }
            _savedConfiguration = configuration;
            OnPropertyChanged(nameof(IsDirty));
        }

        var rate = PollingRateHz;
        if (rate != _savedPollingRate)
        {
            await _service.SetPollingRateAsync(
                rate,
                expectedGeneration: generation);
            _pollingRateHz = rate;
            _savedPollingRate = rate;
            OnPropertyChanged(nameof(IsDirty));
        }
    }

    private DpiConfiguration CreateDraft() => new(_stageCount, _currentStage,
        Stages.Where(stage => stage.Index <= _stageCount)
            .Select(stage => new DpiStage(stage.Index, (int)stage.X, (int)stage.Y))
            .ToArray());

    private static bool HasDpiChanges(DpiConfiguration saved, DpiConfiguration draft) =>
        saved.StageCount != draft.StageCount || saved.CurrentStage != draft.CurrentStage ||
        !saved.Stages.SequenceEqual(draft.Stages);

    private static int DefaultDpi(int index) => Math.Min(800 << (index - 1), RapooLimits.MaxDpi);

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
        OnPropertyChanged(nameof(IsDirty));
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
