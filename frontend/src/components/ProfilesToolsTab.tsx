import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type {
  AppPreferences,
  DeviceSnapshot,
  PollingMode,
  Preset,
  ServiceState,
  UICatalog,
} from "@/types/rapoo";

interface ProfilesToolsTabProps {
  state: ServiceState;
  snapshot: DeviceSnapshot | null;
  catalog: UICatalog | null;
  presets: Preset[];
  prefs: AppPreferences | null;
  writable: boolean;
  onSavePreset: (name: string) => Promise<boolean>;
  onApplyPreset: (preset: Preset) => Promise<boolean>;
  onDeletePreset: (id: string) => Promise<boolean>;
  onSetPollingMode: (mode: PollingMode) => Promise<void>;
  onToggleDemoMode: (enabled: boolean) => void;
  onUpdateCloseToTray: (enabled: boolean) => Promise<void>;
  onUpdateAutoStart: (enabled: boolean) => Promise<void>;
  onFactoryReset: () => Promise<boolean>;
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b py-2.5 last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-right">{value}</span>
    </div>
  );
}

function SettingRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex min-h-14 items-center justify-between gap-4 border-b py-3 first:pt-0 last:border-0 last:pb-0">
      <span className="text-sm">{label}</span>
      <div className="w-48 shrink-0">{children}</div>
    </div>
  );
}

function connectionText(state: ServiceState): string {
  if (!state.connected) return "未接続";
  if (state.demoMode) return "デモ";
  if (state.sleeping) return "スリープ中";
  return state.connection === "wired" ? "USB-C" : "4K ワイヤレス";
}

export function ProfilesToolsTab({
  state,
  snapshot,
  catalog,
  presets,
  prefs,
  writable,
  onSavePreset,
  onApplyPreset,
  onDeletePreset,
  onSetPollingMode,
  onToggleDemoMode,
  onUpdateCloseToTray,
  onUpdateAutoStart,
  onFactoryReset,
}: ProfilesToolsTabProps) {
  const [saveOpen, setSaveOpen] = useState(false);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!name.trim()) {
      toast.error("プロファイル名を入力してください");
      return;
    }
    setSaving(true);
    try {
      const ok = await onSavePreset(name.trim());
      if (ok) {
        setName("");
        setSaveOpen(false);
      }
    } finally {
      setSaving(false);
    }
  };

  const copyUdev = async () => {
    if (!catalog?.linuxUdevInstallCmd) return;
    try {
      await navigator.clipboard.writeText(catalog.linuxUdevInstallCmd);
      toast.success("udev 設定コマンドをコピーしました");
    } catch {
      toast.error("コピーに失敗しました");
    }
  };

  const battery = snapshot?.battery.percentage ?? state.percentage;
  const batteryText = state.hasBattery || snapshot ? `${battery}%` : "—";
  const firmware = snapshot?.firmware?.mouseString ? `v${snapshot.firmware.mouseString}` : "—";
  const dongle = snapshot?.firmware?.dongleString
    ? `v${snapshot.firmware.dongleString}`
    : state.connection === "wired"
      ? "有線"
      : "—";

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="flex-row items-center justify-between">
          <CardTitle>プリセット</CardTitle>
          <Button size="sm" disabled={!snapshot?.dpi} onClick={() => setSaveOpen(true)}>
            保存
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名前</TableHead>
                <TableHead className="w-40 text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {presets.map((preset) => (
                <TableRow key={preset.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{preset.name}</span>
                      {preset.builtIn && <Badge variant="secondary">標準</Badge>}
                    </div>
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      <Button
                        size="sm"
                        disabled={!writable}
                        onClick={() => void onApplyPreset(preset)}
                      >
                        適用
                      </Button>
                      {!preset.builtIn && (
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="outline" size="sm">
                              削除
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>「{preset.name}」を削除しますか？</AlertDialogTitle>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>キャンセル</AlertDialogCancel>
                              <AlertDialogAction
                                variant="destructive"
                                onClick={() => void onDeletePreset(preset.id)}
                              >
                                削除
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>デバイス</CardTitle>
        </CardHeader>
        <CardContent className="text-sm">
          <InfoRow label="接続" value={connectionText(state)} />
          <InfoRow label="ファームウェア" value={firmware} />
          <InfoRow label="レシーバー" value={dongle} />
          <InfoRow label="電池" value={batteryText} />

          {(catalog?.platform === "linux" || state.permissionDenied) && catalog?.linuxUdevInstallCmd && (
            <div className="flex flex-col gap-2 border-t pt-4">
              <label className="text-sm font-medium">udev 設定</label>
              <div className="flex gap-2">
                <Input readOnly value={catalog.linuxUdevInstallCmd} className="font-mono" />
                <Button variant="outline" onClick={() => void copyUdev()}>
                  コピー
                </Button>
              </div>
            </div>
          )}

          <div className="border-t pt-4">
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="destructive" size="sm" disabled={!writable}>
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
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>アプリ</CardTitle>
        </CardHeader>
        <CardContent>
          <SettingRow label="バックグラウンド取得">
            <Select
              value={state.mode}
              onValueChange={(value) => void onSetPollingMode(value as PollingMode)}
            >
              <SelectTrigger aria-label="バックグラウンド取得">
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
              <SettingRow label="閉じるボタンでトレイに格納">
                <div className="flex justify-end">
                  <Switch
                    checked={prefs.closeToTray}
                    onCheckedChange={(value) => void onUpdateCloseToTray(value)}
                    aria-label="閉じるボタンでトレイに格納"
                  />
                </div>
              </SettingRow>
              <SettingRow label="Windows スタートアップ">
                <div className="flex justify-end">
                  <Switch
                    checked={prefs.autoStart}
                    onCheckedChange={(value) => void onUpdateAutoStart(value)}
                    aria-label="Windows スタートアップ"
                  />
                </div>
              </SettingRow>
            </>
          )}

          {state.demoMode && (
            <SettingRow label="デモモード">
              <div className="flex justify-end">
                <Button variant="outline" size="sm" onClick={() => onToggleDemoMode(false)}>
                  終了
                </Button>
              </div>
            </SettingRow>
          )}
        </CardContent>
      </Card>

      <Dialog open={saveOpen} onOpenChange={setSaveOpen}>
        <DialogContent>
          <form onSubmit={(event) => void handleSave(event)} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>プリセットを保存</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-2">
              <label htmlFor="preset-name" className="text-sm font-medium">
                名前
              </label>
              <Input
                id="preset-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="例: FPS 800 DPI"
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setSaveOpen(false)}>
                キャンセル
              </Button>
              <Button type="submit" disabled={saving}>
                保存
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
