import { estimateBatteryHours, normalizeDpi } from "@/lib/utils";
import type {
  AppPreferences,
  ButtonAction,
  ButtonActionType,
  ButtonMapping,
  ButtonSlotSpec,
  DebounceStep,
  DeviceSnapshot,
  DPIConfig,
  HIDKeyInfo,
  PollingMode,
  Preset,
  SensorPatch,
  ServiceState,
  UICatalog,
} from "@/types/rapoo";

interface WailsAppBindings {
  GetCatalog(): Promise<UICatalog>;
  GetState(): Promise<ServiceState>;
  RefreshNow(): Promise<void>;
  ReadSnapshot(): Promise<DeviceSnapshot>;
  SetDemoMode(enabled: boolean): Promise<DeviceSnapshot>;
  SetPollingMode(mode: string): Promise<ServiceState>;
  SetPollingRate(hz: number): Promise<DeviceSnapshot>;
  SetCurrentDPIGear(stage: number): Promise<DeviceSnapshot>;
  SetDPIConfig(cfg: DPIConfig): Promise<DeviceSnapshot>;
  UpdateSensorConfig(patch: SensorPatch): Promise<DeviceSnapshot>;
  SetButtonMapping(buttonId: number, action: ButtonAction): Promise<DeviceSnapshot>;
  ResetButtonsToDefault(): Promise<DeviceSnapshot>;
  FactoryReset(): Promise<DeviceSnapshot>;
  ListPresets(): Promise<Preset[]>;
  SavePreset(id: string, name: string, description: string): Promise<Preset>;
  ApplyPreset(id: string): Promise<DeviceSnapshot>;
  DeletePreset(id: string): Promise<void>;
  ExportPresetJSON(id: string): Promise<string>;
  ImportPresetJSON(rawJson: string): Promise<Preset>;
  GetAppPreferences(): Promise<AppPreferences>;
  SetCloseToTray(enabled: boolean): Promise<AppPreferences>;
  SetAutoStart(enabled: boolean): Promise<AppPreferences>;
}

interface WailsRuntime {
  EventsOn(eventName: string, callback: (...data: unknown[]) => void): () => void;
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: WailsAppBindings;
      };
    };
    runtime?: WailsRuntime;
  }
}

function getWailsApp(): WailsAppBindings | undefined {
  return typeof window !== "undefined" ? window.go?.main?.App : undefined;
}

function getWailsRuntime(): WailsRuntime | undefined {
  return typeof window !== "undefined" ? window.runtime : undefined;
}

// ============================================================================
// Standalone Fallback Mock (used in Vitest & standalone `vite dev`)
// ============================================================================

const DEFAULT_DEBOUNCE_STEPS: DebounceStep[] = [
  { index: 0, rawByte: 0x01, ms: 0.5, label: "0.5 ms" },
  { index: 1, rawByte: 0x02, ms: 1, label: "1 ms" },
  { index: 2, rawByte: 0x04, ms: 2, label: "2 ms" },
  { index: 3, rawByte: 0x08, ms: 4, label: "4 ms" },
  { index: 4, rawByte: 0x10, ms: 8, label: "8 ms" },
  { index: 5, rawByte: 0x18, ms: 12, label: "12 ms" },
  { index: 6, rawByte: 0x20, ms: 16, label: "16 ms" },
];

const DEFAULT_BUTTON_SLOTS: ButtonSlotSpec[] = [
  { id: 0, nameJa: "左クリック", nameEn: "Left Click", view: "top", addr: 0x0600, defaultAction: "left_click", clickable: true },
  { id: 1, nameJa: "右クリック", nameEn: "Right Click", view: "top", addr: 0x0608, defaultAction: "right_click", clickable: true },
  { id: 2, nameJa: "ミドルボタン (ホイール押込)", nameEn: "Middle Click", view: "top", addr: 0x0604, defaultAction: "middle_click", clickable: true },
  { id: 7, nameJa: "進む (サイド前)", nameEn: "Forward", view: "top", addr: 0x0614, defaultAction: "forward", clickable: true },
  { id: 8, nameJa: "戻る (サイド後)", nameEn: "Back", view: "top", addr: 0x0618, defaultAction: "back", clickable: true },
  { id: 5, nameJa: "DPI + ボタン", nameEn: "DPI+", view: "top", addr: 0x060c, defaultAction: "dpi_plus", clickable: true },
  { id: 6, nameJa: "DPI - ボタン", nameEn: "DPI-", view: "top", addr: 0x0610, defaultAction: "dpi_minus", clickable: true },
  { id: 11, nameJa: "ホイール上スクロール", nameEn: "Scroll Up", view: "top", addr: 0x0624, defaultAction: "wheel_default", clickable: false },
  { id: 12, nameJa: "ホイール下スクロール", nameEn: "Scroll Down", view: "top", addr: 0x0628, defaultAction: "wheel_default", clickable: false },
  { id: 13, nameJa: "底面 DIY ボタン", nameEn: "Bottom DIY Button", view: "bottom", addr: 0x0634, defaultAction: "receiver_switch", clickable: true },
];

