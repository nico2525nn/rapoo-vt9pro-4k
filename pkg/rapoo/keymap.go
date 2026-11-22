package rapoo

import (
	"fmt"
	"strings"
)

// Onboard button mapping table base addresses.
const (
	AddrButtonTableStart uint32 = 0x0600
	AddrButtonBlock1     uint32 = 0x0600 // 24 bytes (slots 0..5: 0x0600..0x0617)
	AddrButtonBlock2     uint32 = 0x0618 // 24 bytes (slots 6..11: 0x0618..0x062F)
	AddrButtonBlock3     uint32 = 0x0630 // 8 bytes  (slots 12..13: 0x0630..0x0637)
	ButtonTableTotalSize        = 56     // Covers 0x0600..0x0637 (including DIY button at 0x0634)
)

// ButtonID matches official Rapoo VT9PRO_4K_WirelessLayout.json key IDs.
type ButtonID int

const (
	ButtonLeft      ButtonID = 0
	ButtonRight     ButtonID = 1
	ButtonMiddle    ButtonID = 2
	ButtonDPIPlus   ButtonID = 5
	ButtonDPIMinus  ButtonID = 6
	ButtonForward   ButtonID = 7
	ButtonBack      ButtonID = 8
	ButtonWheelUp   ButtonID = 11
	ButtonWheelDown ButtonID = 12
	ButtonBottomDIY ButtonID = 13
)

// ButtonActionType identifies a logical action assigned to a physical button.
type ButtonActionType string

const (
	ActionLeftClick      ButtonActionType = "left_click"
	ActionRightClick     ButtonActionType = "right_click"
	ActionMiddleClick    ButtonActionType = "middle_click"
	ActionForward        ButtonActionType = "forward"
	ActionBack           ButtonActionType = "back"
	ActionWheelDefault   ButtonActionType = "wheel_default"
	ActionFireTriple     ButtonActionType = "fire_triple"
	ActionDPIPlus        ButtonActionType = "dpi_plus"
	ActionDPIMinus       ButtonActionType = "dpi_minus"
	ActionDPICycle       ButtonActionType = "dpi_cycle"
	ActionSniperDPI      ButtonActionType = "sniper_dpi"
	ActionProfileSwitch  ButtonActionType = "profile_switch"
	ActionPollingSwitch  ButtonActionType = "polling_switch"
	ActionReceiverSwitch ButtonActionType = "receiver_switch"
	ActionMediaPlayPause ButtonActionType = "media_play_pause"
	ActionMediaStop      ButtonActionType = "media_stop"
	ActionMediaPrev      ButtonActionType = "media_prev"
	ActionMediaNext      ButtonActionType = "media_next"
	ActionMediaVolUp     ButtonActionType = "media_vol_up"
	ActionMediaVolDown   ButtonActionType = "media_vol_down"
	ActionMediaMute      ButtonActionType = "media_mute"
	ActionMediaPlayer    ButtonActionType = "media_player"
	ActionAppMail        ButtonActionType = "app_mail"
	ActionAppCalculator  ButtonActionType = "app_calculator"
	ActionAppExplorer    ButtonActionType = "app_explorer"
	ActionKeyboard       ButtonActionType = "keyboard"
	ActionDisabled       ButtonActionType = "disabled"
	ActionUnknown        ButtonActionType = "unknown"
)

// KeyModifiers represents standard USB HID keyboard modifier bits in byte[2] of [0x02, 0x00, mod, key].
type KeyModifiers struct {
	Ctrl  bool `json:"ctrl"`
	Shift bool `json:"shift"`
	Alt   bool `json:"alt"`
	Meta  bool `json:"meta"`
}

// Mask encodes KeyModifiers into the USB HID keyboard modifier byte.
func (m KeyModifiers) Mask() byte {
	var b byte
	if m.Ctrl {
		b |= 0x01
	}
	if m.Shift {
		b |= 0x02
	}
	if m.Alt {
		b |= 0x04
	}
	if m.Meta {
		b |= 0x08
	}
	return b
}

