import { Cpu, Crosshair, FolderCog, Moon, MousePointer, Sparkles, WifiOff } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Toaster, toast } from "sonner";
import { ButtonsTab } from "@/components/ButtonsTab";
import { DpiPollingTab } from "@/components/DpiPollingTab";
import { HeaderBar } from "@/components/HeaderBar";
import { ProfilesToolsTab } from "@/components/ProfilesToolsTab";
import { SensorTab } from "@/components/SensorTab";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TooltipProvider } from "@/components/ui/tooltip";
import { rapooApi } from "@/lib/wails";
import type {
  AppPreferences,
  ButtonAction,
  DeviceSnapshot,
  DPIConfig,
  PollingMode,
  Preset,
  SensorPatch,
  ServiceState,
  UICatalog,
} from "@/types/rapoo";

const INITIAL_STATE: ServiceState = {
  connected: false,
  demoMode: false,
  hasBattery: false,
  percentage: 0,
  charging: false,
  mouseOnline: false,
  sleeping: false,
  estimatedHours: 0,
  mode: "normal",
  permissionDenied: false,
  platform: "windows",
  updatedAt: "",
};

export default function App() {
  const [darkMode, setDarkMode] = useState<boolean>(() => {
    if (typeof window === "undefined") return true;
    const saved = window.localStorage?.getItem("rapoo-theme");
    return saved ? saved === "dark" : true;
  });

  const [activeTab, setActiveTab] = useState<string>("dpi");
  const [catalog, setCatalog] = useState<UICatalog | null>(null);
  const [state, setState] = useState<ServiceState>(INITIAL_STATE);
  const [snapshot, setSnapshot] = useState<DeviceSnapshot | null>(null);
  const [presets, setPresets] = useState<Preset[]>([]);
  const [prefs, setPrefs] = useState<AppPreferences | null>(null);
  const [busy, setBusy] = useState<boolean>(false);

  const hasLoadedSnapshotRef = useRef<boolean>(false);

  useEffect(() => {
    const root = document.documentElement;
    if (darkMode) {
      root.classList.add("dark");
    } else {
      root.classList.remove("dark");
    }
    try {
      window.localStorage?.setItem("rapoo-theme", darkMode ? "dark" : "light");
    } catch {
      // Ignore storage errors
    }
  }, [darkMode]);

  const fetchSnapshotQuietly = useCallback(async () => {
    try {
      const snap = await rapooApi.readSnapshot();
      setSnapshot(snap);
      if (snap.dpi) {
        hasLoadedSnapshotRef.current = true;
      }
    } catch {
      // Handled via state banners (disconnected / sleeping)
    }
  }, []);

  // Initial load + Wails event subscriptions
  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      const [cat, st, presetList, appPrefs] = await Promise.all([
        rapooApi.getCatalog(),
        rapooApi.getState(),
        rapooApi.listPresets(),
        rapooApi.getAppPreferences(),
      ]);
      if (cancelled) return;
      setCatalog(cat);
      setState(st);
      setPresets(presetList);
      setPrefs(appPrefs);

      if (st.connected) {
        try {
          const snap = await rapooApi.readSnapshot();
          if (!cancelled) {
            setSnapshot(snap);
            if (snap.dpi) hasLoadedSnapshotRef.current = true;
          }
        } catch {
          // Sleeping or transient disconnect
        }
      }
    };

    void bootstrap();

    const offState = rapooApi.subscribeEvent<ServiceState>("rapoo:state", (nextState) => {
      setState(nextState);
      if (!nextState.connected) {
        hasLoadedSnapshotRef.current = false;
        setSnapshot(null);
      } else if (nextState.mouseOnline && !hasLoadedSnapshotRef.current && nextState.mode !== "passive_only") {
        void fetchSnapshotQuietly();
      }
    });

    const offSnap = rapooApi.subscribeEvent<DeviceSnapshot>("rapoo:snapshot", (nextSnap) => {
      setSnapshot(nextSnap);
      if (nextSnap?.dpi) {
        hasLoadedSnapshotRef.current = true;
      }
    });

    const offWake = rapooApi.subscribeEvent<boolean>("rapoo:wake", () => {
      void fetchSnapshotQuietly();
    });

    return () => {
      cancelled = true;
      offState();
      offSnap();
      offWake();
    };
  }, [fetchSnapshotQuietly]);

  const runMutation = async (fn: () => Promise<DeviceSnapshot>, successMsg?: string) => {
    setBusy(true);
    try {
      const nextSnap = await fn();
      setSnapshot(nextSnap);
      if (nextSnap?.dpi) hasLoadedSnapshotRef.current = true;
      const nextState = await rapooApi.getState();
      setState(nextState);
      if (successMsg) {
        toast.success(successMsg);
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "設定の保存に失敗しました");
    } finally {
      setBusy(false);
    }
  };

  const handleManualRefresh = async () => {
    setBusy(true);
    try {
      await rapooApi.refreshNow();
      const st = await rapooApi.getState();
      setState(st);
      if (st.connected && st.mode !== "passive_only") {
        const snap = await rapooApi.readSnapshot();
        setSnapshot(snap);
        if (snap.dpi) hasLoadedSnapshotRef.current = true;
        toast.success("マウスの最新設定を読み込みました");
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "マウス情報の取得に失敗しました");
    } finally {
      setBusy(false);
    }
  };

  const handleToggleDemoMode = async (enabled: boolean) => {
    setBusy(true);
    try {
      const snap = await rapooApi.setDemoMode(enabled);
      setSnapshot(snap);
      hasLoadedSnapshotRef.current = Boolean(snap?.dpi);
      const st = await rapooApi.getState();
      setState(st);
      toast.success(enabled ? "デモモードを開始しました" : "実機モードに切り替えました");
    } catch {
      const st = await rapooApi.getState();
      setState(st);
      if (!enabled) {
        hasLoadedSnapshotRef.current = false;
        setSnapshot(null);
        toast.info("デモモードを終了しました（実機未接続）");
      }
    } finally {
      setBusy(false);
    }
  };

  const handleSetPollingMode = async (mode: PollingMode) => {
    const next = await rapooApi.setPollingMode(mode);
    setState(next);
    if (mode === "passive_only") {
      toast.info("ゲーム中ロックを有効化しました（0-TX 完全受動モード）");
    } else {
      toast.success("通常通信モードに戻りました");
      if (next.connected && !hasLoadedSnapshotRef.current) {
        void fetchSnapshotQuietly();
      }
    }
  };

  const writable =
    Boolean(snapshot?.dpi) &&
    state.connected &&
    (state.mouseOnline || state.demoMode) &&
    !state.sleeping &&
    state.mode !== "passive_only";

  const hasDeviceData = Boolean(snapshot?.dpi && snapshot?.sensor);

  return (
    <TooltipProvider delayDuration={220}>
      <div className="min-h-screen flex flex-col bg-background text-foreground">
        <HeaderBar
          state={state}
          snapshot={snapshot}
          catalog={catalog}
          busy={busy}
          darkMode={darkMode}
          onToggleTheme={() => setDarkMode((d) => !d)}
          onRefresh={() => void handleManualRefresh()}
          onToggleDemoMode={(en) => void handleToggleDemoMode(en)}
          onToggleGamingLock={(locked) => void handleSetPollingMode(locked ? "passive_only" : "normal")}
          onNavigateTab={setActiveTab}
        />

        <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-4">
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="w-full sm:w-auto grid grid-cols-4 sm:inline-flex">
              <TabsTrigger value="dpi">
                <Crosshair className="h-3.5 w-3.5 hidden sm:inline" />
                DPI & レート
              </TabsTrigger>
              <TabsTrigger value="sensor">
                <Cpu className="h-3.5 w-3.5 hidden sm:inline" />
                センサー & スイッチ
              </TabsTrigger>
              <TabsTrigger value="buttons">
                <MousePointer className="h-3.5 w-3.5 hidden sm:inline" />
                10ボタン割り当て
              </TabsTrigger>
              <TabsTrigger value="profiles">
                <FolderCog className="h-3.5 w-3.5 hidden sm:inline" />
                プロファイル & 診断
              </TabsTrigger>
            </TabsList>

            {/* Tab 1: DPI & Polling Rate */}
            <TabsContent value="dpi">
              {hasDeviceData && snapshot ? (
                <DpiPollingTab
                  snapshot={snapshot}
                  writable={writable}
                  busy={busy}
                  pollingRates={catalog?.pollingRates ?? [125, 250, 500, 1000, 2000, 4000]}
                  onSaveDpiConfig={(cfg: DPIConfig) => runMutation(() => rapooApi.setDPIConfig(cfg))}
                  onSelectActiveStage={(stage: number) =>
                    runMutation(
                      () => rapooApi.setCurrentDPIGear(stage),
                      `Stage ${stage} に切り替えました`
                    )
                  }
                  onChangePollingRate={(hz: number) =>
                    runMutation(
                      () => rapooApi.setPollingRate(hz),
                      `ポーリングレートを ${hz} Hz に変更しました`
                    )
                  }
                />
              ) : (
                <DeviceUnavailableCard
                  state={state}
                  onEnableDemo={() => void handleToggleDemoMode(true)}
                  onRefresh={() => void handleManualRefresh()}
                />
              )}
            </TabsContent>

            {/* Tab 2: Sensor & Switch Debounce */}
            <TabsContent value="sensor">
              {hasDeviceData && snapshot ? (
                <SensorTab
                  snapshot={snapshot}
                  debounceSteps={catalog?.debounceSteps ?? []}
                  writable={writable}
                  busy={busy}
                  onUpdateSensor={(patch: SensorPatch) =>
                    runMutation(() => rapooApi.updateSensorConfig(patch))
                  }
                />
              ) : (
                <DeviceUnavailableCard
                  state={state}
                  onEnableDemo={() => void handleToggleDemoMode(true)}
                  onRefresh={() => void handleManualRefresh()}
                />
              )}
            </TabsContent>

            {/* Tab 3: 10 Physical Buttons */}
            <TabsContent value="buttons">
              {hasDeviceData && snapshot ? (
                <ButtonsTab
                  snapshot={snapshot}
                  supportedKeys={catalog?.supportedKeys ?? []}
                  writable={writable}
                  busy={busy}
                  onSaveButtonMapping={(buttonId: number, action: ButtonAction) =>
                    runMutation(
                      () => rapooApi.setButtonMapping(buttonId, action),
                      `ボタン割り当てを「${action.labelJa}」に更新しました`
                    )
                  }
                  onResetAllButtons={() =>
                    runMutation(
                      () => rapooApi.resetButtonsToDefault(),
                      "全10ボタンを初期値に戻しました"
                    )
                  }
                />
              ) : (
                <DeviceUnavailableCard
                  state={state}
                  onEnableDemo={() => void handleToggleDemoMode(true)}
                  onRefresh={() => void handleManualRefresh()}
                />
              )}
            </TabsContent>

            {/* Tab 4: Profiles, Diagnostics & System Settings (Always accessible!) */}
            <TabsContent value="profiles">
              <ProfilesToolsTab
                state={state}
                snapshot={snapshot}
                catalog={catalog}
                presets={presets}
                prefs={prefs}
                writable={writable}
                busy={busy}
                onSavePreset={async (name, desc) => {
                  setBusy(true);
                  try {
                    await rapooApi.savePreset("", name, desc);
                    setPresets(await rapooApi.listPresets());
                    toast.success(`プロファイル「${name}」を保存しました`);
                  } catch (e) {
                    toast.error(e instanceof Error ? e.message : "プロファイルの保存に失敗しました");
                  } finally {
                    setBusy(false);
                  }
                }}
                onApplyPreset={(id) =>
                  runMutation(
                    () => rapooApi.applyPreset(id),
                    "プロファイルをマウス本体に適用しました"
                  )
                }
                onDeletePreset={async (id) => {
                  try {
                    await rapooApi.deletePreset(id);
                    setPresets(await rapooApi.listPresets());
                    toast.success("プロファイルを削除しました");
                  } catch (e) {
                    toast.error(e instanceof Error ? e.message : "削除に失敗しました");
                  }
                }}
                onImportPreset={async (rawJson) => {
                  try {
                    const imported = await rapooApi.importPresetJson(rawJson);
                    setPresets(await rapooApi.listPresets());
                    toast.success(`「${imported.name}」をインポートしました`);
                  } catch (e) {
                    toast.error(e instanceof Error ? e.message : "JSONのインポートに失敗しました");
                  }
                }}
                onSetPollingMode={handleSetPollingMode}
                onToggleDemoMode={(en) => void handleToggleDemoMode(en)}
                onUpdateCloseToTray={async (en) => {
                  const next = await rapooApi.setCloseToTray(en);
                  setPrefs(next);
                }}
                onUpdateAutoStart={async (en) => {
                  try {
                    const next = await rapooApi.setAutoStart(en);
                    setPrefs(next);
                    toast.success(en ? "Windows 自動起動を有効にしました" : "Windows 自動起動を解除しました");
                  } catch (e) {
                    toast.error(e instanceof Error ? e.message : "自動起動設定の変更に失敗しました");
                  }
                }}
                onFactoryReset={() =>
                  runMutation(
                    () => rapooApi.factoryReset(),
                    "マウス本体を工場出荷時の設定に初期化しました"
                  )
                }
              />
            </TabsContent>
          </Tabs>
        </main>

        <Toaster
          theme={darkMode ? "dark" : "light"}
          position="bottom-right"
          richColors
          toastOptions={{
            className: "text-xs",
          }}
        />
      </div>
    </TooltipProvider>
  );
}