const DEFAULT_KEYS: HIDKeyInfo[] = [
  { code: 0x04, name: "A", domCode: "KeyA" },
  { code: 0x05, name: "B", domCode: "KeyB" },
  { code: 0x06, name: "C", domCode: "KeyC" },
  { code: 0x07, name: "D", domCode: "KeyD" },
  { code: 0x08, name: "E", domCode: "KeyE" },
  { code: 0x09, name: "F", domCode: "KeyF" },
  { code: 0x0a, name: "G", domCode: "KeyG" },
  { code: 0x14, name: "Q", domCode: "KeyQ" },
  { code: 0x15, name: "R", domCode: "KeyR" },
  { code: 0x19, name: "V", domCode: "KeyV" },
  { code: 0x1b, name: "X", domCode: "KeyX" },
  { code: 0x1d, name: "Z", domCode: "KeyZ" },
  { code: 0x1e, name: "1", domCode: "Digit1" },
  { code: 0x1f, name: "2", domCode: "Digit2" },
  { code: 0x20, name: "3", domCode: "Digit3" },
  { code: 0x21, name: "4", domCode: "Digit4" },
  { code: 0x22, name: "5", domCode: "Digit5" },
  { code: 0x28, name: "Enter", domCode: "Enter" },
  { code: 0x29, name: "Esc", domCode: "Escape" },
  { code: 0x2a, name: "Backspace", domCode: "Backspace" },
  { code: 0x2b, name: "Tab", domCode: "Tab" },
  { code: 0x2c, name: "Space", domCode: "Space" },
  { code: 0x3a, name: "F1", domCode: "F1" },
  { code: 0x3b, name: "F2", domCode: "F2" },
  { code: 0x3c, name: "F3", domCode: "F3" },
  { code: 0x3d, name: "F4", domCode: "F4" },
  { code: 0x3e, name: "F5", domCode: "F5" },
  { code: 0x45, name: "F12", domCode: "F12" },
  { code: 0x4c, name: "Delete", domCode: "Delete" },
  { code: 0x4f, name: "Right", domCode: "ArrowRight" },
  { code: 0x50, name: "Left", domCode: "ArrowLeft" },
  { code: 0x51, name: "Down", domCode: "ArrowDown" },
  { code: 0x52, name: "Up", domCode: "ArrowUp" },
];

