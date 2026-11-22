import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { DeviceSnapshot, Preset } from "@/types/rapoo";

interface ProfilesToolsTabProps {
  snapshot: DeviceSnapshot | null;
  presets: Preset[];
  writable: boolean;
  onSavePreset: (name: string) => Promise<boolean>;
  onApplyPreset: (preset: Preset) => Promise<boolean>;
  onDeletePreset: (id: string) => Promise<boolean>;
}

export function ProfilesToolsTab({
  snapshot,
  presets,
  writable,
  onSavePreset,
  onApplyPreset,
  onDeletePreset,
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

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="items-center">
          <CardTitle>プリセット</CardTitle>
          <CardAction>
            <Button variant="outline" disabled={!snapshot?.dpi} onClick={() => setSaveOpen(true)}>
              保存
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          {presets.length === 0 ? (
            <p className="text-sm text-muted-foreground">保存したプリセットはありません。</p>
          ) : (
            <Table className="table-fixed [&_th:first-child]:pl-0 [&_td:first-child]:pl-0 [&_th:last-child]:pr-0 [&_td:last-child]:pr-0">
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>名前</TableHead>
                  <TableHead className="w-40 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {presets.map((preset) => (
                  <TableRow key={preset.id}>
                    <TableCell>
                      <span className="whitespace-normal [overflow-wrap:anywhere]">{preset.name}</span>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={!writable}
                          onClick={() => void onApplyPreset(preset)}
                        >
                          適用
                        </Button>
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
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
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
