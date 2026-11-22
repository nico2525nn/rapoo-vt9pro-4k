import type { CSSProperties } from "react";
import { Cpu, Crosshair, FolderCog, MousePointer } from "lucide-react";
import { cn } from "@/lib/utils";
import { windowApi } from "@/lib/wails";
import type { DeviceSnapshot, ServiceState } from "@/types/rapoo";

const dragStyle = { "--wails-draggable": "drag" } as CSSProperties;

interface AppSidebarProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  activeTab: string;
  onNavigate: (tab: string) => void;
}

const NAV_ITEMS = [
  { value: "dpi", label: "DPI", icon: Crosshair },
  { value: "sensor", label: "センサー", icon: Cpu },
  { value: "buttons", label: "ボタン", icon: MousePointer },
  { value: "profiles", label: "プロファイル", icon: FolderCog },
] as const;

function connectionLabel(state: ServiceState): string {
  if (!state.connected) return "未接続";
  if (state.demoMode) return "デモ";
  if (state.sleeping) return "スリープ";
  return state.connection === "wired" ? "USB-C" : "4K";
}

function activeDpiLabel(snapshot: DeviceSnapshot | null): string | null {
  if (!snapshot?.dpi) return null;
  const index = Math.min(Math.max(snapshot.dpi.currentStage - 1, 0), snapshot.dpi.stages.length - 1);
  const stage = snapshot.dpi.stages[index];
  if (!stage) return null;
  return snapshot.dpi.independentXY ? `${stage.x}/${stage.y} DPI` : `${stage.x} DPI`;
}

export function AppSidebar({ state, snapshot, activeTab, onNavigate }: AppSidebarProps) {
  const percentage = snapshot?.battery.percentage ?? state.percentage;
  const hasBattery = Boolean(snapshot) || state.hasBattery;
  const rate = snapshot?.sensor?.pollingRateHz ?? state.pollingRateHz;
  const status = [
    connectionLabel(state),
    state.mode === "passive_only" ? "0-TX" : null,
    activeDpiLabel(snapshot),
    rate ? `${rate} Hz` : null,
    `電池 ${hasBattery ? `${percentage}%` : "--%"}`,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <aside className="flex w-60 shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground">
      <div
        className="border-b border-sidebar-border px-4 py-3"
        style={dragStyle}
        onDoubleClick={() => void windowApi.toggleMaximise()}
      >
        <p className="truncate text-sm font-semibold">Rapoo VT9 Pro 4K</p>
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
                "relative flex h-10 w-full items-center gap-3 rounded-md px-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring",
                active
                  ? "bg-sidebar-accent text-sidebar-accent-foreground"
                  : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground"
              )}
            >
              <Icon className="size-4 shrink-0" />
              {item.label}
            </button>
          );
        })}
      </nav>

      <p className="truncate border-t border-sidebar-border px-4 py-2.5 text-xs text-muted-foreground">
        {status}
      </p>
    </aside>
  );
}