// DecodeModifiers decodes a USB HID keyboard modifier byte.
func DecodeModifiers(mask byte) KeyModifiers {
	return KeyModifiers{
		Ctrl:  (mask&0x01) != 0 || (mask&0x10) != 0,
		Shift: (mask&0x02) != 0 || (mask&0x20) != 0,
		Alt:   (mask&0x04) != 0 || (mask&0x40) != 0,
		Meta:  (mask&0x08) != 0 || (mask&0x80) != 0,
	}
}

// ButtonAction represents a decoded or user-requested button action.
type ButtonAction struct {
	Type      ButtonActionType `json:"type"`
	LabelJA   string           `json:"labelJa"`
	Category  string           `json:"category"` // "mouse", "dpi", "media", "keyboard", "hardware", "none"
	Modifiers KeyModifiers     `json:"modifiers,omitempty"`
	KeyCode   byte             `json:"keyCode,omitempty"` // USB HID Usage Page 0x07 code
	KeyName   string           `json:"keyName,omitempty"` // e.g. "A", "F5", "Space"
	Raw       [4]byte          `json:"raw"`
}

// ButtonSlotSpec defines static hardware layout metadata for each of the 10 VT9 Pro 4K buttons.
type ButtonSlotSpec struct {
	ID            ButtonID         `json:"id"`
	NameJA        string           `json:"nameJa"`
	NameEN        string           `json:"nameEn"`
	View          string           `json:"view"`          // "top" or "bottom"
	Addr          uint32           `json:"addr"`          // Exact onboard flash address (0x0600..0x0634)
	DefaultAction ButtonActionType `json:"defaultAction"` // Factory default function
	Clickable     bool             `json:"clickable"`     // True if physical switch (eligible to hold Left Click)
}

// VT9Pro4KButtons lists all 10 remappable buttons on the Rapoo VT9 Pro 4K in logical UI order.
// Verified 10/10 against RapooGameDev.exe vslot[22] (0x1125790) and live RTX5050 memory dump.
var VT9Pro4KButtons = []ButtonSlotSpec{
	{ID: ButtonLeft, NameJA: "左クリック", NameEN: "Left Click", View: "top", Addr: 0x0600, DefaultAction: ActionLeftClick, Clickable: true},
	{ID: ButtonRight, NameJA: "右クリック", NameEN: "Right Click", View: "top", Addr: 0x0608, DefaultAction: ActionRightClick, Clickable: true},
	{ID: ButtonMiddle, NameJA: "ミドルボタン (ホイール押込)", NameEN: "Middle Click", View: "top", Addr: 0x0604, DefaultAction: ActionMiddleClick, Clickable: true},
	{ID: ButtonForward, NameJA: "進む (サイド前)", NameEN: "Forward", View: "top", Addr: 0x0614, DefaultAction: ActionForward, Clickable: true},
	{ID: ButtonBack, NameJA: "戻る (サイド後)", NameEN: "Back", View: "top", Addr: 0x0618, DefaultAction: ActionBack, Clickable: true},
	{ID: ButtonDPIPlus, NameJA: "DPI + ボタン", NameEN: "DPI+", View: "top", Addr: 0x060C, DefaultAction: ActionDPIPlus, Clickable: true},
	{ID: ButtonDPIMinus, NameJA: "DPI - ボタン", NameEN: "DPI-", View: "top", Addr: 0x0610, DefaultAction: ActionDPIMinus, Clickable: true},
	{ID: ButtonWheelUp, NameJA: "ホイール上スクロール", NameEN: "Scroll Up", View: "top", Addr: 0x0624, DefaultAction: ActionWheelDefault, Clickable: false},
	{ID: ButtonWheelDown, NameJA: "ホイール下スクロール", NameEN: "Scroll Down", View: "top", Addr: 0x0628, DefaultAction: ActionWheelDefault, Clickable: false},
	{ID: ButtonBottomDIY, NameJA: "底面 DIY ボタン", NameEN: "Bottom DIY Button", View: "bottom", Addr: 0x0634, DefaultAction: ActionReceiverSwitch, Clickable: true},
}

