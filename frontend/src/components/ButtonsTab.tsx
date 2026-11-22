import { useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ACTION_LABEL_MAP } from "@/lib/wails";
import type {
  ButtonAction,
  ButtonActionType,
  ButtonMapping,
  DeviceSnapshot,
  HIDKeyInfo,
  KeyModifiers,
} from "@/types/rapoo";

interface ButtonsTabProps {
  snapshot: DeviceSnapshot;
  supportedKeys: HIDKeyInfo[];
  writable: boolean;
  readOnlyReason?: string;
  onSaveButtonMapping: (buttonId: number, action: ButtonAction) => Promise<boolean>;
  onResetAllButtons: () => Promise<boolean>;
}

const GROUPS: Array<{ label: string; types: ButtonActionType[] }> = [
  {
    label: "マウス",
    types: ["left_click", "right_click", "middle_click", "forward", "back", "wheel_default", "fire_triple"],
  },
  {
    label: "DPI・ハードウェア",
    types: ["dpi_plus", "dpi_minus", "dpi_cycle", "sniper_dpi", "polling_switch", "profile_switch", "receiver_switch"],
  },
  {
    label: "メディア・アプリ",
    types: [
      "media_play_pause",
      "media_stop",
      "media_prev",
      "media_next",
      "media_vol_up",
      "media_vol_down",
      "media_mute",
      "media_player",
      "app_calculator",
      "app_explorer",
      "app_mail",
    ],
  },
];

const EMPTY_MODS: KeyModifiers = { ctrl: false, shift: false, alt: false, meta: false };

function formatCombo(mods: KeyModifiers, keyName: string): string {
  const parts: string[] = [];
  if (mods.ctrl) parts.push("Ctrl");
  if (mods.shift) parts.push("Shift");
  if (mods.alt) parts.push("Alt");
  if (mods.meta) parts.push("Win");
  if (keyName) parts.push(keyName);
  return parts.length > 0 ? parts.join(" + ") : "未設定";
}

function hasModifier(mods: KeyModifiers): boolean {
  return mods.ctrl || mods.shift || mods.alt || mods.meta;
}

