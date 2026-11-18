import {
  Activity,
  AlertTriangle,
  Bookmark,
  Check,
  Copy,
  Download,
  FileUp,
  FolderPlus,
  HardDrive,
  Info,
  Play,
  RotateCcw,
  Trash2,
} from "lucide-react";
import { useRef, useState } from "react";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { rapooApi } from "@/lib/wails";
import type {
  AppPreferences,
  DeviceSnapshot,
  PollingMode,
  Preset,
  ServiceState,
  UICatalog,
} from "@/types/rapoo";

interface ProfilesToolsTabProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  catalog: UICatalog | null;
  presets: Preset[];
  prefs: AppPreferences | null;
  writable: boolean;
  busy: boolean;
  onSavePreset: (name: string, description: string) => Promise<void>;
  onApplyPreset: (id: string) => Promise<void>;
  onDeletePreset: (id: string) => Promise<void>;
  onImportPreset: (rawJson: string) => Promise<void>;
  onSetPollingMode: (mode: PollingMode) => Promise<void>;
  onToggleDemoMode: (enabled: boolean) => void;
  onUpdateCloseToTray: (enabled: boolean) => Promise<void>;
  onUpdateAutoStart: (enabled: boolean) => Promise<void>;
  onFactoryReset: () => Promise<void>;
}

