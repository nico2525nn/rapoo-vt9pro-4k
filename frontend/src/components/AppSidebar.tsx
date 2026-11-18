import { Cpu, Crosshair, FolderCog, Moon, MousePointer, RefreshCw, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { DeviceSnapshot, ServiceState } from "@/types/rapoo";

interface AppSidebarProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  busy: boolean;
  darkMode: boolean;
  activeTab: string;
  onNavigate: (tab: string) => void;
  onToggleTheme: () => void;
  onRefresh: () => void;
}

const NAV_ITEMS = [
  { value: "dpi", label: "DPI", icon: Crosshair },
  { value: "sensor", label: "センサー", icon: Cpu },
  { value: "buttons", label: "ボタン", icon: MousePointer },
  { value: "profiles", label: "プロファイル", icon: FolderCog },
] as const;

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

export function AppSidebar({
  state,
  snapshot,
  busy,
  darkMode,
  activeTab,
  onNavigate,
  onToggleTheme,
  onRefresh,
}: AppSidebarProps) {
  const percentage = snapshot?.battery.percentage ?? state.percentage;
  const hasBattery = Boolean(snapshot) || state.hasBattery;
  const batteryText = hasBattery ? `${percentage}%` : "--%";
  const rate = snapshot?.sensor?.pollingRateHz ?? state.pollingRateHz;
  const dpi = activeDpiLabel(snapshot);

  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
      <div className="flex h-14 items-center gap-2 border-b border-sidebar-border px-4">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-sidebar-primary text-xs font-bold text-sidebar-primary-foreground">
          VT9
        </div>
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold">Rapoo VT9 Pro 4K</p>
          <p className="truncate text-xs text-muted-foreground">
            {connectionLabel(state)} · 電池 {batteryText}
          </p>
        </div>
      </div>

      <nav className="flex flex-1 flex-col gap-1 p-3">
        {NAV_ITEMS.map((item) => {
          const Icon = item.icon;
          const active = activeTab === item.value;
          return (
            <Button
              key={item.value}
              variant={active ? "secondary" : "ghost"}
              className="h-9 w-full justify-start gap-2 px-3"
              aria-current={active ? "page" : undefined}
              onClick={() => onNavigate(item.value)}
            >
              <Icon className="size-4" />
              {item.label}
            </Button>
          );
        })}
      </nav>

      <div className="border-t border-sidebar-border p-3">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0">
            <p className="truncate text-xs text-muted-foreground">{dpi ?? "—"}</p>
            <p className="truncate text-xs text-muted-foreground">
              {rate ? `${rate} Hz` : "—"}
              {state.mode === "passive_only" ? " · 0-TX" : ""}
            </p>
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
                  <RefreshCw className={cn("size-4", busy && "animate-spin")} />
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
                  {darkMode ? <Sun className="size-4" /> : <Moon className="size-4" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>{darkMode ? "ライトテーマ" : "ダークテーマ"}</TooltipContent>
            </Tooltip>
          </div>
        </div>
      </div>
    </aside>
  );
}
