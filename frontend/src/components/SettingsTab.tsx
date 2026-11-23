import { toast } from "sonner";
import { SettingRow } from "@/components/SettingRow";
import { t, type Language } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import type { AppPreferences, DeviceSnapshot, PollingMode, ServiceState, UICatalog } from "@/types/rapoo";

interface SettingsTabProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  catalog: UICatalog | null;
  prefs: AppPreferences | null;
  writable: boolean;
  busy: boolean;
  darkMode: boolean;
  onChangeTheme: (dark: boolean) => void;
  language: Language;
  onChangeLanguage: (language: Language) => void;
  onSetPollingMode: (mode: PollingMode) => Promise<void>;
  onToggleDemoMode: (enabled: boolean) => void;
  onUpdateCloseToTray: (enabled: boolean) => Promise<void>;
  onUpdateAutoStart: (enabled: boolean) => Promise<void>;
  onUpdateStartHidden: (enabled: boolean) => Promise<void>;
  onFactoryReset: () => Promise<boolean>;
  onOpenFirmwareUpdater: () => Promise<void>;
  onRefresh: () => void;
}

export function SettingsTab({
  state,
  snapshot,
  catalog,
  prefs,
  writable,
  busy,
  darkMode,
  onChangeTheme,
  language,
  onChangeLanguage,
  onSetPollingMode,
  onToggleDemoMode,
  onUpdateCloseToTray,
  onUpdateAutoStart,
  onUpdateStartHidden,
  onFactoryReset,
  onOpenFirmwareUpdater,
  onRefresh,
}: SettingsTabProps) {
  const copyUdev = async () => {
    if (!catalog?.linuxUdevInstallCmd) return;
    try {
      await navigator.clipboard.writeText(catalog.linuxUdevInstallCmd);
      toast.success(t("Copied the udev setup command"));
    } catch {
      toast.error(t("Copy failed"));
    }
  };

  const firmware = snapshot?.firmware?.mouseString ? `v${snapshot.firmware.mouseString}` : "—";
  const dongle = snapshot?.firmware?.dongleString ? `v${snapshot.firmware.dongleString}` : "—";

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("Device")}</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingRow label={t("Firmware")}>{firmware}</SettingRow>
          <SettingRow label={t("Receiver version")}>{dongle}</SettingRow>

          {(catalog?.platform === "linux" || state.permissionDenied) && catalog?.linuxUdevInstallCmd && (
            <div className="flex flex-col gap-2 border-b py-3">
              <label htmlFor="udev-command" className="text-sm">{t("udev setup")}</label>
              <div className="flex gap-2">
                <Input id="udev-command" readOnly value={catalog.linuxUdevInstallCmd} className="min-w-0 font-mono" />
                <Button variant="outline" onClick={() => void copyUdev()}>
                  {t("Copy")}
                </Button>
              </div>
            </div>
          )}

          {state.platform === "windows" && (
            <SettingRow label={t("Firmware update")}>
              <Button variant="outline" size="sm" onClick={() => void onOpenFirmwareUpdater()}>
                {t("Open official software")}
              </Button>
            </SettingRow>
          )}

          <SettingRow label={t("Reset mouse")}>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm" disabled={!writable}>
                  {t("Restore factory defaults")}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>{t("Reset the mouse?")}</AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("This restores all settings, including DPI, sensor, and button assignments, to factory defaults.")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
                  <AlertDialogAction variant="destructive" onClick={() => void onFactoryReset()}>
                    {t("Reset")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </SettingRow>

          <SettingRow label={t("Device information")}>
            <Button variant="outline" size="sm" disabled={busy} onClick={onRefresh}>
              {t("Reload")}
            </Button>
          </SettingRow>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("App")}</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingRow label={t("Background polling")} description={t("Read interval and configuration lock")}>
            <Select
              value={state.mode}
              onValueChange={(value) => void onSetPollingMode(value as PollingMode)}
            >
              <SelectTrigger className="w-full" aria-label={t("Background polling")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="normal">{t("Normal")}</SelectItem>
                <SelectItem value="low_freq">{t("Power saving")}</SelectItem>
                <SelectItem value="passive_only">{t("In-game lock (0-TX)")}</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          {prefs?.platform === "windows" && (
            <>
              <SettingRow label={t("Minimize to tray on close")} description={t("Keeps running in the background when the window is closed")}>
                <Switch
                  checked={prefs.closeToTray}
                  onCheckedChange={(value) => void onUpdateCloseToTray(value)}
                  aria-label={t("Minimize to tray on close")}
                />
              </SettingRow>
              <SettingRow label={t("Windows startup")} description={t("Launch at sign-in")}>
                <Switch
                  checked={prefs.autoStart}
                  onCheckedChange={(value) => void onUpdateAutoStart(value)}
                  aria-label={t("Windows startup")}
                />
              </SettingRow>
              <SettingRow
                label={t("Start in the notification area")}
                description={t("Start hidden when Windows signs in")}
              >
                <Switch
                  checked={prefs.startHidden}
                  onCheckedChange={(value) => void onUpdateStartHidden(value)}
                  aria-label={t("Start in the notification area")}
                />
              </SettingRow>
            </>
          )}

          <SettingRow label={t("Language")}>
            <Select value={language} onValueChange={(value) => onChangeLanguage(value as Language)}>
              <SelectTrigger className="w-full" aria-label={t("Language")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="en">English</SelectItem>
                <SelectItem value="ja">日本語</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          <SettingRow label={t("Appearance")}>
            <Select value={darkMode ? "dark" : "light"} onValueChange={(value) => onChangeTheme(value === "dark")}>
              <SelectTrigger className="w-full" aria-label={t("Appearance")}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="dark">{t("Dark")}</SelectItem>
                <SelectItem value="light">{t("Light")}</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          <SettingRow label={t("Demo mode")} description={t("Try settings without hardware")}>
            <Button variant="outline" size="sm" disabled={busy} onClick={() => onToggleDemoMode(!state.demoMode)}>
              {state.demoMode ? t("Exit") : t("Start")}
            </Button>
          </SettingRow>
        </CardContent>
      </Card>
    </div>
  );
}
