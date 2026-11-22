using System.Collections.ObjectModel;
using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed class ButtonAssignment(BridgeButtonSlot slot, BridgeButtonAction action)
{
    public BridgeButtonSlot Slot { get; } = slot;
    public int Id => Slot.Id;
    public string Name => LocalizationService.Instance.IsJapanese
        ? Slot.NameJa ?? Slot.NameEn ?? LocalizationService.Instance.Format("ボタン {0}", Id)
        : Slot.NameEn ?? Slot.NameJa ?? LocalizationService.Instance.Format("ボタン {0}", Id);
    public BridgeButtonAction Action { get; } = action;
    public string ActionLabel => LocalizationService.Instance.ActionLabel(Action);
    public string EditLabel => LocalizationService.Instance.Format("{0}を変更", Name);
}

public sealed partial class ButtonsPage : Page, IConfigPageActions
{
    public sealed record ActionOption(string Type, string Label);

    // The bridge catalog contains the keys, but currently has no list of fixed action types.
    private static readonly ActionOption[] Actions =
    [
        new("left_click", "マウス · 左クリック"),
        new("right_click", "マウス · 右クリック"),
        new("middle_click", "マウス · ミドルクリック"),
        new("forward", "マウス · 進む"),
        new("back", "マウス · 戻る"),
        new("wheel_default", "マウス · 標準スクロール"),
        new("fire_triple", "マウス · 左クリック 3 連射"),
        new("dpi_plus", "DPI · 段階を上げる"),
        new("dpi_minus", "DPI · 段階を下げる"),
        new("dpi_cycle", "DPI · ループ切替"),
        new("sniper_dpi", "DPI · 押下中は低 DPI"),
        new("profile_switch", "本体 · オンボード設定切替"),
        new("polling_switch", "本体 · ポーリングレート切替"),
        new("receiver_switch", "本体 · レシーバー切替"),
        new("media_play_pause", "メディア · 再生 / 一時停止"),
        new("media_stop", "メディア · 停止"),
        new("media_prev", "メディア · 前の曲"),
        new("media_next", "メディア · 次の曲"),
        new("media_vol_up", "メディア · 音量アップ"),
        new("media_vol_down", "メディア · 音量ダウン"),
        new("media_mute", "メディア · ミュート"),
        new("media_player", "アプリ · メディアプレーヤー"),
        new("app_mail", "アプリ · メール"),
        new("app_calculator", "アプリ · 電卓"),
        new("app_explorer", "アプリ · エクスプローラー"),
        new("keyboard", "キーボード"),
        new("disabled", "無効"),
    ];

    private List<BridgeKey>? _keys;
    private List<ButtonAssignment> _saved = [];
    private readonly Dictionary<int, BridgeButtonAction> _draft = [];
    private int _deviceRevision;
    private MainViewModel ViewModel => App.Instance.ViewModel;
    public PageFeedback Feedback { get; } = new();
    public ObservableCollection<ButtonAssignment> Assignments { get; } = [];

    private bool HasMappings => _saved.Count > 0 && _deviceRevision == ViewModel.DeviceRevision;
    private bool CanEdit => CanReload && HasMappings;
    public bool CanReload => !Feedback.IsBusy && ViewModel.DeviceService is not null && ViewModel.CanConfigureDevice;
    public bool CanApply => CanEdit && _draft.Count > 0;
    public event EventHandler? ActionsChanged;

    public ButtonsPage()
    {
        InitializeComponent();
        LocalizationService.Instance.Apply(this);
        Feedback.PropertyChanged += (_, e) =>
        {
            if (e.PropertyName == nameof(PageFeedback.IsBusy))
            {
                UpdateControls();
            }
        };
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        ViewModel.PropertyChanged += DeviceStatusChanged;
        UpdateControls();
        if ((HasMappings && _draft.Count > 0) || ViewModel.DeviceService is not { } service)
        {
            return;
        }

        await Feedback.RunAsync(async () =>
        {
            await ViewModel.InitializeAsync();
            if (service.CachedSnapshot is { Buttons: not null } snapshot)
            {
                await LoadKeysAsync(service);
                LoadAssignments(snapshot, discardDraft: true);
            }
            else if (ViewModel.CanConfigureDevice)
            {
                await LoadAsync(service);
            }
        });
    }

    private void Page_Unloaded(object sender, RoutedEventArgs e) => ViewModel.PropertyChanged -= DeviceStatusChanged;

