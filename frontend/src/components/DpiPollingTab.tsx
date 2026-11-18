import { Check, Crosshair, Gauge, Minus, Plus, Zap } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import {
  cn,
  DPI_STEP,
  estimateBatteryHours,
  MAX_DPI,
  MIN_DPI,
  normalizeDpi,
  QUICK_DPI_PRESETS,
  STAGE_COLORS,
} from "@/lib/utils";
import type { DeviceSnapshot, DPIConfig, DPIStage } from "@/types/rapoo";

interface DpiPollingTabProps {
  snapshot: DeviceSnapshot;
  writable: boolean;
  pollingRates: number[];
  onSaveDpiConfig: (cfg: DPIConfig) => Promise<boolean>;
  onSelectActiveStage: (stage: number) => Promise<boolean>;
  onChangePollingRate: (hz: number) => Promise<boolean>;
}

const POLLING_META: Record<number, { intervalMs: string; tag: string }> = {
  125: { intervalMs: "8.0 ms", tag: "超省電力" },
  250: { intervalMs: "4.0 ms", tag: "オフィス" },
  500: { intervalMs: "2.0 ms", tag: "長寿命・安定" },
  1000: { intervalMs: "1.0 ms", tag: "ゲーミング標準" },
  2000: { intervalMs: "0.5 ms", tag: "高速ワイヤレス" },
  4000: { intervalMs: "0.25 ms", tag: "4K 競技最速" },
};

function ensureAllStages(dpi: DPIConfig): DPIStage[] {
  const base = dpi.allStages?.length === 7 ? dpi.allStages : dpi.stages;
  return Array.from({ length: 7 }, (_, i) => {
    const found = base.find((s) => s.index === i + 1);
    return found ? { ...found } : { index: i + 1, x: 800, y: 800, enabled: i < dpi.stageCount };
  });
}