export function ProfilesToolsTab({
  state,
  snapshot,
  catalog,
  presets,
  prefs,
  writable,
  busy,
  onSavePreset,
  onApplyPreset,
  onDeletePreset,
  onImportPreset,
  onSetPollingMode,
  onToggleDemoMode,
  onUpdateCloseToTray,
  onUpdateAutoStart,
  onFactoryReset,
}: ProfilesToolsTabProps) {
  // Save Preset Dialog
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [presetName, setPresetName] = useState("");
  const [presetDesc, setPresetDesc] = useState("");

  // Import JSON Dialog
  const [importDialogOpen, setImportDialogOpen] = useState(false);
  const [importText, setImportText] = useState("");
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  // Copied state
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [copiedUdev, setCopiedUdev] = useState(false);

  const handleSaveSubmit = async () => {
    if (!presetName.trim()) {
      toast.error("プロファイル名を入力してください");
      return;
    }
    await onSavePreset(presetName, presetDesc);
    setPresetName("");
    setPresetDesc("");
    setSaveDialogOpen(false);
  };

  const handleExportPreset = async (preset: Preset) => {
    try {
      const jsonStr = await rapooApi.exportPresetJson(preset.id);
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(jsonStr);
      }
      setCopiedId(preset.id);
      setTimeout(() => setCopiedId(null), 1800);

      // Also offer file download in browser/webview
      const blob = new Blob([jsonStr], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${preset.name.replace(/[\\/:*?"<>|]/g, "_")}.json`;
      a.click();
      URL.revokeObjectURL(url);
      toast.success(`「${preset.name}」のJSONを書き出し＆コピーしました`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "JSONの書き出しに失敗しました");
    }
  };

  const handleImportSubmit = async () => {
    if (!importText.trim()) {
      toast.error("JSONデータを貼り付けるかファイルを選択してください");
      return;
    }
    await onImportPreset(importText);
    setImportText("");
    setImportDialogOpen(false);
  };

  const copyLinuxUdev = async () => {
    if (!catalog?.linuxUdevInstallCmd) return;
    try {
      await navigator.clipboard.writeText(catalog.linuxUdevInstallCmd);
      setCopiedUdev(true);
      toast.success("Linux udev 設定コマンドをコピーしました");
      setTimeout(() => setCopiedUdev(false), 2000);
    } catch {
      toast.error("クリップボードへのコピーに失敗しました");
    }
  };

  return (
    <div className="grid gap-4 lg:grid-cols-12">
      {/* Left 7 cols: Preset Library + Safe Polling/Chattering Tester */}
      <div className="space-y-4 lg:col-span-7">
        {/* Preset Library */}
        <Card>
          <CardHeader className="pb-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <Bookmark className="h-4 w-4 text-primary" />
                  プロファイルライブラリ
                </CardTitle>
                <CardDescription>
                  DPI・ポーリングレート・センサー設定・10ボタン配置を一括で保存・切り替え・共有できます。
                </CardDescription>
              </div>
              <div className="flex items-center gap-2">
                <Button size="sm" variant="outline" onClick={() => setImportDialogOpen(true)}>
                  <FileUp className="h-3.5 w-3.5" />
                  JSON読込
                </Button>
                <Button
                  size="sm"
                  disabled={!snapshot?.dpi || busy}
                  onClick={() => setSaveDialogOpen(true)}
                >
                  <FolderPlus className="h-3.5 w-3.5" />
                  現在の設定を保存
                </Button>
              </div>
            </div>
          </CardHeader>

          <CardContent className="space-y-2.5">
            {presets.map((preset) => {
              const activeStageIdx = Math.max(
                0,
                Math.min(preset.dpi.stages.length - 1, preset.dpi.currentStage - 1)
              );
              const mainDpi = preset.dpi.stages[activeStageIdx]?.x ?? 800;

              return (
                <div
                  key={preset.id}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 rounded-lg border bg-background/45 p-3 hover:border-muted-foreground/40 transition-colors"
                >
                  <div className="space-y-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-sm font-semibold">{preset.name}</span>
                      {preset.builtIn ? (
                        <Badge variant="secondary" className="text-[10px]">
                          公式推奨プリセット
                        </Badge>
                      ) : (
                        <Badge variant="default" className="text-[10px]">
                          マイ設定
                        </Badge>
                      )}
                    </div>
                    {preset.description && (
                      <p className="text-xs text-muted-foreground leading-relaxed">{preset.description}</p>
                    )}
                    <div className="flex flex-wrap gap-1.5 pt-0.5 text-[11px] font-mono text-muted-foreground tabular-nums">
                      <span className="rounded bg-muted px-1.5 py-0.5">{preset.pollingHz} Hz</span>
                      <span className="rounded bg-muted px-1.5 py-0.5">
                        {mainDpi} DPI ({preset.dpi.stageCount}段階)
                      </span>
                      {preset.sensor.liftOffLevel !== undefined && (
                        <span className="rounded bg-muted px-1.5 py-0.5">
                          LOD {(1.0 + preset.sensor.liftOffLevel * 0.1).toFixed(1)}mm
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-1.5 shrink-0">
                    <Button
                      size="sm"
                      disabled={!writable || busy}
                      onClick={() => void onApplyPreset(preset.id)}
                      className="h-8"
                    >
                      <Play className="h-3 w-3" />
                      適用
                    </Button>
                    <Button
                      size="icon"
                      variant="outline"
                      className="h-8 w-8"
                      aria-label={`${preset.name} をJSON書き出し`}
                      onClick={() => void handleExportPreset(preset)}
                    >
                      {copiedId === preset.id ? (
                        <Check className="h-3.5 w-3.5 text-emerald-500" />
                      ) : (
                        <Download className="h-3.5 w-3.5" />
                      )}
                    </Button>
                    {!preset.builtIn && (
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            size="icon"
                            variant="outline"
                            className="h-8 w-8 text-destructive hover:text-destructive"
                            aria-label={`${preset.name} を削除`}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>「{preset.name}」を削除しますか？</AlertDialogTitle>
                            <AlertDialogDescription>
                              保存済みのマイプロファイルを削除します（現在のマウス本体の設定は変わりません）。
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>キャンセル</AlertDialogCancel>
                            <AlertDialogAction
                              variant="destructive"
                              onClick={() => void onDeletePreset(preset.id)}
                            >
                              削除する
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    )}
                  </div>
                </div>
              );
            })}
          </CardContent>
        </Card>

        {/* Browser-side Zero-HID Polling Rate & Double-Click Diagnostic Tester */}
        <PollingAndSwitchTester />
      </div>

      {/* Right 5 cols: Firmware, Background Mode, OS Integration, Linux udev, Factory Reset */}
      <div className="space-y-4 lg:col-span-5">
        {/* Hardware & Firmware Info */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2">
              <HardDrive className="h-4 w-4 text-primary" />
              デバイス情報 & システム連携
            </CardTitle>
            <CardDescription>
              ファームウェアバージョンと常駐監視動作を設定します。
            </CardDescription>
          </CardHeader>

          <CardContent className="space-y-4">
            {/* Firmware Grid */}
            <div className="grid grid-cols-2 gap-2.5 text-xs">
              <div className="rounded-lg border bg-muted/35 p-2.5">
                <span className="text-muted-foreground block text-[11px]">マウス本体 FW</span>
                <span className="font-mono text-sm font-semibold tabular-nums">
                  {snapshot?.firmware?.mouseString ? `v${snapshot.firmware.mouseString}` : "未取得"}
                </span>
              </div>
              <div className="rounded-lg border bg-muted/35 p-2.5">
                <span className="text-muted-foreground block text-[11px]">4K レシーバー FW</span>
                <span className="font-mono text-sm font-semibold tabular-nums">
                  {snapshot?.firmware?.dongleString
                    ? `v${snapshot.firmware.dongleString}`
                    : state.connection === "wired"
                      ? "有線直結"
                      : "未取得"}
                </span>
              </div>
            </div>

            {/* Background Telemetry Polling Mode */}
            <div className="space-y-1.5">
              <label className="text-xs font-medium block">バックグラウンド電池取得モード</label>
              <Select
                value={state.mode}
                onValueChange={(v) => void onSetPollingMode(v as PollingMode)}
              >
                <SelectTrigger className="h-9 text-xs" aria-label="バックグラウンド電池取得モード">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="normal">通常モード（60秒周期 + リアルタイム受動通知）</SelectItem>
                  <SelectItem value="low_freq">省電力モード（5分周期 + リアルタイム受動通知）</SelectItem>
                  <SelectItem value="passive_only">
                    ゲーム優先・完全受動モード（0-TX送信停止・設定ロック）
                  </SelectItem>
                </SelectContent>
              </Select>
              <p className="text-[11px] text-muted-foreground leading-relaxed">
                ※ どのモードでも MI_00（4000Hz マウス入力エンドポイント）には一切触れず、専用ベンダーインターフェース（COL04/COL06）のみを使用します。
              </p>
            </div>

            {/* OS Preferences */}
            <div className="divide-y divide-border/70 border-t border-border/70 pt-2">
              {prefs?.platform === "windows" && (
                <>
                  <div className="flex items-center justify-between gap-3 py-2.5">
                    <div className="space-y-0.5">
                      <label htmlFor="pref-close-tray" className="text-xs font-medium cursor-pointer">
                        閉じるボタン (×) でタスクトレイに格納
                      </label>
                      <p className="text-[11px] text-muted-foreground">
                        バックグラウンドでバッテリー残量（%）アイコンを常駐表示します
                      </p>
                    </div>
                    <Switch
                      id="pref-close-tray"
                      checked={prefs.closeToTray}
                      onCheckedChange={(v) => void onUpdateCloseToTray(v)}
                    />
                  </div>

                  <div className="flex items-center justify-between gap-3 py-2.5">
                    <div className="space-y-0.5">
                      <label htmlFor="pref-autostart" className="text-xs font-medium cursor-pointer">
                        Windows サインイン時に自動起動
                      </label>
                      <p className="text-[11px] text-muted-foreground">
                        HKCU\...\Run に登録します（管理者権限不要）
                      </p>
                    </div>
                    <Switch
                      id="pref-autostart"
                      checked={prefs.autoStart}
                      onCheckedChange={(v) => void onUpdateAutoStart(v)}
                    />
                  </div>
                </>
              )}

              <div className="flex items-center justify-between gap-3 py-2.5">
                <div className="space-y-0.5">
                  <label htmlFor="pref-demo-mode" className="text-xs font-medium cursor-pointer">
                    仮想 VT9 Pro 4K デモモード
                  </label>
                  <p className="text-[11px] text-muted-foreground">
                    実機へ書き込まずに全UI・プロファイル機能をテストします
                  </p>
                </div>
                <Switch
                  id="pref-demo-mode"
                  checked={state.demoMode}
                  onCheckedChange={onToggleDemoMode}
                />
              </div>
            </div>

            {/* Linux udev helper (shown on Linux or when permissionDenied) */}
            {(catalog?.platform === "linux" || state.permissionDenied) && catalog?.linuxUdevInstallCmd && (
              <div className="rounded-lg border bg-muted/40 p-3 space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold flex items-center gap-1.5">
                    <Info className="h-3.5 w-3.5 text-primary" />
                    Linux udev アクセス権限設定
                  </span>
                  <Button size="sm" variant="outline" className="h-7 text-xs" onClick={copyLinuxUdev}>
                    {copiedUdev ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                    コピー
                  </Button>
                </div>
                <code className="block rounded bg-background p-2 font-mono text-[10px] break-all text-muted-foreground">
                  {catalog.linuxUdevRule}
                </code>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Danger Zone: Factory Reset */}
        <Card className="border-destructive/30">
          <CardHeader className="pb-2.5">
            <CardTitle className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-4 w-4" />
              工場出荷時リセット (本体初期化)
            </CardTitle>
            <CardDescription>
              マウス本体のオンボードメモリ（DPI・レート・センサー設定・全10ボタン）を購入時の初期状態へ完全リセットします。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" size="sm" disabled={!writable || busy}>
                  <RotateCcw className="h-3.5 w-3.5" />
                  マウス本体を工場出荷状態に初期化
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>本当に工場出荷時リセットを実行しますか？</AlertDialogTitle>
                  <AlertDialogDescription>
                    マウス本体に保存されている DPI ステージ、ポーリングレート、LOD、デバウンス時間、10ボタン割り当てがすべて初期値に戻ります（保存済みのマイプロファイルはPC上に残ります）。
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>キャンセル</AlertDialogCancel>
                  <AlertDialogAction variant="destructive" onClick={() => void onFactoryReset()}>
                    初期化する
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </CardContent>
        </Card>
      </div>

      {/* Save Current Config Dialog */}
      <Dialog open={saveDialogOpen} onOpenChange={setSaveDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>現在のマウス設定をプロファイルとして保存</DialogTitle>
            <DialogDescription>
              現在読み込まれている DPI・ポーリングレート・センサー設定・10ボタン配置を保存します。
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <label htmlFor="preset-name-input" className="text-xs font-medium">
                プロファイル名 <span className="text-destructive">*</span>
              </label>
              <Input
                id="preset-name-input"
                placeholder="例: VALORANT 800DPI 4000Hz"
                value={presetName}
                onChange={(e) => setPresetName(e.target.value)}
              />
            </div>
            <div className="space-y-1">
              <label htmlFor="preset-desc-input" className="text-xs font-medium">
                メモ（任意）
              </label>
              <Input
                id="preset-desc-input"
                placeholder="例: LOD 1.0mm / サイドボタンにスキル割り当て"
                value={presetDesc}
                onChange={(e) => setPresetDesc(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSaveDialogOpen(false)}>
              キャンセル
            </Button>
            <Button onClick={() => void handleSaveSubmit()}>保存する</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Import Preset JSON Dialog */}
      <Dialog open={importDialogOpen} onOpenChange={setImportDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>プロファイル JSON の読み込み</DialogTitle>
            <DialogDescription>
              書き出した `.json` ファイルを選択するか、JSON テキストを貼り付けてインポートします。
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <input
              ref={fileInputRef}
              type="file"
              accept=".json,application/json"
              className="hidden"
              onChange={async (e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                const txt = await file.text();
                setImportText(txt);
              }}
            />
            <Button
              size="sm"
              variant="outline"
              onClick={() => fileInputRef.current?.click()}
            >
              <FileUp className="h-3.5 w-3.5" />
              JSON ファイルを選択...
            </Button>

            <textarea
              aria-label="インポートするJSON文字列"
              rows={6}
              value={importText}
              onChange={(e) => setImportText(e.target.value)}
              placeholder='{"name": "My Preset", "pollingHz": 4000, ...}'
              className="w-full rounded-md border border-input bg-background p-2.5 font-mono text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setImportDialogOpen(false)}>
              キャンセル
            </Button>
            <Button onClick={() => void handleImportSubmit()}>インポート</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// ============================================================================
// Safe Client-Side Polling Rate (Hz) & Double-Click Chattering Diagnostic
// ============================================================================

function PollingAndSwitchTester() {
  const [liveHz, setLiveHz] = useState<number>(0);
  const [peakHz, setPeakHz] = useState<number>(0);
  const [clickCount, setClickCount] = useState<number>(0);
  const [chatterCount, setChatterCount] = useState<number>(0);
  const [lastClickGapMs, setLastClickGapMs] = useState<number | null>(null);

  const sampleTimestampsRef = useRef<number[]>([]);
  const lastDownByButtonRef = useRef<Record<number, number>>({});

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const now = performance.now();
    const nativeEv = e.nativeEvent as PointerEvent;
    const coalesced = typeof nativeEv.getCoalescedEvents === "function" ? nativeEv.getCoalescedEvents() : [];
    const count = Math.max(1, coalesced.length);

    const buf = sampleTimestampsRef.current;
    for (let i = 0; i < count; i++) {
      buf.push(now);
    }
    const cutoff = now - 500; // 500ms sliding window
    while (buf.length > 0 && buf[0] < cutoff) {
      buf.shift();
    }

    if (buf.length >= 4) {
      const rate = Math.round(buf.length * 2);
      setLiveHz(rate);
      setPeakHz((prev) => Math.max(prev, rate));
    }
  };

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    const now = performance.now();
    const prev = lastDownByButtonRef.current[e.button];
    lastDownByButtonRef.current[e.button] = now;

    setClickCount((c) => c + 1);
    if (prev !== undefined) {
      const gap = Math.round((now - prev) * 10) / 10;
      setLastClickGapMs(gap);
      if (gap < 42) {
        setChatterCount((c) => c + 1);
      }
    }
  };

  const resetTester = () => {
    sampleTimestampsRef.current = [];
    lastDownByButtonRef.current = {};
    setLiveHz(0);
    setPeakHz(0);
    setClickCount(0);
    setChatterCount(0);
    setLastClickGapMs(null);
  };

  return (
    <Card>
      <CardHeader className="pb-2.5">
        <div className="flex items-center justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Activity className="h-4 w-4 text-primary" />
              実測ポーリングレート & チャタリング診断パッド
            </CardTitle>
            <CardDescription>
              USBコマンドを一切送信せず、OS入力イベントから実効Hzとデバウンス異常（42ms未満の二重反応）を計測します。
            </CardDescription>
          </div>
          <Button size="sm" variant="ghost" className="h-7 text-xs" onClick={resetTester}>
            リセット
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <div
          onPointerMove={handlePointerMove}
          onPointerDown={handlePointerDown}
          onContextMenu={(e) => e.preventDefault()}
          className="flex h-28 cursor-crosshair flex-col items-center justify-center rounded-lg border-2 border-dashed border-border bg-muted/30 px-4 text-center hover:border-primary/60 transition-colors"
        >
          <p className="text-xs font-medium">
            この枠内でマウスを素早く円を描くように動かすか、クリックしてテスト
          </p>
          <p className="mt-1 text-[11px] text-muted-foreground">
            ※ WebView / モニターのフレーム統合によりブラウザ表示上の上限が出る場合があります
          </p>
        </div>

        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 text-xs tabular-nums">
          <div className="rounded-md border bg-background/60 p-2.5">
            <span className="text-[10px] text-muted-foreground block">現在レート</span>
            <span className="text-base font-bold text-primary">{liveHz} Hz</span>
          </div>
          <div className="rounded-md border bg-background/60 p-2.5">
            <span className="text-[10px] text-muted-foreground block">ピーク計測値</span>
            <span className="text-base font-bold">{peakHz} Hz</span>
          </div>
          <div className="rounded-md border bg-background/60 p-2.5">
            <span className="text-[10px] text-muted-foreground block">クリック間隔</span>
            <span className="text-base font-bold">
              {lastClickGapMs !== null ? `${lastClickGapMs} ms` : "--"}
            </span>
          </div>
          <div className="rounded-md border bg-background/60 p-2.5">
            <span className="text-[10px] text-muted-foreground block">チャタリング検知</span>
            <span
              className={cn(
                "text-base font-bold",
                chatterCount > 0 ? "text-destructive" : "text-emerald-500"
              )}
            >
              {chatterCount} 回 / {clickCount}
            </span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
