import {
  AlertTriangle,
  Keyboard,
  MousePointer,
  RotateCcw,
  Sliders,
  Volume2,
  Ban,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
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
  busy: boolean;
  onSaveButtonMapping: (buttonId: number, action: ButtonAction) => Promise<void>;
  onResetAllButtons: () => Promise<void>;
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
  return parts.length > 0 ? parts.join(" + ") : "未選択";
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
  busy,
  onSaveButtonMapping,
  onResetAllButtons,
}: ButtonsTabProps) {
  const buttons = snapshot.buttons ?? [];
  const [diagramView, setDiagramView] = useState<"top" | "bottom">("top");
  const [hoveredId, setHoveredId] = useState<number | null>(null);
  const [editingMapping, setEditingMapping] = useState<ButtonMapping | null>(null);

  // Dialog draft state
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

  const openEditor = (mapping: ButtonMapping) => {
    setEditingMapping(mapping);
    setCapturingKey(false);
    const act = mapping.action;
    setDraftType(act.type === "unknown" ? mapping.slot.defaultAction : act.type);

    if (act.type === "keyboard") {
      setDraftTab("keyboard");
      setDraftMods(act.modifiers ?? { ctrl: false, shift: false, alt: false, meta: false });
      setDraftKeyCode(act.keyCode || 0x04);
      setDraftKeyName(act.keyName || "A");
    } else if (DPI_HW_ACTIONS.includes(act.type)) {
      setDraftTab("dpi");
    } else if (MEDIA_ACTIONS.includes(act.type)) {
      setDraftTab("media");
    } else if (act.type === "disabled") {
      setDraftTab("disabled");
    } else {
      setDraftTab("mouse");
    }

    if (mapping.slot.view === "bottom") {
      setDiagramView("bottom");
    } else {
      setDiagramView("top");
    }
  };

  // Keystroke recorder inside dialog
  useEffect(() => {
    if (!capturingKey) return;
    const onKeyDown = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();

      if (["Control", "Shift", "Alt", "Meta"].includes(e.key)) {
        setDraftMods({
          ctrl: e.ctrlKey,
          shift: e.shiftKey,
          alt: e.altKey,
          meta: e.metaKey,
        });
        return;
      }

      const found = supportedKeys.find((k) => k.domCode.toLowerCase() === e.code.toLowerCase());
      if (found) {
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

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [capturingKey, supportedKeys]);

  // Check if editing button is the ONLY physical clickable button holding left_click
  const isSoleLeftClickHolder = useMemo(() => {
    if (!editingMapping) return false;
    const clickableLefts = buttons.filter((b) => b.slot.clickable && b.action.type === "left_click");
    return clickableLefts.length === 1 && clickableLefts[0].slot.id === editingMapping.slot.id;
  }, [buttons, editingMapping]);

  const wouldViolateLeftClickGuard = isSoleLeftClickHolder && draftType !== "left_click";

  const handleConfirmSave = async () => {
    if (!editingMapping || wouldViolateLeftClickGuard) return;

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
      action = {
        type: draftType,
        category: meta.category,
        labelJa: meta.labelJa,
      };
    }

    await onSaveButtonMapping(editingMapping.slot.id, action);
    setEditingMapping(null);
  };

  const buttonById = useMemo(() => {
    const m = new Map<number, ButtonMapping>();
    buttons.forEach((b) => m.set(b.slot.id, b));
    return m;
  }, [buttons]);

  return (
    <div className="grid gap-4 lg:grid-cols-12">
      {/* Left 5 cols: Interactive VT9 Pro 4K Top/Bottom SVG Blueprint */}
      <div className="lg:col-span-5">
        <Card className="h-full flex flex-col">
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between gap-2">
              <CardTitle className="flex items-center gap-2">
                <MousePointer className="h-4 w-4 text-primary" />
                物理レイアウト図
              </CardTitle>
              <div className="inline-flex rounded-md border bg-muted/60 p-0.5 text-xs">
                <button
                  type="button"
                  onClick={() => setDiagramView("top")}
                  className={cn(
                    "rounded px-2.5 py-1 font-medium transition-colors",
                    diagramView === "top" ? "bg-card text-foreground shadow-sm" : "text-muted-foreground"
                  )}
                >
                  天面・サイド (9)
                </button>
                <button
                  type="button"
                  onClick={() => setDiagramView("bottom")}
                  className={cn(
                    "rounded px-2.5 py-1 font-medium transition-colors",
                    diagramView === "bottom" ? "bg-card text-foreground shadow-sm" : "text-muted-foreground"
                  )}
                >
                  底面 DIY (1)
                </button>
              </div>
            </div>
            <CardDescription>
              図上の各ボタンをクリックしても直接割り当てを変更できます。
            </CardDescription>
          </CardHeader>

          <CardContent className="flex-1 flex flex-col items-center justify-center py-2">
            {diagramView === "top" ? (
              <MouseTopSvg
                hoveredId={hoveredId}
                onHover={setHoveredId}
                onSelect={(id) => {
                  const found = buttonById.get(id);
                  if (found && writable) openEditor(found);
                }}
              />
            ) : (
              <MouseBottomSvg
                hoveredId={hoveredId}
                onHover={setHoveredId}
                onSelect={(id) => {
                  const found = buttonById.get(id);
                  if (found && writable) openEditor(found);
                }}
              />
            )}

            <div className="mt-2 text-center text-[11px] text-muted-foreground">
              {hoveredId !== null && buttonById.get(hoveredId) ? (
                <span>
                  選択中: <strong className="text-foreground">{buttonById.get(hoveredId)!.slot.nameJa}</strong> →{" "}
                  <span className="text-primary font-semibold">
                    {buttonById.get(hoveredId)!.action.labelJa}
                  </span>
                </span>
              ) : (
                <span>全10箇所（スクロール上下・底面DIYボタンを含む）をオンボード保存</span>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Right 7 cols: 10 Physical Buttons Table */}
      <div className="lg:col-span-7">
        <Card>
          <CardHeader className="pb-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle>全 10 ボタン割り当て一覧</CardTitle>
                <CardDescription>
                  変更はマウス本体のフラッシュメモリ（0x0600〜0x0637）へ即座に書き込まれ、別PCでも有効です。
                </CardDescription>
              </div>

              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="outline" size="sm" disabled={!writable || busy}>
                    <RotateCcw className="h-3.5 w-3.5" />
                    全ボタンを初期値に戻す
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>全10ボタンの割り当てを初期化しますか？</AlertDialogTitle>
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
                          <span
                            title="初期値から変更済み"
                            className="h-1.5 w-1.5 rounded-full bg-primary shrink-0"
                          />
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
                      disabled={!writable || busy}
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
                      isDefault={editingMapping.slot.defaultAction === act}
                      onClick={() => setDraftType(act)}
                    />
                  ))}
                </TabsContent>

                <TabsContent value="keyboard" className="space-y-3 pt-1">
                  {/* Live Keystroke Recorder */}
                  <div className="rounded-lg border bg-muted/40 p-3 space-y-2.5">
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-xs font-medium">ショートカット / 単一キー設定</span>
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
                        {capturingKey ? "キーボードのキーを押してください..." : "キー入力をキャプチャ"}
                      </Button>
                    </div>

                    <div className="flex items-center justify-center rounded-md border bg-background py-3 text-base font-mono font-semibold text-primary">
                      {formatCombo(draftMods, draftKeyName)}
                    </div>

                    {/* Modifier toggles + Manual Key Dropdown */}
                    <div className="grid gap-2 sm:grid-cols-2 pt-1">
                      <div className="space-y-1">
                        <span className="text-[11px] text-muted-foreground">修飾キー (同時押し)</span>
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
                                  "flex-1 rounded border py-1 text-xs font-medium transition-colors",
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
                            const found = supportedKeys.find((k) => k.code === code);
                            if (found) {
                              setDraftType("keyboard");
                              setDraftKeyCode(found.code);
                              setDraftKeyName(found.name);
                            }
                          }}
                        >
                          <SelectTrigger className="h-8 text-xs" aria-label="割り当てるキーを選択">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
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
                    isDefault={false}
                    onClick={() => setDraftType("disabled")}
                  />
                </TabsContent>
              </Tabs>

              <DialogFooter>
                <Button variant="outline" onClick={() => setEditingMapping(null)}>
                  キャンセル
                </Button>
                <Button disabled={wouldViolateLeftClickGuard || busy} onClick={() => void handleConfirmSave()}>
                  この割り当てを保存
                </Button>
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
  isDefault,
  onClick,
}: {
  selected: boolean;
  label: string;
  isDefault: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={cn(
        "flex items-center justify-between rounded-md border px-3 py-2 text-left text-xs font-medium transition-all",
        selected
          ? "border-primary bg-primary/15 text-primary ring-1 ring-primary/40"
          : "bg-background/60 hover:bg-accent"
      )}
    >
      <span className="truncate">{label}</span>
      {isDefault && (
        <span className="ml-1.5 shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
          標準
        </span>
      )}
    </button>
  );
}

