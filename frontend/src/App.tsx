import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { Toaster, toast } from "sonner";
import { AppSidebar, NAV_ITEMS, type Page } from "@/components/AppSidebar";
import { ButtonsTab } from "@/components/ButtonsTab";
import { DpiPollingTab } from "@/components/DpiPollingTab";
import { ProfilesToolsTab } from "@/components/ProfilesToolsTab";
import { SettingsTab } from "@/components/SettingsTab";
import { SensorTab } from "@/components/SensorTab";
import { WindowControls } from "@/components/WindowControls";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { normalizeLanguage, setLanguage as setI18nLanguage, t, translateError, type Language } from "@/lib/i18n";
import { rapooApi, windowApi } from "@/lib/wails";
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

const dragStyle = { "--wails-draggable": "drag" } as CSSProperties;

export default function App() {
  const [darkMode, setDarkMode] = useState<boolean>(() => {
    if (typeof window === "undefined") return true;
    const saved = window.localStorage?.getItem("rapoo-theme");
    return saved ? saved === "dark" : true;
  });

  const [activeTab, setActiveTab] = useState<Page>("dpi");
  const [catalog, setCatalog] = useState<UICatalog | null>(null);
  const [state, setState] = useState<ServiceState>(INITIAL_STATE);
  const [snapshot, setSnapshot] = useState<DeviceSnapshot | null>(null);
  const [presets, setPresets] = useState<Preset[]>([]);
  const [prefs, setPrefs] = useState<AppPreferences | null>(null);
  const [busy, setBusy] = useState<boolean>(false);
  const [language, setLanguageState] = useState<Language>("en");

  const hasLoadedSnapshotRef = useRef<boolean>(false);
  const snapshotFloorRef = useRef<{ generation: number; revision: number }>({
    generation: 0,
    revision: 0,
  });

  const updateState = useCallback((next: ServiceState) => {
    setState((prev) => {
      const prevGeneration = prev.generation ?? 0;
      const nextGeneration = next.generation ?? 0;
      if (prevGeneration > 0 && nextGeneration > 0 && nextGeneration < prevGeneration) {
        return prev;
      }
      const prevRevision = prev.revision ?? 0;
      const nextRevision = next.revision ?? 0;
      if (prevGeneration > 0 && nextGeneration > 0 && prevGeneration === nextGeneration &&
          prevRevision > 0 && nextRevision > 0 && nextRevision < prevRevision) {
        return prev;
      }
      return next;
    });
  }, []);

  const updateSnapshot = useCallback((next: DeviceSnapshot | null) => {
    if (next === null) {
      setSnapshot(null);
      // Keep the floor: an in-flight response from the disconnected device
      // must not reappear after this point.
      return;
    }
    const floor = snapshotFloorRef.current;
    const nextGeneration = next.generation ?? 0;
    const nextRevision = next.stateRevision ?? 0;
    if (nextGeneration > 0 && floor.generation > 0 && nextGeneration < floor.generation) {
      return;
    }
    if (nextGeneration > 0 && nextGeneration === floor.generation &&
        nextRevision > 0 && floor.revision > 0 && nextRevision < floor.revision) {
      return;
    }
    if (nextGeneration > 0) {
      floor.generation = nextGeneration;
      floor.revision = nextRevision > 0 ? nextRevision : floor.revision;
    }
    setSnapshot(next);
  }, []);

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

  useEffect(() => {
    if (state.connected && state.lastError) {
      toast.error(translateError(state.lastError), { id: "device-error" });
    }
  }, [state.connected, state.lastError]);

  const fetchSnapshotQuietly = useCallback(async () => {
    try {
      const snap = await rapooApi.readSnapshot();
      updateSnapshot(snap);
      if (snap.dpi) {
        hasLoadedSnapshotRef.current = true;
      }
    } catch {
      // Connection and sleep state remain available in the sidebar.
    }
  }, [updateSnapshot]);

  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      const [cat, st, appPrefs] = await Promise.all([
        rapooApi.getCatalog(),
        rapooApi.getState(),
        rapooApi.getAppPreferences(),
      ]);
      if (cancelled) return;
      setCatalog(cat);
      updateState(st);
      setPrefs(appPrefs);
      try {
        setPresets(await rapooApi.listPresets());
      } catch (error) {
        toast.error(error instanceof Error ? error.message : t("Failed to load profiles"));
      }
      const initialLanguage = normalizeLanguage(appPrefs.language);
      setI18nLanguage(initialLanguage);
      setLanguageState(initialLanguage);

      if (st.connected) {
        try {
          const snap = await rapooApi.readSnapshot();
          if (!cancelled) {
            updateSnapshot(snap);
            if (snap.dpi) hasLoadedSnapshotRef.current = true;
          }
        } catch {
          // Sleeping or transient disconnect.
        }
      }
    };

    void bootstrap().catch((error) => {
      if (!cancelled) {
        toast.error(error instanceof Error ? error.message : t("Failed to initialize"));
      }
    });

    const offState = rapooApi.subscribeEvent<ServiceState>("rapoo:state", (nextState) => {
      updateState(nextState);
      if (!nextState.connected) {
        hasLoadedSnapshotRef.current = false;
        updateSnapshot(null);
      } else if (nextState.mouseOnline && !hasLoadedSnapshotRef.current && nextState.mode !== "passive_only") {
        void fetchSnapshotQuietly();
      }
    });

    const offSnap = rapooApi.subscribeEvent<DeviceSnapshot>("rapoo:snapshot", (nextSnap) => {
      updateSnapshot(nextSnap);
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
  }, [fetchSnapshotQuietly, updateState, updateSnapshot]);

  useEffect(() => {
    setI18nLanguage(language);
  }, [language]);

  const handleChangeLanguage = async (next: Language) => {
    setI18nLanguage(next);
    setLanguageState(next);
    try {
      setPrefs(await rapooApi.setLanguage(next));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Failed to change language"));
    }
  };

  const runMutation = async (
    fn: () => Promise<DeviceSnapshot>,
    successMessage?: string
  ): Promise<boolean> => {
    setBusy(true);
    try {
      const nextSnapshot = await fn();
      updateSnapshot(nextSnapshot);
      if (nextSnapshot?.dpi) hasLoadedSnapshotRef.current = true;
      updateState(await rapooApi.getState());
      if (successMessage) toast.success(successMessage);
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Failed to save settings"));
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
      updateState(nextState);
      if (nextState.connected && nextState.mode !== "passive_only") {
        const snap = await rapooApi.readSnapshot();
        updateSnapshot(snap);
        if (snap.dpi) hasLoadedSnapshotRef.current = true;
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Failed to read mouse information"));
    } finally {
      setBusy(false);
    }
  };

  const handleToggleDemoMode = async (enabled: boolean) => {
    setBusy(true);
    try {
      const snap = await rapooApi.setDemoMode(enabled);
      updateSnapshot(snap);
      hasLoadedSnapshotRef.current = Boolean(snap?.dpi);
      updateState(await rapooApi.getState());
      toast.success(enabled ? t("Demo mode started") : t("Switched to hardware mode"));
    } catch {
      const nextState = await rapooApi.getState();
      updateState(nextState);
      if (!enabled) {
        hasLoadedSnapshotRef.current = false;
        updateSnapshot(null);
        toast.info(t("Demo mode ended"));
      }
    } finally {
      setBusy(false);
    }
  };

  const handleSetPollingMode = async (mode: PollingMode) => {
    try {
      const nextState = await rapooApi.setPollingMode(mode);
      updateState(nextState);
      if (mode === "passive_only") {
        toast.info(t("In-game lock enabled"));
      } else {
        toast.success(t("Returned to normal polling mode"));
        if (nextState.connected && !hasLoadedSnapshotRef.current) {
          void fetchSnapshotQuietly();
        }
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Failed to change mode"));
    }
  };

  const writable =
    Boolean(snapshot?.dpi) &&
    state.connected &&
    (state.mouseOnline || state.demoMode) &&
    !state.sleeping &&
    state.mode !== "passive_only" &&
    !busy;

  const hasDeviceData = Boolean(snapshot?.dpi && snapshot?.sensor);

  const handleSavePreset = async (name: string): Promise<boolean> => {
    try {
      await rapooApi.savePreset("", name, "");
      setPresets(await rapooApi.listPresets());
      toast.success(t("Saved preset “{name}”", { name }));
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Failed to save preset"));
      return false;
    }
  };

  const handleDeletePreset = async (id: string): Promise<boolean> => {
    try {
      await rapooApi.deletePreset(id);
      setPresets(await rapooApi.listPresets());
      toast.success(t("Preset deleted"));
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Delete failed"));
      return false;
    }
  };

  const handleOpenFirmwareUpdater = async () => {
    try {
      const path = await rapooApi.openFirmwareUpdater();
      if (path) toast.success(t("Launched the official software"));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("Could not launch the official software"));
    }
  };

  const unavailable = (
    <DeviceUnavailableCard
      state={state}
      busy={busy}
      onEnableDemo={() => void handleToggleDemoMode(true)}
      onRefresh={() => void handleManualRefresh()}
    />
  );

  return (
    <div className="relative flex h-screen overflow-hidden bg-background">
      <AppSidebar
        state={state}
        snapshot={snapshot}
        activeTab={activeTab}
        onNavigate={setActiveTab}
      />

      <div className="flex min-w-0 flex-1 flex-col">
        <div
          className="mr-36 h-10 shrink-0"
          style={dragStyle}
          onDoubleClick={() => void windowApi.toggleMaximise()}
          aria-hidden="true"
        />
        <main key={activeTab} className="min-h-0 flex-1 overflow-y-auto bg-content-layer">
          <div className="@container px-6 pt-3 pb-8 lg:px-8 xl:px-10">
            <h1 className="mb-6 text-[1.75rem] leading-tight font-semibold">
              {t(NAV_ITEMS.find((item) => item.value === activeTab)?.label ?? "")}
            </h1>

            {activeTab === "dpi" &&
              (hasDeviceData && snapshot ? (
                <DpiPollingTab
                  snapshot={snapshot}
                  writable={writable}
                  pollingRates={catalog?.pollingRates ?? [125, 250, 500, 1000, 2000, 4000]}
                  onSaveDpiConfig={(config: DPIConfig) => runMutation(() => rapooApi.setDPIConfig(config))}
                  onChangePollingRate={(hz: number) =>
                    runMutation(() => rapooApi.setPollingRate(hz), t("Changed polling rate to {hz} Hz", { hz }))
                  }
                />
              ) : (
                unavailable
              ))}

            {activeTab === "sensor" &&
              (hasDeviceData && snapshot ? (
                <SensorTab
                  snapshot={snapshot}
                  debounceSteps={catalog?.debounceSteps ?? []}
                  writable={writable}
                  onUpdateSensor={(patch: SensorPatch) => runMutation(() => rapooApi.updateSensorConfig(patch))}
                />
              ) : (
                unavailable
              ))}

            {activeTab === "buttons" &&
              (hasDeviceData && snapshot ? (
                <ButtonsTab
                  snapshot={snapshot}
                  supportedKeys={catalog?.supportedKeys ?? []}
                  writable={writable}
                  readOnlyReason={
                    state.mode === "passive_only"
                      ? t("Cannot change while in-game lock is active.")
                      : state.sleeping
                        ? t("The mouse is sleeping.")
                        : undefined
                  }
                  onSaveButtonMapping={(buttonId: number, action: ButtonAction) =>
                    runMutation(
                      () => rapooApi.setButtonMapping(buttonId, action),
                      t("Button assignment updated")
                    )
                  }
                  onResetAllButtons={() =>
                    runMutation(() => rapooApi.resetButtonsToDefault(), t("Button assignments reset"))
                  }
                />
              ) : (
                unavailable
              ))}

            {activeTab === "profiles" && (
              <ProfilesToolsTab
                snapshot={snapshot}
                presets={presets}
                writable={writable}
                onSavePreset={handleSavePreset}
                onApplyPreset={(preset) =>
                  runMutation(
                    () => rapooApi.applyPreset(preset.id),
                    t("Applied preset “{name}”", { name: preset.name })
                  )
                }
                onDeletePreset={handleDeletePreset}
              />
            )}

            {activeTab === "settings" && (
              <SettingsTab
                state={state}
                snapshot={snapshot}
                catalog={catalog}
                prefs={prefs}
                writable={writable}
                busy={busy}
                darkMode={darkMode}
                onChangeTheme={setDarkMode}
                language={language}
                onChangeLanguage={handleChangeLanguage}
                onRefresh={() => void handleManualRefresh()}
                onSetPollingMode={handleSetPollingMode}
                onToggleDemoMode={(enabled) => void handleToggleDemoMode(enabled)}
                onUpdateCloseToTray={async (enabled) => {
                  const next = await rapooApi.setCloseToTray(enabled);
                  setPrefs(next);
                  toast.success(enabled ? t("Closing will minimize to the tray") : t("Closing will exit the app"));
                }}
                onUpdateAutoStart={async (enabled) => {
                  try {
                    const next = await rapooApi.setAutoStart(enabled);
                    setPrefs(next);
                    toast.success(enabled ? t("Added to startup") : t("Removed from startup"));
                  } catch (error) {
                    toast.error(error instanceof Error ? error.message : t("Failed to change auto-start"));
                  }
                }}
                onUpdateStartHidden={async (enabled) => {
                  try {
                    const next = await rapooApi.setStartHidden(enabled);
                    setPrefs(next);
                    toast.success(
                      enabled
                        ? t("Startup will open in the notification area")
                        : t("Startup will open the window"),
                    );
                  } catch (error) {
                    toast.error(error instanceof Error ? error.message : t("Failed to change startup mode"));
                  }
                }}
                onFactoryReset={() =>
                  runMutation(() => rapooApi.factoryReset(), t("Restored factory defaults"))
                }
                onOpenFirmwareUpdater={handleOpenFirmwareUpdater}
              />
            )}
          </div>
        </main>
      </div>

      <WindowControls />
      <Toaster theme={darkMode ? "dark" : "light"} position="bottom-right" />
    </div>
  );
}

