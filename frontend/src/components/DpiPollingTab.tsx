import { useEffect, useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { normalizeDpi } from "@/lib/utils";
import type { DeviceSnapshot, DPIConfig, DPIStage } from "@/types/rapoo";

interface DpiPollingTabProps {
  snapshot: DeviceSnapshot;
  writable: boolean;
  pollingRates: number[];
  onSaveDpiConfig: (cfg: DPIConfig) => Promise<boolean>;
  onChangePollingRate: (hz: number) => Promise<boolean>;
}

const MAX_STAGE = 7;

function allStagesFrom(dpi: DPIConfig): DPIStage[] {
  return Array.from({ length: MAX_STAGE }, (_, i) => {
    const found =
      dpi.allStages?.find((stage) => stage.index === i + 1) ??
      dpi.stages.find((stage) => stage.index === i + 1);
    return found
      ? { ...found, index: i + 1 }
      : { index: i + 1, x: 800, y: 800, enabled: i < dpi.stageCount };
  });
}

export function DpiPollingTab({
  snapshot,
  writable,
  pollingRates,
  onSaveDpiConfig,
  onChangePollingRate,
}: DpiPollingTabProps) {
  const { dpi, sensor, model } = snapshot;
  const [stages, setStages] = useState<DPIStage[]>(dpi ? allStagesFrom(dpi) : []);
  const [stageCount, setStageCount] = useState<number>(dpi?.stageCount ?? 1);
  const [currentStage, setCurrentStage] = useState<number>(dpi?.currentStage ?? 1);
  const [independentXY, setIndependentXY] = useState<boolean>(dpi?.independentXY ?? false);

  useEffect(() => {
    if (!dpi) return;
    setStages(allStagesFrom(dpi));
    setStageCount(dpi.stageCount);
    setCurrentStage(dpi.currentStage);
    setIndependentXY(dpi.independentXY);
  }, [dpi]);

  if (!dpi || !sensor) return null;

  const persist = async (
    nextStages: DPIStage[],
    nextCount: number,
    nextCurrent: number,
    nextIndependent: boolean
  ) => {
    const normalized = nextStages.map((stage) => ({
      ...stage,
      x: normalizeDpi(stage.x),
      y: normalizeDpi(nextIndependent ? stage.y : stage.x),
    }));
    const config: DPIConfig = {
      currentStage: Math.min(nextCurrent, nextCount),
      stageCount: nextCount,
      independentXY: nextIndependent,
      stages: normalized
        .slice(0, nextCount)
        .map((stage, i) => ({ ...stage, index: i + 1, enabled: true })),
      allStages: normalized.map((stage, i) => ({ ...stage, index: i + 1, enabled: i < nextCount })),
    };

    const ok = await onSaveDpiConfig(config);
    if (!ok) {
      setStages(allStagesFrom(dpi));
      setStageCount(dpi.stageCount);
      setCurrentStage(dpi.currentStage);
      setIndependentXY(dpi.independentXY);
    }
  };

  const changeStageCount = (value: string) => {
    const next = Number(value);
    const nextCurrent = Math.min(currentStage, next);
    setStageCount(next);
    setCurrentStage(nextCurrent);
    void persist(stages, next, nextCurrent, independentXY);
  };

  const changeCurrentStage = (value: string) => {
    const next = Number(value);
    setCurrentStage(next);
    void persist(stages, stageCount, next, independentXY);
  };

  const changeIndependentXY = (checked: boolean) => {
    const nextStages = checked ? stages : stages.map((stage) => ({ ...stage, y: stage.x }));
    setIndependentXY(checked);
    setStages(nextStages);
    void persist(nextStages, stageCount, currentStage, checked);
  };

  const commitStage = (stage: DPIStage, axis: "x" | "y") => {
    const normalized = normalizeDpi(stage[axis]);
    const savedStage =
      dpi.allStages?.find((item) => item.index === stage.index) ??
      dpi.stages.find((item) => item.index === stage.index);
    if (savedStage && savedStage[axis] === normalized) {
      return;
    }
    const nextStages = stages.map((item) =>
      item.index === stage.index ? { ...item, [axis]: normalized } : item
    );
    setStages(nextStages);
    void persist(nextStages, stageCount, currentStage, independentXY);
  };

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>ステージ</CardTitle>
          <CardDescription>変更は自動で保存されます。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid gap-4 lg:grid-cols-3">
            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium">ステージ数</label>
              <Select value={String(stageCount)} onValueChange={changeStageCount} disabled={!writable}>
                <SelectTrigger className="w-full" aria-label="ステージ数">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Array.from({ length: MAX_STAGE }, (_, i) => i + 1).map((count) => (
                    <SelectItem key={count} value={String(count)}>
                      {count}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium">使用ステージ</label>
              <Select value={String(currentStage)} onValueChange={changeCurrentStage} disabled={!writable}>
                <SelectTrigger className="w-full" aria-label="使用ステージ">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Array.from({ length: stageCount }, (_, i) => i + 1).map((stage) => (
                    <SelectItem key={stage} value={String(stage)}>
                      Stage {stage}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium">X/Y 独立</label>
              <div className="flex h-10 items-center">
                <Switch
                  checked={independentXY}
                  onCheckedChange={changeIndependentXY}
                  disabled={!writable}
                  aria-label="X/Y 独立"
                />
              </div>
            </div>
          </div>

          <Table className="[&_th:first-child]:pl-0 [&_td:first-child]:pl-0 [&_th:last-child]:pr-0 [&_td:last-child]:pr-0">
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-28">ステージ</TableHead>
                <TableHead>X (DPI)</TableHead>
                {independentXY && <TableHead>Y (DPI)</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {stages.slice(0, stageCount).map((stage) => (
                <TableRow key={stage.index}>
                  <TableCell className="font-medium">Stage {stage.index}</TableCell>
                  <TableCell>
                    <Input
                      type="number"
                      min={50}
                      max={26000}
                      step={50}
                      value={stage.x}
                      aria-label={`Stage ${stage.index} X`}
                      disabled={!writable}
                      onChange={(event) =>
                        setStages((prev) =>
                          prev.map((item) =>
                            item.index === stage.index ? { ...item, x: Number(event.target.value) } : item
                          )
                        )
                      }
                      onBlur={() => commitStage(stage, "x")}
                      onKeyDown={(event) => {
                        if (event.key === "Enter") event.currentTarget.blur();
                      }}
                      className="w-32"
                    />
                  </TableCell>
                  {independentXY && (
                    <TableCell>
                      <Input
                        type="number"
                        min={50}
                        max={26000}
                        step={50}
                        value={stage.y}
                        aria-label={`Stage ${stage.index} Y`}
                        disabled={!writable}
                        onChange={(event) =>
                          setStages((prev) =>
                            prev.map((item) =>
                              item.index === stage.index ? { ...item, y: Number(event.target.value) } : item
                            )
                          )
                        }
                        onBlur={() => commitStage(stage, "y")}
                        onKeyDown={(event) => {
                          if (event.key === "Enter") event.currentTarget.blur();
                        }}
                        className="w-32"
                      />
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>ポーリングレート</CardTitle>
          <CardDescription>応答間隔。高いほど低遅延。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Select
            value={String(sensor.pollingRateHz)}
            onValueChange={(value) => void onChangePollingRate(Number(value))}
            disabled={!writable}
          >
            <SelectTrigger className="w-full" aria-label="ポーリングレート">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {pollingRates.map((hz) => (
                <SelectItem key={hz} value={String(hz)} disabled={hz > model.maxRateHz}>
                  {hz} Hz
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {model.maxRateHz < 4000 && (
            <p className="text-xs text-muted-foreground">
              接続中のため最大 {model.maxRateHz} Hz まで選択できます。
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
