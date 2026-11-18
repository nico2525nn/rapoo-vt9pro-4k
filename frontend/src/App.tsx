import { useCallback, useEffect, useRef, useState } from "react";
import { Toaster, toast } from "sonner";
import { ButtonsTab } from "@/components/ButtonsTab";
import { DpiPollingTab } from "@/components/DpiPollingTab";
import { HeaderBar } from "@/components/HeaderBar";
import { ProfilesToolsTab } from "@/components/ProfilesToolsTab";
import { SensorTab } from "@/components/SensorTab";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
      // Ignore storage errors.
    }
    void rapooApi.setWindowTheme(darkMode);
  }, [darkMode]);

  const fetchSnapshotQuietly = useCallback(async () => {
    try {
      const snap = await rapooApi.readSnapshot();
      setSnapshot(snap);
      if (snap.dpi) {
        hasLoadedSnapshotRef.current = true;
      }
    } catch {
      // The header shows connection and sleep state.
    }
  }, []);

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
          // Sleeping or transient disconnect.
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

  const runMutation = async (
    fn: () => Promise<DeviceSnapshot>,
    successMessage?: string
  ): Promise<boolean> => {
    setBusy(true);
    try {
      const nextSnapshot = await fn();
      setSnapshot(nextSnapshot);
      if (nextSnapshot?.dpi) hasLoadedSnapshotRef.current = true;
      setState(await rapooApi.getState());
      if (successMessage) toast.success(successMessage);
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "設定の保存に失敗しました");
      return false;
    } finally {
      setBusy(false);
    }
  };

  const handleManualRefresh = async () => {
    setBusy(true);
    try {
      await rapooApi.refreshNow();
      const nextState = await rapooApi.getState();
      setState(nextState);
      if (nextState.connected && nextState.mode !== "passive_only") {
        const snap = await rapooApi.readSnapshot();
        setSnapshot(snap);
        if (snap.dpi) hasLoadedSnapshotRef.current = true;
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "マウス情報の取得に失敗しました");
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
      setState(await rapooApi.getState());
      toast.success(enabled ? "デモモードを開始しました" : "実機モードに切り替えました");
    } catch {
      const nextState = await rapooApi.getState();
      setState(nextState);
      if (!enabled) {
        hasLoadedSnapshotRef.current = false;
        setSnapshot(null);
        toast.info("デモモードを終了しました");
      }
    } finally {
      setBusy(false);
    }
  };

  const handleSetPollingMode = async (mode: PollingMode) => {
    try {
      const nextState = await rapooApi.setPollingMode(mode);
      setState(nextState);
      if (mode === "passive_only") {
        toast.info("ゲーム中ロックを有効にしました");
      } else {
        toast.success("通常の取得モードに戻りました");
        if (nextState.connected && !hasLoadedSnapshotRef.current) {
          void fetchSnapshotQuietly();
        }
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "モードの変更に失敗しました");
    }
  };

  const writable =
    Boolean(snapshot?.dpi) &&
    state.connected &&
    (state.mouseOnline || state.demoMode) &&
    !state.sleeping &&
    state.mode !== "passive_only";

  const hasDeviceData = Boolean(snapshot?.dpi && snapshot?.sensor);

  const handleSavePreset = async (name: string): Promise<boolean> => {
    try {
      await rapooApi.savePreset("", name, "");
      setPresets(await rapooApi.listPresets());
      toast.success(`プリセット「${name}」を保存しました`);
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "プリセットの保存に失敗しました");
      return false;
    }
  };

  const handleDeletePreset = async (id: string): Promise<boolean> => {
    try {
      await rapooApi.deletePreset(id);
      setPresets(await rapooApi.listPresets());
      toast.success("プリセットを削除しました");
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "削除に失敗しました");
      return false;
    }
  };

  const unavailable = (
    <DeviceUnavailableCard
      state={state}
      onEnableDemo={() => void handleToggleDemoMode(true)}
      onRefresh={() => void handleManualRefresh()}
    />
  );

  return (
    <TooltipProvider delayDuration={300}>
      <div className="flex min-h-screen flex-col bg-background">
        <HeaderBar
          state={state}
          snapshot={snapshot}
          busy={busy}
          darkMode={darkMode}
          onToggleTheme={() => setDarkMode((value) => !value)}
          onRefresh={() => void handleManualRefresh()}
        />

        <main className="mx-auto w-full max-w-4xl flex-1 px-6 py-4">
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="grid w-full grid-cols-4">
              <TabsTrigger value="dpi">DPI</TabsTrigger>
              <TabsTrigger value="sensor">センサー</TabsTrigger>
              <TabsTrigger value="buttons">ボタン</TabsTrigger>
              <TabsTrigger value="profiles">プロファイル</TabsTrigger>
            </TabsList>

            <TabsContent value="dpi">
              {hasDeviceData && snapshot ? (
                <DpiPollingTab
                  snapshot={snapshot}
                  writable={writable}
                  pollingRates={catalog?.pollingRates ?? [125, 250, 500, 1000, 2000, 4000]}
                  onSaveDpiConfig={(config: DPIConfig) => runMutation(() => rapooApi.setDPIConfig(config))}
                  onChangePollingRate={(hz: number) =>
                    runMutation(() => rapooApi.setPollingRate(hz), `${hz} Hz に変更しました`)
                  }
                />
              ) : (
                unavailable
              )}
            </TabsContent>

            <TabsContent value="sensor">
              {hasDeviceData && snapshot ? (
                <SensorTab
                  snapshot={snapshot}
                  debounceSteps={catalog?.debounceSteps ?? []}
                  writable={writable}
                  onUpdateSensor={(patch: SensorPatch) => runMutation(() => rapooApi.updateSensorConfig(patch))}
                />
              ) : (
                unavailable
              )}
            </TabsContent>

            <TabsContent value="buttons">
              {hasDeviceData && snapshot ? (
                <ButtonsTab
                  snapshot={snapshot}
                  supportedKeys={catalog?.supportedKeys ?? []}
                  writable={writable}
                  readOnlyReason={
                    state.mode === "passive_only"
                      ? "ゲーム中ロック中は変更できません。"
                      : state.sleeping
                        ? "マウスがスリープ中です。"
                        : undefined
                  }
                  onSaveButtonMapping={(buttonId: number, action: ButtonAction) =>
                    runMutation(
                      () => rapooApi.setButtonMapping(buttonId, action),
                      "ボタン割り当てを変更しました"
                    )
                  }
                  onResetAllButtons={() =>
                    runMutation(() => rapooApi.resetButtonsToDefault(), "ボタン割り当てを初期化しました")
                  }
                />
              ) : (
                unavailable
              )}
            </TabsContent>

            <TabsContent value="profiles">
              <ProfilesToolsTab
                state={state}
                snapshot={snapshot}
                catalog={catalog}
                presets={presets}
                prefs={prefs}
                writable={writable}
                onSavePreset={handleSavePreset}
                onApplyPreset={(preset) =>
                  runMutation(
                    () => rapooApi.applyPreset(preset.id),
                    `プリセット「${preset.name}」を適用しました`
                  )
                }
                onDeletePreset={handleDeletePreset}
                onSetPollingMode={handleSetPollingMode}
                onToggleDemoMode={(enabled) => void handleToggleDemoMode(enabled)}
                onUpdateCloseToTray={async (enabled) => {
                  const next = await rapooApi.setCloseToTray(enabled);
                  setPrefs(next);
                  toast.success(enabled ? "閉じたらトレイに格納します" : "閉じたらアプリを終了します");
                }}
                onUpdateAutoStart={async (enabled) => {
                  try {
                    const next = await rapooApi.setAutoStart(enabled);
                    setPrefs(next);
                    toast.success(enabled ? "スタートアップに登録しました" : "スタートアップから解除しました");
                  } catch (error) {
                    toast.error(error instanceof Error ? error.message : "自動起動の変更に失敗しました");
                  }
                }}
                onFactoryReset={() =>
                  runMutation(() => rapooApi.factoryReset(), "工場出荷時設定に戻しました")
                }
              />
            </TabsContent>
          </Tabs>
        </main>

        <Toaster theme={darkMode ? "dark" : "light"} position="bottom-right" />
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
  const sleeping = state.connected && state.sleeping;
  const unavailableMessage = sleeping
    ? "マウスを動かすと設定を読み込みます。"
    : state.permissionDenied
      ? "hidraw へのアクセス権限がありません。プロファイルタブの udev 設定を確認してください。"
      : "4K レシーバーまたは USB-C ケーブルを接続してください。";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{sleeping ? "マウスがスリープ中です" : "マウスが接続されていません"}</CardTitle>
        <CardDescription>{unavailableMessage}</CardDescription>
      </CardHeader>
      <CardContent className="flex gap-2">
        <Button onClick={onEnableDemo}>デモモード</Button>
        <Button variant="outline" onClick={onRefresh}>
          再読み込み
        </Button>
      </CardContent>
    </Card>
  );
}
