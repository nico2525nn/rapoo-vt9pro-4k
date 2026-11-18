import { Loader2, Moon, RefreshCw, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
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

  return (
    <header className="border-b">
      <div className="mx-auto flex h-14 max-w-4xl items-center justify-between gap-4 px-6">
        <div className="flex min-w-0 items-baseline gap-3">
          <h1 className="truncate text-sm font-semibold">Rapoo VT9 Pro 4K</h1>
          <p className="truncate text-xs text-muted-foreground">
            {connectionLabel(state)} · 電池 {batteryText}
            {charging ? " · 充電中" : ""}
          </p>
          {busy && <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" aria-label="読み込み中" />}
        </div>

        <div className="flex shrink-0 items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={onRefresh}
            disabled={busy || !state.connected}
            aria-label="再読み込み"
          >
            <RefreshCw className={cn("h-4 w-4", busy && "animate-spin")} />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={onToggleTheme}
            aria-label={darkMode ? "ライトテーマに切り替え" : "ダークテーマに切り替え"}
          >
            {darkMode ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </Button>
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
