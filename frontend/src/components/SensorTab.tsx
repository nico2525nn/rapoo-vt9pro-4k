import { useEffect, useState } from "react";
import { SettingRow } from "@/components/SettingRow";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { t } from "@/lib/i18n";
import type { DebounceStep, DeviceSnapshot, SensorPatch } from "@/types/rapoo";

interface SensorTabProps {
  snapshot: DeviceSnapshot;
  debounceSteps: DebounceStep[];
  writable: boolean;
  onUpdateSensor: (patch: SensorPatch) => Promise<boolean>;
}

const SLEEP_OPTIONS = [2, 5, 10, 15, 30, 60, 120];

export function SensorTab({ snapshot, debounceSteps, writable, onUpdateSensor }: SensorTabProps) {
  const sensor = snapshot.sensor;
  const isGenB = snapshot.model.protocolGen === 2;
  const [angle, setAngle] = useState(sensor?.sensorAngleDeg ?? 0);

  useEffect(() => {
    if (sensor) setAngle(sensor.sensorAngleDeg);
  }, [sensor]);

  if (!sensor) return null;

  const debounceValue = (step: DebounceStep) => (isGenB ? (step.index === 0 ? 0 : Math.round(step.ms)) : step.index);
  const selectedDebounce = (step: DebounceStep, isPress: boolean) => {
    if (isGenB) {
      const target = step.index === 0 ? 0 : Math.round(step.ms);
      return (isPress ? sensor.pressDebounceMs : sensor.releaseDebounceMs) === target;
    }
    return (isPress ? sensor.pressDebounceStep : sensor.releaseDebounceStep) === step.index;
  };

  const commitAngle = async (nextValue = angle) => {
    const next = Math.max(-30, Math.min(30, Math.round(nextValue)));
    setAngle(next);
    const ok = await onUpdateSensor({ sensorAngleDeg: next });
    if (!ok) setAngle(sensor.sensorAngleDeg);
  };

  const sleepOptions = Array.from(new Set([...SLEEP_OPTIONS, sensor.sleepMinutes])).sort((a, b) => a - b);

  const toggle = (key: keyof SensorPatch, value: boolean) => {
    void onUpdateSensor({ [key]: value } as SensorPatch);
  };

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("Tracking")}</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingRow label={t("Lift-off distance")} description={t("Height at which the sensor responds")}>
            <Select
              value={String(sensor.liftOffLevel)}
              onValueChange={(value) => void onUpdateSensor({ liftOffLevel: Number(value) })}
              disabled={!writable}
            >
              <SelectTrigger className="w-full" aria-label={t("Lift-off distance")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Array.from({ length: 11 }, (_, level) => (
                  <SelectItem key={level} value={String(level)}>
                    {(1.0 + level * 0.1).toFixed(1)} mm
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>

          <SettingRow label={t("Sensor angle")} description={t("Horizontal axis correction angle")}>
            <div className="flex w-full items-center gap-2">
              <Input
                type="number"
                min={-30}
                max={30}
                step={1}
                value={angle}
                aria-label={t("Sensor angle")}
                disabled={!writable}
                onChange={(event) => setAngle(Number(event.target.value))}
                onBlur={() => void commitAngle()}
                onKeyDown={(event) => {
                  if (event.key === "Enter") event.currentTarget.blur();
                }}
                className="min-w-0 flex-1"
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => void commitAngle(0)}
                disabled={!writable}
                className="shrink-0"
              >
                0°
              </Button>
            </div>
          </SettingRow>

          <SettingRow label="Motion Sync" description={t("Sync with the polling interval")}>
            <Switch
              checked={sensor.motionSync}
              onCheckedChange={(value) => toggle("motionSync", value)}
              disabled={!writable}
              aria-label="Motion Sync"
            />
          </SettingRow>

          <SettingRow label={t("Ripple control")} description={t("Reduces jitter at high DPI")}>
            <Switch
              checked={sensor.rippleControl}
              onCheckedChange={(value) => toggle("rippleControl", value)}
              disabled={!writable}
              aria-label={t("Ripple control")}
            />
          </SettingRow>

          <SettingRow label={t("Linear correction")} description={t("Corrects hand shake into straight lines")}>
            <Switch
              checked={sensor.linearCorrection}
              onCheckedChange={(value) => toggle("linearCorrection", value)}
              disabled={!writable}
              aria-label={t("Linear correction")}
            />
          </SettingRow>

          <SettingRow label={t("Glass surface mode")} description={t("Improves tracking on glass")}>
            <Switch
              checked={sensor.glassMode}
              onCheckedChange={(value) => toggle("glassMode", value)}
              disabled={!writable}
              aria-label={t("Glass surface mode")}
            />
          </SettingRow>

        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Switches & power")}</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingRow label={t("Press debounce")} description={t("Prevents chatter when pressing")}>
            <Select
              value={String(
                debounceSteps.find((step) => selectedDebounce(step, true))?.index ?? sensor.pressDebounceStep
              )}
              onValueChange={(value) => {
                const step = debounceSteps.find((item) => item.index === Number(value));
                if (step) void onUpdateSensor({ pressDebounceStep: debounceValue(step) });
              }}
              disabled={!writable}
            >
              <SelectTrigger className="w-full" aria-label={t("Press debounce")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {debounceSteps.map((step) => (
                  <SelectItem key={step.index} value={String(step.index)}>
                    {step.index === 0 && isGenB ? "0 ms" : step.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>

          <SettingRow label={t("Release debounce")} description={t("Prevents chatter when releasing")}>
            <Select
              value={String(
                debounceSteps.find((step) => selectedDebounce(step, false))?.index ?? sensor.releaseDebounceStep
              )}
              onValueChange={(value) => {
                const step = debounceSteps.find((item) => item.index === Number(value));
                if (step) void onUpdateSensor({ releaseDebounceStep: debounceValue(step) });
              }}
              disabled={!writable}
            >
              <SelectTrigger className="w-full" aria-label={t("Release debounce")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {debounceSteps.map((step) => (
                  <SelectItem key={step.index} value={String(step.index)}>
                    {step.index === 0 && isGenB ? "0 ms" : step.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>

          <SettingRow label={t("Sleep time")} description={t("Enter sleep after inactivity")}>
            <Select
              value={String(sensor.sleepMinutes)}
              onValueChange={(value) => void onUpdateSensor({ sleepMinutes: Number(value) })}
              disabled={!writable}
            >
              <SelectTrigger className="w-full" aria-label={t("Sleep time")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {sleepOptions.map((minutes) => (
                  <SelectItem key={minutes} value={String(minutes)}>
                    {t("{minutes} min", { minutes })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>

          <SettingRow label={t("LED power saving")} description={t("Reduces LED on-time")}>
            <Switch
              checked={sensor.ledLowPowerMode > 0}
              onCheckedChange={(value) => void onUpdateSensor({ ledLowPowerMode: value ? 1 : 0 })}
              disabled={!writable}
              aria-label={t("LED power saving")}
            />
          </SettingRow>
        </CardContent>
      </Card>
    </div>
  );
}
