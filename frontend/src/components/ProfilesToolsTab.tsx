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
import { t } from "@/lib/i18n";
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
      toast.error(t("Enter a preset name"));
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
          <CardTitle>{t("Presets")}</CardTitle>
          <CardAction>
            <Button variant="outline" disabled={!snapshot?.dpi} onClick={() => setSaveOpen(true)}>
              {t("Save")}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          {presets.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("No saved presets.")}</p>
          ) : (
            <Table className="table-fixed [&_th:first-child]:pl-0 [&_td:first-child]:pl-0 [&_th:last-child]:pr-0 [&_td:last-child]:pr-0">
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("Name")}</TableHead>
                  <TableHead className="w-40 text-right">{t("Actions")}</TableHead>
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
                          {t("Apply")}
                        </Button>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="outline" size="sm">
                              {t("Delete")}
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>{t("Delete “{name}”?", { name: preset.name })}</AlertDialogTitle>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
                              <AlertDialogAction
                                variant="destructive"
                                onClick={() => void onDeletePreset(preset.id)}
                              >
                                {t("Delete")}
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
              <DialogTitle>{t("Save preset")}</DialogTitle>
            </DialogHeader>
            <div className="flex flex-col gap-2">
              <label htmlFor="preset-name" className="text-sm font-medium">
                {t("Name")}
              </label>
              <Input
                id="preset-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="e.g. FPS 800 DPI"
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setSaveOpen(false)}>
                {t("Cancel")}
              </Button>
              <Button type="submit" disabled={saving}>
                {t("Save")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