// LookupButtonSlot returns the ButtonSlotSpec for a ButtonID.
func LookupButtonSlot(id ButtonID) (ButtonSlotSpec, bool) {
	for _, s := range VT9Pro4KButtons {
		if s.ID == id {
			return s, true
		}
	}
	return ButtonSlotSpec{}, false
}

// ButtonMapping pairs a physical button slot with its active onboard ButtonAction.
type ButtonMapping struct {
	Slot   ButtonSlotSpec `json:"slot"`
	Action ButtonAction   `json:"action"`
}

type fixedActionEntry struct {
	Type     ButtonActionType
	LabelJA  string
	Category string
	Raw      [4]byte
}

var fixedActions = []fixedActionEntry{
	{ActionLeftClick, "左クリック", "mouse", [4]byte{0x03, 0x00, 0x01, 0x00}},
	{ActionRightClick, "右クリック", "mouse", [4]byte{0x03, 0x00, 0x02, 0x00}},
	{ActionMiddleClick, "ミドルクリック", "mouse", [4]byte{0x03, 0x00, 0x04, 0x00}},
	{ActionForward, "進む (Mouse 5)", "mouse", [4]byte{0x03, 0x00, 0x10, 0x00}},
	{ActionBack, "戻る (Mouse 4)", "mouse", [4]byte{0x03, 0x00, 0x08, 0x00}},
	{ActionWheelDefault, "標準スクロール", "mouse", [4]byte{0xFF, 0xFF, 0xFF, 0xFF}},
	{ActionFireTriple, "ファイアキー (左3連射)", "mouse", [4]byte{0x03, 0x00, 0x01, 0x01}},

	{ActionDPIPlus, "DPI + (段階を上げる)", "dpi", [4]byte{0x08, 0x00, 0x05, 0x00}},
	{ActionDPIMinus, "DPI - (段階を下げる)", "dpi", [4]byte{0x08, 0x00, 0x06, 0x00}},
	{ActionDPICycle, "DPI ループ切替", "dpi", [4]byte{0x08, 0x00, 0x03, 0x00}},
	{ActionSniperDPI, "スナイパーボタン (押下中低DPI)", "dpi", [4]byte{0x09, 0x00, 0x01, 0x00}},

	{ActionPollingSwitch, "ポーリングレート切替", "hardware", [4]byte{0x0A, 0x00, 0x01, 0x00}},
	{ActionReceiverSwitch, "レシーバー切替", "hardware", [4]byte{0x0A, 0x00, 0x00, 0x00}},
	{ActionProfileSwitch, "オンボード設定切替", "hardware", [4]byte{0x09, 0x00, 0x03, 0x00}},

	{ActionMediaPlayPause, "再生 / 一時停止", "media", [4]byte{0x04, 0x00, 0x00, 0xCD}},
	{ActionMediaStop, "停止", "media", [4]byte{0x04, 0x00, 0x00, 0xB7}},
	{ActionMediaPrev, "前の曲", "media", [4]byte{0x04, 0x00, 0x00, 0xB6}},
	{ActionMediaNext, "次の曲", "media", [4]byte{0x04, 0x00, 0x00, 0xB5}},
	{ActionMediaVolUp, "音量アップ", "media", [4]byte{0x04, 0x00, 0x00, 0xE9}},
	{ActionMediaVolDown, "音量ダウン", "media", [4]byte{0x04, 0x00, 0x00, 0xEA}},
	{ActionMediaMute, "ミュート (消音)", "media", [4]byte{0x04, 0x00, 0x00, 0xE2}},
	{ActionMediaPlayer, "メディアプレーヤー起動", "media", [4]byte{0x04, 0x00, 0x01, 0x83}},
	{ActionAppCalculator, "電卓を起動", "media", [4]byte{0x04, 0x00, 0x01, 0x92}},
	{ActionAppExplorer, "エクスプローラーを起動", "media", [4]byte{0x04, 0x00, 0x01, 0x94}},
	{ActionAppMail, "メールを起動", "media", [4]byte{0x04, 0x00, 0x01, 0x8A}},

	{ActionDisabled, "無効 (ボタンOFF)", "none", [4]byte{0x07, 0x00, 0x00, 0x00}},
}