export const ACTION_LABEL_MAP: Record<ButtonActionType, { labelJa: string; category: string }> = {
  left_click: { labelJa: "左クリック", category: "mouse" },
  right_click: { labelJa: "右クリック", category: "mouse" },
  middle_click: { labelJa: "ミドルクリック", category: "mouse" },
  forward: { labelJa: "進む (Mouse 5)", category: "mouse" },
  back: { labelJa: "戻る (Mouse 4)", category: "mouse" },
  wheel_default: { labelJa: "標準スクロール", category: "mouse" },
  fire_triple: { labelJa: "ファイアキー (左3連射)", category: "mouse" },
  dpi_plus: { labelJa: "DPI + (段階を上げる)", category: "dpi" },
  dpi_minus: { labelJa: "DPI - (段階を下げる)", category: "dpi" },
  dpi_cycle: { labelJa: "DPI ループ切替", category: "dpi" },
  sniper_dpi: { labelJa: "スナイパーボタン (押下中低DPI)", category: "dpi" },
  polling_switch: { labelJa: "ポーリングレート切替", category: "hardware" },
  receiver_switch: { labelJa: "レシーバー切替", category: "hardware" },
  profile_switch: { labelJa: "オンボード設定切替", category: "hardware" },
  media_play_pause: { labelJa: "再生 / 一時停止", category: "media" },
  media_stop: { labelJa: "停止", category: "media" },
  media_prev: { labelJa: "前の曲", category: "media" },
  media_next: { labelJa: "次の曲", category: "media" },
  media_vol_up: { labelJa: "音量アップ", category: "media" },
  media_vol_down: { labelJa: "音量ダウン", category: "media" },
  media_mute: { labelJa: "ミュート (消音)", category: "media" },
  media_player: { labelJa: "メディアプレーヤー起動", category: "media" },
  app_calculator: { labelJa: "電卓を起動", category: "media" },
  app_explorer: { labelJa: "エクスプローラーを起動", category: "media" },
  app_mail: { labelJa: "メールを起動", category: "media" },
  keyboard: { labelJa: "キーボード入力", category: "keyboard" },
  disabled: { labelJa: "無効 (ボタンOFF)", category: "none" },
  unknown: { labelJa: "カスタム設定", category: "hardware" },
};

function makeDefaultButtons(): ButtonMapping[] {
  return DEFAULT_BUTTON_SLOTS.map((slot) => {
    const meta = ACTION_LABEL_MAP[slot.defaultAction];
    let labelJa = meta.labelJa;
    if (slot.id === 11) labelJa = "上スクロール (標準)";
    if (slot.id === 12) labelJa = "下スクロール (標準)";
    return {
      slot,
      action: {
        type: slot.defaultAction,
        labelJa,
        category: meta.category,
      },
    };
  });
}

function makeDefaultSnapshot(): DeviceSnapshot {
  const allStages = [
    { index: 1, x: 400, y: 400, enabled: true },
    { index: 2, x: 800, y: 800, enabled: true },
    { index: 3, x: 1200, y: 1200, enabled: true },
    { index: 4, x: 1600, y: 1600, enabled: true },
    { index: 5, x: 3200, y: 3200, enabled: true },
    { index: 6, x: 6400, y: 6400, enabled: true },
    { index: 7, x: 26000, y: 26000, enabled: true },
  ];
  return {
    model: {
      name: "Rapoo VT9 Pro 4K (デモ)",
      vendorId: 0x24ae,
      productId: 0x1211,
      connection: "wireless_4k",
      protocolGen: 1,
      maxRateHz: 4000,
    },
    commandHidPath: "virtual://vt9pro4k/col04",
    reportHidPath: "virtual://vt9pro4k/col06",
    battery: {
      percentage: 82,
      charging: false,
      online: true,
      sleeping: false,
      source: "command",
      updatedAt: new Date().toISOString(),
    },
    estimatedHours: estimateBatteryHours(82, 1000),
    firmware: {
      mouseRaw: 1030,
      mouseString: "10.30",
      dongleRaw: 1029,
      dongleString: "10.29",
    },
    dpi: {
      currentStage: 2,
      stageCount: 7,
      independentXY: false,
      stages: allStages.map((s) => ({ ...s })),
      allStages: allStages.map((s) => ({ ...s })),
    },
    sensor: {
      pollingRateHz: 1000,
      wireless8kSwitch: 0,
      silentHeightLod: 2,
      liftOffLevel: 1,
      liftOffDistanceMm: 1.1,
      motionSync: false,
      pressDebounceRaw: 0x08,
      pressDebounceStep: 3,
      pressDebounceMs: 4,
      pressDebounceLabel: "4 ms",
      releaseDebounceRaw: 0x08,
      releaseDebounceStep: 3,
      releaseDebounceMs: 4,
      releaseDebounceLabel: "4 ms",
      sleepMinutes: 10,
      rippleControl: false,
      linearCorrection: false,
      sensorAngleDeg: 0,
      glassMode: false,
      ledLowPowerMode: 0,
    },
    buttons: makeDefaultButtons(),
    demoMode: true,
  };
}

