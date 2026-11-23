using System.Runtime.CompilerServices;
using System.Text.Json;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;

namespace RapooControl.Services;

/// <summary>
/// English is the default UI language. The XAML stays Japanese-readable and
/// this service translates the visual tree (and dynamic messages) at runtime.
/// The choice is persisted in winui-prefs.json and published as RAPOO_LANG.
/// </summary>
public sealed class LocalizationService
{
    public static LocalizationService Instance { get; } = new();

#if WINDOWS
    // Unpackaged WinUI can resolve its automatic UI font to Meiryo UI.
    private static readonly FontFamily WindowsUiFont = new("Segoe UI Variable Text, Yu Gothic UI");
#endif

    private static readonly Dictionary<string, string> English = new()
    {
        ["概要"] = "Overview",
        ["センサー"] = "Sensor",
        ["ボタン"] = "Buttons",
        ["プロファイル"] = "Profiles",
        ["設定"] = "Settings",
        ["デバイス"] = "Device",
        ["アプリ"] = "App",
        ["モデル"] = "Model",
        ["ファームウェア"] = "Firmware",
        ["マウス ファームウェア"] = "Mouse firmware",
        ["レシーバー ファームウェア"] = "Receiver firmware",
        ["ファームウェア更新"] = "Firmware update",
        ["公式ソフトを開く"] = "Open official software",
        ["ファイルを選択"] = "Choose file",
        ["RapooGameDev.exe または FirmwareExe.exe を選択してください。"] = "Choose RapooGameDev.exe or FirmwareExe.exe.",
        ["マウスの初期化"] = "Reset mouse",
        ["工場出荷時設定に戻す"] = "Restore factory defaults",
        ["デバイス情報"] = "Device information",
        ["再読み込み"] = "Reload",
        ["読み取り間隔と設定ロック"] = "Read interval and configuration lock",
        ["バックグラウンド取得"] = "Background polling",
        ["通常"] = "Normal",
        ["省電力"] = "Power saving",
        ["ゲーム優先モード（0-TX）"] = "In-game lock (0-TX)",
        ["0-TX 中はマウスへの読み書きを停止します。設定を変更するには通常に戻してください。"] =
            "0-TX stops all reads and writes to the mouse. Return to Normal to change settings.",
        ["0-TX に切り替え"] = "Switch to 0-TX",
        ["通常に戻す"] = "Return to Normal",
        ["デモモード"] = "Demo mode",
        ["実機を使わずに設定を試す"] = "Try settings without hardware",
        ["開始"] = "Start",
        ["終了"] = "Exit",
        ["オン"] = "On",
        ["オフ"] = "Off",
        ["外観"] = "Appearance",
        ["ダーク"] = "Dark",
        ["ライト"] = "Light",
        ["言語"] = "Language",
        ["接続と動作"] = "Connection & behavior",
        ["接続先が変わりました。設定を再読み込みしてください。"] =
            "The connection changed. Reload the settings.",
        ["マウスが接続されていません。レシーバーまたは USB ケーブルを確認してください。"] =
            "Mouse not connected. Check the receiver or USB cable.",
        ["マウスがスリープ中です。マウスを動かして復帰させてください。"] =
            "The mouse is asleep. Move it to wake it.",
        ["モックデータを表示しています。"] = "Showing mock data.",
        ["このページの設定は実機またはデモモードで利用できます。"] =
            "These settings require a device or demo mode.",
        ["0-TX モード中です。変更するには「設定」で通常に戻してください。"] =
            "0-TX mode is active. Return to Normal in Settings to make changes.",
        ["DPI設定を読み取れませんでした。"] = "Could not read the DPI settings.",
        ["センサー設定を読み取れませんでした。"] = "Could not read the sensor settings.",
        ["ボタンの割り当てを読み取れませんでした。"] = "Could not read the button assignments.",
        ["キーボードの選択肢を読み取れませんでした。"] = "Could not read the keyboard choices.",
        ["デバウンスの選択肢を読み取れませんでした。"] = "Could not read the debounce choices.",
        ["デバイス設定がまだ準備できていません。"] = "The device settings are not ready yet.",
        ["デバイス設定を読み取れませんでした。"] = "Could not read the device settings.",
        ["rapoo-bridge.exe が見つかりません。RapooControl.exe と同じフォルダに配置してください。"] =
            "rapoo-bridge.exe was not found. Place it next to RapooControl.exe.",
        ["rapoo-bridge.exe が応答しませんでした。"] = "rapoo-bridge.exe did not respond.",
        ["rapoo-bridge.exe が起動していません。"] = "rapoo-bridge.exe is not running.",
        ["rapoo-bridge.exe が終了しました。"] = "rapoo-bridge.exe exited.",
        ["rapoo-bridge.exe が終了しました (code {0})."] = "rapoo-bridge.exe exited (code {0}).",
        ["rapoo-bridge: {0} の応答を解釈できません: {1}"] =
            "rapoo-bridge: could not parse the {0} response: {1}",
        ["サービスの初期化に失敗しました: {0}: {1}"] =
            "Service initialization failed: {0}: {1}",
        ["DPI は 50 単位で設定できます。X と Y に異なる値を設定できます。"] =
            "DPI can be set in steps of 50. X and Y can use different values.",
        ["DPI、センサー、ボタンなど、マウス本体の設定が工場出荷時の状態に戻ります。"] =
            "This restores the mouse's DPI, sensor, buttons, and other settings to factory defaults.",
        ["マウスを初期化しますか？"] = "Reset the mouse?",
        ["割り当て"] = "Assignments",
        ["初期化"] = "Reset",
        ["キャンセル"] = "Cancel",
        ["保存"] = "Save",
        ["削除"] = "Delete",
        ["適用"] = "Apply",
        ["変更"] = "Change",
        ["名前"] = "Name",
        ["説明（任意）"] = "Description (optional)",
        ["現在の設定を保存"] = "Save current settings",
        ["保存したプロファイル"] = "Saved profiles",
        ["保存したプロファイルはありません。"] = "No saved profiles.",
        ["プロファイルを保存"] = "Save profile",
        ["プロファイルを削除しますか？"] = "Delete this profile?",
        ["プロファイルを保存しました。"] = "Profile saved.",
        ["プロファイルを削除しました。"] = "Profile deleted.",
        ["処理中…"] = "Working...",
        ["再読み込みしました。"] = "Reloaded.",
        ["適用しました。"] = "Applied.",
        ["変更はありません。"] = "No changes.",
        ["通常モードに戻しました。"] = "Returned to Normal mode.",
        ["0-TX モードに切り替えました。"] = "Switched to 0-TX mode.",
        ["デモモードを開始しました。"] = "Demo mode started.",
        ["デモモードを終了しました。"] = "Demo mode ended.",
        ["工場出荷時設定に戻しました。"] = "Restored factory defaults.",
        ["公式ソフトを起動しました。"] = "Launched the official software.",
        ["RapooGameDev.exe または FirmwareExe.exe を選択してください。"] =
            "Select RapooGameDev.exe or FirmwareExe.exe.",
        ["操作不能を防ぐため、少なくとも1つの物理ボタンに「左クリック」を残す必要があります。先に別のボタンへ左クリックを割り当ててください。"] =
            "To prevent an unusable mouse, keep Left Click on at least one physical button. Assign Left Click to another button first.",
        ["既定の割り当てに戻す"] = "Restore defaults",
        ["本体 · オンボード設定切替"] = "Hardware · Onboard profile switch",
        ["本体 · ポーリングレート切替"] = "Hardware · Polling rate switch",
        ["本体 · レシーバー切替"] = "Hardware · Receiver switch",
        ["マウス · 左クリック"] = "Mouse · Left Click",
        ["マウス · 右クリック"] = "Mouse · Right Click",
        ["マウス · ミドルクリック"] = "Mouse · Middle Click",
        ["マウス · 進む"] = "Mouse · Forward",
        ["マウス · 戻る"] = "Mouse · Back",
        ["マウス · 標準スクロール"] = "Mouse · Standard scroll",
        ["マウス · 左クリック 3 連射"] = "Mouse · Triple left click",
        ["DPI · 段階を上げる"] = "DPI · Raise stage",
        ["DPI · 段階を下げる"] = "DPI · Lower stage",
        ["DPI · ループ切替"] = "DPI · Cycle",
        ["DPI · 押下中は低 DPI"] = "DPI · Low DPI while held",
        ["メディア · 再生 / 一時停止"] = "Media · Play / Pause",
        ["メディア · 停止"] = "Media · Stop",
        ["メディア · 前の曲"] = "Media · Previous track",
        ["メディア · 次の曲"] = "Media · Next track",
        ["メディア · 音量アップ"] = "Media · Volume up",
        ["メディア · 音量ダウン"] = "Media · Volume down",
        ["メディア · ミュート"] = "Media · Mute",
        ["アプリ · メディアプレーヤー"] = "App · Media player",
        ["アプリ · メール"] = "App · Mail",
        ["アプリ · 電卓"] = "App · Calculator",
        ["アプリ · エクスプローラー"] = "App · File Explorer",
        ["キーボード"] = "Keyboard",
        ["無効"] = "Disabled",
        ["なし（修飾キーのみ）"] = "None (modifiers only)",
        ["アクション"] = "Action",
        ["アクションを選択"] = "Select an action",
        ["キー"] = "Key",
        ["トラッキング"] = "Tracking",
        ["スイッチと電源"] = "Switches & power",
        ["リフトオフディスタンス"] = "Lift-off distance",
        ["センサー角度"] = "Sensor angle",
        ["リップル補正"] = "Ripple control",
        ["直線補正"] = "Linear correction",
        ["ガラス面モード"] = "Glass surface mode",
        ["押下デバウンス"] = "Press debounce",
        ["解放デバウンス"] = "Release debounce",
        ["スリープ時間（分）"] = "Sleep time (minutes)",
        ["LED 省電力"] = "LED power saving",
        ["ポーリングレート"] = "Polling rate",
        ["ステージ"] = "Stage",
        ["ステージ数"] = "Stage count",
        ["使用ステージ"] = "Active stage",
        ["未接続"] = "Not connected",
        ["スリープ中"] = "Sleeping",
        ["スリープ"] = "Sleeping",
        ["4K ワイヤレス"] = "4K wireless",
        ["USB-C 有線"] = "USB-C wired",
        ["バッテリー駆動"] = "Battery",
        ["充電中"] = "Charging",
        ["オフライン"] = "Offline",
        ["{0}を変更"] = "Change {0}",
        ["ボタン {0}"] = "Button {0}",
        ["キー: {0}"] = "Key: {0}",
        ["Rapoo VT9 Pro 4K (デモ)"] = "Rapoo VT9 Pro 4K (Demo)",
        ["バッテリー"] = "Battery",
        ["未取得"] = "Not available",
        ["閉じたらトレイに格納"] = "Minimize to tray on close",
        ["Windows スタートアップ"] = "Start with Windows",
        ["サインイン時に起動"] = "Start at sign-in",
        ["スタートアップ時にトレイへ格納"] = "Start in the notification area",
        ["スタートアップ設定を保存できませんでした。"] = "Could not save the startup setting.",
        ["マウスからの応答を待っています。マウスを動かしてください。"] = "Waiting for the mouse to respond. Move it to wake it.",
        ["言語設定を保存できませんでした。"] = "Could not save the language setting.",
        ["開く"] = "Open Rapoo Control",
        ["Rapoo Control を開く"] = "Open Rapoo Control",
        ["hidraw へのアクセス権限がありません。「設定」の udev 設定を確認してください。"] =
            "No permission to access hidraw. Check the udev setup in Settings.",
        ["udev 設定"] = "udev setup",
        ["コピー"] = "Copy",
        ["udev 設定コマンドをコピーしました。"] = "Copied the udev setup command.",
        ["Linux の /dev/hidraw にアクセスするには、以下の udev ルールを適用してください。"] =
            "Apply the udev rule below to access /dev/hidraw on Linux.",
        ["デバイスエラー: {0}"] = "Device error: {0}",
        ["通常モード"] = "Normal mode",
        ["低頻度モード"] = "Low frequency",
        ["省電力モード"] = "Power saving",
        ["省電力モードに切り替えました。"] = "Switched to power-saving mode.",
        ["通常モードに戻しました。"] = "Returned to Normal mode.",
        ["0-TX モードに切り替えました。"] = "Switched to 0-TX mode.",
        ["0-TX (完全受動)"] = "0-TX (passive)",
        ["通知領域で実行中です。トレイアイコンをクリックすると再表示します。"] = "Still running in the notification area. Click the tray icon to reopen.",
        ["通知領域に残量を表示しています。"] = "Battery status is shown in the notification area.",
        ["デモ"] = "Demo",
        ["言語を日本語に変更しました。"] = "Language changed to Japanese.",
        ["設定を適用しました。"] = "Settings applied.",
        ["保存中…"] = "Saving...",
        ["割り当てを保存しました。"] = "Assignment saved.",
        ["既定の割り当てに戻しました。"] = "Restored the default assignments.",
        ["適用に失敗しました: {0}"] = "Apply failed: {0}",
        ["デバイスの状態とクイック統計"] = "Device status and quick stats",
        ["ステージとポーリングレート"] = "Stages and polling rate",
        ["トラッキング、表面、電源"] = "Tracking, surface and power",
        ["10 個のボタン割り当て"] = "10 programmable buttons",
        ["設定の保存と切り替え"] = "Save and switch configurations",
        ["アプリ、自動起動、Linux 権限"] = "App, startup and Linux permissions",
        ["アプリ情報"] = "App information",
        ["ビルドと診断ログ"] = "Build and diagnostic logs",
        ["バージョン"] = "Version",
        ["デバイス バックエンド"] = "Device backend",
        ["ログ フォルダーを開く"] = "Open log folder",
        ["接続とファームウェアの現在値"] = "Current connection and firmware",
        ["デバイス詳細"] = "Device details",
        ["クイック操作"] = "Quick actions",
        ["状態の更新とデバイスの動作モード"] = "Refresh state and device behavior",
        ["状態を再読み込み"] = "Refresh status",
        ["設定を開く"] = "Open settings",
        ["デモモードを開始"] = "Start demo mode",
        ["デモモードを終了"] = "Stop demo mode",
        ["接続"] = "Connection",
        ["動作モード"] = "Operating mode",
        ["更新 {0:HH:mm:ss}"] = "Updated {0:HH:mm:ss}",
        ["ファームウェアとメンテナンス"] = "Firmware and maintenance",
        ["公式の Rapoo ソフトを開きます"] = "Open the official Rapoo software",
        ["DPI、センサー、ボタンを工場出荷時に戻します"] = "Restore DPI, sensor and buttons to factory defaults",
        ["言語とスタートアップ"] = "Language and startup",
        ["サインイン時に自動起動"] = "Launch automatically at sign-in",
        ["バックグラウンドで動作を続けます"] = "Keep running in the background",
        ["ウィンドウを表示せず通知領域で開始"] = "Start in the notification area without showing the window",
        ["バックグラウンド取得とデモモード"] = "Background polling and demo mode",
        ["実機なしで画面と操作を確認"] = "Try the interface without hardware",
        ["0-TX 中はマウスへの読み書きを停止します。変更するには通常に戻してください。"] =
            "0-TX stops all reads and writes to the mouse. Return to Normal to change settings.",
        ["ポーリングレート"] = "Polling rate",
        ["マウスが位置を報告する間隔。高いほど滑らかで、消費電力も増えます。"] =
            "How often the mouse reports its position. Higher is smoother and uses more power.",
        ["レート"] = "Rate",
        ["ステージ構成"] = "Stage configuration",
        ["DPI ボタンで切り替える段数を決めます。"] = "Choose how many stages the DPI button cycles through.",
        ["DPI ステージ"] = "DPI stages",
        ["X と Y を個別に設定できます。値は 50 DPI 刻みです。"] =
            "X and Y can be set independently. Values snap to 50 DPI.",
        ["既定に戻す"] = "Restore defaults",
        ["リフトオフとセンサーの向き"] = "Lift-off and sensor orientation",
        ["リフトオフディスタンス"] = "Lift-off distance",
        ["マウスを持ち上げて反応を止める高さ"] = "Height at which tracking stops when the mouse is lifted",
        ["センサー角度"] = "Sensor angle",
        ["センサーの回転補正（-30°〜30°）"] = "Sensor rotation correction (-30° to 30°)",
        ["ポーリング周期に合わせて読み取りを同期"] = "Synchronize reads with the polling interval",
        ["表面と補正"] = "Surface and correction",
        ["マウスパッドやガラス面での挙動"] = "Behavior on mouse pads and glass",
        ["透明な表面でもトラッキングを維持"] = "Keep tracking on transparent surfaces",
        ["低 DPI での微細な揺れを抑制"] = "Reduce jitter at low DPI",
        ["ゆっくり動かした時の直線性を補正"] = "Correct straight-line accuracy at slow speeds",
        ["クリック応答とバッテリー持ち"] = "Click response and battery life",
        ["押下デバウンス"] = "Press debounce",
        ["押した瞬間のチャタリング防止"] = "Prevent chatter when the button is pressed",
        ["解放デバウンス"] = "Release debounce",
        ["離した瞬間のチャタリング防止"] = "Prevent chatter when the button is released",
        ["スリープ時間"] = "Sleep time",
        ["無操作でスリープするまでの分数"] = "Minutes of inactivity before sleeping",
        ["照明を抑えてバッテリーを節約"] = "Dim the lighting to save battery",
        ["デバイスマップ"] = "Device map",
        ["VT9 Pro 4K の 10 個の割り当て可能ボタン"] = "The 10 programmable buttons on the VT9 Pro 4K",
        ["左クリックを割り当てたボタンを最低 1 つ残してください。"] =
            "Keep at least one button assigned to Left Click.",
        ["ボタン割り当て"] = "Button assignments",
        ["各ボタンをクリックしてアクションを変更"] = "Click a button to change its action",
        ["設定の保存"] = "Save configuration",
        ["現在の DPI、センサー、ボタン割り当てをまとめて保存します。"] =
            "Save the current DPI, sensor and button assignments together.",
        ["プロファイルについて"] = "About profiles",
        ["保存したプロファイルは一覧から選んで「適用」すると、マウス本体へ書き込まれます。プロファイルはこの PC に保存されます。"] =
            "Select a saved profile and apply it to write the settings to the mouse. Profiles are stored on this PC.",
        ["設定を調整して保存"] = "Adjust the settings and save",
        ["一覧から選択して適用"] = "Select a profile and apply",
        ["上の「現在の設定を保存」から作成できます。"] = "Create one with Save current settings above.",
    };