function DeviceUnavailableCard({
  state,
  onEnableDemo,
  onRefresh,
}: {
  state: ServiceState;
  onEnableDemo: () => void;
  onRefresh: () => void;
}) {
  if (state.connected && state.sleeping) {
    return (
      <Card className="border-dashed">
        <CardContent className="flex flex-col items-center justify-center py-14 text-center space-y-3">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-amber-500/15 text-amber-500">
            <Moon className="h-6 w-6" />
          </div>
          <div className="space-y-1 max-w-md">
            <h2 className="text-base font-semibold">ワイヤレスマウスがスリープ中です</h2>
            <p className="text-xs text-muted-foreground leading-relaxed">
              4Kワイヤレスレシーバーは接続されています。マウス本体を軽く動かすか電源スイッチをONにすると、自動的にオンボード設定を読み込みます。
            </p>
          </div>
          <div className="flex items-center gap-2 pt-1">
            <Button size="sm" variant="outline" onClick={onRefresh}>
              再読み込み
            </Button>
            <Button size="sm" variant="secondary" onClick={onEnableDemo}>
              デモモードを見る
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="border-dashed">
      <CardContent className="flex flex-col items-center justify-center py-14 text-center space-y-3">
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/15 text-primary">
          <WifiOff className="h-6 w-6" />
        </div>
        <div className="space-y-1 max-w-md">
          <h2 className="text-base font-semibold">Rapoo VT9 Pro 4K を接続してください</h2>
          <p className="text-xs text-muted-foreground leading-relaxed">
            4KワイヤレスレシーバーまたはUSB-CケーブルをPCへ接続すると自動で認識します。実機が手元にない場合でも、仮想シミュレーターですべての機能を今すぐ体験できます。
          </p>
        </div>
        <div className="flex items-center gap-2 pt-1">
          <Button onClick={onEnableDemo}>
            <Sparkles className="h-4 w-4" />
            デモモードで全機能を試す
          </Button>
          <Button variant="outline" onClick={onRefresh}>
            再検索
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