class FallbackSimulator {
  private listeners = new Map<string, Set<(...args: unknown[]) => void>>();
  private demoMode = false;
  private mode: PollingMode = "normal";
  private snap: DeviceSnapshot = makeDefaultSnapshot();
  private prefs: AppPreferences = {
    closeToTray: true,
    autoStart: false,
    platform: "windows",
  };
  private presets: Preset[] = [
    {
      id: "builtin-fps-4k",
      name: "FPS 競技用 (4000Hz・超低遅延)",
      description: "800 DPI・4000Hz・デバウンス 1ms・直線/波紋補正OFF・誤爆防止のため天面DPIボタン無効化",
      builtIn: true,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
      pollingHz: 4000,
      dpi: {
        currentStage: 2,
        stageCount: 3,
        independentXY: false,
        stages: [
          { index: 1, x: 400, y: 400, enabled: true },
          { index: 2, x: 800, y: 800, enabled: true },
          { index: 3, x: 1600, y: 1600, enabled: true },
        ],
        allStages: [
          { index: 1, x: 400, y: 400, enabled: true },
          { index: 2, x: 800, y: 800, enabled: true },
          { index: 3, x: 1600, y: 1600, enabled: true },
          { index: 4, x: 3200, y: 3200, enabled: false },
          { index: 5, x: 3200, y: 3200, enabled: false },
          { index: 6, x: 6400, y: 6400, enabled: false },
          { index: 7, x: 1600, y: 1600, enabled: false },
        ],
      },
      sensor: {
        liftOffLevel: 0,
        motionSync: false,
        pressDebounceStep: 1,
        releaseDebounceStep: 1,
        sleepMinutes: 15,
        rippleControl: false,
        linearCorrection: false,
        sensorAngleDeg: 0,
        glassMode: false,
      },
    },
    {
      id: "builtin-balanced-1k",
      name: "オールラウンド標準 (1000Hz)",
      description: "1600 DPI・1000Hz・Motion Sync ON・デバウンス 4ms・バッテリー持続約160時間のバランス設定",
      builtIn: true,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
      pollingHz: 1000,
      dpi: {
        currentStage: 4,
        stageCount: 5,
        independentXY: false,
        stages: [
          { index: 1, x: 400, y: 400, enabled: true },
          { index: 2, x: 800, y: 800, enabled: true },
          { index: 3, x: 1200, y: 1200, enabled: true },
          { index: 4, x: 1600, y: 1600, enabled: true },
          { index: 5, x: 3200, y: 3200, enabled: true },
        ],
        allStages: [
          { index: 1, x: 400, y: 400, enabled: true },
          { index: 2, x: 800, y: 800, enabled: true },
          { index: 3, x: 1200, y: 1200, enabled: true },
          { index: 4, x: 1600, y: 1600, enabled: true },
          { index: 5, x: 3200, y: 3200, enabled: true },
          { index: 6, x: 6400, y: 6400, enabled: false },
          { index: 7, x: 1000, y: 1000, enabled: false },
        ],
      },
      sensor: {
        liftOffLevel: 1,
        motionSync: true,
        pressDebounceStep: 3,
        releaseDebounceStep: 3,
        sleepMinutes: 10,
        rippleControl: false,
        linearCorrection: false,
        sensorAngleDeg: 0,
        glassMode: false,
      },
    },
    {
      id: "builtin-desktop-eco",
      name: "デスクトップ省電力 (500Hz・長寿命)",
      description: "1200 DPI・500Hz・5分スリープ・作業やブラウジングで最大約185時間駆動する省電力設定",
      builtIn: true,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
      pollingHz: 500,
      dpi: {
        currentStage: 2,
        stageCount: 4,
        independentXY: false,
        stages: [
          { index: 1, x: 800, y: 800, enabled: true },
          { index: 2, x: 1200, y: 1200, enabled: true },
          { index: 3, x: 1600, y: 1600, enabled: true },
          { index: 4, x: 2400, y: 2400, enabled: true },
        ],
        allStages: [
          { index: 1, x: 800, y: 800, enabled: true },
          { index: 2, x: 1200, y: 1200, enabled: true },
          { index: 3, x: 1600, y: 1600, enabled: true },
          { index: 4, x: 2400, y: 2400, enabled: true },
          { index: 5, x: 3200, y: 3200, enabled: false },
          { index: 6, x: 6400, y: 6400, enabled: false },
          { index: 7, x: 1000, y: 1000, enabled: false },
        ],
      },
      sensor: {
        liftOffLevel: 2,
        motionSync: true,
        pressDebounceStep: 4,
        releaseDebounceStep: 4,
        sleepMinutes: 5,
        rippleControl: false,
        linearCorrection: false,
        sensorAngleDeg: 0,
        glassMode: false,
      },
    },
  ];

