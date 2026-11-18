import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

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

export const STAGE_COLORS = [
  { hex: "#ef4444", label: "レッド", badgeBg: "bg-red-500/15 text-red-400 border-red-500/30" },
  { hex: "#3b82f6", label: "ブルー", badgeBg: "bg-blue-500/15 text-blue-400 border-blue-500/30" },
  { hex: "#10b981", label: "エメラルド", badgeBg: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30" },
  { hex: "#f59e0b", label: "アンバー", badgeBg: "bg-amber-500/15 text-amber-400 border-amber-500/30" },
  { hex: "#a855f7", label: "パープル", badgeBg: "bg-purple-500/15 text-purple-400 border-purple-500/30" },
  { hex: "#06b6d4", label: "シアン", badgeBg: "bg-cyan-500/15 text-cyan-400 border-cyan-500/30" },
  { hex: "#ec4899", label: "ピンク", badgeBg: "bg-pink-500/15 text-pink-400 border-pink-500/30" },
] as const;

export const QUICK_DPI_PRESETS = [400, 800, 1000, 1200, 1600, 2400, 3200, 6400] as const;
