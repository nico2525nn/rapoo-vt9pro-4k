import { Compass, Cpu, HelpCircle, Lightbulb, Moon, RotateCcw, Sparkles, Timer } from "lucide-react";
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { DebounceStep, DeviceSnapshot, SensorPatch } from "@/types/rapoo";

interface SensorTabProps {
  snapshot: DeviceSnapshot;
  debounceSteps: DebounceStep[];
  writable: boolean;
  onUpdateSensor: (patch: SensorPatch) => Promise<boolean>;
}

const SLEEP_PRESETS = [5, 10, 15, 30, 60] as const;

export function SensorTab({ snapshot, debounceSteps, writable, onUpdateSensor }: SensorTabProps) {
  const sensor = snapshot.sensor;
  const isGenB = snapshot.model.protocolGen === 2;
  const isWired = snapshot.model.connection === "wired";

  const [lodLevel, setLodLevel] = useState<number>(() => sensor?.liftOffLevel ?? 1);
  const [angleDeg, setAngleDeg] = useState<number>(() => sensor?.sensorAngleDeg ?? 0);
  const [sleepMin, setSleepMin] = useState<number>(() => sensor?.sleepMinutes ?? 10);

  useEffect(() => {
    if (!sensor) return;
    setLodLevel(sensor.liftOffLevel);
    setAngleDeg(sensor.sensorAngleDeg);
    setSleepMin(sensor.sleepMinutes);
  }, [sensor]);

  if (!sensor) return null;

  const lodMm = (1.0 + lodLevel * 0.1).toFixed(1);

  const commitLod = async (nextLevel: number) => {
    setLodLevel(nextLevel);
    const ok = await onUpdateSensor({ liftOffLevel: nextLevel });
    if (!ok) setLodLevel(sensor.liftOffLevel);
  };

  const commitAngle = async (nextAngle: number) => {
    setAngleDeg(nextAngle);
    const ok = await onUpdateSensor({ sensorAngleDeg: nextAngle });
    if (!ok) setAngleDeg(sensor.sensorAngleDeg);
  };

  const commitSleep = async (nextSleep: number) => {
    setSleepMin(nextSleep);
    const ok = await onUpdateSensor({ sleepMinutes: nextSleep });
    if (!ok) setSleepMin(sensor.sleepMinutes);
  };

  const getDebouncePatchValue = (step: DebounceStep): number => {
    return isGenB ? (step.index === 0 ? 0 : Math.round(step.ms)) : step.index;
  };

  const isDebounceStepSelected = (step: DebounceStep, isPress: boolean): boolean => {
    if (isGenB) {
      const targetMs = step.index === 0 ? 0 : Math.round(step.ms);
      return (isPress ? sensor.pressDebounceMs : sensor.releaseDebounceMs) === targetMs;
    }
    return (isPress ? sensor.pressDebounceStep : sensor.releaseDebounceStep) === step.index;
  };

  return (
    <div className="grid gap-4 md:grid-cols-12">
      {/* Left 6 cols: PAW3395 Tracking & LOD & Sensor Angle */}
      <div className="space-y-4 md:col-span-6">
        {/* Lift-Off Distance */}
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-2">
              <CardTitle className="flex items-center gap-2">
                <Cpu className="h-4 w-4 text-primary" />
                リフトオフディスタンス (LOD)
              </CardTitle>
              <Badge variant="default" className="font-mono tabular-nums text-xs">
                {lodMm} mm
              </Badge>
            </div>
            <CardDescription>
              マウスを持ち上げた際にトラッキングを停止する高さ（1.0 mm 〜 2.0 mm・11段階）を設定します。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3.5">
            {/* Visual Height Indicator */}
            <div className="flex items-center justify-between gap-4 rounded-lg border bg-muted/35 px-3.5 py-2.5">
              <div className="space-y-0.5 text-xs">
                <div className="font-medium">
                  センサー検知高度: <span className="text-primary font-bold tabular-nums">{lodMm} mm</span>
                </div>
                <div className="text-muted-foreground text-[11px]">
                  ローセンシで頻繁にマウスを持ち上げる場合は 1.0〜1.2 mm が推奨です
                </div>
              </div>
              <svg
                width="76"
                height="42"
                viewBox="0 0 76 42"
                aria-hidden="true"
                className="shrink-0 text-primary"
              >
                <line x1="6" y1="36" x2="70" y2="36" stroke="currentColor" strokeOpacity="0.4" strokeWidth="2" />
                <g transform={`translate(0, ${-(lodLevel * 1.1)})`}>
                  <rect
                    x="18"
                    y="12"
                    width="40"
                    height="14"
                    rx="7"
                    fill="currentColor"
                    fillOpacity="0.18"
                    stroke="currentColor"
                    strokeWidth="1.5"
                  />
                  <line
                    x1="38"
                    y1="26"
                    x2="38"
                    y2={36 + lodLevel * 1.1}
                    stroke="currentColor"
                    strokeDasharray="2 2"
                    strokeWidth="1.3"
                  />
                </g>
              </svg>
            </div>

            <Slider
              aria-label="リフトオフディスタンス"
              min={0}
              max={10}
              step={1}
              disabled={!writable}
              value={[lodLevel]}
              onValueChange={([v]) => setLodLevel(v)}
              onValueCommit={([v]) => void commitLod(v)}
            />

            <div className="flex items-center justify-between gap-2">
              <span className="text-[11px] text-muted-foreground tabular-nums">1.0 mm (低)</span>
              <div className="flex gap-1.5">
                {[
                  { level: 0, label: "1.0 mm" },
                  { level: 2, label: "1.2 mm" },
                  { level: 5, label: "1.5 mm" },
                  { level: 10, label: "2.0 mm" },
                ].map((opt) => (
                  <Button
                    key={opt.level}
                    size="sm"
                    variant={lodLevel === opt.level ? "default" : "outline"}
                    disabled={!writable}
                    className="h-7 px-2 text-xs tabular-nums"
                    onClick={() => void commitLod(opt.level)}
                  >
                    {opt.label}
                  </Button>
                ))}
              </div>
              <span className="text-[11px] text-muted-foreground tabular-nums">2.0 mm (高)</span>
            </div>
          </CardContent>
        </Card>

        {/* Sensor Angle Calibration */}
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-2">
              <CardTitle className="flex items-center gap-2">
                <Compass className="h-4 w-4 text-primary" />
                センサー角度調整 (アングルチューニング)
              </CardTitle>
              <div className="flex items-center gap-1.5">
                <Badge variant="outline" className="font-mono tabular-nums">
                  {angleDeg > 0 ? `+${angleDeg}°` : `${angleDeg}°`}
                </Badge>
                {angleDeg !== 0 && (
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-7 px-2 text-xs"
                    disabled={!writable}
                    onClick={() => void commitAngle(0)}
                  >
                    <RotateCcw className="h-3 w-3" />
                    0°に戻す
                  </Button>
                )}
              </div>
            </div>
            <CardDescription>
              持ち方の傾きに合わせて水平軸を -30° 〜 +30° の範囲で補正します。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between gap-4 rounded-lg border bg-muted/35 p-3">
              <div className="text-xs space-y-1">
                <div className="font-medium">水平スワイプ軸プレビュー</div>
                <p className="text-[11px] text-muted-foreground leading-relaxed">
                  手首を自然に振ったときにカーソルが斜めに動く場合、1°ずつ微調整すると真横に揃います。
                </p>
              </div>
              <svg
                width="68"
                height="68"
                viewBox="0 0 68 68"
                aria-hidden="true"
                className="shrink-0 rounded-full border bg-background/80"
              >
                <circle cx="34" cy="34" r="26" fill="none" stroke="currentColor" strokeOpacity="0.15" />
                <line x1="8" y1="34" x2="60" y2="34" stroke="currentColor" strokeOpacity="0.2" strokeDasharray="2 2" />
                <line x1="34" y1="8" x2="34" y2="60" stroke="currentColor" strokeOpacity="0.2" strokeDasharray="2 2" />
                <g transform={`rotate(${angleDeg} 34 34)`} className="text-primary">
                  <line x1="10" y1="34" x2="58" y2="34" stroke="currentColor" strokeWidth="2" />
                  <line x1="34" y1="14" x2="34" y2="54" stroke="currentColor" strokeWidth="1.5" strokeOpacity="0.6" />
                  <polygon points="58,34 52,31 52,37" fill="currentColor" />
                </g>
              </svg>
            </div>

            <Slider
              aria-label="センサー角度"
              min={-30}
              max={30}
              step={1}
              disabled={!writable}
              value={[angleDeg]}
              onValueChange={([v]) => setAngleDeg(v)}
              onValueCommit={([v]) => void commitAngle(v)}
            />
            <div className="flex justify-between text-[11px] text-muted-foreground tabular-nums">
              <span>-30° (反時計回り)</span>
              <span>0° (標準)</span>
              <span>+30° (時計回り)</span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Right 6 cols: Sensor DSP Toggles + Switch Debounce + Idle Sleep & LED */}
      <div className="space-y-4 md:col-span-6">
        {/* Sensor Processing Toggles */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2">
              <Sparkles className="h-4 w-4 text-primary" />
              センサー信号処理 (DSP)
            </CardTitle>
            <CardDescription>PAW3395 内部アルゴリズムの ON/OFF を切り替えます。</CardDescription>
          </CardHeader>
          <CardContent className="divide-y divide-border/70">
            <SensorSwitchRow
              id="sw-motion-sync"
              title="Motion Sync (モーションシンク)"
              badge="追いエイム重視 / 4K最速はOFF"
              description="センサー計測タイミングをUSBポーリング周期に同期し、滑らかで均一な視点移動を実現します。"
              tooltip="同期処理により 1000Hz時は平均約0.5ms、4000Hz時は平均約0.12ms の待機が生じます。トラッキング重視タイトルではON、初動最速重視のフリック系FPSではOFFも選ばれます。"
              checked={sensor.motionSync}
              disabled={!writable}
              onCheckedChange={(v) => void onUpdateSensor({ motionSync: v })}
            />
            <SensorSwitchRow
              id="sw-linear"
              title="直線補正 (アングルスナップ)"
              badge="FPSではOFF推奨"
              description="水平・垂直に近いマウス移動を直線に補正します。イラスト・CAD作業向けです。"
              checked={sensor.linearCorrection}
              disabled={!writable}
              onCheckedChange={(v) => void onUpdateSensor({ linearCorrection: v })}
            />
            <SensorSwitchRow
              id="sw-ripple"
              title="波紋補正 (リップルコントロール)"
              badge="9000DPI未満はOFF推奨"
              description="超高DPI設定時の細かなジッター（揺れ）を平滑化します。"
              checked={sensor.rippleControl}
              disabled={!writable}
              onCheckedChange={(v) => void onUpdateSensor({ rippleControl: v })}
            />
            <SensorSwitchRow
              id="sw-glass"
              title="ガラス面モード"
              description="ガラス製マウスパッドや光沢のあるデスク面でのトラッキング感度を最適化します。"
              checked={sensor.glassMode}
              disabled={!writable}
              onCheckedChange={(v) => void onUpdateSensor({ glassMode: v })}
            />
          </CardContent>
        </Card>

        {/* Switch Debounce & Wireless Sleep */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2">
              <Timer className="h-4 w-4 text-primary" />
              クリックデバウンス & 省電力設定
            </CardTitle>
            <CardDescription>
              スイッチの応答速度（押下・解放個別）と省電力動作を設定します。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Press Debounce */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs">
                <span className="font-medium">押下デバウンス時間 (Press)</span>
                <span className="font-mono text-primary font-semibold tabular-nums">
                  {sensor.pressDebounceLabel}
                </span>
              </div>
              <div className="grid grid-cols-7 gap-1" role="group" aria-label="押下デバウンス時間選択">
                {debounceSteps.map((step) => {
                  const selected = isDebounceStepSelected(step, true);
                  const displayVal = isGenB && step.index === 0 ? "0" : String(step.ms);
                  return (
                    <button
                      key={step.index}
                      type="button"
                      disabled={!writable}
                      aria-label={`押下デバウンス ${displayVal} ms`}
                      onClick={() =>
                        void onUpdateSensor({ pressDebounceStep: getDebouncePatchValue(step) })
                      }
                      aria-pressed={selected}
                      className={cn(
                        "rounded-md border py-1.5 text-center text-xs font-medium tabular-nums transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background disabled:opacity-40",
                        selected
                          ? "border-primary bg-primary text-primary-foreground"
                          : "bg-background/50 hover:bg-accent"
                      )}
                    >
                      {displayVal}
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Release Debounce */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between text-xs">
                <span className="font-medium">解放デバウンス時間 (Release)</span>
                <span className="font-mono text-primary font-semibold tabular-nums">
                  {sensor.releaseDebounceLabel}
                </span>
              </div>
              <div className="grid grid-cols-7 gap-1" role="group" aria-label="解放デバウンス時間選択">
                {debounceSteps.map((step) => {
                  const selected = isDebounceStepSelected(step, false);
                  const displayVal = isGenB && step.index === 0 ? "0" : String(step.ms);
                  return (
                    <button
                      key={step.index}
                      type="button"
                      disabled={!writable}
                      aria-label={`解放デバウンス ${displayVal} ms`}
                      onClick={() =>
                        void onUpdateSensor({ releaseDebounceStep: getDebouncePatchValue(step) })
                      }
                      aria-pressed={selected}
                      className={cn(
                        "rounded-md border py-1.5 text-center text-xs font-medium tabular-nums transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background disabled:opacity-40",
                        selected
                          ? "border-primary bg-primary text-primary-foreground"
                          : "bg-background/50 hover:bg-accent"
                      )}
                    >
                      {displayVal}
                    </button>
                  );
                })}
              </div>
              <p className="text-[11px] text-muted-foreground leading-relaxed">
                単位: ms • 初弾を最速化しつつ指離し時のチャタリングを防ぐ <strong>押下 1ms + 解放 2〜4ms</strong> の組み合わせも競技FPSで人気です。
              </p>
            </div>

            {/* Idle Sleep Timer */}
            <div className="space-y-2 border-t border-border/60 pt-3">
              <div className="flex items-center justify-between gap-2 text-xs flex-wrap">
                <span className="font-medium flex items-center gap-1.5">
                  <Moon className="h-3.5 w-3.5 text-muted-foreground" />
                  ワイヤレス自動スリープ移行時間
                  {isWired && (
                    <Badge variant="outline" className="text-[10px] px-1.5 py-0 font-normal">
                      有線中は常時ON（無線用として保存）
                    </Badge>
                  )}
                </span>
                <span className="font-mono font-semibold text-primary tabular-nums">{sleepMin} 分</span>
              </div>

              <Slider
                aria-label="ワイヤレス自動スリープ時間"
                min={2}
                max={120}
                step={1}
                disabled={!writable}
                value={[sleepMin]}
                onValueChange={([v]) => setSleepMin(v)}
                onValueCommit={([v]) => void commitSleep(v)}
              />

              <div className="flex flex-wrap items-center justify-between gap-1.5 pt-0.5">
                <span className="text-[11px] text-muted-foreground">クイック設定:</span>
                <div className="flex gap-1.5">
                  {SLEEP_PRESETS.map((m) => (
                    <Button
                      key={m}
                      size="sm"
                      variant={sleepMin === m ? "default" : "outline"}
                      disabled={!writable}
                      className="h-7 px-2.5 text-xs tabular-nums"
                      onClick={() => void commitSleep(m)}
                    >
                      {m}分
                    </Button>
                  ))}
                </div>
              </div>
            </div>

            {/* Onboard LED Low-Power Mode (0x08C6) */}
            <div className="flex items-start justify-between gap-3 border-t border-border/60 pt-3">
              <div className="space-y-0.5">
                <label htmlFor="sw-led-eco" className="text-xs font-medium cursor-pointer flex items-center gap-1.5">
                  <Lightbulb className="h-3.5 w-3.5 text-muted-foreground" />
                  本体インジケーター省電力モード
                </label>
                <p className="text-[11px] text-muted-foreground leading-relaxed">
                  マウス天面のステータスLED点灯時間を抑えてバッテリー消費を節約します。
                </p>
              </div>
              <Switch
                id="sw-led-eco"
                checked={sensor.ledLowPowerMode > 0}
                disabled={!writable}
                onCheckedChange={(v) => void onUpdateSensor({ ledLowPowerMode: v ? 1 : 0 })}
                className="mt-0.5"
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

interface SensorSwitchRowProps {
  id: string;
  title: string;
  description: string;
  badge?: string;
  tooltip?: string;
  checked: boolean;
  disabled: boolean;
  onCheckedChange: (checked: boolean) => void;
}

function SensorSwitchRow({
  id,
  title,
  description,
  badge,
  tooltip,
  checked,
  disabled,
  onCheckedChange,
}: SensorSwitchRowProps) {
  return (
    <div className="flex items-start justify-between gap-3 py-3 first:pt-0 last:pb-0">
      <div className="space-y-1">
        <div className="flex items-center gap-1.5 flex-wrap">
          <label htmlFor={id} className="text-xs sm:text-sm font-medium cursor-pointer">
            {title}
          </label>
          {badge && (
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
              {badge}
            </Badge>
          )}
          {tooltip && (
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  aria-label={`${title} の詳細説明`}
                  className="inline-flex h-6 w-6 -my-1 items-center justify-center rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <HelpCircle className="h-3.5 w-3.5" />
                </button>
              </TooltipTrigger>
              <TooltipContent>{tooltip}</TooltipContent>
            </Tooltip>
          )}
        </div>
        <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>
      </div>
      <Switch id={id} checked={checked} disabled={disabled} onCheckedChange={onCheckedChange} className="mt-0.5" />
    </div>
  );
}
