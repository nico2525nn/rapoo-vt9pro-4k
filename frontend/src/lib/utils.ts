import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";
import type { DeviceSnapshot, Preset } from "@/types/rapoo";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export const MIN_DPI = 50;
export const MAX_DPI = 26000;
export const DPI_STEP = 50;

export function normalizeDpi(value: number): number {
  if (!Number.isFinite(value)) return 800;
  let v = Math.round(value);
  if (v < MIN_DPI) v = MIN_DPI;
  if (v > MAX_DPI) v = MAX_DPI;
  const rem = v % DPI_STEP;
  if (rem >= DPI_STEP / 2) {
    v += DPI_STEP - rem;
  } else {
    v -= rem;
  }
  return Math.min(MAX_DPI, Math.max(MIN_DPI, v));
}

export function estimateBatteryHours(percentage: number, pollingRateHz: number): number {
  if (percentage <= 0) return 0;
  const pct = Math.min(100, Math.max(0, percentage));
  let fullHours = 200;
  if (pollingRateHz >= 4000) fullHours = 48;
  else if (pollingRateHz >= 2000) fullHours = 95;
  else if (pollingRateHz >= 1000) fullHours = 160;
  else if (pollingRateHz >= 500) fullHours = 185;
  return Math.round(fullHours * (pct / 100) * 10) / 10;
}

// Matches the physical 7-stage onboard LED blink spectrum of the Rapoo VT9 Pro 4K.
export const STAGE_COLORS = [
  { hex: "#ef4444", label: "レッド" },
  { hex: "#f97316", label: "オレンジ" },
  { hex: "#eab308", label: "イエロー" },
  { hex: "#22c55e", label: "グリーン" },
  { hex: "#06b6d4", label: "シアン" },
  { hex: "#3b82f6", label: "ブルー" },
  { hex: "#a855f7", label: "パープル" },
] as const;

export const QUICK_DPI_PRESETS = [400, 800, 1000, 1200, 1600, 2400, 3200, 6400] as const;

// Compares a Preset against the live DeviceSnapshot (accounting for wired 1000Hz clamping).
export function isPresetMatchingSnapshot(preset: Preset, snap: DeviceSnapshot | null): boolean {
  if (!snap?.dpi || !snap?.sensor) return false;
  const maxRate = snap.model.maxRateHz || 4000;
  const expectedHz = Math.min(preset.pollingHz, maxRate);
  if (expectedHz > 0 && snap.sensor.pollingRateHz !== expectedHz) return false;

  if (
    snap.dpi.stageCount !== preset.dpi.stageCount ||
    snap.dpi.currentStage !== preset.dpi.currentStage
  ) {
    return false;
  }
  for (let i = 0; i < preset.dpi.stageCount; i++) {
    const a = snap.dpi.stages[i];
    const b = preset.dpi.stages[i];
    if (!a || !b || a.x !== b.x || a.y !== b.y) return false;
  }

  const p = preset.sensor;
  const s = snap.sensor;
  if (p.liftOffLevel !== undefined && s.liftOffLevel !== p.liftOffLevel) return false;
  if (p.motionSync !== undefined && s.motionSync !== p.motionSync) return false;
  if (p.pressDebounceStep !== undefined && s.pressDebounceStep !== p.pressDebounceStep) return false;
  if (p.releaseDebounceStep !== undefined && s.releaseDebounceStep !== p.releaseDebounceStep) return false;
  if (p.rippleControl !== undefined && s.rippleControl !== p.rippleControl) return false;
  if (p.linearCorrection !== undefined && s.linearCorrection !== p.linearCorrection) return false;
  if (p.sensorAngleDeg !== undefined && s.sensorAngleDeg !== p.sensorAngleDeg) return false;
  if (p.glassMode !== undefined && s.glassMode !== p.glassMode) return false;

  if (preset.buttons && preset.buttons.length > 0 && snap.buttons && snap.buttons.length > 0) {
    for (const pb of preset.buttons) {
      const sb = snap.buttons.find((x) => x.slot.id === pb.slot.id);
      if (!sb || sb.action.type !== pb.action.type) return false;
      if (pb.action.type === "keyboard") {
        if ((sb.action.keyCode ?? 0) !== (pb.action.keyCode ?? 0)) return false;
      }
    }
  }
  return true;
}