  resetForTests() {
    this.demoMode = false;
    this.mode = "normal";
    this.snap = makeDefaultSnapshot();
    this.listeners.clear();
  }

  on(eventName: string, cb: (...args: unknown[]) => void): () => void {
    if (!this.listeners.has(eventName)) {
      this.listeners.set(eventName, new Set());
    }
    this.listeners.get(eventName)!.add(cb);
    return () => {
      this.listeners.get(eventName)?.delete(cb);
    };
  }

  private emit(eventName: string, payload: unknown) {
    this.listeners.get(eventName)?.forEach((cb) => cb(payload));
  }

  getCatalog(): UICatalog {
    return {
      platform: this.prefs.platform,
      debounceSteps: DEFAULT_DEBOUNCE_STEPS,
      buttons: DEFAULT_BUTTON_SLOTS,
      supportedKeys: DEFAULT_KEYS,
      pollingRates: [125, 250, 500, 1000, 2000, 4000],
      linuxUdevRule:
        'SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", ATTRS{idProduct}=="1211|4411|1227|4427", MODE="0660", TAG+="uaccess"',
      linuxUdevInstallCmd:
        'echo \'SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", MODE="0660", TAG+="uaccess"\' | sudo tee /etc/udev/rules.d/70-rapoo-vt9pro.rules && sudo udevadm control --reload-rules && sudo udevadm trigger',
    };
  }

  getState(): ServiceState {
    if (!this.demoMode) {
      return {
        connected: false,
        demoMode: false,
        hasBattery: false,
        percentage: 0,
        charging: false,
        mouseOnline: false,
        sleeping: false,
        estimatedHours: 0,
        mode: this.mode,
        permissionDenied: false,
        platform: this.prefs.platform,
        updatedAt: new Date().toISOString(),
      };
    }
    const dpi = this.snap.dpi!;
    const activeIdx = Math.max(0, Math.min(dpi.stages.length - 1, dpi.currentStage - 1));
    return {
      connected: true,
      demoMode: true,
      deviceName: this.snap.model.name,
      connection: this.snap.model.connection,
      productId: this.snap.model.productId,
      maxRateHz: this.snap.model.maxRateHz,
      hasBattery: true,
      percentage: this.snap.battery.percentage,
      charging: this.snap.battery.charging,
      mouseOnline: this.snap.battery.online,
      sleeping: this.snap.battery.sleeping,
      estimatedHours: this.snap.estimatedHours,
      activeDpi: dpi.stages[activeIdx]?.x ?? 800,
      activeStage: dpi.currentStage,
      stageCount: dpi.stageCount,
      pollingRateHz: this.snap.sensor?.pollingRateHz ?? 1000,
      mode: this.mode,
      permissionDenied: false,
      platform: this.prefs.platform,
      updatedAt: new Date().toISOString(),
    };
  }

  private assertWritable() {
    if (this.mode === "passive_only") {
      throw new Error("ゲーム中ロック (完全受動モード) が有効なため設定を変更できません。上部のロックを解除してください。");
    }
    if (!this.demoMode) {
      throw new Error("Rapoo VT9 Pro 4K (4Kレシーバー または USBケーブル) が見つかりません。");
    }
  }

  private publishAll() {
    this.emit("rapoo:state", this.getState());
    if (this.demoMode) {
      this.emit("rapoo:snapshot", structuredClone(this.snap));
    }
  }

  readSnapshot(): DeviceSnapshot {
    if (!this.demoMode) {
      throw new Error("Rapoo VT9 Pro 4K (4Kレシーバー または USBケーブル) が見つかりません。");
    }
    return structuredClone(this.snap);
  }

