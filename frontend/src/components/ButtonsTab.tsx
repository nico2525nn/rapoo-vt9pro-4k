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
import { getLanguage, t } from "@/lib/i18n";
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

const ACTION_EN: Record<ButtonActionType, string> = {
  left_click: "Left Click",
  right_click: "Right Click",
  middle_click: "Middle Click",
  forward: "Forward (Mouse 5)",
  back: "Back (Mouse 4)",
  wheel_default: "Standard scroll",
  fire_triple: "Fire key (3-shot burst)",
  dpi_plus: "DPI + (raise stage)",
  dpi_minus: "DPI - (lower stage)",
  dpi_cycle: "DPI cycle",
  sniper_dpi: "Sniper button (low DPI while held)",
  polling_switch: "Polling rate switch",
  receiver_switch: "Receiver switch",
  profile_switch: "Onboard profile switch",
  media_play_pause: "Play / Pause",
  media_stop: "Stop",
  media_prev: "Previous track",
  media_next: "Next track",
  media_vol_up: "Volume up",
  media_vol_down: "Volume down",
  media_mute: "Mute",
  media_player: "Open media player",
  app_calculator: "Open calculator",
  app_explorer: "Open File Explorer",
  app_mail: "Open mail",
  keyboard: "Keyboard input",
  disabled: "Disabled (button off)",
  unknown: "Custom",
};

const ACTION_HINT_EN: Partial<Record<ButtonActionType, string>> = {
  fire_triple: "Three left clicks per press",
  dpi_cycle: "Cycle through DPI stages",
  sniper_dpi: "Low DPI only while held",
  polling_switch: "Cycle the polling rate on each press",
  receiver_switch: "Switch the receiver target",
  profile_switch: "Switch onboard profiles",
};

function actionLabel(action: ButtonAction): string {
  if (action.type === "keyboard") return action.labelJa;
  return getLanguage() === "ja"
    ? (ACTION_LABEL_MAP[action.type]?.labelJa ?? action.labelJa)
    : (ACTION_EN[action.type] ?? action.labelJa);
}

function actionText(type: ButtonActionType): string {
  return getLanguage() === "ja" ? ACTION_LABEL_MAP[type].labelJa : ACTION_EN[type];
}

const GROUPS: Array<{ label: string; types: ButtonActionType[] }> = [
  {
    label: "Mouse",
    types: ["left_click", "right_click", "middle_click", "forward", "back", "wheel_default", "fire_triple"],
  },
  {
    label: "DPI & hardware",
    types: ["dpi_plus", "dpi_minus", "dpi_cycle", "sniper_dpi", "polling_switch", "profile_switch", "receiver_switch"],
  },
  {
    label: "Media & apps",
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
  return parts.length > 0 ? parts.join(" + ") : t("Not set");
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
        <CardTitle>{t("Assignments")}</CardTitle>
        <CardAction>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="outline" size="sm" disabled={!writable}>
                {t("Reset")}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t("Reset all buttons to defaults?")}</AlertDialogTitle>
                <AlertDialogDescription>
                  {t("This restores all 10 button assignments to their factory defaults.")}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t("Cancel")}</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={() => void onResetAllButtons()}>
                  {t("Reset")}
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
              <TableHead>{t("Button")}</TableHead>
              <TableHead>{t("Assignment")}</TableHead>
              <TableHead className="w-24 text-right">{t("Actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {buttons.map((mapping) => (
              <TableRow key={mapping.slot.id}>
                <TableCell className="font-medium">{getLanguage() === "ja" ? mapping.slot.nameJa : mapping.slot.nameEn}</TableCell>
                <TableCell>{actionLabel(mapping.action)}</TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={!writable}
                    aria-label={t("Change {button}", { button: getLanguage() === "ja" ? mapping.slot.nameJa : mapping.slot.nameEn })}
                    onClick={() => openEditor(mapping)}
                  >
                    {t("Change")}
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
            <DialogTitle>{editing ? (getLanguage() === "ja" ? editing.slot.nameJa : editing.slot.nameEn) : ""}</DialogTitle>
          </DialogHeader>

          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium">{t("Action")}</label>
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
                <SelectTrigger className="w-full" aria-label={t("Action")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {GROUPS.map((group) => (
                    <SelectGroup key={group.label}>
                      <SelectLabel>{t(group.label)}</SelectLabel>
                      {group.types.map((type) => (
                        <SelectItem key={type} value={type}>
                          {actionText(type)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  ))}
                  <SelectGroup>
                    <SelectLabel>{t("Other")}</SelectLabel>
                    <SelectItem value="keyboard">{t("Keyboard")}</SelectItem>
                    <SelectItem value="disabled">{t("Disabled")}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              {ACTION_LABEL_MAP[actionType]?.hint && (
                <p className="text-xs text-muted-foreground">{getLanguage() === "ja" ? ACTION_LABEL_MAP[actionType].hint : ACTION_HINT_EN[actionType]}</p>
              )}
            </div>

            {actionType === "keyboard" && (
              <div className="flex flex-col gap-2">
                <label className="text-sm font-medium">{t("Key")}</label>
                <Input
                  readOnly
                  value={capturing ? t("Press a key...") : formatCombo(mods, keyName)}
                  aria-label={t("Key")}
                  className="cursor-pointer"
                  onClick={startCapture}
                />
                <p className="text-xs text-muted-foreground">
                  {capturing ? (
                    <>
                      <Kbd>Esc</Kbd> {t("to cancel.")}
                    </>
                  ) : (
                    t("Click and press a key. Modifier-only bindings are also supported.")
                  )}
                </p>
              </div>
            )}

            {violatesGuard && (
              <Alert variant="destructive">
                <AlertTitle>{t("Left-click protection")}</AlertTitle>
                <AlertDescription>
                  {t("At least one button must keep Left Click.")}
                </AlertDescription>
              </Alert>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={closeEditor}>
              {t("Cancel")}
            </Button>
            <Button onClick={() => void handleSave()} disabled={saving || violatesGuard || invalidKeyboard}>
              {t("Save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
