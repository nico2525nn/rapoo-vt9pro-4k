import { useEffect, useState } from "react";
import { SettingRow } from "@/components/SettingRow";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getLanguage, t } from "@/lib/i18n";
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
  const locale = getLanguage() === "ja" ? "ja-JP" : "en-US";
  const [stages, setStages] = useState<DPIStage[]>(dpi ? allStagesFrom(dpi) : []);
  const [stageCount, setStageCount] = useState<number>(dpi?.stageCount ?? 1);
  const [currentStage, setCurrentStage] = useState<number>(dpi?.currentStage ?? 1);
  // The device reports independent axes only once an enabled stage has different X and Y values.
  const [editingBothAxes, setEditingBothAxes] = useState(false);
  const independentXY = Boolean(dpi?.independentXY) || editingBothAxes;

  useEffect(() => {
    if (!dpi) return;
    setStages(allStagesFrom(dpi));
    setStageCount(dpi.stageCount);
    setCurrentStage(dpi.currentStage);
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
    setEditingBothAxes(checked);
    if (!checked && stages.some((stage) => stage.x !== stage.y)) {
      const nextStages = stages.map((stage) => ({ ...stage, y: stage.x }));
      setStages(nextStages);
      void persist(nextStages, stageCount, currentStage, false);
    }
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
        <CardContent>
          <SettingRow
            label={t("Polling rate")}
            description={model.maxRateHz < 4000 ? t("Up to {hz} Hz on this connection", { hz: model.maxRateHz.toLocaleString(locale) }) : undefined}
          >
            <Select
              value={String(sensor.pollingRateHz)}
              onValueChange={(value) => void onChangePollingRate(Number(value))}
              disabled={!writable}
            >
              <SelectTrigger className="w-full" aria-label={t("Polling rate")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {pollingRates.map((hz) => (
                  <SelectItem key={hz} value={String(hz)} disabled={hz > model.maxRateHz}>
                    {hz.toLocaleString(locale)} Hz
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Stages")}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid gap-4 @min-[28rem]:grid-cols-3">
            <div className="flex flex-col gap-2">
              <span className="text-sm font-medium">{t("Stage count")}</span>
              <Select value={String(stageCount)} onValueChange={changeStageCount} disabled={!writable}>
                <SelectTrigger className="w-full" aria-label={t("Stage count")}>
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
              <span className="text-sm font-medium">{t("Active stage")}</span>
              <Select value={String(currentStage)} onValueChange={changeCurrentStage} disabled={!writable}>
                <SelectTrigger className="w-full" aria-label={t("Active stage")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Array.from({ length: stageCount }, (_, i) => i + 1).map((stage) => (
                    <SelectItem key={stage} value={String(stage)}>
                      {t("Stage {stage}", { stage })}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-2">
              <span className="text-sm font-medium">{t("Independent X/Y")}</span>
              <div className="flex h-8 items-center">
                <Switch
                  checked={independentXY}
                  onCheckedChange={changeIndependentXY}
                  disabled={!writable}
                  aria-label={t("Independent X/Y")}
                />
              </div>
            </div>
          </div>

          <Table className="[&_th:first-child]:pl-0 [&_td:first-child]:pl-0 [&_th:last-child]:pr-0 [&_td:last-child]:pr-0">
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-28">{t("Stage")}</TableHead>
                <TableHead>X (DPI)</TableHead>
                {independentXY && <TableHead>Y (DPI)</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {stages.slice(0, stageCount).map((stage) => (
                <TableRow key={stage.index}>
                  <TableCell className="font-medium">{t("Stage {stage}", { stage: stage.index })}</TableCell>
                  <TableCell>
                    <Input
                      type="number"
                      min={50}
                      max={26000}
                      step={50}
                      value={stage.x}
                      aria-label={t("Stage {stage} X", { stage: stage.index })}
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
                      className="w-full max-w-36 tabular-nums"
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
                        aria-label={t("Stage {stage} Y", { stage: stage.index })}
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
                        className="w-full max-w-36 tabular-nums"
                      />
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

    </div>
  );
}