  setDemoMode(enabled: boolean): DeviceSnapshot {
    this.demoMode = enabled;
    if (enabled && this.mode === "passive_only") {
      this.mode = "normal";
    }
    this.publishAll();
    if (!enabled) {
      throw new Error("Rapoo VT9 Pro 4K (4Kレシーバー または USBケーブル) が見つかりません。");
    }
    return structuredClone(this.snap);
  }

  setPollingMode(mode: PollingMode): ServiceState {
    this.mode = mode;
    const st = this.getState();
    this.emit("rapoo:state", st);
    return st;
  }

  setPollingRate(hz: number): DeviceSnapshot {
    this.assertWritable();
    this.snap.sensor!.pollingRateHz = hz;
    this.snap.estimatedHours = estimateBatteryHours(this.snap.battery.percentage, hz);
    this.publishAll();
    return structuredClone(this.snap);
  }

  setCurrentDPIGear(stage: number): DeviceSnapshot {
    this.assertWritable();
    const dpi = this.snap.dpi!;
    dpi.currentStage = Math.max(1, Math.min(dpi.stageCount, stage));
    this.publishAll();
    return structuredClone(this.snap);
  }

  setDPIConfig(cfg: DPIConfig): DeviceSnapshot {
    this.assertWritable();
    const count = Math.max(1, Math.min(7, cfg.stageCount));
    const baseAll = cfg.allStages?.length === 7 ? cfg.allStages : this.snap.dpi!.allStages;
    const allStages = baseAll.map((st, i) => {
      const incoming = cfg.stages.find((s) => s.index === i + 1) ?? cfg.allStages?.find((s) => s.index === i + 1) ?? st;
      return {
        index: i + 1,
        x: normalizeDpi(incoming.x),
        y: normalizeDpi(cfg.independentXY ? incoming.y : incoming.x),
        enabled: i < count,
      };
    });
    const stages = allStages.slice(0, count);
    const independentXY = stages.some((s) => s.x !== s.y);
    const currentStage = Math.max(1, Math.min(count, cfg.currentStage));
    this.snap.dpi = {
      currentStage,
      stageCount: count,
      independentXY,
      stages,
      allStages,
    };
    this.publishAll();
    return structuredClone(this.snap);
  }

  updateSensorConfig(patch: SensorPatch): DeviceSnapshot {
    this.assertWritable();
    const sc = this.snap.sensor!;
    if (patch.liftOffLevel !== undefined) {
      sc.liftOffLevel = Math.max(0, Math.min(10, patch.liftOffLevel));
      sc.liftOffDistanceMm = Math.round((1.0 + sc.liftOffLevel * 0.1) * 10) / 10;
      sc.silentHeightLod = sc.liftOffLevel + 1;
    }
    if (patch.motionSync !== undefined) sc.motionSync = patch.motionSync;
    if (patch.pressDebounceStep !== undefined) {
      const step = DEFAULT_DEBOUNCE_STEPS[patch.pressDebounceStep] ?? DEFAULT_DEBOUNCE_STEPS[3];
      sc.pressDebounceStep = step.index;
      sc.pressDebounceRaw = step.rawByte;
      sc.pressDebounceMs = Math.round(step.ms);
      sc.pressDebounceLabel = step.label;
    }
    if (patch.releaseDebounceStep !== undefined) {
      const step = DEFAULT_DEBOUNCE_STEPS[patch.releaseDebounceStep] ?? DEFAULT_DEBOUNCE_STEPS[3];
      sc.releaseDebounceStep = step.index;
      sc.releaseDebounceRaw = step.rawByte;
      sc.releaseDebounceMs = Math.round(step.ms);
      sc.releaseDebounceLabel = step.label;
    }
    if (patch.sleepMinutes !== undefined) {
      sc.sleepMinutes = Math.max(2, Math.min(120, patch.sleepMinutes));
    }
    if (patch.rippleControl !== undefined) sc.rippleControl = patch.rippleControl;
    if (patch.linearCorrection !== undefined) sc.linearCorrection = patch.linearCorrection;
    if (patch.sensorAngleDeg !== undefined) {
      sc.sensorAngleDeg = Math.max(-30, Math.min(30, patch.sensorAngleDeg));
    }
    if (patch.glassMode !== undefined) sc.glassMode = patch.glassMode;
    this.publishAll();
    return structuredClone(this.snap);
  }

