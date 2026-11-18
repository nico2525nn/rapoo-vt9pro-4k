import { Loader2, Moon, RefreshCw, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { DeviceSnapshot, ServiceState } from "@/types/rapoo";

interface HeaderBarProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  busy: boolean;
  darkMode: boolean;
  onToggleTheme: () => void;
  onRefresh: () => void;
}

function connectionLabel(state: ServiceState): string {
  if (!state.connected) return "未接続";
  if (state.demoMode) return "デモモード";
  if (state.sleeping) return "スリープ中";
  return state.connection === "wired" ? "USB-C" : "4K ワイヤレス";
}

function activeDpiLabel(snapshot: DeviceSnapshot | null): string | null {
  if (!snapshot?.dpi) return null;
  const index = Math.min(Math.max(snapshot.dpi.currentStage - 1, 0), snapshot.dpi.stages.length - 1);
  const stage = snapshot.dpi.stages[index];
  if (!stage) return null;
  return snapshot.dpi.independentXY ? `${stage.x}/${stage.y} DPI` : `${stage.x} DPI`;
}

export function HeaderBar({
  state,
  snapshot,
  busy,
  darkMode,
  onToggleTheme,
  onRefresh,
}: HeaderBarProps) {
  const percentage = snapshot?.battery.percentage ?? state.percentage;
  const hasBattery = Boolean(snapshot) || state.hasBattery;
  const charging = snapshot?.battery.charging ?? state.charging;
  const batteryText = hasBattery ? `${percentage}%` : "--%";
  const rate = snapshot?.sensor?.pollingRateHz ?? state.pollingRateHz;

  const summary = [
    connectionLabel(state),
    state.mode === "passive_only" ? "0-TX ロック" : null,
    activeDpiLabel(snapshot),
    rate ? `${rate} Hz` : null,
    `電池 ${batteryText}${charging ? " (充電中)" : ""}`,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <header className="sticky top-0 z-20 border-b bg-background">
      <div className="mx-auto flex h-14 max-w-4xl items-center justify-between gap-4 px-6">
        <div className="flex min-w-0 items-baseline gap-3">
          <h1 className="shrink-0 text-sm font-semibold">Rapoo VT9 Pro 4K</h1>
          <p className="truncate text-xs text-muted-foreground">{summary}</p>
          {busy && <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" aria-label="読み込み中" />}
        </div>

        <div className="flex shrink-0 items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={onRefresh}
                disabled={busy || !state.connected}
                aria-label="再読み込み"
              >
                <RefreshCw className={cn("h-4 w-4", busy && "animate-spin")} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>再読み込み</TooltipContent>
          </Tooltip>
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
            <TooltipContent>{darkMode ? "ライトテーマ" : "ダークテーマ"}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {state.lastError && (
        <div role="alert" className="border-t bg-destructive/10 px-6 py-2 text-xs text-destructive">
          {state.lastError}
        </div>
      )}
    </header>
  );
}
