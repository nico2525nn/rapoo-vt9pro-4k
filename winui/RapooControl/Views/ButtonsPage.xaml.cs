using System.Collections.ObjectModel;
using System.ComponentModel;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Controls;
using RapooControl.Services;
using RapooControl.ViewModels;

namespace RapooControl.Views;

public sealed class ButtonAssignment(int id, string name, BridgeButtonAction action)
{
    public int Id { get; } = id;
    public string Name { get; } = name;
    public BridgeButtonAction Action { get; } = action;
    public string ActionLabel => Action.LabelJa ?? Action.Type ?? "—";
    public string EditLabel => $"{Name}を変更";
}

public sealed partial class ButtonsPage : Page
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
    private MainViewModel ViewModel => App.Instance.ViewModel;
    public PageFeedback Feedback { get; } = new();
    public ObservableCollection<ButtonAssignment> Assignments { get; } = [];

    public ButtonsPage()
    {
        InitializeComponent();
        Feedback.PropertyChanged += (_, _) => UpdateControls();
    }

    private async void Page_Loaded(object sender, RoutedEventArgs e)
    {
        ViewModel.PropertyChanged += DeviceStatusChanged;
        UpdateControls();
        if (ViewModel.DeviceService is not { } service)
        {
            return;
        }

        await Feedback.RunAsync(async () =>
        {
            await ViewModel.InitializeAsync();
            if (ViewModel.CanConfigureDevice)
            {
                await LoadAsync(service);
            }
            else if (service.CachedSnapshot is { Buttons: not null } snapshot)
            {
                await LoadKeysAsync(service);
                LoadAssignments(snapshot);
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
        LoadAssignments(await service.RefreshSnapshotAsync());
    }

    private async Task LoadKeysAsync(RealRapooService service)
    {
        _keys ??= (await service.GetCatalogAsync()).SupportedKeys
            ?? throw new InvalidOperationException("キーボードの選択肢を読み取れませんでした。");
    }

    private void LoadAssignments(BridgeSnapshot snapshot)
    {
        var mappings = snapshot.Buttons ?? throw new InvalidOperationException("ボタンの割り当てを読み取れませんでした。");
        Assignments.Clear();
        foreach (var mapping in mappings)
        {
            if (mapping.Slot is { } slot && mapping.Action is { } action)
            {
                Assignments.Add(new ButtonAssignment(slot.Id, slot.NameJa ?? slot.NameEn ?? $"ボタン {slot.Id}", action));
            }
        }

        UpdateControls();
    }

    private async void Refresh_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is { } service && !ViewModel.IsPassive)
        {
            await Feedback.RunAsync(() => LoadAsync(service), "再読み込みしました。");
        }
    }

    private async void Reset_Click(object sender, RoutedEventArgs e)
    {
        if (ViewModel.DeviceService is { } service && ViewModel.CanConfigureDevice)
        {
            await Feedback.RunAsync(async () => LoadAssignments(await service.ResetButtonsAsync()), "既定の割り当てに戻しました。");
        }
    }

    private async void Edit_Click(object sender, RoutedEventArgs e)
    {
        if (sender is not Button { Tag: ButtonAssignment assignment } ||
            ViewModel.DeviceService is not { } service || !ViewModel.CanConfigureDevice || _keys is null)
        {
            return;
        }

        var current = assignment.Action;
        var actions = new ComboBox
        {
            Header = "アクション",
            PlaceholderText = "アクションを選択",
            HorizontalAlignment = HorizontalAlignment.Stretch,
            ItemsSource = Actions,
            DisplayMemberPath = nameof(ActionOption.Label),
            SelectedItem = Actions.FirstOrDefault(action => action.Type == current.Type),
        };

        var availableKeys = new List<BridgeKey> { new() { Name = "なし（修飾キーのみ）" } };
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
        var message = new TextBlock { TextWrapping = TextWrapping.Wrap, Visibility = Visibility.Collapsed };
        AutomationProperties.SetLiveSetting(message, AutomationLiveSetting.Polite);

        var content = new StackPanel { Spacing = 8, Children = { actions, keyboard, message } };
        var dialog = new ContentDialog
        {
            XamlRoot = XamlRoot,
            Title = assignment.Name,
            Content = content,
            PrimaryButtonText = "保存",
            CloseButtonText = "キャンセル",
        };

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

        dialog.PrimaryButtonClick += async (_, args) =>
        {
            var deferral = args.GetDeferral();
            content.IsHitTestVisible = false;
            dialog.IsPrimaryButtonEnabled = false;
            try
            {
                if (!ViewModel.CanConfigureDevice)
                {
                    throw new InvalidOperationException(ViewModel.SettingsNotice);
                }
                if (actions.SelectedItem is not ActionOption choice)
                {
                    args.Cancel = true;
                    return;
                }
                var action = new BridgeButtonAction { Type = choice.Type };
                if (choice.Type == "keyboard")
                {
                    action.KeyCode = (keys.SelectedItem as BridgeKey)?.Code ?? 0;
                    action.Modifiers = new BridgeKeyModifiers
                    {
                        Ctrl = ctrl.IsChecked == true,
                        Shift = shift.IsChecked == true,
                        Alt = alt.IsChecked == true,
                        Meta = win.IsChecked == true,
                    };
                }

                message.Visibility = Visibility.Visible;
                message.Text = "保存中…";
                LoadAssignments(await service.SetButtonAsync(assignment.Id, action));
                Feedback.Message = "割り当てを保存しました。";
            }
            catch (Exception ex)
            {
                args.Cancel = true;
                message.Visibility = Visibility.Visible;
                message.Text = ex.Message;
            }
            finally
            {
                content.IsHitTestVisible = true;
                UpdateEditor();
                deferral.Complete();
            }
        };

        await dialog.ShowAsync();
    }

    private void UpdateControls()
    {
        var supported = ViewModel.DeviceService is not null;
        var editable = supported && !Feedback.IsBusy && ViewModel.CanConfigureDevice;
        AssignmentsCard.Visibility = Assignments.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        AssignmentList.IsEnabled = editable;
        ResetButton.IsEnabled = editable;
        RefreshButton.IsEnabled = supported && !Feedback.IsBusy && !ViewModel.IsPassive;
        DeviceNotice.Text = ViewModel.SettingsNotice;
        DeviceNotice.Visibility = DeviceNotice.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
    }
}