  setButtonMapping(buttonId: number, action: ButtonAction): DeviceSnapshot {
    this.assertWritable();
    const next = (this.snap.buttons ?? makeDefaultButtons()).map((b) =>
      b.slot.id === buttonId ? { ...b, action: { ...action } } : b
    );
    const hasLeft = next.some((b) => b.slot.clickable && b.action.type === "left_click");
    if (!hasLeft) {
      throw new Error(
        "操作不能を防ぐため、少なくとも1つの物理ボタンに「左クリック」を残す必要があります。先に別のボタンへ左クリックを割り当ててください。"
      );
    }
    this.snap.buttons = next;
    this.publishAll();
    return structuredClone(this.snap);
  }

  resetButtonsToDefault(): DeviceSnapshot {
    this.assertWritable();
    this.snap.buttons = makeDefaultButtons();
    this.publishAll();
    return structuredClone(this.snap);
  }

  factoryReset(): DeviceSnapshot {
    this.assertWritable();
    this.snap = makeDefaultSnapshot();
    this.publishAll();
    return structuredClone(this.snap);
  }

  listPresets(): Preset[] {
    return structuredClone(this.presets);
  }

  savePreset(id: string, name: string, description: string): Preset {
    if (!name.trim()) throw new Error("プロファイル名を入力してください");
    const sc = this.snap.sensor!;
    const now = new Date().toISOString();
    const pid = id && !id.startsWith("builtin-") ? id : `preset-${Date.now()}`;
    const preset: Preset = {
      id: pid,
      name: name.trim(),
      description: description.trim(),
      builtIn: false,
      createdAt: now,
      updatedAt: now,
      pollingHz: sc.pollingRateHz,
      dpi: structuredClone(this.snap.dpi!),
      sensor: {
        liftOffLevel: sc.liftOffLevel,
        motionSync: sc.motionSync,
        pressDebounceStep: sc.pressDebounceStep,
        releaseDebounceStep: sc.releaseDebounceStep,
        sleepMinutes: sc.sleepMinutes,
        rippleControl: sc.rippleControl,
        linearCorrection: sc.linearCorrection,
        sensorAngleDeg: sc.sensorAngleDeg,
        glassMode: sc.glassMode,
      },
      buttons: structuredClone(this.snap.buttons),
    };
    const idx = this.presets.findIndex((p) => p.id === pid);
    if (idx >= 0) this.presets[idx] = preset;
    else this.presets.push(preset);
    return structuredClone(preset);
  }

  applyPreset(id: string): DeviceSnapshot {
    this.assertWritable();
    const p = this.presets.find((x) => x.id === id);
    if (!p) throw new Error("指定されたプロファイルが見つかりません");
    this.setPollingRate(p.pollingHz);
    this.setDPIConfig(p.dpi);
    this.updateSensorConfig(p.sensor);
    if (p.buttons?.length) {
      this.snap.buttons = structuredClone(p.buttons);
    }
    this.publishAll();
    return structuredClone(this.snap);
  }

  deletePreset(id: string) {
    this.presets = this.presets.filter((p) => p.id !== id || p.builtIn);
  }

  exportPresetJson(id: string): string {
    const p = this.presets.find((x) => x.id === id);
    if (!p) throw new Error("指定されたプロファイルが見つかりません");
    return JSON.stringify(p, null, 2);
  }

  importPresetJson(rawJson: string): Preset {
    const parsed = JSON.parse(rawJson) as Preset;
    if (!parsed.name?.trim()) throw new Error("プロファイル名が含まれていません");
    const now = new Date().toISOString();
    const preset: Preset = {
      ...parsed,
      id: `preset-${Date.now()}`,
      builtIn: false,
      createdAt: now,
      updatedAt: now,
    };
    this.presets.push(preset);
    return structuredClone(preset);
  }

  getPrefs(): AppPreferences {
    return { ...this.prefs };
  }

  setCloseToTray(enabled: boolean): AppPreferences {
    this.prefs.closeToTray = enabled;
    return { ...this.prefs };
  }

  setAutoStart(enabled: boolean): AppPreferences {
    this.prefs.autoStart = enabled;
    return { ...this.prefs };
  }
}

export const fallbackSim = new FallbackSimulator();