    private static readonly Dictionary<string, string> ActionEnglish = new()
    {
        ["left_click"] = "Mouse · Left Click",
        ["right_click"] = "Mouse · Right Click",
        ["middle_click"] = "Mouse · Middle Click",
        ["forward"] = "Mouse · Forward",
        ["back"] = "Mouse · Back",
        ["wheel_default"] = "Mouse · Standard scroll",
        ["fire_triple"] = "Mouse · Triple left click",
        ["dpi_plus"] = "DPI · Raise stage",
        ["dpi_minus"] = "DPI · Lower stage",
        ["dpi_cycle"] = "DPI · Cycle",
        ["sniper_dpi"] = "DPI · Low DPI while held",
        ["profile_switch"] = "Hardware · Onboard profile switch",
        ["polling_switch"] = "Hardware · Polling rate switch",
        ["receiver_switch"] = "Hardware · Receiver switch",
        ["media_play_pause"] = "Media · Play / Pause",
        ["media_stop"] = "Media · Stop",
        ["media_prev"] = "Media · Previous track",
        ["media_next"] = "Media · Next track",
        ["media_vol_up"] = "Media · Volume up",
        ["media_vol_down"] = "Media · Volume down",
        ["media_mute"] = "Media · Mute",
        ["media_player"] = "App · Media player",
        ["app_mail"] = "App · Mail",
        ["app_calculator"] = "App · Calculator",
        ["app_explorer"] = "App · File Explorer",
        ["keyboard"] = "Keyboard",
        ["disabled"] = "Disabled",
        ["unknown"] = "Custom",
    };