    private void DeviceStatusChanged(object? sender, PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainViewModel.CanConfigureDevice))
        {
            UpdateControls();
        }
    }

    private async Task LoadAsync(RealRapooService service)
    {
        await LoadKeysAsync(service);
        LoadAssignments(await service.RefreshSnapshotAsync(), discardDraft: true);
    }

    private async Task LoadKeysAsync(RealRapooService service)
    {
        _keys ??= (await service.GetCatalogAsync()).SupportedKeys
            ?? throw new InvalidOperationException("キーボードの選択肢を読み取れませんでした。");
    }

    private void LoadAssignments(BridgeSnapshot snapshot, bool discardDraft)
    {
        var mappings = snapshot.Buttons ?? throw new InvalidOperationException("ボタンの割り当てを読み取れませんでした。");
        var saved = new List<ButtonAssignment>();
        foreach (var mapping in mappings)
        {
            if (mapping.Slot is { } slot && mapping.Action is { } action)
            {
                saved.Add(new ButtonAssignment(slot, action));
            }
        }

        if (discardDraft)
        {
            _draft.Clear();
        }
        _saved = saved;
        _deviceRevision = ViewModel.DeviceRevision;
        ShowDraft();
    }

    private void ShowDraft()
    {
        Assignments.Clear();
        foreach (var mapping in _saved)
        {
            Assignments.Add(_draft.TryGetValue(mapping.Id, out var draft)
                ? new ButtonAssignment(mapping.Slot, draft)
                : mapping);
        }
        UpdateControls();
    }

    public Task ReloadAsync() => CanReload && ViewModel.DeviceService is { } service
        ? Feedback.RunAsync(() => LoadAsync(service), PageFeedback.Reloaded)
        : Task.CompletedTask;

    public Task ApplyAsync()
    {
        if (!CanApply || ViewModel.DeviceService is not { } service)
        {
            return Task.CompletedTask;
        }

        var revision = _deviceRevision;
        return Feedback.RunAsync(async () =>
        {
            if (!Assignments.Any(mapping => mapping.Slot.Clickable && mapping.Action.Type == "left_click"))
            {
                throw new InvalidOperationException("操作不能を防ぐため、少なくとも1つの物理ボタンに「左クリック」を残す必要があります。先に別のボタンへ左クリックを割り当ててください。");
            }

            // Add replacement left-click bindings before changing the old one.
            var pending = _draft.OrderByDescending(change => change.Value.Type == "left_click").ToArray();
            foreach (var (id, action) in pending)
            {
                if (revision != ViewModel.DeviceRevision)
                {
                    throw new InvalidOperationException("接続先が変わりました。設定を再読み込みしてください。");
                }
                var snapshot = await service.SetButtonAsync(id, action);
                if (revision != ViewModel.DeviceRevision)
                {
                    throw new InvalidOperationException("接続先が変わりました。設定を再読み込みしてください。");
                }
                if (snapshot.Buttons is null)
                {
                    throw new InvalidOperationException("ボタンの割り当てを読み取れませんでした。");
                }
                _draft.Remove(id);
                LoadAssignments(snapshot, discardDraft: false);
            }
        }, PageFeedback.Applied);
    }

    private void Reset_Click(object sender, RoutedEventArgs e)
    {
        if (!CanEdit)
        {
            return;
        }

        _draft.Clear();
        foreach (var mapping in _saved)
        {
            if (mapping.Slot.DefaultAction is { } type && mapping.Action.Type != type)
            {
                var label = Actions.FirstOrDefault(option => option.Type == type)?.Label ?? type;
                _draft[mapping.Id] = new BridgeButtonAction { Type = type, LabelJa = label };
            }
        }
        Feedback.Message = _draft.Count == 0 ? PageFeedback.NoChanges : string.Empty;
        ShowDraft();
    }

    private async void Edit_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button { Tag: ButtonAssignment assignment } || !CanEdit || _keys is null)
        {
            return;
        }

        var current = assignment.Action;
        var localizedActions = Actions
            .Select(option => new ActionOption(option.Type, LocalizationService.Instance.T(option.Label)))
            .ToArray();
        var actions = new ComboBox
        {
            Header = "アクション",
            PlaceholderText = "アクションを選択",
            HorizontalAlignment = HorizontalAlignment.Stretch,
            ItemsSource = localizedActions,
            DisplayMemberPath = nameof(ActionOption.Label),
            SelectedItem = localizedActions.FirstOrDefault(action => action.Type == current.Type),
        };

        var availableKeys = new List<BridgeKey>
        {
            new() { Name = LocalizationService.Instance.T("なし（修飾キーのみ）") },
        };
        availableKeys.AddRange(_keys);
        if (current.KeyCode != 0 && !availableKeys.Any(key => key.Code == current.KeyCode))
        {
            availableKeys.Add(new BridgeKey { Code = current.KeyCode, Name = current.KeyName ?? $"0x{current.KeyCode:X2}" });
        }

        var keys = new ComboBox
        {
            Header = "キー",
            HorizontalAlignment = HorizontalAlignment.Stretch,
            ItemsSource = availableKeys,
            DisplayMemberPath = nameof(BridgeKey.Name),
            SelectedItem = availableKeys.First(key => key.Code == current.KeyCode),
        };
        var ctrl = new CheckBox { Content = "Ctrl", IsChecked = current.Modifiers?.Ctrl ?? false, MinWidth = 0 };
        var shift = new CheckBox { Content = "Shift", IsChecked = current.Modifiers?.Shift ?? false, MinWidth = 0 };
        var alt = new CheckBox { Content = "Alt", IsChecked = current.Modifiers?.Alt ?? false, MinWidth = 0 };
        var win = new CheckBox { Content = "Win", IsChecked = current.Modifiers?.Meta ?? false, MinWidth = 0 };
        var modifiers = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8, Children = { ctrl, shift, alt, win } };
        var keyboard = new StackPanel { Spacing = 8, Children = { keys, modifiers } };
        var content = new StackPanel { Spacing = 8, Children = { actions, keyboard } };
        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = assignment.Name,
            Content = content,
            PrimaryButtonText = "保存",
            CloseButtonText = "キャンセル",
        };
        LocalizationService.Instance.ApplyDialog(dialog);

        bool HasModifier() => ctrl.IsChecked == true || shift.IsChecked == true || alt.IsChecked == true || win.IsChecked == true;

        void UpdateEditor()
        {
            var selected = actions.SelectedItem as ActionOption;
            var isKeyboard = selected?.Type == "keyboard";
            keyboard.Visibility = isKeyboard ? Visibility.Visible : Visibility.Collapsed;
            dialog.IsPrimaryButtonEnabled = selected is not null &&
                (!isKeyboard || keys.SelectedItem is BridgeKey { Code: > 0 } || HasModifier());
        }

        actions.SelectionChanged += (_, _) => UpdateEditor();
        keys.SelectionChanged += (_, _) => UpdateEditor();
        foreach (var modifier in new[] { ctrl, shift, alt, win })
        {
            modifier.Checked += (_, _) => UpdateEditor();
            modifier.Unchecked += (_, _) => UpdateEditor();
        }
        UpdateEditor();

        if (await dialog.ShowAsync() != ContentDialogResult.Primary || !CanEdit ||
            actions.SelectedItem is not ActionOption choice)
        {
            return;
        }

        var action = new BridgeButtonAction { Type = choice.Type, LabelJa = choice.Label };
        if (choice.Type == "keyboard")
        {
            var key = keys.SelectedItem as BridgeKey;
            action.KeyCode = key?.Code ?? 0;
            action.Modifiers = new BridgeKeyModifiers
            {
                Ctrl = ctrl.IsChecked == true,
                Shift = shift.IsChecked == true,
                Alt = alt.IsChecked == true,
                Meta = win.IsChecked == true,
            };
            var parts = new List<string>();
            if (action.Modifiers.Ctrl) { parts.Add("Ctrl"); }
            if (action.Modifiers.Shift) { parts.Add("Shift"); }
            if (action.Modifiers.Alt) { parts.Add("Alt"); }
            if (action.Modifiers.Meta) { parts.Add("Win"); }
            if (key is { Code: > 0 }) { parts.Add(key.Name); }
            action.LabelJa = string.Join(" + ", parts);
        }

        var saved = _saved.First(mapping => mapping.Id == assignment.Id).Action;
        if (SameAction(action, saved))
        {
            _draft.Remove(assignment.Id);
        }
        else
        {
            _draft[assignment.Id] = action;
        }

        Feedback.Message = _draft.Count == 0 ? PageFeedback.NoChanges : string.Empty;
        ShowDraft();
    }

    private static bool SameAction(BridgeButtonAction a, BridgeButtonAction b) =>
        a.Type == b.Type && (a.Type != "keyboard" || a.KeyCode == b.KeyCode &&
            (a.Modifiers?.Ctrl ?? false) == (b.Modifiers?.Ctrl ?? false) &&
            (a.Modifiers?.Shift ?? false) == (b.Modifiers?.Shift ?? false) &&
            (a.Modifiers?.Alt ?? false) == (b.Modifiers?.Alt ?? false) &&
            (a.Modifiers?.Meta ?? false) == (b.Modifiers?.Meta ?? false));

    private void UpdateControls()
    {
        AssignmentList.Visibility = HasMappings ? Visibility.Visible : Visibility.Collapsed;
        AssignmentList.IsEnabled = CanEdit;
        ResetButton.IsEnabled = CanEdit;
        DeviceNotice.Text = ViewModel.SettingsNotice;
        DeviceNotice.Visibility = DeviceNotice.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
        ActionsChanged?.Invoke(this, EventArgs.Empty);
    }
}