function unwrapError(err: unknown): Error {
  if (err instanceof Error) return err;
  if (typeof err === "string") return new Error(err);
  return new Error(String(err));
}

export const rapooApi = {
  async getCatalog(): Promise<UICatalog> {
    const app = getWailsApp();
    return app ? app.GetCatalog() : fallbackSim.getCatalog();
  },
  async getState(): Promise<ServiceState> {
    const app = getWailsApp();
    return app ? app.GetState() : fallbackSim.getState();
  },
  async refreshNow(): Promise<void> {
    const app = getWailsApp();
    if (app) await app.RefreshNow();
  },
  async readSnapshot(): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.ReadSnapshot() : fallbackSim.readSnapshot();
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async setDemoMode(enabled: boolean): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.SetDemoMode(enabled) : fallbackSim.setDemoMode(enabled);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async setPollingMode(mode: PollingMode): Promise<ServiceState> {
    const app = getWailsApp();
    return app ? app.SetPollingMode(mode) : fallbackSim.setPollingMode(mode);
  },
  async setPollingRate(hz: number): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.SetPollingRate(hz) : fallbackSim.setPollingRate(hz);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async setCurrentDPIGear(stage: number): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.SetCurrentDPIGear(stage) : fallbackSim.setCurrentDPIGear(stage);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async setDPIConfig(cfg: DPIConfig): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.SetDPIConfig(cfg) : fallbackSim.setDPIConfig(cfg);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async updateSensorConfig(patch: SensorPatch): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.UpdateSensorConfig(patch) : fallbackSim.updateSensorConfig(patch);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async setButtonMapping(buttonId: number, action: ButtonAction): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.SetButtonMapping(buttonId, action) : fallbackSim.setButtonMapping(buttonId, action);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async resetButtonsToDefault(): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.ResetButtonsToDefault() : fallbackSim.resetButtonsToDefault();
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async factoryReset(): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.FactoryReset() : fallbackSim.factoryReset();
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async listPresets(): Promise<Preset[]> {
    const app = getWailsApp();
    return app ? app.ListPresets() : fallbackSim.listPresets();
  },
  async savePreset(id: string, name: string, description: string): Promise<Preset> {
    const app = getWailsApp();
    try {
      return app ? await app.SavePreset(id, name, description) : fallbackSim.savePreset(id, name, description);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async applyPreset(id: string): Promise<DeviceSnapshot> {
    const app = getWailsApp();
    try {
      return app ? await app.ApplyPreset(id) : fallbackSim.applyPreset(id);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async deletePreset(id: string): Promise<void> {
    const app = getWailsApp();
    try {
      if (app) await app.DeletePreset(id);
      else fallbackSim.deletePreset(id);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async exportPresetJson(id: string): Promise<string> {
    const app = getWailsApp();
    try {
      return app ? await app.ExportPresetJSON(id) : fallbackSim.exportPresetJson(id);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async importPresetJson(rawJson: string): Promise<Preset> {
    const app = getWailsApp();
    try {
      return app ? await app.ImportPresetJSON(rawJson) : fallbackSim.importPresetJson(rawJson);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  async getAppPreferences(): Promise<AppPreferences> {
    const app = getWailsApp();
    return app ? app.GetAppPreferences() : fallbackSim.getPrefs();
  },
  async setCloseToTray(enabled: boolean): Promise<AppPreferences> {
    const app = getWailsApp();
    return app ? app.SetCloseToTray(enabled) : fallbackSim.setCloseToTray(enabled);
  },
  async setAutoStart(enabled: boolean): Promise<AppPreferences> {
    const app = getWailsApp();
    try {
      return app ? await app.SetAutoStart(enabled) : fallbackSim.setAutoStart(enabled);
    } catch (e) {
      throw unwrapError(e);
    }
  },
  subscribeEvent<T>(eventName: string, handler: (payload: T) => void): () => void {
    const rt = getWailsRuntime();
    if (rt) {
      const off = rt.EventsOn(eventName, (...args: unknown[]) => {
        handler(args[0] as T);
      });
      return typeof off === "function" ? off : () => {};
    }
    return fallbackSim.on(eventName, (...args: unknown[]) => {
      handler(args[0] as T);
    });
  },
};