    private readonly ConditionalWeakTable<DependencyObject, ElementState> _states = new();
    private readonly string _prefsPath;

    private LocalizationService()
    {
        var dir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "rapoo-vt9pro-4k");
        Directory.CreateDirectory(dir);
        _prefsPath = Path.Combine(dir, "winui-prefs.json");
    }

    public event EventHandler? LanguageChanged;

    public string Language { get; private set; } = "en";

    public bool IsJapanese => Language == "ja";

    public void Initialize()
    {
        // A saved choice wins; RAPOO_LANG is the initial default for a fresh profile.
        var systemDefault = System.Globalization.CultureInfo.CurrentUICulture.TwoLetterISOLanguageName == "ja"
            ? "ja"
            : "en";
        var language = TryNormalize(Environment.GetEnvironmentVariable("RAPOO_LANG"), out var environmentLanguage)
            ? environmentLanguage
            : systemDefault;
        if (File.Exists(_prefsPath))
        {
            try
            {
                using var document = JsonDocument.Parse(File.ReadAllText(_prefsPath));
                if (document.RootElement.TryGetProperty("language", out var value) &&
                    TryNormalize(value.GetString(), out var savedLanguage))
                {
                    language = savedLanguage;
                }
            }
            catch
            {
                // Keep the environment/default language.
            }
        }

        Language = language;
        Environment.SetEnvironmentVariable("RAPOO_LANG", language);
    }

    public bool SetLanguage(string language)
    {
        var next = Normalize(language);
        if (next == Language)
        {
            return true;
        }

        Language = next;
        Environment.SetEnvironmentVariable("RAPOO_LANG", next);
        var saved = true;
        try
        {
            var temporary = _prefsPath + ".tmp";
            File.WriteAllText(temporary, JsonSerializer.Serialize(new { language = next },
                new JsonSerializerOptions { WriteIndented = true }));
            File.Move(temporary, _prefsPath, overwrite: true);
        }
        catch
        {
            saved = false;
        }

        LanguageChanged?.Invoke(this, EventArgs.Empty);
        return saved;
    }

    public string T(string source)
    {
        if (IsJapanese)
        {
            return source;
        }
        return English.TryGetValue(source, out var translation) ? translation : source;
    }

    public string Format(string source, params object[] arguments) =>
        string.Format(T(source), arguments);

    /// <summary>Translates a device/validation error, never exposing Japanese in English mode.</summary>
    public string Error(string source)
    {
        if (IsJapanese)
        {
            return source;
        }
        if (English.TryGetValue(source, out var translation))
        {
            return translation;
        }
        if (ContainsJapanese(source))
        {
            LogError(source);
            return "The device reported an error. Check the connection and try again. Details were written to the localization error log.";
        }
        return source;
    }

    public string ActionLabel(BridgeButtonAction? action)
    {
        if (action is null)
        {
            return string.Empty;
        }
        if (action.Type == "keyboard")
        {
            return action.LabelJa ?? action.KeyName ?? T("キーボード");
        }
        if (IsJapanese)
        {
            return action.LabelJa ?? action.Type ?? string.Empty;
        }
        return ActionEnglish.TryGetValue(action.Type ?? string.Empty, out var translation)
            ? translation
            : action.LabelJa ?? action.Type ?? string.Empty;
    }

    /// <summary>Translates XAML content, including controls that have yet to be displayed.</summary>
    public void Apply(DependencyObject? root) => Apply(root, new HashSet<DependencyObject>());

    private void Apply(DependencyObject? root, HashSet<DependencyObject> visited, bool translate = true)
    {
        if (root is null || !visited.Add(root))
        {
            return;
        }
        translate &= root is not FrameworkElement { Tag: "loc-skip" };
#if WINDOWS
        if (root is Page page)
        {
            // Keep the fallback language in sync with the page labels.
            page.Language = IsJapanese ? "ja-JP" : "en-US";
        }
        ApplyWindowsFont(root);
#endif
        if (translate)
        {
            TranslateElement(root);
        }
        if (root is ContentControl { Content: DependencyObject content })
        {
            Apply(content, visited, translate);
        }
        if (root is ItemsControl items)
        {
            foreach (var item in items.Items.OfType<DependencyObject>())
            {
                Apply(item, visited, translate);
            }
        }
        var count = VisualTreeHelper.GetChildrenCount(root);
        for (var i = 0; i < count; i++)
        {
            Apply(VisualTreeHelper.GetChild(root, i), visited, translate);
        }
    }

