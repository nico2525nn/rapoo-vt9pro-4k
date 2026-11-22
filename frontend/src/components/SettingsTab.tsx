import { toast } from "sonner";
import { SettingRow } from "@/components/SettingRow";
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
  onSetPollingMode: (mode: PollingMode) => Promise<void>;
  onToggleDemoMode: (enabled: boolean) => void;
  onUpdateCloseToTray: (enabled: boolean) => Promise<void>;
  onUpdateAutoStart: (enabled: boolean) => Promise<void>;
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
  onSetPollingMode,
  onToggleDemoMode,
  onUpdateCloseToTray,
  onUpdateAutoStart,
  onFactoryReset,
  onOpenFirmwareUpdater,
  onRefresh,
}: SettingsTabProps) {
  const copyUdev = async () => {
    if (!catalog?.linuxUdevInstallCmd) return;
    try {
      await navigator.clipboard.writeText(catalog.linuxUdevInstallCmd);
      toast.success("udev 設定コマンドをコピーしました");
    } catch {
      toast.error("コピーに失敗しました");
    }
  };

  const firmware = snapshot?.firmware?.mouseString ? `v${snapshot.firmware.mouseString}` : "—";
  const dongle = snapshot?.firmware?.dongleString ? `v${snapshot.firmware.dongleString}` : "—";

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>デバイス</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingRow label="ファームウェア">{firmware}</SettingRow>
          <SettingRow label="レシーバーのバージョン">{dongle}</SettingRow>

          {(catalog?.platform === "linux" || state.permissionDenied) && catalog?.linuxUdevInstallCmd && (
            <div className="flex flex-col gap-2 border-b py-3">
              <label htmlFor="udev-command" className="text-sm">udev 設定</label>
              <div className="flex gap-2">
                <Input id="udev-command" readOnly value={catalog.linuxUdevInstallCmd} className="min-w-0 font-mono" />
                <Button variant="outline" onClick={() => void copyUdev()}>
                  コピー
                </Button>
              </div>
            </div>
          )}

          {state.platform === "windows" && (
            <SettingRow label="ファームウェア更新">
              <Button variant="outline" size="sm" onClick={() => void onOpenFirmwareUpdater()}>
                公式ソフトを開く
              </Button>
            </SettingRow>
          )}

          <SettingRow label="マウスの初期化">
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm" disabled={!writable}>
                  工場出荷時設定に戻す
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>マウスを初期化しますか？</AlertDialogTitle>
                  <AlertDialogDescription>
                    DPI・センサー・ボタン割り当てを含むすべての設定を工場出荷時の状態に戻します。
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>キャンセル</AlertDialogCancel>
                  <AlertDialogAction variant="destructive" onClick={() => void onFactoryReset()}>
                    初期化
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </SettingRow>

          <SettingRow label="デバイス情報">
            <Button variant="outline" size="sm" disabled={busy} onClick={onRefresh}>
              再読み込み
            </Button>
          </SettingRow>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>アプリ</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingRow label="バックグラウンド取得" description="読み取り間隔と設定ロック">
            <Select
              value={state.mode}
              onValueChange={(value) => void onSetPollingMode(value as PollingMode)}
            >
              <SelectTrigger className="w-full" aria-label="バックグラウンド取得">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="normal">通常</SelectItem>
                <SelectItem value="low_freq">省電力</SelectItem>
                <SelectItem value="passive_only">ゲーム中ロック (0-TX)</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          {prefs?.platform === "windows" && (
            <>
              <SettingRow label="閉じるボタンでトレイに格納" description="ウィンドウを閉じてもバックグラウンドで実行">
                <Switch
                  checked={prefs.closeToTray}
                  onCheckedChange={(value) => void onUpdateCloseToTray(value)}
                  aria-label="閉じるボタンでトレイに格納"
                />
              </SettingRow>
              <SettingRow label="Windows スタートアップ" description="サインイン時に起動">
                <Switch
                  checked={prefs.autoStart}
                  onCheckedChange={(value) => void onUpdateAutoStart(value)}
                  aria-label="Windows スタートアップ"
                />
              </SettingRow>
            </>
          )}

          <SettingRow label="外観">
            <Select value={darkMode ? "dark" : "light"} onValueChange={(value) => onChangeTheme(value === "dark")}>
              <SelectTrigger className="w-full" aria-label="外観">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="dark">ダーク</SelectItem>
                <SelectItem value="light">ライト</SelectItem>
              </SelectContent>
            </Select>
          </SettingRow>

          <SettingRow label="デモモード" description="実機を使わずに設定を試す">
            <Button variant="outline" size="sm" disabled={busy} onClick={() => onToggleDemoMode(!state.demoMode)}>
              {state.demoMode ? "終了" : "開始"}
            </Button>
          </SettingRow>
        </CardContent>
      </Card>
    </div>
  );
}