// HIDKeyInfo maps standard USB HID Usage Page 0x07 codes to display names and DOM KeyboardEvent.code.
type HIDKeyInfo struct {
	Code    byte   `json:"code"`
	Name    string `json:"name"`
	DOMCode string `json:"domCode"`
}

// SupportedHIDKeys provides a comprehensive USB HID Usage Page 0x07 table for key remapping.
var SupportedHIDKeys = []HIDKeyInfo{
	{0x04, "A", "KeyA"}, {0x05, "B", "KeyB"}, {0x06, "C", "KeyC"}, {0x07, "D", "KeyD"},
	{0x08, "E", "KeyE"}, {0x09, "F", "KeyF"}, {0x0A, "G", "KeyG"}, {0x0B, "H", "KeyH"},
	{0x0C, "I", "KeyI"}, {0x0D, "J", "KeyJ"}, {0x0E, "K", "KeyK"}, {0x0F, "L", "KeyL"},
	{0x10, "M", "KeyM"}, {0x11, "N", "KeyN"}, {0x12, "O", "KeyO"}, {0x13, "P", "KeyP"},
	{0x14, "Q", "KeyQ"}, {0x15, "R", "KeyR"}, {0x16, "S", "KeyS"}, {0x17, "T", "KeyT"},
	{0x18, "U", "KeyU"}, {0x19, "V", "KeyV"}, {0x1A, "W", "KeyW"}, {0x1B, "X", "KeyX"},
	{0x1C, "Y", "KeyY"}, {0x1D, "Z", "KeyZ"},
	{0x1E, "1", "Digit1"}, {0x1F, "2", "Digit2"}, {0x20, "3", "Digit3"}, {0x21, "4", "Digit4"},
	{0x22, "5", "Digit5"}, {0x23, "6", "Digit6"}, {0x24, "7", "Digit7"}, {0x25, "8", "Digit8"},
	{0x26, "9", "Digit9"}, {0x27, "0", "Digit0"},
	{0x28, "Enter", "Enter"}, {0x29, "Esc", "Escape"}, {0x2A, "Backspace", "Backspace"},
	{0x2B, "Tab", "Tab"}, {0x2C, "Space", "Space"},
	{0x2D, "-", "Minus"}, {0x2E, "=", "Equal"}, {0x2F, "[", "BracketLeft"}, {0x30, "]", "BracketRight"},
	{0x31, "\\", "Backslash"}, {0x33, ";", "Semicolon"}, {0x34, "'", "Quote"},
	{0x35, "`", "Backquote"}, {0x36, ",", "Comma"}, {0x37, ".", "Period"}, {0x38, "/", "Slash"},
	{0x39, "CapsLock", "CapsLock"},
	{0x3A, "F1", "F1"}, {0x3B, "F2", "F2"}, {0x3C, "F3", "F3"}, {0x3D, "F4", "F4"},
	{0x3E, "F5", "F5"}, {0x3F, "F6", "F6"}, {0x40, "F7", "F7"}, {0x41, "F8", "F8"},
	{0x42, "F9", "F9"}, {0x43, "F10", "F10"}, {0x44, "F11", "F11"}, {0x45, "F12", "F12"},
	{0x46, "PrintScreen", "PrintScreen"}, {0x47, "ScrollLock", "ScrollLock"}, {0x48, "Pause", "Pause"},
	{0x49, "Insert", "Insert"}, {0x4A, "Home", "Home"}, {0x4B, "PageUp", "PageUp"},
	{0x4C, "Delete", "Delete"}, {0x4D, "End", "End"}, {0x4E, "PageDown", "PageDown"},
	{0x4F, "Right", "ArrowRight"}, {0x50, "Left", "ArrowLeft"}, {0x51, "Down", "ArrowDown"}, {0x52, "Up", "ArrowUp"},
	{0x54, "Num /", "NumpadDivide"}, {0x55, "Num *", "NumpadMultiply"}, {0x56, "Num -", "NumpadSubtract"},
	{0x57, "Num +", "NumpadAdd"}, {0x58, "Num Enter", "NumpadEnter"},
	{0x59, "Num 1", "Numpad1"}, {0x5A, "Num 2", "Numpad2"}, {0x5B, "Num 3", "Numpad3"},
	{0x5C, "Num 4", "Numpad4"}, {0x5D, "Num 5", "Numpad5"}, {0x5E, "Num 6", "Numpad6"},
	{0x5F, "Num 7", "Numpad7"}, {0x60, "Num 8", "Numpad8"}, {0x61, "Num 9", "Numpad9"},
	{0x62, "Num 0", "Numpad0"}, {0x63, "Num .", "NumpadDecimal"},
}