#if WINDOWS
    private static void ApplyWindowsFont(DependencyObject element)
    {
        switch (element)
        {
            case TextBlock text when text.FontFamily.Source is "XamlAutoFontFamily" or "Meiryo UI":
                text.FontFamily = WindowsUiFont;
                break;
            case Control control when control.FontFamily.Source is "XamlAutoFontFamily" or "Meiryo UI":
                control.FontFamily = WindowsUiFont;
                break;
        }
    }
#endif

    public void ApplyDialog(ContentDialog dialog)
    {
        dialog.Title = T(dialog.Title?.ToString() ?? string.Empty);
        dialog.PrimaryButtonText = T(dialog.PrimaryButtonText);
        dialog.SecondaryButtonText = T(dialog.SecondaryButtonText);
        dialog.CloseButtonText = T(dialog.CloseButtonText);
        if (dialog.Content is string text)
        {
            dialog.Content = T(text);
        }
        else
        {
            Apply(dialog.Content as DependencyObject);
        }
    }

    private void TranslateElement(DependencyObject element)
    {
        var state = _states.GetValue(element, _ => new ElementState());

        if (element is TextBlock textBlock)
        {
            state.Text ??= textBlock.Text ?? string.Empty;
            textBlock.Text = T(state.Text);
        }

        if (element is ContentControl contentControl && contentControl.Content is string content)
        {
            state.Content ??= content;
            contentControl.Content = T((string)state.Content);
        }

        if (element is TextBox textBox)
        {
            state.Header ??= textBox.Header;
            if (state.Header is string header)
            {
                textBox.Header = T(header);
            }
            state.Placeholder ??= textBox.PlaceholderText ?? string.Empty;
            textBox.PlaceholderText = T(state.Placeholder);
        }

        if (element is ComboBox comboBox)
        {
            state.Header ??= comboBox.Header;
            if (state.Header is string header)
            {
                comboBox.Header = T(header);
            }
            state.Placeholder ??= comboBox.PlaceholderText ?? string.Empty;
            comboBox.PlaceholderText = T(state.Placeholder);
        }

        if (element is ToggleSwitch toggle)
        {
            state.Header ??= toggle.Header;
            if (state.Header is string header)
            {
                toggle.Header = T(header);
            }
            state.OnContent ??= toggle.ReadLocalValue(ToggleSwitch.OnContentProperty) == DependencyProperty.UnsetValue
                ? "オン"
                : toggle.OnContent;
            if (state.OnContent is string on)
            {
                toggle.OnContent = T(on);
            }
            state.OffContent ??= toggle.ReadLocalValue(ToggleSwitch.OffContentProperty) == DependencyProperty.UnsetValue
                ? "オフ"
                : toggle.OffContent;
            if (state.OffContent is string off)
            {
                toggle.OffContent = T(off);
            }
        }

        if (ToolTipService.GetToolTip(element) is string toolTip)
        {
            state.ToolTip ??= toolTip;
            ToolTipService.SetToolTip(element, T((string)state.ToolTip));
        }

        var automationName = AutomationProperties.GetName(element);
        if (!string.IsNullOrEmpty(automationName))
        {
            state.AutomationName ??= automationName;
            AutomationProperties.SetName(element, T(state.AutomationName));
        }
    }

    private static readonly object ErrorLogGate = new();

    private static void LogError(string source)
    {
        try
        {
            var directory = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "rapoo-vt9pro-4k");
            Directory.CreateDirectory(directory);
            lock (ErrorLogGate)
            {
                File.AppendAllText(
                    Path.Combine(directory, "localization-errors.log"),
                    $"{DateTimeOffset.Now:O} {source}{Environment.NewLine}");
            }
        }
        catch
        {
            // Diagnostics must not break the UI.
        }
    }

    private static bool ContainsJapanese(string value) =>
        value.Any(ch => ch is >= '\u3040' and <= '\u30ff' or >= '\u4e00' and <= '\u9faf');

    private static string Normalize(string? value) =>
        TryNormalize(value, out var language) ? language : "en";

    private static bool TryNormalize(string? value, out string language)
    {
        var normalized = (value ?? string.Empty).Trim().ToLowerInvariant();
        if (normalized == "ja" || normalized == "jp" || normalized.StartsWith("ja-"))
        {
            language = "ja";
            return true;
        }
        if (normalized == "en" || normalized.StartsWith("en-"))
        {
            language = "en";
            return true;
        }
        language = "en";
        return false;
    }

    private sealed class ElementState
    {
        public string? Text;
        public object? Content;
        public object? Header;
        public string? Placeholder;
        public object? ToolTip;
        public string? AutomationName;
        public object? OnContent;
        public object? OffContent;
    }
}
