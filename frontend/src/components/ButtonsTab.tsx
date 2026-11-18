import {
  AlertTriangle,
  Ban,
  Keyboard,
  Lock,
  MousePointer,
  RotateCcw,
  Sliders,
  Volume2,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
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

const MOUSE_ACTIONS: ButtonActionType[] = [
  "left_click",
  "right_click",
  "middle_click",
  "forward",
  "back",
  "wheel_default",
  "fire_triple",
];

const DPI_HW_ACTIONS: ButtonActionType[] = [
  "dpi_plus",
  "dpi_minus",
  "dpi_cycle",
  "sniper_dpi",
  "polling_switch",
  "receiver_switch",
  "profile_switch",
];

const MEDIA_ACTIONS: ButtonActionType[] = [
  "media_play_pause",
  "media_vol_up",
  "media_vol_down",
  "media_mute",
  "media_prev",
  "media_next",
  "media_stop",
  "media_player",
  "app_calculator",
  "app_explorer",
  "app_mail",
];

function formatCombo(mods: KeyModifiers, keyName: string): string {
  const parts: string[] = [];
  if (mods.ctrl) parts.push("Ctrl");
  if (mods.shift) parts.push("Shift");
  if (mods.alt) parts.push("Alt");
  if (mods.meta) parts.push("Win");
  if (keyName) parts.push(keyName);
  return parts.length > 0 ? parts.join(" + ") : "キー未選択";
}

function categoryBadgeVariant(cat: string): "default" | "secondary" | "outline" | "warning" | "destructive" {
  switch (cat) {
    case "mouse":
      return "default";
    case "dpi":
    case "hardware":
      return "warning";
    case "keyboard":
      return "secondary";
    case "none":
      return "outline";
    default:
      return "secondary";
  }
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
  const [diagramView, setDiagramView] = useState<"top" | "bottom">("top");
  const [hoveredId, setHoveredId] = useState<number | null>(null);
  const [editingMapping, setEditingMapping] = useState<ButtonMapping | null>(null);

  // Dialog draft state (supports keyCode === 0 for standalone modifier keys like Shift/Ctrl/Alt)
  const [draftTab, setDraftTab] = useState<string>("mouse");
  const [draftType, setDraftType] = useState<ButtonActionType>("left_click");
  const [draftMods, setDraftMods] = useState<KeyModifiers>({
    ctrl: false,
    shift: false,
    alt: false,
    meta: false,
  });
  const [draftKeyCode, setDraftKeyCode] = useState<number>(0x04);
  const [draftKeyName, setDraftKeyName] = useState<string>("A");
  const [capturingKey, setCapturingKey] = useState<boolean>(false);
  const pendingModifierCaptureRef = useRef<KeyModifiers | null>(null);

  const openEditor = (mapping: ButtonMapping) => {
    setEditingMapping(mapping);
    setCapturingKey(false);
    pendingModifierCaptureRef.current = null;
    const act = mapping.action;
    setDraftType(act.type === "unknown" ? mapping.slot.defaultAction : act.type);

    if (act.type === "keyboard") {
      setDraftTab("keyboard");
      setDraftMods(act.modifiers ?? { ctrl: false, shift: false, alt: false, meta: false });
      setDraftKeyCode(act.keyCode ?? 0);
      setDraftKeyName(act.keyName ?? "");
    } else if (DPI_HW_ACTIONS.includes(act.type)) {
      setDraftTab("dpi");
    } else if (MEDIA_ACTIONS.includes(act.type)) {
      setDraftTab("media");
    } else if (act.type === "disabled") {
      setDraftTab("disabled");
    } else {
      setDraftTab("mouse");
    }

    setDiagramView(mapping.slot.view === "bottom" ? "bottom" : "top");
  };

  // Keystroke recorder supporting Escape-to-cancel AND standalone modifier keys (Shift / Ctrl / Alt)
  useEffect(() => {
    if (!capturingKey) return;

    const onKeyDown = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();

      if (e.code === "Escape" && !e.ctrlKey && !e.shiftKey && !e.altKey && !e.metaKey) {
        pendingModifierCaptureRef.current = null;
        setCapturingKey(false);
        return;
      }

      if (["Control", "Shift", "Alt", "Meta"].includes(e.key)) {
        const mods: KeyModifiers = {
          ctrl: e.ctrlKey || e.key === "Control",
          shift: e.shiftKey || e.key === "Shift",
          alt: e.altKey || e.key === "Alt",
          meta: e.metaKey || e.key === "Meta",
        };
        pendingModifierCaptureRef.current = mods;
        setDraftMods(mods);
        setDraftKeyCode(0);
        setDraftKeyName("");
        return;
      }

      const found = supportedKeys.find((k) => k.domCode.toLowerCase() === e.code.toLowerCase());
      if (found) {
        pendingModifierCaptureRef.current = null;
        setDraftMods({
          ctrl: e.ctrlKey,
          shift: e.shiftKey,
          alt: e.altKey,
          meta: e.metaKey,
        });
        setDraftKeyCode(found.code);
        setDraftKeyName(found.name);
        setDraftType("keyboard");
        setCapturingKey(false);
      }
    };

    const onKeyUp = (e: KeyboardEvent) => {
      if (["Control", "Shift", "Alt", "Meta"].includes(e.key) && pendingModifierCaptureRef.current) {
        e.preventDefault();
        e.stopPropagation();
        setDraftMods(pendingModifierCaptureRef.current);
        setDraftKeyCode(0);
        setDraftKeyName("");
        setDraftType("keyboard");
        pendingModifierCaptureRef.current = null;
        setCapturingKey(false);
      }
    };

    window.addEventListener("keydown", onKeyDown, true);
    window.addEventListener("keyup", onKeyUp, true);
    return () => {
      window.removeEventListener("keydown", onKeyDown, true);
      window.removeEventListener("keyup", onKeyUp, true);
    };
  }, [capturingKey, supportedKeys]);

  // Check if editing button is the ONLY physical clickable button holding left_click
  const isSoleLeftClickHolder = useMemo(() => {
    if (!editingMapping) return false;
    const clickableLefts = buttons.filter((b) => b.slot.clickable && b.action.type === "left_click");
    return clickableLefts.length === 1 && clickableLefts[0].slot.id === editingMapping.slot.id;
  }, [buttons, editingMapping]);

  const wouldViolateLeftClickGuard = isSoleLeftClickHolder && draftType !== "left_click";
  const hasAnyKeyOrMod =
    draftKeyCode !== 0 || draftMods.ctrl || draftMods.shift || draftMods.alt || draftMods.meta;
  const invalidKeyboardCombo = draftType === "keyboard" && !hasAnyKeyOrMod;

  const handleConfirmSave = async () => {
    if (!editingMapping || wouldViolateLeftClickGuard || invalidKeyboardCombo) return;

    let action: ButtonAction;
    if (draftType === "keyboard") {
      action = {
        type: "keyboard",
        category: "keyboard",
        labelJa: `キー: ${formatCombo(draftMods, draftKeyName)}`,
        modifiers: draftMods,
        keyCode: draftKeyCode,
        keyName: draftKeyName,
      };
    } else {
      const meta = ACTION_LABEL_MAP[draftType] ?? { labelJa: draftType, category: "mouse" };
      let labelJa = meta.labelJa;
      if (draftType === "wheel_default") {
        if (editingMapping.slot.id === 11) labelJa = "上スクロール (標準)";
        if (editingMapping.slot.id === 12) labelJa = "下スクロール (標準)";
      }
      action = {
        type: draftType,
        category: meta.category,
        labelJa,
      };
    }

    const ok = await onSaveButtonMapping(editingMapping.slot.id, action);
    if (ok) {
      setEditingMapping(null);
    }
  };

  const handleResetSingleButton = async () => {
    if (!editingMapping) return;
    const defType = editingMapping.slot.defaultAction;
    if (isSoleLeftClickHolder && defType !== "left_click") return;
    const meta = ACTION_LABEL_MAP[defType];
    let labelJa = meta.labelJa;
    if (editingMapping.slot.id === 11) labelJa = "上スクロール (標準)";
    if (editingMapping.slot.id === 12) labelJa = "下スクロール (標準)";
    const ok = await onSaveButtonMapping(editingMapping.slot.id, {
      type: defType,
      category: meta.category,
      labelJa,
    });
    if (ok) {
      setEditingMapping(null);
    }
  };

  const buttonById = useMemo(() => {
    const m = new Map<number, ButtonMapping>();
    buttons.forEach((b) => m.set(b.slot.id, b));
    return m;
  }, [buttons]);

  return (
    <div className="grid gap-4 md:grid-cols-12 items-start">
      {/* Left 5 cols: Sticky Interactive VT9 Pro 4K Top/Bottom SVG Blueprint */}
      <div className="md:col-span-5 md:sticky md:top-16">
        <Card className="flex flex-col">
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between gap-2">
              <CardTitle className="flex items-center gap-2">
                <MousePointer className="h-4 w-4 text-primary" />
                物理レイアウト図
              </CardTitle>
              <div className="inline-flex rounded-md border bg-muted/60 p-0.5 text-xs" role="group" aria-label="表示面切替">
                <button
                  type="button"
                  aria-pressed={diagramView === "top"}
                  onClick={() => setDiagramView("top")}
                  className={cn(
                    "rounded px-2.5 py-1 font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                    diagramView === "top" ? "bg-card text-foreground shadow-sm" : "text-muted-foreground"
                  )}
                >
                  天面・サイド (9)
                </button>
                <button
                  type="button"
                  aria-pressed={diagramView === "bottom"}
                  onClick={() => setDiagramView("bottom")}
                  className={cn(
                    "rounded px-2.5 py-1 font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                    diagramView === "bottom" ? "bg-card text-foreground shadow-sm" : "text-muted-foreground"
                  )}
                >
                  底面 DIY (1)
                </button>
              </div>
            </div>
            <CardDescription>
              {writable
                ? "図上のボタンをクリック（またはTab選択）して直接割り当てを変更できます。"
                : readOnlyReason || "現在は読み取り専用表示です。"}
            </CardDescription>
          </CardHeader>

          <CardContent className="flex flex-col items-center justify-center py-2">
            {diagramView === "top" ? (
              <MouseTopSvg
                hoveredId={hoveredId}
                writable={writable}
                buttonById={buttonById}
                onHover={setHoveredId}
                onSelect={(id) => {
                  const found = buttonById.get(id);
                  if (found && writable) openEditor(found);
                }}
              />
            ) : (
              <MouseBottomSvg
                hoveredId={hoveredId}
                writable={writable}
                buttonById={buttonById}
                onHover={setHoveredId}
                onSelect={(id) => {
                  const found = buttonById.get(id);
                  if (found && writable) openEditor(found);
                }}
              />
            )}

            <div className="mt-2 text-center text-[11px] text-muted-foreground min-h-[18px]">
              {hoveredId !== null && buttonById.get(hoveredId) ? (
                <span>
                  選択中: <strong className="text-foreground">{buttonById.get(hoveredId)!.slot.nameJa}</strong> →{" "}
                  <span className="text-primary font-semibold">
                    {buttonById.get(hoveredId)!.action.labelJa}
                  </span>
                </span>
              ) : (
                <span>スクロール上下・底面DIYボタンを含む全10箇所を本体メモリへ保存</span>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Right 7 cols: 10 Physical Buttons Table */}
      <div className="md:col-span-7">
        <Card>
          <CardHeader className="pb-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle>ボタン割り当て一覧（全10箇所）</CardTitle>
                <CardDescription>
                  変更はマウス本体の内蔵メモリへ直接保存され、アプリを閉じた後や別PCでもそのまま動作します。
                </CardDescription>
              </div>

              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="outline" size="sm" disabled={!writable}>
                    <RotateCcw className="h-3.5 w-3.5" />
                    全ボタンを初期値に戻す
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>全10箇所の割り当てを初期化しますか？</AlertDialogTitle>
                    <AlertDialogDescription>
                      天面・サイド・ホイール・底面DIYボタンのすべての割り当てが工場出荷時の標準設定に戻ります（DPIやセンサー設定は維持されます）。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>キャンセル</AlertDialogCancel>
                    <AlertDialogAction onClick={() => void onResetAllButtons()}>
                      初期値に戻す
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>

            {!writable && readOnlyReason && (
              <div className="mt-2 flex items-center gap-2 rounded-md border bg-muted/50 px-3 py-2 text-xs text-muted-foreground">
                <Lock className="h-3.5 w-3.5 shrink-0 text-amber-600 dark:text-amber-400" />
                <span>{readOnlyReason}</span>
              </div>
            )}
          </CardHeader>

          <CardContent className="space-y-1.5">
            {buttons.map((item, idx) => {
              const isDefault = item.action.type === item.slot.defaultAction;
              const isHighlighted = hoveredId === item.slot.id;

              return (
                <div
                  key={item.slot.id}
                  onMouseEnter={() => {
                    setHoveredId(item.slot.id);
                    setDiagramView(item.slot.view === "bottom" ? "bottom" : "top");
                  }}
                  onMouseLeave={() => setHoveredId(null)}
                  className={cn(
                    "flex items-center justify-between gap-3 rounded-lg border px-3 py-2 transition-colors",
                    isHighlighted ? "border-primary bg-primary/5" : "bg-background/40 hover:bg-accent/40"
                  )}
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-bold tabular-nums">
                      {idx + 1}
                    </span>
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5 flex-wrap">
                        <span className="text-xs sm:text-sm font-medium truncate">{item.slot.nameJa}</span>
                        {item.slot.view === "bottom" && (
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                            底面
                          </Badge>
                        )}
                        {!isDefault && (
                          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                            変更済
                          </Badge>
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    <Badge variant={categoryBadgeVariant(item.action.category)} className="font-medium">
                      {item.action.labelJa}
                    </Badge>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 px-2.5 text-xs"
                      disabled={!writable}
                      aria-label={`${item.slot.nameJa} の割り当てを変更`}
                      onClick={() => openEditor(item)}
                    >
                      変更
                    </Button>
                  </div>
                </div>
              );
            })}
          </CardContent>
        </Card>
      </div>

      {/* Action Assignment Modal */}
      <Dialog open={editingMapping !== null} onOpenChange={(open) => !open && setEditingMapping(null)}>
        <DialogContent className="max-w-xl">
          {editingMapping && (
            <>
              <DialogHeader>
                <DialogTitle>「{editingMapping.slot.nameJa}」の割り当て変更</DialogTitle>
                <DialogDescription>
                  現在: <strong>{editingMapping.action.labelJa}</strong> • 割り当てたい機能を選択してください。
                </DialogDescription>
              </DialogHeader>

              {/* Hardware Onboard Macro Overwrite Warning */}
              {editingMapping.action.type === "unknown" && (
                <div
                  role="alert"
                  className="flex items-start gap-2 rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300"
                >
                  <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
                  <div>
                    <p className="font-semibold">オンボードマクロ / カスタム設定の上書き注意</p>
                    <p className="mt-0.5 leading-relaxed">
                      現在このボタンには「{editingMapping.action.labelJa}」が保存されています。別の機能を保存すると上書きされます（維持する場合はキャンセルを押してください）。
                    </p>
                  </div>
                </div>
              )}

              {/* Left-Click Safety Guard */}
              {wouldViolateLeftClickGuard && (
                <div
                  role="alert"
                  className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive"
                >
                  <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
                  <div>
                    <p className="font-semibold">左クリック保護ガード</p>
                    <p className="mt-0.5 leading-relaxed">
                      PCが操作不能になるのを防ぐため、少なくとも1つの物理ボタンに「左クリック」を残す必要があります。このボタンを変更するには、先に別のボタンへ「左クリック」を割り当ててください。
                    </p>
                  </div>
                </div>
              )}

              <Tabs
                value={draftTab}
                onValueChange={(val) => {
                  setDraftTab(val);
                  if (val === "disabled") setDraftType("disabled");
                  if (val === "keyboard") setDraftType("keyboard");
                }}
              >
                <TabsList className="grid w-full grid-cols-5 h-9">
                  <TabsTrigger value="mouse" className="text-xs px-2">
                    <MousePointer className="h-3 w-3 hidden sm:inline" />
                    マウス
                  </TabsTrigger>
                  <TabsTrigger value="dpi" className="text-xs px-2">
                    <Sliders className="h-3 w-3 hidden sm:inline" />
                    DPI・本体
                  </TabsTrigger>
                  <TabsTrigger value="keyboard" className="text-xs px-2">
                    <Keyboard className="h-3 w-3 hidden sm:inline" />
                    キー入力
                  </TabsTrigger>
                  <TabsTrigger value="media" className="text-xs px-2">
                    <Volume2 className="h-3 w-3 hidden sm:inline" />
                    メディア
                  </TabsTrigger>
                  <TabsTrigger value="disabled" className="text-xs px-2">
                    <Ban className="h-3 w-3 hidden sm:inline" />
                    無効化
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="mouse" className="grid grid-cols-2 gap-2 pt-1">
                  {MOUSE_ACTIONS.filter(
                    (act) =>
                      act !== "wheel_default" ||
                      editingMapping.slot.id === 11 ||
                      editingMapping.slot.id === 12
                  ).map((act) => (
                    <ActionOptionButton
                      key={act}
                      selected={draftType === act}
                      label={ACTION_LABEL_MAP[act].labelJa}
                      hint={ACTION_LABEL_MAP[act].hint}
                      isDefault={editingMapping.slot.defaultAction === act}
                      onClick={() => setDraftType(act)}
                    />
                  ))}
                </TabsContent>

                <TabsContent value="dpi" className="grid grid-cols-2 gap-2 pt-1">
                  {DPI_HW_ACTIONS.map((act) => (
                    <ActionOptionButton
                      key={act}
                      selected={draftType === act}
                      label={ACTION_LABEL_MAP[act].labelJa}
                      hint={ACTION_LABEL_MAP[act].hint}
                      isDefault={editingMapping.slot.defaultAction === act}
                      onClick={() => setDraftType(act)}
                    />
                  ))}
                </TabsContent>

                <TabsContent value="keyboard" className="space-y-3 pt-1">
                  <div className="rounded-lg border bg-muted/40 p-3 space-y-2.5">
                    <div className="flex items-center justify-between gap-2 flex-wrap">
                      <span className="text-xs font-medium">
                        単一キー / 修飾キー単体（Shift・Ctrl等） / ショートカット
                      </span>
                      <Button
                        size="sm"
                        variant={capturingKey ? "default" : "outline"}
                        className="h-7 text-xs"
                        onClick={() => {
                          setDraftType("keyboard");
                          setCapturingKey((c) => !c);
                        }}
                      >
                        <Keyboard className="h-3.5 w-3.5" />
                        {capturingKey ? "キーを押して確定 (Escで中断)..." : "キー入力をキャプチャ"}
                      </Button>
                    </div>

                    <div className="flex items-center justify-center rounded-md border bg-background py-3 text-base font-mono font-semibold text-primary">
                      {formatCombo(draftMods, draftKeyName)}
                    </div>

                    {/* Modifier toggles + Manual Key Dropdown (including standalone modifier code 0) */}
                    <div className="grid gap-2 sm:grid-cols-2 pt-1">
                      <div className="space-y-1">
                        <span className="text-[11px] text-muted-foreground">修飾キー（単体または同時押し）</span>
                        <div className="flex gap-1.5">
                          {(
                            [
                              ["ctrl", "Ctrl"],
                              ["shift", "Shift"],
                              ["alt", "Alt"],
                              ["meta", "Win"],
                            ] as const
                          ).map(([modKey, label]) => {
                            const active = draftMods[modKey];
                            return (
                              <button
                                key={modKey}
                                type="button"
                                aria-pressed={active}
                                onClick={() => {
                                  setDraftType("keyboard");
                                  setDraftMods((m) => ({ ...m, [modKey]: !m[modKey] }));
                                }}
                                className={cn(
                                  "flex-1 rounded border py-1 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                                  active
                                    ? "border-primary bg-primary text-primary-foreground"
                                    : "bg-background hover:bg-accent"
                                )}
                              >
                                {label}
                              </button>
                            );
                          })}
                        </div>
                      </div>

                      <div className="space-y-1">
                        <span className="text-[11px] text-muted-foreground">メインキー選択</span>
                        <Select
                          value={String(draftKeyCode)}
                          onValueChange={(val) => {
                            const code = Number(val);
                            setDraftType("keyboard");
                            if (code === 0) {
                              setDraftKeyCode(0);
                              setDraftKeyName("");
                              return;
                            }
                            const found = supportedKeys.find((k) => k.code === code);
                            if (found) {
                              setDraftKeyCode(found.code);
                              setDraftKeyName(found.name);
                            }
                          }}
                        >
                          <SelectTrigger className="h-8 text-xs" aria-label="割り当てるキーを選択">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="0">なし（修飾キー単体のみ）</SelectItem>
                            {supportedKeys.map((k) => (
                              <SelectItem key={k.code} value={String(k.code)}>
                                {k.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  </div>
                </TabsContent>

                <TabsContent value="media" className="grid grid-cols-2 gap-2 pt-1 max-h-56 overflow-y-auto pr-1">
                  {MEDIA_ACTIONS.map((act) => (
                    <ActionOptionButton
                      key={act}
                      selected={draftType === act}
                      label={ACTION_LABEL_MAP[act].labelJa}
                      isDefault={editingMapping.slot.defaultAction === act}
                      onClick={() => setDraftType(act)}
                    />
                  ))}
                </TabsContent>

                <TabsContent value="disabled" className="pt-1">
                  <ActionOptionButton
                    selected={draftType === "disabled"}
                    label="無効 (誤押し防止のためボタンをOFFにする)"
                    hint="試合中の誤爆防止に最適"
                    isDefault={false}
                    onClick={() => setDraftType("disabled")}
                  />
                </TabsContent>
              </Tabs>

              <DialogFooter className="flex-wrap sm:justify-between gap-2">
                <div>
                  {editingMapping.action.type !== editingMapping.slot.defaultAction && (
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={
                        isSoleLeftClickHolder && editingMapping.slot.defaultAction !== "left_click"
                      }
                      onClick={() => void handleResetSingleButton()}
                    >
                      <RotateCcw className="h-3.5 w-3.5" />
                      標準（{ACTION_LABEL_MAP[editingMapping.slot.defaultAction].labelJa}）に戻す
                    </Button>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <Button variant="outline" onClick={() => setEditingMapping(null)}>
                    キャンセル
                  </Button>
                  <Button
                    disabled={wouldViolateLeftClickGuard || invalidKeyboardCombo}
                    onClick={() => void handleConfirmSave()}
                  >
                    この割り当てを保存
                  </Button>
                </div>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function ActionOptionButton({
  selected,
  label,
  hint,
  isDefault,
  onClick,
}: {
  selected: boolean;
  label: string;
  hint?: string;
  isDefault: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={cn(
        "flex flex-col justify-center rounded-md border px-3 py-2 text-left transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        selected
          ? "border-primary bg-primary/15 text-primary ring-1 ring-primary/40"
          : "bg-background/60 hover:bg-accent"
      )}
    >
      <div className="flex items-center justify-between gap-1 w-full">
        <span className="text-xs font-medium truncate">{label}</span>
        {isDefault && (
          <span className="ml-1 shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
            標準
          </span>
        )}
      </div>
      {hint && <span className="mt-0.5 text-[10px] text-muted-foreground leading-snug">{hint}</span>}
    </button>
  );
}

// ============================================================================
// Accessible Interactive Rapoo VT9 Pro 4K Top & Bottom SVG Diagrams
// ============================================================================

interface MouseSvgProps {
  hoveredId: number | null;
  writable: boolean;
  buttonById: Map<number, ButtonMapping>;
  onHover: (id: number | null) => void;
  onSelect: (id: number) => void;
}

function buildHotspotProps(
  id: number,
  writable: boolean,
  hoveredId: number | null,
  buttonById: Map<number, ButtonMapping>,
  onHover: (id: number | null) => void,
  onSelect: (id: number) => void,
  accentPink = false
) {
  const m = buttonById.get(id);
  const label = m ? `${m.slot.nameJa}（現在: ${m.action.labelJa}）` : `ボタン ${id}`;
  const isHovered = hoveredId === id;

  return {
    role: "button" as const,
    tabIndex: writable ? 0 : -1,
    "aria-disabled": !writable,
    "aria-label": label,
    onMouseEnter: () => onHover(id),
    onMouseLeave: () => onHover(null),
    onFocus: () => onHover(id),
    onBlur: () => onHover(null),
    onClick: () => {
      if (writable) onSelect(id);
    },
    onKeyDown: (e: React.KeyboardEvent) => {
      if (!writable) return;
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        onSelect(id);
      }
    },
    className: cn(
      "transition-all focus:outline-none",
      writable ? "cursor-pointer" : "cursor-not-allowed opacity-70",
      accentPink
        ? isHovered
          ? "fill-pink-500/50 stroke-pink-500 stroke-[2.5]"
          : "fill-pink-500/25 hover:fill-pink-500/40 stroke-pink-600 dark:stroke-pink-400 stroke-[1.8]"
        : isHovered
          ? "fill-primary/45 stroke-primary stroke-[2.2]"
          : "fill-primary/10 hover:fill-primary/30 stroke-border stroke-[1.5]"
    ),
  };
}

function MouseTopSvg({ hoveredId, writable, buttonById, onHover, onSelect }: MouseSvgProps) {
  const hp = (id: number) => buildHotspotProps(id, writable, hoveredId, buttonById, onHover, onSelect);

  return (
    <svg
      viewBox="0 0 260 360"
      className="w-full max-w-[230px] select-none"
      role="group"
      aria-label="Rapoo VT9 Pro 4K 天面・サイドボタン選択図"
    >
      <path
        d="M130 16 C82 16 58 46 56 112 C54 176 62 214 60 256 C58 308 82 344 130 344 C178 344 202 308 200 256 C198 214 206 176 204 112 C202 46 178 16 130 16 Z"
        className="fill-muted/50 stroke-muted-foreground/40 stroke-[2]"
      />

      {/* Left Click (0) */}
      <path d="M126 20 C86 21 63 49 61 112 L62 142 L126 142 Z" {...hp(0)} />
      {/* Right Click (1) */}
      <path d="M134 20 C174 21 197 49 199 112 L198 142 L134 142 Z" {...hp(1)} />
      {/* Scroll Up (11) */}
      <rect x="119" y="48" width="22" height="18" rx="5" {...hp(11)} />
      {/* Middle Click (2) */}
      <rect x="118" y="68" width="24" height="28" rx="6" {...hp(2)} />
      {/* Scroll Down (12) */}
      <rect x="119" y="98" width="22" height="18" rx="5" {...hp(12)} />
      {/* DPI+ (5) */}
      <rect x="122" y="124" width="16" height="16" rx="4" {...hp(5)} />
      {/* DPI- (6) */}
      <rect x="122" y="144" width="16" height="16" rx="4" {...hp(6)} />
      {/* Forward (7) */}
      <rect x="48" y="146" width="12" height="34" rx="5" {...hp(7)} />
      {/* Back (8) */}
      <rect x="49" y="186" width="12" height="34" rx="5" {...hp(8)} />

      <path
        d="M118 268 L130 292 L142 268"
        fill="none"
        className="stroke-muted-foreground/35 stroke-[2.5]"
        strokeLinecap="round"
        strokeLinejoin="round"
      />

      <SvgCallout x={90} y={84} label="1" active={hoveredId === 0} />
      <SvgCallout x={170} y={84} label="2" active={hoveredId === 1} />
      <SvgCallout x={130} y={82} label="3" active={hoveredId === 2} />
      <SvgCallout x={34} y={163} label="4" active={hoveredId === 7} />
      <SvgCallout x={34} y={203} label="5" active={hoveredId === 8} />
      <SvgCallout x={152} y={132} label="6" active={hoveredId === 5} />
      <SvgCallout x={152} y={152} label="7" active={hoveredId === 6} />
      <SvgCallout x={130} y={57} label="8" active={hoveredId === 11} />
      <SvgCallout x={130} y={107} label="9" active={hoveredId === 12} />
    </svg>
  );
}

function MouseBottomSvg({ hoveredId, writable, buttonById, onHover, onSelect }: MouseSvgProps) {
  const diyProps = buildHotspotProps(13, writable, hoveredId, buttonById, onHover, onSelect, true);

  return (
    <svg
      viewBox="0 0 260 360"
      className="w-full max-w-[230px] select-none"
      role="group"
      aria-label="Rapoo VT9 Pro 4K 底面DIYボタン選択図"
    >
      <path
        d="M130 16 C82 16 58 46 56 112 C54 176 62 214 60 256 C58 308 82 344 130 344 C178 344 202 308 200 256 C198 214 206 176 204 112 C202 46 178 16 130 16 Z"
        className="fill-muted/50 stroke-muted-foreground/40 stroke-[2]"
      />
      <path
        d="M84 36 C105 24 155 24 176 36 C170 52 90 52 84 36 Z"
        className="fill-background/70 stroke-border stroke-[1.2]"
      />
      <path
        d="M80 306 C105 326 155 326 180 306 C172 292 88 292 80 306 Z"
        className="fill-background/70 stroke-border stroke-[1.2]"
      />

      <rect
        x="116"
        y="156"
        width="28"
        height="38"
        rx="14"
        className="fill-background stroke-muted-foreground/50 stroke-[1.5]"
      />
      <text x="130" y="210" textAnchor="middle" className="fill-muted-foreground text-[9px] font-mono">
        PAW3395
      </text>

      <rect
        x="96"
        y="112"
        width="16"
        height="26"
        rx="4"
        className="fill-background/70 stroke-border stroke-[1.2]"
      />
      <text x="104" y="106" textAnchor="middle" className="fill-muted-foreground text-[8px]">
        電源スイッチ
      </text>

      {/* Bottom DIY Programmable Button (id: 13) */}
      <circle cx="156" cy="125" r="14" {...diyProps} />
      <SvgCallout x={156} y={125} label="10" active={hoveredId === 13} />
      <text
        x="156"
        y="104"
        textAnchor="middle"
        className="fill-pink-700 dark:fill-pink-400 text-[9px] font-semibold"
      >
        DIY ボタン
      </text>

      <circle
        cx="130"
        cy="258"
        r="24"
        className="fill-background/50 stroke-border stroke-[1.2]"
        strokeDasharray="3 2"
      />
      <text x="130" y="261" textAnchor="middle" className="fill-muted-foreground text-[8px]">
        4K収納カバー
      </text>
    </svg>
  );
}

function SvgCallout({ x, y, label, active }: { x: number; y: number; label: string; active: boolean }) {
  return (
    <g className="pointer-events-none" aria-hidden="true">
      <circle
        cx={x}
        cy={y}
        r="8.5"
        className={cn(
          "transition-colors",
          active ? "fill-primary stroke-background stroke-[1.5]" : "fill-card/95 stroke-border"
        )}
      />
      <text
        x={x}
        y={y + 3}
        textAnchor="middle"
        className={cn(
          "text-[9px] font-bold tabular-nums",
          active ? "fill-primary-foreground" : "fill-foreground"
        )}
      >
        {label}
      </text>
    </g>
  );
}
