import {
  AlertTriangle,
  BatteryCharging,
  Cable,
  Check,
  Copy,
  FlaskConical,
  Loader2,
  Moon,
  RefreshCw,
  Shield,
  ShieldAlert,
  Sparkles,
  Sun,
  Wifi,
  WifiOff,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { DeviceSnapshot, ServiceState, UICatalog } from "@/types/rapoo";

interface HeaderBarProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  catalog: UICatalog | null;
  hasDeviceData: boolean;
  activeTab: string;
  busy: boolean;
  savedRecently: boolean;
  darkMode: boolean;
  onToggleTheme: () => void;
  onRefresh: () => void;
  onToggleDemoMode: (enabled: boolean) => void;
  onToggleGamingLock: (locked: boolean) => void;
  onNavigateTab: (tab: string) => void;
}

export function HeaderBar({
  state,
  snapshot,
  catalog,
  hasDeviceData,
  activeTab,
  busy,
  savedRecently,
  darkMode,
  onToggleTheme,
  onRefresh,
  onToggleDemoMode,
  onToggleGamingLock,
  onNavigateTab,
}: HeaderBarProps) {
  const [copiedUdev, setCopiedUdev] = useState(false);

  const isGamingLocked = state.mode === "passive_only";
  const pct = snapshot?.battery.percentage ?? state.percentage;
  const hasBat = Boolean(snapshot) || state.hasBattery;
  const charging = snapshot?.battery.charging ?? state.charging;
  const estHours = snapshot?.estimatedHours ?? state.estimatedHours;

  const activeDpi = (() => {
    if (snapshot?.dpi) {
      const idx = Math.max(0, Math.min(snapshot.dpi.stages.length - 1, snapshot.dpi.currentStage - 1));
      const st = snapshot.dpi.stages[idx];
      if (st) return snapshot.dpi.independentXY ? `${st.x}/${st.y}` : `${st.x}`;
    }
    return state.activeDpi ? `${state.activeDpi}` : null;
  })();

  const activeRate = snapshot?.sensor?.pollingRateHz ?? state.pollingRateHz ?? null;
  const sensorSummary = snapshot?.sensor
    ? {
        lodMm: snapshot.sensor.liftOffDistanceMm.toFixed(1),
        debounce: snapshot.sensor.pressDebounceLabel,
      }
    : null;

  // High-contrast colors in both Light & Dark themes (WCAG AA >= 4.5:1)
  const batteryTone = !hasBat
    ? "text-muted-foreground"
    : pct <= 15
      ? "text-red-700 dark:text-red-400"
      : pct <= 30
        ? "text-amber-700 dark:text-amber-400"
        : "text-emerald-700 dark:text-emerald-400";

  const batteryFillColor = !hasBat
    ? "bg-muted-foreground/40"
    : pct <= 15
      ? "bg-red-500"
      : pct <= 30
        ? "bg-amber-500"
        : "bg-emerald-500";

  const copyUdevCommand = async () => {
    if (!catalog?.linuxUdevInstallCmd) return;
    try {
      await navigator.clipboard.writeText(catalog.linuxUdevInstallCmd);
      setCopiedUdev(true);
      toast.success("udev 設定コマンドをクリップボードにコピーしました");
      setTimeout(() => setCopiedUdev(false), 2000);
    } catch {
      toast.error("コピーに失敗しました。プロファイル＆診断タブから手動でコピーしてください。");
    }
  };

  // Avoid duplicate full-width banners when DeviceUnavailableCard is already centered right below
  const showDisconnectedBanner =
    !state.connected && !state.permissionDenied && !state.lastError && activeTab === "profiles";
  const showSleepingBanner =
    state.connected && state.sleeping && !state.demoMode && (hasDeviceData || activeTab === "profiles");

  return (
    <header className="border-b bg-card/85 backdrop-blur-md sticky top-0 z-30">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-2.5">
        {/* Left: Brand + Connection State */}
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/15 border border-primary/30 text-primary font-bold tracking-tight text-xs">
            VT9
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="text-sm font-bold tracking-tight truncate">
                {state.deviceName || "Rapoo VT9 Pro 4K"}
              </h1>
              {state.demoMode ? (
                <Badge variant="default" className="gap-1">
                  <FlaskConical className="h-3 w-3" />
                  デモモード
                </Badge>
              ) : !state.connected ? (
                <Badge variant="outline" className="text-muted-foreground gap-1">
                  <WifiOff className="h-3 w-3" />
                  未接続
                </Badge>
              ) : state.sleeping ? (
                <Badge variant="warning" className="gap-1">
                  <Moon className="h-3 w-3" />
                  スリープ中
                </Badge>
              ) : state.connection === "wired" ? (
                <Badge variant="success" className="gap-1">
                  <Cable className="h-3 w-3" />
                  USB-C 有線
                </Badge>
              ) : (
                <Badge variant="success" className="gap-1">
                  <Wifi className="h-3 w-3" />
                  4K ワイヤレス
                </Badge>
              )}

              {busy ? (
                <span
                  aria-live="polite"
                  className="inline-flex items-center gap-1 text-[11px] text-primary font-medium"
                >
                  <Loader2 className="h-3 w-3 animate-spin" />
                  同期中...
                </span>
              ) : savedRecently ? (
                <span
                  aria-live="polite"
                  className="inline-flex items-center gap-1 text-[11px] text-emerald-700 dark:text-emerald-400 font-medium"
                >
                  <Check className="h-3 w-3" />
                  保存済み
                </span>
              ) : null}
            </div>
            <p className="text-[11px] text-muted-foreground truncate">
              PAW3395 光学センサー • オンボードメモリ直結コントロール
            </p>
          </div>
        </div>

        {/* Center/Right: Telemetry Summary & Quick Controls */}
        <div className="flex items-center gap-2 flex-wrap">
          {state.connected && (
            <>
              {/* Battery Pill */}
              <Tooltip>
                <TooltipTrigger asChild>
                  <div
                    className="flex items-center gap-2 rounded-md border bg-background/70 px-2.5 py-1 text-xs tabular-nums"
                    aria-label={hasBat ? `バッテリー残量 ${pct}%` : "バッテリー未取得"}
                  >
                    {charging ? (
                      <BatteryCharging className="h-4 w-4 text-emerald-700 dark:text-emerald-400 shrink-0" />
                    ) : (
                      <div className="relative flex h-3.5 w-6 items-center rounded-[3px] border border-current p-[1.5px] text-muted-foreground">
                        <div
                          className={cn("h-full rounded-[1.5px] transition-all", batteryFillColor)}
                          style={{ width: `${hasBat ? Math.max(8, Math.min(100, pct)) : 0}%` }}
                        />
                        <div className="absolute -right-[3px] top-1/2 h-1.5 w-[2px] -translate-y-1/2 rounded-r-[1px] bg-current" />
                      </div>
                    )}
                    <span className={cn("font-semibold", batteryTone)}>
                      {hasBat ? `${pct}%` : "--%"}
                    </span>
                    {hasBat && state.connection !== "wired" && !charging && estHours > 0 && (
                      <span className="hidden md:inline text-[11px] text-muted-foreground">
                        (残り約{estHours}h)
                      </span>
                    )}
                    {charging && (
                      <span className="hidden sm:inline text-[11px] text-emerald-700 dark:text-emerald-400 font-medium">
                        充電中
                      </span>
                    )}
                  </div>
                </TooltipTrigger>
                <TooltipContent>
                  {charging
                    ? `USB充電中 (${pct}%)`
                    : hasBat
                      ? `バッテリー残量 ${pct}%${
                          state.connection !== "wired"
                            ? ` • 現在のポーリングレートで残り約 ${estHours} 時間駆動`
                            : ""
                        }`
                      : "バッテリー情報を待機中"}
                </TooltipContent>
              </Tooltip>

              {/* Quick DPI & Rate Summary Pill */}
              {(activeDpi || activeRate) && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      onClick={() => onNavigateTab("dpi")}
                      className="hidden sm:flex items-center gap-1.5 rounded-md border bg-background/70 px-2.5 py-1 text-xs tabular-nums hover:bg-accent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background"
                    >
                      {activeDpi && <span className="font-semibold">{activeDpi} DPI</span>}
                      {activeDpi && activeRate && <span className="text-muted-foreground">•</span>}
                      {activeRate && <span className="text-primary font-semibold">{activeRate} Hz</span>}
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>クリックして DPI・ポーリングレート設定を開く</TooltipContent>
                </Tooltip>
              )}

              {/* Quick Sensor (LOD & Debounce) Summary Pill */}
              {sensorSummary && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      onClick={() => onNavigateTab("sensor")}
                      className="hidden lg:flex items-center gap-1.5 rounded-md border bg-background/70 px-2.5 py-1 text-xs tabular-nums hover:bg-accent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background"
                    >
                      <span className="text-muted-foreground">LOD</span>
                      <span className="font-semibold">{sensorSummary.lodMm}mm</span>
                      <span className="text-muted-foreground">•</span>
                      <span className="font-semibold">{sensorSummary.debounce}</span>
                    </button>
                  </TooltipTrigger>
                  <TooltipContent>クリックして センサー・デバウンス設定を開く</TooltipContent>
                </Tooltip>
              )}
            </>
          )}

          {/* Gaming Lock (0-TX Passive Mode) */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant={isGamingLocked ? "default" : "outline"}
                size="sm"
                onClick={() => onToggleGamingLock(!isGamingLocked)}
                aria-pressed={isGamingLocked}
                className={cn(
                  "gap-1.5",
                  isGamingLocked && "bg-amber-500 text-slate-950 hover:bg-amber-500/90 border-amber-500"
                )}
              >
                {isGamingLocked ? <ShieldAlert className="h-3.5 w-3.5" /> : <Shield className="h-3.5 w-3.5" />}
                <span>{isGamingLocked ? "ゲーム中ロック ON" : "ゲーム中ロック"}</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p className="font-semibold mb-0.5">ゲーム優先・完全受動モード (0-TX)</p>
              <p>
                ONにするとアプリからマウスへの定期通信を完全停止し、誤操作による設定変更もロックします（マウス側からの受動バッテリー通知のみ受信）。
              </p>
            </TooltipContent>
          </Tooltip>

          {/* Refresh */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                size="icon"
                onClick={onRefresh}
                disabled={isGamingLocked}
                aria-label="マウス設定を再読み込み"
              >
                <RefreshCw className={cn("h-3.5 w-3.5", busy && "animate-spin")} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              {isGamingLocked
                ? "ゲーム中ロックを解除すると再読み込みできます"
                : "マウス本体の最新状態を今すぐ再取得"}
            </TooltipContent>
          </Tooltip>

          {/* Theme Toggle */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={onToggleTheme}
                aria-label={darkMode ? "ライトテーマに切り替え" : "ダークテーマに切り替え"}
              >
                {darkMode ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
              </Button>
            </TooltipTrigger>
            <TooltipContent>{darkMode ? "ライトモードに切替" : "ダークモードに切替"}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {/* Contextual Banners */}
      {state.permissionDenied && (
        <div className="border-t border-destructive/30 bg-destructive/10 px-4 py-2.5">
          <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 text-xs">
            <div className="flex items-center gap-2 text-destructive font-medium">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              <span>
                Linux の <code className="font-mono">/dev/hidraw*</code> へのアクセス権限がありません。udev
                ルールを設定してUSBレシーバーを挿し直してください。
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" variant="outline" onClick={copyUdevCommand}>
                {copiedUdev ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                udev 設定コマンドをコピー
              </Button>
              <Button size="sm" variant="secondary" onClick={() => onToggleDemoMode(true)}>
                デモモードで試す
              </Button>
            </div>
          </div>
        </div>
      )}

      {!state.permissionDenied && state.lastError && (
        <div role="alert" className="border-t border-destructive/30 bg-destructive/10 px-4 py-2">
          <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 text-xs">
            <div className="flex items-center gap-2 text-destructive font-medium">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              <span>{state.lastError}</span>
            </div>
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={onRefresh}>
              再接続を試す
            </Button>
          </div>
        </div>
      )}

      {showDisconnectedBanner && (
        <div className="border-t bg-muted/60 px-4 py-2">
          <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 text-xs">
            <div className="flex items-center gap-2 text-muted-foreground">
              <WifiOff className="h-4 w-4 shrink-0 text-primary" />
              <span>
                <strong className="text-foreground">Rapoo VT9 Pro 4K が検出されていません。</strong>{" "}
                実機なしでもデモモードで全機能をお試しいただけます。
              </span>
            </div>
            <Button size="sm" className="h-7 text-xs" onClick={() => onToggleDemoMode(true)}>
              <Sparkles className="h-3.5 w-3.5" />
              デモモードで全機能を試す
            </Button>
          </div>
        </div>
      )}

      {state.demoMode && (
        <div className="border-t border-primary/25 bg-primary/10 px-4 py-2">
          <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 text-xs">
            <div className="flex items-center gap-2">
              <FlaskConical className="h-3.5 w-3.5 text-primary shrink-0" />
              <span>
                <strong>仮想 VT9 Pro 4K デモモード動作中:</strong>{" "}
                すべての設定変更・プロファイル保存を安全にシミュレーションしています（実機には書き込みません）。
              </span>
            </div>
            <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => onToggleDemoMode(false)}>
              実機接続に戻る
            </Button>
          </div>
        </div>
      )}

      {showSleepingBanner && (
        <div className="border-t border-amber-500/35 bg-amber-500/10 px-4 py-2">
          <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 text-xs">
            <div className="flex items-center gap-2 text-amber-800 dark:text-amber-300">
              <Moon className="h-3.5 w-3.5 shrink-0" />
              <span>
                <strong>ワイヤレスマウスがスリープ中です。</strong>{" "}
                マウスを軽く動かすと自動復帰し、設定変更が可能になります
                {hasDeviceData ? "（直前の設定を表示中）" : ""}。
              </span>
            </div>
          </div>
        </div>
      )}

      {isGamingLocked && (
        <div className="border-t border-amber-500/35 bg-amber-500/10 px-4 py-1.5">
          <div className="mx-auto flex max-w-6xl items-center justify-between gap-2 text-xs">
            <span className="text-amber-800 dark:text-amber-300">
              <strong>ゲーム中ロック有効:</strong>{" "}
              4000Hzポーリングの安定性を最優先するため、USBコマンド送信と設定変更を一時ロックしています。
            </span>
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
              onClick={() => onToggleGamingLock(false)}
            >
              ロック解除
            </Button>
          </div>
        </div>
      )}
    </header>
  );
}