// ============================================================================
// Interactive Rapoo VT9 Pro 4K Top & Bottom SVG Diagrams
// ============================================================================

interface MouseSvgProps {
  hoveredId: number | null;
  onHover: (id: number | null) => void;
  onSelect: (id: number) => void;
}

function MouseTopSvg({ hoveredId, onHover, onSelect }: MouseSvgProps) {
  const hotspotClass = (id: number) =>
    cn(
      "cursor-pointer transition-all",
      hoveredId === id
        ? "fill-primary/45 stroke-primary stroke-[2.2]"
        : "fill-primary/10 hover:fill-primary/30 stroke-border stroke-[1.5]"
    );

  return (
    <svg
      viewBox="0 0 260 360"
      className="w-full max-w-[235px] select-none"
      role="img"
      aria-label="Rapoo VT9 Pro 4K 天面ボタン配置図"
    >
      {/* Outer Symmetrical Shell */}
      <path
        d="M130 16 C82 16 58 46 56 112 C54 176 62 214 60 256 C58 308 82 344 130 344 C178 344 202 308 200 256 C198 214 206 176 204 112 C202 46 178 16 130 16 Z"
        className="fill-muted/50 stroke-muted-foreground/40 stroke-[2]"
      />

      {/* Left Main Trigger (id: 0) */}
      <path
        d="M126 20 C86 21 63 49 61 112 L62 142 L126 142 Z"
        className={hotspotClass(0)}
        onMouseEnter={() => onHover(0)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(0)}
      />

      {/* Right Main Trigger (id: 1) */}
      <path
        d="M134 20 C174 21 197 49 199 112 L198 142 L134 142 Z"
        className={hotspotClass(1)}
        onMouseEnter={() => onHover(1)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(1)}
      />

      {/* Scroll Up Zone (id: 11) */}
      <rect
        x="119"
        y="48"
        width="22"
        height="18"
        rx="5"
        className={hotspotClass(11)}
        onMouseEnter={() => onHover(11)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(11)}
      />

      {/* Middle Click Wheel (id: 2) */}
      <rect
        x="118"
        y="68"
        width="24"
        height="28"
        rx="6"
        className={hotspotClass(2)}
        onMouseEnter={() => onHover(2)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(2)}
      />

      {/* Scroll Down Zone (id: 12) */}
      <rect
        x="119"
        y="98"
        width="22"
        height="18"
        rx="5"
        className={hotspotClass(12)}
        onMouseEnter={() => onHover(12)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(12)}
      />

      {/* DPI+ Button (id: 5) */}
      <rect
        x="122"
        y="124"
        width="16"
        height="16"
        rx="4"
        className={hotspotClass(5)}
        onMouseEnter={() => onHover(5)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(5)}
      />

      {/* DPI- Button (id: 6) */}
      <rect
        x="122"
        y="144"
        width="16"
        height="16"
        rx="4"
        className={hotspotClass(6)}
        onMouseEnter={() => onHover(6)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(6)}
      />

      {/* Side Forward Button (id: 7) */}
      <rect
        x="48"
        y="146"
        width="12"
        height="34"
        rx="5"
        className={hotspotClass(7)}
        onMouseEnter={() => onHover(7)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(7)}
      />

      {/* Side Back Button (id: 8) */}
      <rect
        x="49"
        y="186"
        width="12"
        height="34"
        rx="5"
        className={hotspotClass(8)}
        onMouseEnter={() => onHover(8)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(8)}
      />

      {/* Subtle V Logo Mark */}
      <path
        d="M118 268 L130 292 L142 268"
        fill="none"
        className="stroke-muted-foreground/35 stroke-[2.5]"
        strokeLinecap="round"
        strokeLinejoin="round"
      />

      {/* Number Callouts matching List indices */}
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

function MouseBottomSvg({ hoveredId, onHover, onSelect }: MouseSvgProps) {
  const active = hoveredId === 13;
  return (
    <svg
      viewBox="0 0 260 360"
      className="w-full max-w-[235px] select-none"
      role="img"
      aria-label="Rapoo VT9 Pro 4K 底面DIYボタン配置図"
    >
      {/* Bottom Chassis */}
      <path
        d="M130 16 C82 16 58 46 56 112 C54 176 62 214 60 256 C58 308 82 344 130 344 C178 344 202 308 200 256 C198 214 206 176 204 112 C202 46 178 16 130 16 Z"
        className="fill-muted/50 stroke-muted-foreground/40 stroke-[2]"
      />

      {/* Top & Bottom PTFE Skates */}
      <path
        d="M84 36 C105 24 155 24 176 36 C170 52 90 52 84 36 Z"
        className="fill-background/70 stroke-border stroke-[1.2]"
      />
      <path
        d="M80 306 C105 326 155 326 180 306 C172 292 88 292 80 306 Z"
        className="fill-background/70 stroke-border stroke-[1.2]"
      />

      {/* PAW3395 Optical Sensor Window */}
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

      {/* Power Slide Switch (Non-programmable hardware power switch) */}
      <rect
        x="96"
        y="112"
        width="16"
        height="26"
        rx="4"
        className="fill-background/70 stroke-border stroke-[1.2]"
      />
      <text x="104" y="106" textAnchor="middle" className="fill-muted-foreground text-[8px]">
        ON/OFF
      </text>

      {/* Bottom DIY Pink/Accent Programmable Button (id: 13 at 0x0634) */}
      <circle
        cx="156"
        cy="125"
        r="14"
        className={cn(
          "cursor-pointer transition-all",
          active
            ? "fill-pink-500/50 stroke-pink-400 stroke-[2.5]"
            : "fill-pink-500/25 hover:fill-pink-500/40 stroke-pink-500/70 stroke-[1.8]"
        )}
        onMouseEnter={() => onHover(13)}
        onMouseLeave={() => onHover(null)}
        onClick={() => onSelect(13)}
      />
      <SvgCallout x={156} y={125} label="10" active={active} />
      <text x="156" y="104" textAnchor="middle" className="fill-pink-400 text-[9px] font-semibold">
        DIY ボタン
      </text>

      {/* Magnetic Dongle Storage Cover */}
      <circle
        cx="130"
        cy="258"
        r="24"
        className="fill-background/50 stroke-border stroke-[1.2] stroke-dasharray-[3_2]"
      />
      <text x="130" y="261" textAnchor="middle" className="fill-muted-foreground text-[8px]">
        4K収納カバー
      </text>
    </svg>
  );
}

function SvgCallout({ x, y, label, active }: { x: number; y: number; label: string; active: boolean }) {
  return (
    <g className="pointer-events-none">
      <circle
        cx={x}
        cy={y}
        r="8.5"
        className={cn(
          "transition-colors",
          active ? "fill-primary stroke-background stroke-[1.5]" : "fill-card/90 stroke-border"
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
