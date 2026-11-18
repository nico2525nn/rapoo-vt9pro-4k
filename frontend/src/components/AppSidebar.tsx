import type { CSSProperties } from "react";
import {
  Cpu,
  Crosshair,
  FolderCog,
  Moon,
  MousePointer,
  RefreshCw,
  Sun,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { windowApi } from "@/lib/wails";
import type { DeviceSnapshot, ServiceState } from "@/types/rapoo";

const dragStyle = { "--wails-draggable": "drag" } as CSSProperties;

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
      <div
        className="border-b border-sidebar-border px-4 py-2.5"
        style={dragStyle}
        onDoubleClick={() => void windowApi.toggleMaximise()}
      >
        <p className="truncate text-sm font-semibold">Rapoo VT9 Pro 4K</p>
        <p className="truncate text-xs text-muted-foreground">
          {connectionLabel(state)} · 電池 {batteryText}
        </p>
      </div>

      <nav className="flex flex-1 flex-col gap-1 p-2">
        {NAV_ITEMS.map((item) => {
          const Icon = item.icon;
          const active = activeTab === item.value;
          return (
            <button
              key={item.value}
              type="button"
              aria-current={active ? "page" : undefined}
              onClick={() => onNavigate(item.value)}
              className={cn(
                "flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring",
                active
                  ? "bg-sidebar-accent text-sidebar-accent-foreground"
                  : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground"
              )}
            >
              <Icon className="size-4" />
              {item.label}
            </button>
          );
        })}
      </nav>

      <div className="border-t border-sidebar-border p-4">
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
                  className="size-8"
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
                  className="size-8"
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
