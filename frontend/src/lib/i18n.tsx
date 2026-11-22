export type Language = "en" | "ja";

export type TranslationVars = Record<string, string | number>;

/**
 * Japanese translations keyed by the English source string. English is the
 * default UI language, so any missing key falls back to the key itself.
 */
const JA: Record<string, string> = {
  "Actions": "操作",
  "Active stage": "使用ステージ",
  "Added to startup": "スタートアップに登録しました",
  "App": "アプリ",
  "Appearance": "外観",
  "Applied preset “{name}”": "プリセット「{name}」を適用しました",
  "Apply": "適用",
  "Assignment": "割り当て",
  "Assignments": "割り当て",
  "At least one button must keep Left Click.": "左クリックは1つ以上のボタンに残す必要があります。",
  "Back (Mouse 4)": "戻る (Mouse 4)",
  "Background polling": "バックグラウンド取得",
  "Button": "ボタン",
  "Button assignment updated": "ボタン割り当てを変更しました",
  "Button assignments reset": "ボタン割り当てを初期化しました",
  "Cancel": "キャンセル",
  "Cannot change while in-game lock is active.": "ゲーム中ロック中は変更できません。",
  "Cannot import a preset with no Left Click assignment.":
    "左クリックが1つも割り当てられていないプロファイルはインポートできません",
  "Change": "変更",
  "Change {button}": "{button} を変更",
  "Changed polling rate to {hz} Hz": "{hz} Hz に変更しました",
  "Click and press a key. Modifier-only bindings are also supported.":
    "クリックしてキーを入力します。修飾キー単体も設定できます。",
  "Close": "閉じる",
  "Closing will exit the app": "閉じたらアプリを終了します",
  "Closing will minimize to the tray": "閉じたらトレイに格納します",
  "Connect the 4K receiver or USB-C cable.": "4K レシーバーまたは USB-C ケーブルを接続してください。",
  "Copied the udev setup command": "udev 設定コマンドをコピーしました",
  "Copy": "コピー",
  "Copy failed": "コピーに失敗しました",
  "Corrects hand shake into straight lines": "手ぶれを直線に補正",
  "Could not launch the official software": "公式ソフトを起動できませんでした",
  "Custom": "カスタム設定",
  "DPI": "DPI",
  "DPI & hardware": "DPI・ハードウェア",
  "DPI + (raise stage)": "DPI + (段階を上げる)",
  "DPI - (lower stage)": "DPI - (段階を下げる)",
  "DPI cycle": "DPI ループ切替",
  "Dark": "ダーク",
  "Delete": "削除",
  "Delete “{name}”?": "「{name}」を削除しますか？",
  "Delete failed": "削除に失敗しました",
  "Demo": "デモ",
  "Demo mode": "デモモード",
  "Demo mode ended": "デモモードを終了しました",
  "Demo mode started": "デモモードを開始しました",
  "Device": "デバイス",
  "Device information": "デバイス情報",
  "Disabled": "無効",
  "Disabled (button off)": "無効 (ボタンOFF)",
  "Disconnected": "未接続",
  "Enter a preset name": "プロファイル名を入力してください",
  "Enter sleep after inactivity": "無操作時のスリープ移行",
  "Exit": "終了",
  "Failed to change auto-start": "自動起動の変更に失敗しました",
  "Failed to change language": "言語の変更に失敗しました",
  "Failed to change mode": "モードの変更に失敗しました",
  "Failed to read mouse information": "マウス情報の取得に失敗しました",
  "Failed to save preset": "プリセットの保存に失敗しました",
  "Failed to save settings": "設定の保存に失敗しました",
  "Fire key (3-shot burst)": "ファイアキー (左3連射)",
  "Firmware": "ファームウェア",
  "Firmware update": "ファームウェア更新",
  "Firmware updates are only available in the official Windows software.":
    "ファームウェア更新はWindows版の公式ソフトでのみ利用できます",
  "Forward (Mouse 5)": "進む (Mouse 5)",
  "Game lock (full passive mode) is enabled, so settings cannot be changed. Unlock it at the top of the window.":
    "ゲーム中ロック (完全受動モード) が有効なため設定を変更できません。上部のロックを解除してください。",
  "Glass surface mode": "ガラス面モード",
  "Height at which the sensor responds": "センサーが反応する高さ",
  "Horizontal axis correction angle": "水平軸の補正角度",
  "Improves tracking on glass": "ガラス面での追従を改善",
  "In-game lock (0-TX)": "ゲーム中ロック (0-TX)",
  "In-game lock enabled": "ゲーム中ロックを有効にしました",
  "Independent X/Y": "X/Y 独立",
  "Invalid DPI stage count.": "DPIステージ数が不正です",
  "Keeps running in the background when the window is closed":
    "ウィンドウを閉じてもバックグラウンドで実行",
  "Key": "キー",
  "Keyboard": "キーボード",
  "Keyboard input": "キーボード入力",
  "Language": "言語",
  "Launch at sign-in": "サインイン時に起動",
  "Launched the official software": "公式ソフトを起動しました",
  "LED power saving": "LED 省電力",
  "Left Click": "左クリック",
  "Left-click protection": "左クリック保護",
  "Lift-off distance": "リフトオフディスタンス",
  "Light": "ライト",
  "Linear correction": "直線補正",
  "Low DPI only while held": "押している間だけ低DPI",
  "Main menu": "メインメニュー",
  "Maximize": "最大化",
  "Media & apps": "メディア・アプリ",
  "Middle Click": "ミドルクリック",
  "Minimize": "最小化",
  "Minimize to tray on close": "閉じるボタンでトレイに格納",
  "Motion Sync": "Motion Sync",
  "Mouse": "マウス",
  "Mouse not connected": "マウスが接続されていません",
  "Move the mouse to load settings.": "マウスを動かすと設定を読み込みます。",
  "Mute": "ミュート (消音)",
  "Name": "名前",
  "Next track": "次の曲",
  "No permission to access hidraw. Check the udev setup in Settings.":
    "hidraw へのアクセス権限がありません。「設定」の udev 設定を確認してください。",
  "No saved presets.": "保存したプリセットはありません。",
  "Normal": "通常",
  "Not set": "未設定",
  "Onboard profile switch": "オンボード設定切替",
  "Open calculator": "電卓を起動",
  "Open File Explorer": "エクスプローラーを起動",
  "Open mail": "メールを起動",
  "Open media player": "メディアプレーヤー起動",
  "Open official software": "公式ソフトを開く",
  "Other": "その他",
  "Play / Pause": "再生 / 一時停止",
  "Polling rate": "ポーリングレート",
  "Polling rate switch": "ポーリングレート切替",
  "Power saving": "省電力",
  "Preset deleted": "プリセットを削除しました",
  "Presets": "プリセット",
  "Press a key...": "キーを押してください...",
  "Press debounce": "押下デバウンス",
  "Previous track": "前の曲",
  "Prevents chatter when pressing": "押した時のチャタリング防止",
  "Prevents chatter when releasing": "離した時のチャタリング防止",
  "Rapoo VT9 Pro 4K (4K receiver or USB cable) was not found.":
    "Rapoo VT9 Pro 4K (4Kレシーバー または USBケーブル) が見つかりません。",
  "Rapoo VT9 Pro 4K (Demo)": "Rapoo VT9 Pro 4K (デモ)",
  "Read interval and configuration lock": "読み取り間隔と設定ロック",
  "Receiver switch": "レシーバー切替",
  "Receiver version": "レシーバーのバージョン",
  "Reduces jitter at high DPI": "高DPI時の揺れを抑制",
  "Release debounce": "解放デバウンス",
  "Reload": "再読み込み",
  "Removed from startup": "スタートアップから解除しました",
  "Reset": "初期化",
  "Reset all buttons to defaults?": "全ボタンを初期値に戻しますか？",
  "Reset mouse": "マウスの初期化",
  "Reset the mouse?": "マウスを初期化しますか？",
  "Restore": "元に戻す",
  "Restore factory defaults": "工場出荷時設定に戻す",
  "Restored factory defaults": "工場出荷時設定に戻しました",
  "Returned to normal polling mode": "通常の取得モードに戻りました",
  "Ripple control": "リップル補正",
  "Save": "保存",
  "Save preset": "プリセットを保存",
  "Saved preset “{name}”": "プリセット「{name}」を保存しました",
  "Sensor": "センサー",
  "Sensor angle": "センサー角度",
  "Settings": "設定",
  "Sleep time": "スリープ時間",
  "Sleeping": "スリープ",
  "Sniper button (low DPI while held)": "スナイパーボタン (押下中低DPI)",
  "Stage count": "ステージ数",
  "Stage {stage}": "ステージ {stage}",
  "Stage {stage} X": "ステージ {stage} X",
  "Stage {stage} Y": "ステージ {stage} Y",
  "Stages": "ステージ",
  "Standard scroll": "標準スクロール",
  "Standard scroll down": "下スクロール (標準)",
  "Standard scroll up": "上スクロール (標準)",
  "Start": "開始",
  "Stop": "停止",
  "Switched to hardware mode": "実機モードに切り替えました",
  "Switches & power": "スイッチと電源",
  "Sync with the polling interval": "ポーリング周期に同期",
  "The mouse is sleeping": "マウスがスリープ中です",
  "The mouse is sleeping.": "マウスがスリープ中です。",
  "The preset name is missing.": "プロファイル名が含まれていません",
  "The selected preset was not found.": "指定されたプロファイルが見つかりません",
  "This restores all 10 button assignments to their factory defaults.":
    "10箇所のボタン割り当てを工場出荷時の設定に戻します。",
  "This restores all settings, including DPI, sensor, and button assignments, to factory defaults.":
    "DPI・センサー・ボタン割り当てを含むすべての設定を工場出荷時の状態に戻します。",
  "To prevent an unusable mouse, at least one physical button must keep Left Click. First assign Left Click to another button.":
    "操作不能を防ぐため、少なくとも1つの物理ボタンに「左クリック」を残す必要があります。先に別のボタンへ左クリックを割り当ててください。",
  "to cancel.": "でキャンセルします。",
  "Tracking": "トラッキング",
  "Try settings without hardware": "実機を使わずに設定を試す",
  "udev setup": "udev 設定",
  "Up to {hz} Hz on this connection": "この接続では最大 {hz} Hz",
  "Volume down": "音量ダウン",
  "Volume up": "音量アップ",
  "Windows startup": "Windows スタートアップ",
  "Buttons": "ボタン",
  "Profiles": "プロファイル",
  "Action": "動作",
  "Reduces LED on-time": "LEDの点灯時間を短縮",
  "Stage": "ステージ",
  "{minutes} min": "{minutes} 分",
};

let activeLanguage: Language = "en";

/** Maps Go's language preference values onto the two supported UI languages. */
export function normalizeLanguage(value: string | null | undefined): Language {
  const normalized = (value ?? "").trim().toLowerCase();
  if (normalized === "ja" || normalized === "jp" || normalized.startsWith("ja-")) {
    return "ja";
  }
  return "en";
}

/** The language currently used by the provider, for non-React helpers. */
export function getLanguage(): Language {
  return activeLanguage;
}

/** Translates a single key outside of React (fallback simulator, data helpers). */
export function translate(
  key: string,
  vars?: TranslationVars,
  language: Language = activeLanguage
): string {
  const template = language === "ja" ? JA[key] ?? key : key;
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    Object.prototype.hasOwnProperty.call(vars, name) ? String(vars[name]) : match
  );
}

/** Applies a language immediately; callers re-render after calling this. */
export function setLanguage(language: Language): void {
  activeLanguage = normalizeLanguage(language);
  if (typeof document !== "undefined") {
    document.documentElement.lang = activeLanguage;
  }
}

/** Shorthand used by components. Missing keys fall back to English. */
export function t(key: string, vars?: TranslationVars): string {
  return translate(key, vars, activeLanguage);
}
