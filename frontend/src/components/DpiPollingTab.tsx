import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!dpi) return;
    setStages(allStagesFrom(dpi));
    setStageCount(dpi.stageCount);
    setCurrentStage(dpi.currentStage);
    setIndependentXY(dpi.independentXY);
  }, [dpi]);

  if (!dpi || !sensor) return null;

  const updateStage = (index: number, axis: "x" | "y", value: number) => {
    setStages((prev) =>
      prev.map((stage) => (stage.index === index ? { ...stage, [axis]: value } : stage))
    );
  };

  const changeStageCount = (value: string) => {
    const next = Number(value);
    setStageCount(next);
    setCurrentStage((prev) => Math.min(prev, next));
  };

  const changeIndependentXY = (checked: boolean) => {
    setIndependentXY(checked);
    if (!checked) {
      setStages((prev) => prev.map((stage) => ({ ...stage, y: stage.x })));
    }
  };

  const handleSave = async () => {
    const normalized = stages.map((stage) => ({
      ...stage,
      x: normalizeDpi(stage.x),
      y: normalizeDpi(independentXY ? stage.y : stage.x),
    }));
    const config: DPIConfig = {
      currentStage,
      stageCount,
      independentXY,
      stages: normalized
        .slice(0, stageCount)
        .map((stage, i) => ({ ...stage, index: i + 1, enabled: true })),
      allStages: normalized.map((stage, i) => ({ ...stage, index: i + 1, enabled: i < stageCount })),
    };

    setSaving(true);
    try {
      const ok = await onSaveDpiConfig(config);
      if (!ok) {
        setStages(allStagesFrom(dpi));
        setStageCount(dpi.stageCount);
        setCurrentStage(dpi.currentStage);
        setIndependentXY(dpi.independentXY);
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>DPI</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="space-y-2">
              <label className="text-sm font-medium">ステージ数</label>
              <Select value={String(stageCount)} onValueChange={changeStageCount} disabled={!writable || saving}>
                <SelectTrigger aria-label="ステージ数">
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

            <div className="space-y-2">
              <label className="text-sm font-medium">使用ステージ</label>
              <Select
                value={String(currentStage)}
                onValueChange={(value) => setCurrentStage(Number(value))}
                disabled={!writable || saving}
              >
                <SelectTrigger aria-label="使用ステージ">
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

            <div className="space-y-2">
              <label className="text-sm font-medium">X/Y 独立</label>
              <div className="flex h-10 items-center">
                <Switch
                  checked={independentXY}
                  onCheckedChange={changeIndependentXY}
                  disabled={!writable || saving}
                  aria-label="X/Y 独立"
                />
              </div>
            </div>
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-28">ステージ</TableHead>
                <TableHead>X (DPI)</TableHead>
                <TableHead>Y (DPI)</TableHead>
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
                      disabled={!writable || saving}
                      onChange={(event) => updateStage(stage.index, "x", Number(event.target.value))}
                      onBlur={() => updateStage(stage.index, "x", normalizeDpi(stage.x))}
                      className="w-32"
                    />
                  </TableCell>
                  <TableCell>
                    <Input
                      type="number"
                      min={50}
                      max={26000}
                      step={50}
                      value={independentXY ? stage.y : stage.x}
                      aria-label={`Stage ${stage.index} Y`}
                      disabled={!writable || saving || !independentXY}
                      onChange={(event) => updateStage(stage.index, "y", Number(event.target.value))}
                      onBlur={() => updateStage(stage.index, "y", normalizeDpi(stage.y))}
                      className="w-32"
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          <div className="flex justify-end">
            <Button onClick={() => void handleSave()} disabled={!writable || saving}>
              保存
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>ポーリングレート</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Select
            value={String(sensor.pollingRateHz)}
            onValueChange={(value) => void onChangePollingRate(Number(value))}
            disabled={!writable}
          >
            <SelectTrigger aria-label="ポーリングレート">
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
