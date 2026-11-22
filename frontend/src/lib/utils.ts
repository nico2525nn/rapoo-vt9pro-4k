export { cn } from "cn";

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