export function DpiPollingTab({
  snapshot,
  writable,
  pollingRates,
  onSaveDpiConfig,
  onSelectActiveStage,
  onChangePollingRate,
}: DpiPollingTabProps) {
  const dpi = snapshot.dpi;
  const sensor = snapshot.sensor;

  const [localStages, setLocalStages] = useState<DPIStage[]>(() => (dpi ? ensureAllStages(dpi) : []));
  const [independentXY, setIndependentXY] = useState<boolean>(() => Boolean(dpi?.independentXY));
  const [editingStage, setEditingStage] = useState<number>(() => dpi?.currentStage ?? 1);
  const [xInputText, setXInputText] = useState<string>("800");
  const [yInputText, setYInputText] = useState<string>("800");

  // Track whether the user manually unlocked X/Y on the current stage while X === Y
  const userUnlockedXYRef = useRef<boolean>(false);
  const prevHardwareStageRef = useRef<number>(dpi?.currentStage ?? 1);
  const stepperDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (stepperDebounceRef.current) clearTimeout(stepperDebounceRef.current);
    };
  }, []);

  useEffect(() => {
    if (!dpi) return;
    setLocalStages(ensureAllStages(dpi));
    if (dpi.independentXY) {
      setIndependentXY(true);
    } else if (!userUnlockedXYRef.current) {
      setIndependentXY(false);
    }

    // If hardware stage changed externally (e.g. physical DPI+/DPI- button or preset), follow it
    if (dpi.currentStage !== prevHardwareStageRef.current) {
      prevHardwareStageRef.current = dpi.currentStage;
      setEditingStage(dpi.currentStage);
    } else {
      setEditingStage((prev) => (prev >= 1 && prev <= dpi.stageCount ? prev : dpi.currentStage));
    }
  }, [dpi]);

  const currentStageObj = localStages[editingStage - 1];
  useEffect(() => {
    if (!currentStageObj) return;
    setXInputText(String(currentStageObj.x));
    setYInputText(String(currentStageObj.y));
  }, [editingStage, currentStageObj?.x, currentStageObj?.y]);

  if (!dpi || !sensor) {
    return null;
  }

  const stageCount = dpi.stageCount;
  const activeStage = dpi.currentStage;
  const maxRateHz = snapshot.model.maxRateHz || 4000;

  const rollbackToHardware = () => {
    const clean = ensureAllStages(dpi);
    setLocalStages(clean);
    setIndependentXY(dpi.independentXY);
    const cur = clean[editingStage - 1];
    if (cur) {
      setXInputText(String(cur.x));
      setYInputText(String(cur.y));
    }
  };

  const buildConfig = (
    nextAll: DPIStage[],
    nextCount = stageCount,
    nextActive = activeStage,
    nextXY = independentXY
  ): DPIConfig => {
    const clampedCount = Math.max(1, Math.min(7, nextCount));
    const clampedActive = Math.max(1, Math.min(clampedCount, nextActive));
    const normalizedAll = nextAll.map((s, idx) => {
      const nx = normalizeDpi(s.x);
      const ny = nextXY ? normalizeDpi(s.y) : nx;
      return {
        index: idx + 1,
        x: nx,
        y: ny,
        enabled: idx < clampedCount,
      };
    });
    return {
      currentStage: clampedActive,
      stageCount: clampedCount,
      independentXY: nextXY,
      stages: normalizedAll.slice(0, clampedCount),
      allStages: normalizedAll,
    };
  };

  const previewStageDpi = (stageIdx: number, nextX: number, nextY?: number) => {
    setLocalStages((prev) =>
      prev.map((s) =>
        s.index === stageIdx
          ? {
              ...s,
              x: nextX,
              y: independentXY ? (nextY ?? s.y) : nextX,
            }
          : s
      )
    );
  };

  const commitStageDpi = async (stageIdx: number, rawX: number, rawY?: number) => {
    if (stepperDebounceRef.current) {
      clearTimeout(stepperDebounceRef.current);
      stepperDebounceRef.current = null;
    }
    const nx = normalizeDpi(rawX);
    const ny = independentXY ? normalizeDpi(rawY ?? currentStageObj?.y ?? nx) : nx;
    const nextAll = localStages.map((s) => (s.index === stageIdx ? { ...s, x: nx, y: ny } : s));
    setLocalStages(nextAll);
    setXInputText(String(nx));
    setYInputText(String(ny));
    const ok = await onSaveDpiConfig(buildConfig(nextAll, stageCount, activeStage, independentXY));
    if (!ok) {
      rollbackToHardware();
    }
  };

  // Coalesce rapid +/- stepper clicks or keyboard arrow nudges without dropping focus
  const scheduleCoalescedDpiCommit = (stageIdx: number, rawX: number, rawY?: number) => {
    const nx = normalizeDpi(rawX);
    const ny = independentXY ? normalizeDpi(rawY ?? currentStageObj?.y ?? nx) : nx;
    const nextAll = localStages.map((s) => (s.index === stageIdx ? { ...s, x: nx, y: ny } : s));
    setLocalStages(nextAll);
    setXInputText(String(nx));
    setYInputText(String(ny));

    if (stepperDebounceRef.current) clearTimeout(stepperDebounceRef.current);
    stepperDebounceRef.current = setTimeout(() => {
      stepperDebounceRef.current = null;
      void onSaveDpiConfig(buildConfig(nextAll, stageCount, activeStage, independentXY)).then((ok) => {
        if (!ok) rollbackToHardware();
      });
    }, 150);
  };

  const handleStageCountChange = async (value: string) => {
    const nextCount = Number(value);
    const nextActive = Math.min(activeStage, nextCount);
    if (editingStage > nextCount) {
      setEditingStage(nextCount);
    }
    const ok = await onSaveDpiConfig(buildConfig(localStages, nextCount, nextActive, independentXY));
    if (!ok) rollbackToHardware();
  };

  const handleToggleIndependentXY = async (checked: boolean) => {
    userUnlockedXYRef.current = checked;
    setIndependentXY(checked);
    if (!checked) {
      const synced = localStages.map((s) => ({ ...s, y: s.x }));
      setLocalStages(synced);
      const hadMismatch = localStages.slice(0, stageCount).some((s) => s.x !== s.y);
      if (hadMismatch && writable) {
        const ok = await onSaveDpiConfig(buildConfig(synced, stageCount, activeStage, false));
        if (!ok) rollbackToHardware();
      }
    }
  };

  const handleTileClick = async (stageIndex: number) => {
    setEditingStage(stageIndex);
    if (writable && stageIndex !== activeStage) {
      prevHardwareStageRef.current = stageIndex;
      await onSelectActiveStage(stageIndex);
    }
  };

  const editingColor = STAGE_COLORS[(editingStage - 1) % STAGE_COLORS.length];
  const batteryPctForEstimate =
    snapshot.battery.online || snapshot.battery.percentage > 0 ? snapshot.battery.percentage : 100;
  const hasRealBatteryPct = snapshot.battery.online || snapshot.battery.percentage > 0;

  return (
    <div className="grid gap-4 md:grid-cols-12">
      {/* Left 7 cols: DPI Stages + Fine Editor */}
      <div className="space-y-4 md:col-span-7">
        <Card>
          <CardHeader className="pb-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <Crosshair className="h-4 w-4 text-primary" />
                  オンボード DPI ステージ設定
                </CardTitle>
                <CardDescription>
                  50 〜 26,000 DPI を 50 DPI 刻みで本体メモリに保存します。ステージをクリックすると即座に切り替わります。
                </CardDescription>
              </div>
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-muted-foreground whitespace-nowrap">ステージ数</span>
                  <Select
                    value={String(stageCount)}
                    onValueChange={(v) => void handleStageCountChange(v)}
                    disabled={!writable}
                  >
                    <SelectTrigger className="h-8 w-[96px] text-xs tabular-nums" aria-label="有効なDPIステージ数">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {[1, 2, 3, 4, 5, 6, 7].map((n) => (
                        <SelectItem key={n} value={String(n)}>
                          {n} 段階
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <label className="flex items-center gap-1.5 text-xs cursor-pointer">
                  <Switch
                    checked={independentXY}
                    onCheckedChange={(v) => void handleToggleIndependentXY(v)}
                    disabled={!writable}
                    aria-label="X軸とY軸のDPIを個別に設定"
                  />
                  <span>X/Y 独立</span>
                </label>
              </div>
            </div>
          </CardHeader>

          <CardContent className="space-y-4">
            {/* Semantic Single-Button Stage Grid (Zero nested buttons; 1-click activate + edit) */}
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4" role="group" aria-label="DPIステージ一覧">
              {localStages.slice(0, stageCount).map((st) => {
                const color = STAGE_COLORS[(st.index - 1) % STAGE_COLORS.length];
                const isActive = st.index === activeStage;
                const isEditing = st.index === editingStage;
                return (
                  <button
                    key={st.index}
                    type="button"
                    aria-pressed={isActive}
                    onClick={() => void handleTileClick(st.index)}
                    className={cn(
                      "group relative flex flex-col justify-between rounded-lg border p-2.5 text-left transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background",
                      isEditing
                        ? "border-primary bg-primary/10 ring-1 ring-primary/40"
                        : "hover:border-muted-foreground/50 bg-background/45"
                    )}
                  >
                    <div className="flex items-center justify-between gap-1 mb-1.5 w-full">
                      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                        <span
                          className="h-2.5 w-2.5 rounded-full shrink-0"
                          style={{ backgroundColor: color.hex }}
                        />
                        Stage {st.index}
                      </span>
                      {isActive ? (
                        <span className="inline-flex items-center gap-0.5 rounded bg-emerald-500/15 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-300">
                          <Check className="h-2.5 w-2.5" />
                          適用中
                        </span>
                      ) : writable ? (
                        <span className="rounded border border-border/80 bg-card px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground group-hover:border-primary/60 group-hover:text-primary transition-colors">
                          適用
                        </span>
                      ) : null}
                    </div>

                    <div className="tabular-nums">
                      {independentXY ? (
                        <div className="text-sm font-bold leading-tight">
                          <span>X:{st.x}</span>
                          <span className="mx-1 text-muted-foreground">/</span>
                          <span>Y:{st.y}</span>
                        </div>
                      ) : (
                        <div className="text-lg font-bold tracking-tight">
                          {st.x.toLocaleString()}
                          <span className="ml-1 text-[11px] font-normal text-muted-foreground">DPI</span>
                        </div>
                      )}
                    </div>
                  </button>
                );
              })}
            </div>

            {/* Detailed Editor for Selected Stage */}
            {currentStageObj && (
              <div className="rounded-lg border bg-muted/35 p-3.5 space-y-3.5">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <span
                      className="h-3 w-3 rounded-full"
                      style={{ backgroundColor: editingColor.hex }}
                    />
                    <span className="text-sm font-semibold">
                      Stage {editingStage} の感度編集
                    </span>
                    {editingStage === activeStage ? (
                      <Badge variant="success">現在マウスに適用中</Badge>
                    ) : (
                      <Button
                        size="sm"
                        variant="secondary"
                        className="h-7 text-xs"
                        disabled={!writable}
                        onClick={() => void onSelectActiveStage(editingStage)}
                      >
                        Stage {editingStage} を適用
                      </Button>
                    )}
                  </div>
                  <span className="text-[11px] text-muted-foreground">50 DPI 刻み • 離すと本体へ保存</span>
                </div>

                {/* X (or Unified) Axis Slider + Direct Input */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <label htmlFor="dpi-x-input" className="text-xs font-medium text-muted-foreground">
                      {independentXY ? "X軸 (水平) DPI" : "DPI 感度"}
                    </label>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-7 w-7"
                        disabled={!writable || currentStageObj.x <= MIN_DPI}
                        aria-label="DPIを50下げる"
                        onClick={() => scheduleCoalescedDpiCommit(editingStage, currentStageObj.x - DPI_STEP)}
                      >
                        <Minus className="h-3 w-3" />
                      </Button>
                      <Input
                        id="dpi-x-input"
                        type="number"
                        min={MIN_DPI}
                        max={MAX_DPI}
                        step={DPI_STEP}
                        disabled={!writable}
                        value={xInputText}
                        onChange={(e) => setXInputText(e.target.value)}
                        onBlur={() => {
                          const parsed = Number(xInputText);
                          if (Number.isFinite(parsed) && normalizeDpi(parsed) !== currentStageObj.x) {
                            void commitStageDpi(editingStage, parsed);
                          } else {
                            setXInputText(String(currentStageObj.x));
                          }
                        }}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.currentTarget.blur();
                          }
                        }}
                        className="h-7 w-24 text-right font-mono text-xs tabular-nums [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                      />
                      <Button
                        variant="outline"
                        size="icon"
                        className="h-7 w-7"
                        disabled={!writable || currentStageObj.x >= MAX_DPI}
                        aria-label="DPIを50上げる"
                        onClick={() => scheduleCoalescedDpiCommit(editingStage, currentStageObj.x + DPI_STEP)}
                      >
                        <Plus className="h-3 w-3" />
                      </Button>
                    </div>
                  </div>

                  <Slider
                    aria-label={independentXY ? "X軸 DPI スライダー" : "DPI スライダー"}
                    min={MIN_DPI}
                    max={MAX_DPI}
                    step={DPI_STEP}
                    disabled={!writable}
                    value={[currentStageObj.x]}
                    onValueChange={([val]) => {
                      previewStageDpi(editingStage, val);
                      setXInputText(String(val));
                    }}
                    onValueCommit={([val]) => {
                      scheduleCoalescedDpiCommit(editingStage, val);
                    }}
                  />
                </div>

                {/* Y Axis Slider (when Independent X/Y is ON) */}
                {independentXY && (
                  <div className="space-y-2 pt-1 border-t border-border/60">
                    <div className="flex items-center justify-between gap-2">
                      <label htmlFor="dpi-y-input" className="text-xs font-medium text-muted-foreground">
                        Y軸 (垂直) DPI
                      </label>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-7 w-7"
                          disabled={!writable || currentStageObj.y <= MIN_DPI}
                          aria-label="Y軸DPIを50下げる"
                          onClick={() =>
                            scheduleCoalescedDpiCommit(editingStage, currentStageObj.x, currentStageObj.y - DPI_STEP)
                          }
                        >
                          <Minus className="h-3 w-3" />
                        </Button>
                        <Input
                          id="dpi-y-input"
                          type="number"
                          min={MIN_DPI}
                          max={MAX_DPI}
                          step={DPI_STEP}
                          disabled={!writable}
                          value={yInputText}
                          onChange={(e) => setYInputText(e.target.value)}
                          onBlur={() => {
                            const parsed = Number(yInputText);
                            if (Number.isFinite(parsed) && normalizeDpi(parsed) !== currentStageObj.y) {
                              void commitStageDpi(editingStage, currentStageObj.x, parsed);
                            } else {
                              setYInputText(String(currentStageObj.y));
                            }
                          }}
                          onKeyDown={(e) => {
                            if (e.key === "Enter") {
                              e.currentTarget.blur();
                            }
                          }}
                          className="h-7 w-24 text-right font-mono text-xs tabular-nums [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                        />
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-7 w-7"
                          disabled={!writable || currentStageObj.y >= MAX_DPI}
                          aria-label="Y軸DPIを50上げる"
                          onClick={() =>
                            scheduleCoalescedDpiCommit(editingStage, currentStageObj.x, currentStageObj.y + DPI_STEP)
                          }
                        >
                          <Plus className="h-3 w-3" />
                        </Button>
                      </div>
                    </div>

                    <Slider
                      aria-label="Y軸 DPI スライダー"
                      min={MIN_DPI}
                      max={MAX_DPI}
                      step={DPI_STEP}
                      disabled={!writable}
                      value={[currentStageObj.y]}
                      onValueChange={([val]) => {
                        previewStageDpi(editingStage, currentStageObj.x, val);
                        setYInputText(String(val));
                      }}
                      onValueCommit={([val]) => {
                        scheduleCoalescedDpiCommit(editingStage, currentStageObj.x, val);
                      }}
                    />
                  </div>
                )}

                {/* Quick Esport Presets */}
                <div className="space-y-1.5 pt-1">
                  <span className="text-[11px] text-muted-foreground">定番 DPI ワンクリック入力:</span>
                  <div className="flex flex-wrap gap-1.5">
                    {QUICK_DPI_PRESETS.map((presetVal) => {
                      const selected =
                        currentStageObj.x === presetVal && (!independentXY || currentStageObj.y === presetVal);
                      return (
                        <Button
                          key={presetVal}
                          size="sm"
                          variant={selected ? "default" : "outline"}
                          disabled={!writable}
                          className="h-7 px-2.5 text-xs tabular-nums"
                          onClick={() => void commitStageDpi(editingStage, presetVal, presetVal)}
                        >
                          {presetVal}
                        </Button>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Right 5 cols: Polling Rate & Battery Impact */}
      <div className="space-y-4 md:col-span-5">
        <Card className="h-full">
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2">
              <Gauge className="h-4 w-4 text-primary" />
              ポーリングレート (USBレポートレート)
            </CardTitle>
            <CardDescription>
              1秒あたりの通信回数を選択します。高レートほど入力遅延が短縮され、低レートほどバッテリーが長持ちします。
            </CardDescription>
          </CardHeader>

          <CardContent className="space-y-3">
            {maxRateHz < 4000 && (
              <div className="rounded-md border border-amber-500/35 bg-amber-500/10 p-2.5 text-xs text-amber-800 dark:text-amber-300">
                USB-C 有線接続中はハードウェア仕様により最大 <strong>1000 Hz</strong> までとなります。
                <strong>2000 / 4000 Hz</strong> は付属の 4K ワイヤレスレシーバー接続時に選択できます。
              </div>
            )}

            <div className="grid grid-cols-2 gap-2">
              {pollingRates.map((hz) => {
                const meta = POLLING_META[hz] ?? { intervalMs: "-", tag: "" };
                const isSelected = sensor.pollingRateHz === hz;
                const unsupported = hz > maxRateHz;
                const estH = estimateBatteryHours(batteryPctForEstimate, hz);

                return (
                  <button
                    key={hz}
                    type="button"
                    disabled={!writable || unsupported}
                    onClick={() => void onChangePollingRate(hz)}
                    aria-pressed={isSelected}
                    className={cn(
                      "flex flex-col justify-between rounded-lg border p-3 text-left transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background disabled:opacity-40 disabled:cursor-not-allowed",
                      isSelected
                        ? "border-primary bg-primary/10 ring-1 ring-primary/40"
                        : "bg-background/45 hover:border-muted-foreground/50"
                    )}
                  >
                    <div className="flex items-center justify-between gap-1 w-full">
                      <span className="text-base font-bold tabular-nums tracking-tight flex items-center gap-1">
                        {hz >= 2000 && <Zap className="h-3.5 w-3.5 text-primary" />}
                        {hz} Hz
                      </span>
                      <span className="text-[11px] font-mono text-muted-foreground tabular-nums">
                        {meta.intervalMs}
                      </span>
                    </div>

                    <div className="mt-2 flex items-center justify-between gap-1 text-[11px]">
                      <span className={cn("font-medium", isSelected ? "text-primary" : "text-muted-foreground")}>
                        {meta.tag}
                      </span>
                      {snapshot.model.connection !== "wired" && (
                        <span className="text-muted-foreground tabular-nums">
                          {hasRealBatteryPct ? `残り約${estH}h` : `満充電${estH}h`}
                        </span>
                      )}
                    </div>
                  </button>
                );
              })}
            </div>

            <div className="rounded-lg border bg-muted/35 p-3 text-xs text-muted-foreground space-y-1.5 leading-relaxed">
              <p className="font-medium text-foreground">レート選びの目安（800mAh バッテリー）</p>
              <p>
                • <strong>4000 Hz (0.25ms):</strong> 240Hz以上の高リフレッシュレート環境向け（満充電で約48時間 • ゲーム内「Raw Input」ON推奨）。
              </p>
              <p>
                • <strong>1000 Hz (1.0ms):</strong> 競技性と省電力を高次元で両立（満充電で約160時間駆動）。
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