// LookupHIDKeyName returns a friendly key name for a USB HID Usage Page 0x07 code.
func LookupHIDKeyName(code byte) string {
	for _, k := range SupportedHIDKeys {
		if k.Code == code {
			return k.Name
		}
	}
	if code == 0 {
		return ""
	}
	return fmt.Sprintf("Key(%#02x)", code)
}

// LookupHIDKeyByNameOrDOM resolves a key name ("A", "F4") or DOM code ("KeyA", "F4") to its HID code.
func LookupHIDKeyByNameOrDOM(s string) (byte, string, bool) {
	trimmed := strings.TrimSpace(s)
	for _, k := range SupportedHIDKeys {
		if strings.EqualFold(k.Name, trimmed) || strings.EqualFold(k.DOMCode, trimmed) {
			return k.Code, k.Name, true
		}
	}
	return 0, "", false
}

// FormatKeyCombo formats modifiers and a key name into e.g. "Ctrl + Shift + R".
func FormatKeyCombo(mods KeyModifiers, keyName string) string {
	var parts []string
	if mods.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if mods.Shift {
		parts = append(parts, "Shift")
	}
	if mods.Alt {
		parts = append(parts, "Alt")
	}
	if mods.Meta {
		parts = append(parts, "Win")
	}
	if keyName != "" {
		parts = append(parts, keyName)
	}
	if len(parts) == 0 {
		return "キー入力なし"
	}
	return strings.Join(parts, " + ")
}

// EncodeButtonAction converts a logical ButtonAction into its exact 4-byte onboard representation.
func EncodeButtonAction(act ButtonAction) ([4]byte, error) {
	if act.Type == ActionKeyboard {
		code := act.KeyCode
		if code == 0 && act.KeyName != "" {
			if resolved, _, ok := LookupHIDKeyByNameOrDOM(act.KeyName); ok {
				code = resolved
			}
		}
		if code == 0 && act.Modifiers.Mask() == 0 {
			return [4]byte{}, fmt.Errorf("rapoo: keyboard action requires a key code or modifier")
		}
		return [4]byte{0x02, 0x00, act.Modifiers.Mask(), code}, nil
	}

	for _, entry := range fixedActions {
		if entry.Type == act.Type {
			return entry.Raw, nil
		}
	}
	return [4]byte{}, fmt.Errorf("rapoo: unsupported button action %q", act.Type)
}

