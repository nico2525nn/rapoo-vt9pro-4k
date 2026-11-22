import type { CSSProperties } from "react";
import { Cpu, Crosshair, FolderOpen, Mouse, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import { windowApi } from "@/lib/wails";
import type { DeviceSnapshot, ServiceState } from "@/types/rapoo";

const dragStyle = { "--wails-draggable": "drag" } as CSSProperties;

interface AppSidebarProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  activeTab: Page;
  onNavigate: (tab: Page) => void;
}

export const NAV_ITEMS = [
  { value: "dpi", label: "DPI", icon: Crosshair },
  { value: "sensor", label: "センサー", icon: Cpu },
  { value: "buttons", label: "ボタン", icon: Mouse },
  { value: "profiles", label: "プロファイル", icon: FolderOpen },
  { value: "settings", label: "設定", icon: Settings },
] as const;

export type Page = (typeof NAV_ITEMS)[number]["value"];

const number = new Intl.NumberFormat("ja-JP");

function connectionLabel(state: ServiceState): string {
  if (!state.connected) return "未接続";
  if (state.demoMode) return "デモ";
  if (state.sleeping) return "スリープ";
  return state.connection === "wired" ? "USB-C" : "4K";
}

function activeDpiLabel(snapshot: DeviceSnapshot | null): string | null {
  const dpi = snapshot?.dpi;
  if (!dpi) return null;
  const stage = dpi.stages.find((stage) => stage.index === dpi.currentStage);
  if (!stage) return null;
  return dpi.independentXY ? `${number.format(stage.x)}/${number.format(stage.y)} DPI` : `${number.format(stage.x)} DPI`;
}

export function AppSidebar({ state, snapshot, activeTab, onNavigate }: AppSidebarProps) {
  const percentage = state.hasBattery ? state.percentage : snapshot?.battery.percentage;
  const rate = state.pollingRateHz ?? snapshot?.sensor?.pollingRateHz;
  const dpi = activeDpiLabel(snapshot) ?? (state.activeDpi ? `${number.format(state.activeDpi)} DPI` : null);
  const status = [
    connectionLabel(state),
    state.mode === "passive_only" ? "0-TX" : null,
    state.connected ? dpi : null,
    state.connected && rate ? `${number.format(rate)} Hz` : null,
    state.connected && percentage !== undefined ? `${percentage}%` : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <aside className="flex w-60 shrink-0 flex-col bg-sidebar px-4 text-sidebar-foreground">
      <div
        className="flex h-10 shrink-0 items-center px-2"
        style={dragStyle}
        onDoubleClick={() => void windowApi.toggleMaximise()}
      >
        <p className="truncate text-sm font-semibold">Rapoo Control</p>
      </div>

      <nav aria-label="メインメニュー" className="flex flex-1 flex-col gap-1 pt-3">
        {NAV_ITEMS.map((item) => {
          const Icon = item.icon;
          const active = activeTab === item.value;
          return (
            <Button
              key={item.value}
              type="button"
              variant="ghost"
              aria-current={active ? "page" : undefined}
              onClick={() => onNavigate(item.value)}
              className="h-10 w-full justify-start gap-3 px-3 font-normal text-sidebar-foreground hover:bg-sidebar-accent/70 aria-[current=page]:bg-sidebar-accent dark:hover:bg-sidebar-accent/70 dark:aria-[current=page]:hover:bg-sidebar-accent motion-reduce:transition-none"
            >
              <Icon aria-hidden="true" />
              {item.label}
            </Button>
          );
        })}
      </nav>

      <p role="status" title={status} className="truncate px-2 py-4 text-xs text-muted-foreground tabular-nums">
        {status}
      </p>
    </aside>
  );
}