export function ButtonsTab({
  snapshot,
  supportedKeys,
  writable,
  readOnlyReason,
  onSaveButtonMapping,
  onResetAllButtons,
}: ButtonsTabProps) {
  const buttons = snapshot.buttons ?? [];
  const [editing, setEditing] = useState<ButtonMapping | null>(null);
  const [actionType, setActionType] = useState<ButtonActionType>("left_click");
  const [mods, setMods] = useState<KeyModifiers>(EMPTY_MODS);
  const [keyCode, setKeyCode] = useState(0);
  const [keyName, setKeyName] = useState("");
  const [capturing, setCapturing] = useState(false);
  const [saving, setSaving] = useState(false);
  const pendingModifierRef = useRef<KeyModifiers | null>(null);
  const nonModifierRef = useRef(false);

  const leftClickSlots = useMemo(
    () => buttons.filter((mapping) => mapping.slot.clickable && mapping.action.type === "left_click"),
    [buttons]
  );
  const isSoleLeftClick = Boolean(editing && leftClickSlots.length === 1 && leftClickSlots[0].slot.id === editing.slot.id);
  const violatesGuard = isSoleLeftClick && actionType !== "left_click";
  const invalidKeyboard = actionType === "keyboard" && keyCode === 0 && !hasModifier(mods);

  const openEditor = (mapping: ButtonMapping) => {
    const action = mapping.action;
    setEditing(mapping);
    setActionType(action.type === "unknown" ? mapping.slot.defaultAction : action.type);
    setMods(action.modifiers ?? EMPTY_MODS);
    setKeyCode(action.keyCode ?? 0);
    setKeyName(action.keyName ?? "");
    setCapturing(false);
    pendingModifierRef.current = null;
    nonModifierRef.current = false;
  };

  const closeEditor = () => {
    setEditing(null);
    setCapturing(false);
    pendingModifierRef.current = null;
  };

  useEffect(() => {
    if (!capturing) return;

    const onKeyDown = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();

      if (event.key === "Escape" && !event.ctrlKey && !event.shiftKey && !event.altKey && !event.metaKey) {
        setCapturing(false);
        pendingModifierRef.current = null;
        return;
      }

      if (["Control", "Shift", "Alt", "Meta"].includes(event.key)) {
        const next = {
          ctrl: event.ctrlKey || event.key === "Control",
          shift: event.shiftKey || event.key === "Shift",
          alt: event.altKey || event.key === "Alt",
          meta: event.metaKey || event.key === "Meta",
        };
        pendingModifierRef.current = next;
        setMods(next);
        return;
      }

      const found = supportedKeys.find((key) => key.domCode.toLowerCase() === event.code.toLowerCase());
      if (found) {
        nonModifierRef.current = true;
        pendingModifierRef.current = null;
        setMods({
          ctrl: event.ctrlKey,
          shift: event.shiftKey,
          alt: event.altKey,
          meta: event.metaKey,
        });
        setKeyCode(found.code);
        setKeyName(found.name);
        setCapturing(false);
      }
    };

    const onKeyUp = (event: KeyboardEvent) => {
      if (!["Control", "Shift", "Alt", "Meta"].includes(event.key)) return;
      if (nonModifierRef.current || !pendingModifierRef.current) return;
      event.preventDefault();
      event.stopPropagation();
      setKeyCode(0);
      setKeyName("");
      setCapturing(false);
      pendingModifierRef.current = null;
    };

    window.addEventListener("keydown", onKeyDown, true);
    window.addEventListener("keyup", onKeyUp, true);
    return () => {
      window.removeEventListener("keydown", onKeyDown, true);
      window.removeEventListener("keyup", onKeyUp, true);
    };
  }, [capturing, supportedKeys]);

  const startCapture = () => {
    setCapturing(true);
    pendingModifierRef.current = null;
    nonModifierRef.current = false;
  };

  const handleSave = async () => {
    if (!editing || violatesGuard || invalidKeyboard) return;

    let action: ButtonAction;
    if (actionType === "keyboard") {
      action = {
        type: "keyboard",
        category: "keyboard",
        labelJa: formatCombo(mods, keyName),
        modifiers: mods,
        keyCode,
        keyName,
      };
    } else {
      const meta = ACTION_LABEL_MAP[actionType];
      action = {
        type: actionType,
        category: meta.category,
        labelJa: meta.labelJa,
      };
    }

    setSaving(true);
    try {
      const ok = await onSaveButtonMapping(editing.slot.id, action);
      if (ok) closeEditor();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader className="items-center">
        <CardTitle>割り当て</CardTitle>
        <CardAction>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm" disabled={!writable}>
                初期化
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>全ボタンを初期値に戻しますか？</AlertDialogTitle>
                <AlertDialogDescription>
                  10箇所のボタン割り当てを工場出荷時の設定に戻します。
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>キャンセル</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={() => void onResetAllButtons()}>
                  初期化
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </CardAction>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {!writable && readOnlyReason && <p className="text-xs text-muted-foreground">{readOnlyReason}</p>}
        <Table className="[&_th:first-child]:pl-0 [&_td:first-child]:pl-0 [&_th:last-child]:pr-0 [&_td:last-child]:pr-0">
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead>ボタン</TableHead>
              <TableHead>割り当て</TableHead>
              <TableHead className="w-24 text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {buttons.map((mapping) => (
              <TableRow key={mapping.slot.id}>
                <TableCell className="font-medium">{mapping.slot.nameJa}</TableCell>
                <TableCell>{mapping.action.labelJa}</TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={!writable}
                    aria-label={`${mapping.slot.nameJa} を変更`}
                    onClick={() => openEditor(mapping)}
                  >
                    変更
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>

      <Dialog open={Boolean(editing)} onOpenChange={(open) => !open && closeEditor()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editing?.slot.nameJa}</DialogTitle>
          </DialogHeader>

          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium">動作</label>
              <Select
                value={actionType}
                onValueChange={(value) => {
                  const next = value as ButtonActionType;
                  setActionType(next);
                  if (next === "keyboard" && keyCode === 0 && !hasModifier(mods)) {
                    startCapture();
                  }
                }}
              >
                <SelectTrigger className="w-full" aria-label="動作">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {GROUPS.map((group) => (
                    <SelectGroup key={group.label}>
                      <SelectLabel>{group.label}</SelectLabel>
                      {group.types.map((type) => (
                        <SelectItem key={type} value={type}>
                          {ACTION_LABEL_MAP[type].labelJa}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  ))}
                  <SelectGroup>
                    <SelectLabel>その他</SelectLabel>
                    <SelectItem value="keyboard">キーボード</SelectItem>
                    <SelectItem value="disabled">無効</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              {ACTION_LABEL_MAP[actionType]?.hint && (
                <p className="text-xs text-muted-foreground">{ACTION_LABEL_MAP[actionType].hint}</p>
              )}
            </div>

            {actionType === "keyboard" && (
              <div className="flex flex-col gap-2">
                <label className="text-sm font-medium">キー</label>
                <Input
                  readOnly
                  value={capturing ? "キーを押してください..." : formatCombo(mods, keyName)}
                  aria-label="キー"
                  className="cursor-pointer"
                  onClick={startCapture}
                />
                <p className="text-xs text-muted-foreground">
                  {capturing ? (
                    <>
                      <Kbd>Esc</Kbd> でキャンセルします。
                    </>
                  ) : (
                    "クリックしてキーを入力します。修飾キー単体も設定できます。"
                  )}
                </p>
              </div>
            )}

            {violatesGuard && (
              <Alert variant="destructive">
                <AlertTitle>左クリック保護</AlertTitle>
                <AlertDescription>
                  左クリックは1つ以上のボタンに残す必要があります。
                </AlertDescription>
              </Alert>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={closeEditor}>
              キャンセル
            </Button>
            <Button onClick={() => void handleSave()} disabled={saving || violatesGuard || invalidKeyboard}>
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