// DecodeButtonAction decodes a 4-byte onboard slot from 0x0600..0x0637 into a ButtonAction.
func DecodeButtonAction(slot ButtonSlotSpec, raw [4]byte) ButtonAction {
	// Check exact 4-byte fixed actions first.
	for _, entry := range fixedActions {
		if entry.Raw == raw {
			label := entry.LabelJA
			if entry.Type == ActionWheelDefault {
				switch slot.ID {
				case ButtonWheelUp:
					label = "上スクロール (標準)"
				case ButtonWheelDown:
					label = "下スクロール (標準)"
				}
			}
			return ButtonAction{
				Type:     entry.Type,
				LabelJA:  label,
				Category: entry.Category,
				Raw:      raw,
			}
		}
	}

	// All-zero slot means unassigned / disabled on some firmware revisions.
	if raw == [4]byte{0x00, 0x00, 0x00, 0x00} {
		return ButtonAction{
			Type:     ActionDisabled,
			LabelJA:  "無効 (ボタンOFF)",
			Category: "none",
			Raw:      raw,
		}
	}

	// Keyboard shortcut: [0x02, 0x00, modMask, hidKeyCode]
	if raw[0] == 0x02 {
		mods := DecodeModifiers(raw[2])
		kName := LookupHIDKeyName(raw[3])
		return ButtonAction{
			Type:      ActionKeyboard,
			LabelJA:   "キー: " + FormatKeyCombo(mods, kName),
			Category:  "keyboard",
			Modifiers: mods,
			KeyCode:   raw[3],
			KeyName:   kName,
			Raw:       raw,
		}
	}

	// Macro binding on mouse: [0x05, _, macroIdx, _]
	if raw[0] == 0x05 {
		return ButtonAction{
			Type:     ActionUnknown,
			LabelJA:  fmt.Sprintf("オンボードマクロ #%d", int(raw[2])+1),
			Category: "hardware",
			Raw:      raw,
		}
	}

	return ButtonAction{
		Type:     ActionUnknown,
		LabelJA:  fmt.Sprintf("カスタム (%02X %02X %02X %02X)", raw[0], raw[1], raw[2], raw[3]),
		Category: "hardware",
		Raw:      raw,
	}
}

// DecodeButtonTable decodes the 56-byte onboard keymap region (0x0600..0x0637)
// into all 10 physical VT9 Pro 4K ButtonMappings.
func DecodeButtonTable(mem600 []byte) ([]ButtonMapping, error) {
	if len(mem600) < ButtonTableTotalSize {
		return nil, fmt.Errorf("%w: button memory block too short (%d < %d)", ErrInvalidResponse, len(mem600), ButtonTableTotalSize)
	}
	out := make([]ButtonMapping, 0, len(VT9Pro4KButtons))
	for _, slot := range VT9Pro4KButtons {
		off := int(slot.Addr - AddrButtonTableStart)
		var raw [4]byte
		copy(raw[:], mem600[off:off+4])
		out = append(out, ButtonMapping{
			Slot:   slot,
			Action: DecodeButtonAction(slot, raw),
		})
	}
	return out, nil
}

// ValidateButtonMappings validates a complete 10-slot table against the
// canonical VT9 Pro 4K button layout. Input slot metadata is never trusted.
func ValidateButtonMappings(mappings []ButtonMapping) error {
	if len(mappings) != len(VT9Pro4KButtons) {
		return fmt.Errorf("rapoo: button table has %d entries, want %d", len(mappings), len(VT9Pro4KButtons))
	}

	seen := make(map[ButtonID]bool, len(VT9Pro4KButtons))
	hasLeftClick := false
	for _, m := range mappings {
		slot, ok := LookupButtonSlot(m.Slot.ID)
		if !ok {
			return fmt.Errorf("rapoo: unknown button ID %d", m.Slot.ID)
		}
		if seen[slot.ID] {
			return fmt.Errorf("rapoo: duplicate button ID %d", slot.ID)
		}
		seen[slot.ID] = true
		if _, err := EncodeButtonAction(m.Action); err != nil {
			return fmt.Errorf("rapoo: button %d: %w", slot.ID, err)
		}
		if slot.Clickable && m.Action.Type == ActionLeftClick {
			hasLeftClick = true
		}
	}
	if !hasLeftClick {
		return ErrMissingLeftClick
	}
	for _, slot := range VT9Pro4KButtons {
		if !seen[slot.ID] {
			return fmt.Errorf("rapoo: missing button ID %d", slot.ID)
		}
	}
	return nil
}

// DefaultButtonMappings returns the factory default 10-button mappings for VT9 Pro 4K.
func DefaultButtonMappings() []ButtonMapping {
	out := make([]ButtonMapping, 0, len(VT9Pro4KButtons))
	for _, slot := range VT9Pro4KButtons {
		raw, _ := EncodeButtonAction(ButtonAction{Type: slot.DefaultAction})
		out = append(out, ButtonMapping{
			Slot:   slot,
			Action: DecodeButtonAction(slot, raw),
		})
	}
	return out
}