function DeviceUnavailableCard({
  state,
  onEnableDemo,
  onRefresh,
  busy,
}: {
  state: ServiceState;
  onEnableDemo: () => void;
  onRefresh: () => void;
  busy: boolean;
}) {
  const sleeping = state.connected && state.sleeping;
  const unavailableMessage = sleeping
    ? t("Move the mouse to load settings.")
    : state.permissionDenied
      ? t("No permission to access hidraw. Check the udev setup in Settings.")
      : state.lastError ? translateError(state.lastError) : t("Connect the 4K receiver or USB-C cable.");

  return (
    <Card>
      <CardHeader className="grid-cols-1 @min-[27rem]:grid-cols-[1fr_auto]">
        <CardTitle>{sleeping ? t("The mouse is sleeping") : t("Mouse not connected")}</CardTitle>
        <CardDescription>{unavailableMessage}</CardDescription>
        <CardAction className="col-start-1 row-span-1 row-start-auto mt-2 justify-self-start @min-[27rem]:col-start-2 @min-[27rem]:row-span-2 @min-[27rem]:row-start-1 @min-[27rem]:mt-0 @min-[27rem]:self-center @min-[27rem]:justify-self-end">
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" disabled={busy} onClick={onEnableDemo}>{t("Demo mode")}</Button>
            <Button variant="outline" disabled={busy} onClick={onRefresh}>{t("Reload")}</Button>
          </div>
        </CardAction>
      </CardHeader>
    </Card>
  );
}
