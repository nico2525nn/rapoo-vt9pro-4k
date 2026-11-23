export type ConnectionMode = "wired" | "wireless_4k" | "";

export type PollingMode = "normal" | "low_freq" | "passive_only";

export interface ModelSpec {
  name: string;
  vendorId: number;
  productId: number;
  connection: ConnectionMode;
  protocolGen?: number;
  maxRateHz: number;
}

export interface BatteryState {
  percentage: number;
  charging: boolean;
  online: boolean;
  sleeping: boolean;
  source: string;
  updatedAt: string;
}

export interface FirmwareInfo {
  mouseRaw: number;
  mouseString: string;
  dongleRaw?: number;
  dongleString?: string;
}

export interface DPIStage {
  index: number; // 1..7
  x: number;
  y: number;
  enabled: boolean;
}

export interface DPIConfig {
  currentStage: number; // 1..stageCount
  stageCount: number; // 1..7
  independentXY: boolean;
  stages: DPIStage[];
  allStages: DPIStage[];
}

export interface DebounceStep {
  index: number; // 0..6
  rawByte: number;
  ms: number;
  label: string;
}

export interface SensorConfig {
  pollingRateHz: number;
  wireless8kSwitch: number;
  silentHeightLod: number;
  liftOffLevel: number; // 0..10 (1.0mm..2.0mm)
  liftOffDistanceMm: number;
  motionSync: boolean;
  pressDebounceRaw: number;
  pressDebounceStep: number; // 0..6
  pressDebounceMs: number;
  pressDebounceLabel: string;
  releaseDebounceRaw: number;
  releaseDebounceStep: number; // 0..6
  releaseDebounceMs: number;
  releaseDebounceLabel: string;
  sleepMinutes: number; // 2..120
  rippleControl: boolean;
  linearCorrection: boolean;
  sensorAngleDeg: number; // -30..+30
  glassMode: boolean;
  ledLowPowerMode: number;
}

export interface SensorPatch {
  liftOffLevel?: number;
  motionSync?: boolean;
  pressDebounceStep?: number;
  releaseDebounceStep?: number;
  sleepMinutes?: number;
  rippleControl?: boolean;
  linearCorrection?: boolean;
  sensorAngleDeg?: number;
  glassMode?: boolean;
  ledLowPowerMode?: number;
}

export type ButtonActionType =
  | "left_click"
  | "right_click"
  | "middle_click"
  | "forward"
  | "back"
  | "wheel_default"
  | "fire_triple"
  | "dpi_plus"
  | "dpi_minus"
  | "dpi_cycle"
  | "sniper_dpi"
  | "profile_switch"
  | "polling_switch"
  | "receiver_switch"
  | "media_play_pause"
  | "media_stop"
  | "media_prev"
  | "media_next"
  | "media_vol_up"
  | "media_vol_down"
  | "media_mute"
  | "media_player"
  | "app_mail"
  | "app_calculator"
  | "app_explorer"
  | "keyboard"
  | "disabled"
  | "unknown";

export interface KeyModifiers {
  ctrl: boolean;
  shift: boolean;
  alt: boolean;
  meta: boolean;
}

export interface ButtonAction {
  type: ButtonActionType;
  labelJa: string;
  category: string;
  modifiers?: KeyModifiers;
  keyCode?: number;
  keyName?: string;
  raw?: number[];
}

export interface ButtonSlotSpec {
  id: number;
  nameJa: string;
  nameEn: string;
  view: "top" | "bottom" | string;
  addr: number;
  defaultAction: ButtonActionType;
  clickable: boolean;
}

export interface ButtonMapping {
  slot: ButtonSlotSpec;
  action: ButtonAction;
}

export interface DeviceSnapshot {
  model: ModelSpec;
  commandHidPath: string;
  reportHidPath?: string;
  battery: BatteryState;
  estimatedHours: number;
  firmware?: FirmwareInfo;
  dpi?: DPIConfig;
  sensor?: SensorConfig;
  buttons?: ButtonMapping[];
  demoMode?: boolean;
}

export interface ServiceState {
  connected: boolean;
  demoMode: boolean;
  deviceName?: string;
  connection?: ConnectionMode;
  productId?: number;
  maxRateHz?: number;
  hasBattery: boolean;
  percentage: number;
  charging: boolean;
  mouseOnline: boolean;
  sleeping: boolean;
  estimatedHours: number;
  activeDpi?: number;
  activeStage?: number;
  stageCount?: number;
  pollingRateHz?: number;
  mode: PollingMode;
  permissionDenied: boolean;
  platform: string;
  updatedAt: string;
  lastError?: string;
}

export interface HIDKeyInfo {
  code: number;
  name: string;
  domCode: string;
}

export interface UICatalog {
  platform: string;
  debounceSteps: DebounceStep[];
  buttons: ButtonSlotSpec[];
  supportedKeys: HIDKeyInfo[];
  pollingRates: number[];
  linuxUdevRule: string;
  linuxUdevInstallCmd: string;
}

export interface Preset {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
  dpi: DPIConfig;
  pollingHz: number;
  sensor: SensorPatch;
  buttons?: ButtonMapping[];
}

export interface AppPreferences {
  closeToTray: boolean;
  autoStart: boolean;
  startHidden: boolean;
  platform: string;
  language?: string;
}
