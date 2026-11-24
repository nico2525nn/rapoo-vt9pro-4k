package rapoo

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
)

func TestBuildAndParseBattery(t *testing.T) {
	tx := BuildBatteryQuery(TargetWirelessMouse, OpcodesGenA.GetBattery)
	if len(tx) != 32 {
		t.Fatalf("len(tx) = %d, want 32", len(tx))
	}
	if tx[0] != 0xBA || tx[1] != 0xA5 || tx[2] != 0xAA {
		t.Fatalf("unexpected battery query header: % x", tx[:4])
	}

	now := time.Unix(1700000000, 0)

	// Windows-style response (without leading 0xBA)
	winResp := []byte{0x01, 0x01, 30, 0x00}
	gotWin, err := ParseBatteryResponse(winResp, now)
	if err != nil {
		t.Fatalf("ParseBatteryResponse(win) error: %v", err)
	}
	if gotWin.Percentage != 30 || gotWin.Charging || !gotWin.Online {
		t.Fatalf("unexpected win battery state: %+v", gotWin)
	}

	// Linux hidraw-style response (with leading 0xBA, charging == 2)
	linResp := []byte{0xBA, 0x01, 0x02, 100, 0x00}
	gotLin, err := ParseBatteryResponse(linResp, now)
	if err != nil {
		t.Fatalf("ParseBatteryResponse(linux) error: %v", err)
	}
	if gotLin.Percentage != 100 || !gotLin.Charging || !gotLin.Online {
		t.Fatalf("unexpected linux battery state: %+v", gotLin)
	}

	// Sleeping wireless mouse (status 0x02)
	sleepResp := []byte{0x02, 0x00, 0x00}
	gotSleep, err := ParseBatteryResponse(sleepResp, now)
	if !errors.Is(err, ErrMouseAsleep) || !gotSleep.Sleeping || gotSleep.Online {
		t.Fatalf("expected ErrMouseAsleep, got state=%+v err=%v", gotSleep, err)
	}
}

func TestBuildFactoryResetGenAAndGenB(t *testing.T) {
	genA := BuildFactoryReset(TargetWirelessMouse, OpcodesGenA)
	if genA[0] != 0xBA || genA[1] != 0xA5 || genA[2] != 0xAD || genA[3] != 0x52 || genA[4] != 0x3D {
		t.Fatalf("unexpected Gen-A factory reset packet: % x", genA[:8])
	}
	genB := BuildFactoryReset(TargetLocalOrWired, OpcodesGenB)
	if genB[0] != 0xBA || genB[1] != 0xFF || genB[2] != 0xD0 || genB[3] != 0x00 {
		t.Fatalf("unexpected Gen-B factory reset packet: % x", genB[:8])
	}
}

func TestParseAsyncReport(t *testing.T) {
	now := time.Unix(1700000000, 0)

	pktBB := []byte{0xBB, 0x00, 0x00, 0x01, 0x03, 0x02, 64, 0x00}
	ev, ok := ParseAsyncReport(pktBB, now)
	if !ok || ev.Kind != EventBattery || ev.Battery == nil {
		t.Fatalf("expected battery event, got ok=%v ev=%+v", ok, ev)
	}
	if ev.Battery.Percentage != 64 || !ev.Battery.Charging || !ev.Battery.Online {
		t.Fatalf("unexpected async battery: %+v", ev.Battery)
	}

	pkt06 := []byte{0x06, 0x02, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	evWake, ok := ParseAsyncReport(pkt06, now)
	if !ok || evWake.Kind != EventWake {
		t.Fatalf("expected wake event, got ok=%v ev=%+v", ok, evWake)
	}

	pktBD := []byte{0xBD, 0x01, 0x20, 0x00}
	ev2, ok := ParseAsyncReport(pktBD, now)
	if !ok || ev2.Kind != EventProfile || ev2.ProfileID != 2 {
		t.Fatalf("expected profile 2 event, got ok=%v ev=%+v", ok, ev2)
	}
}

func TestLiveHardwareDumpSensorAndKeymapDecoding(t *testing.T) {
	// Exact bytes dumped read-only from user's live Rapoo VT9 Pro 4K (0x24AE:0x1211) on RTX5050:
	mem880 := []byte{0x01, 0x00, 0x01, 0xFF}
	mem884 := []byte{0x02, 0x00, 0x01, 0x00}
	mem8C0 := []byte{0x08, 0x08, 0x0A, 0x03, 0x00, 0x00, 0x01, 0x00}

	sc, err := DecodeSensorConfigForGeneration(mem880, mem884, mem8C0, true)
	if err != nil {
		t.Fatalf("DecodeSensorConfigForGeneration: %v", err)
	}
	if sc.PollingRateHz != 1000 {
		t.Fatalf("PollingRateHz = %d, want 1000", sc.PollingRateHz)
	}
	if sc.LiftOffLevel != 1 || sc.LiftOffDistanceMm != 1.1 {
		t.Fatalf("LOD = lvl %d (%.1fmm), want lvl 1 (1.1mm)", sc.LiftOffLevel, sc.LiftOffDistanceMm)
	}
	if sc.MotionSync {
		t.Fatalf("MotionSync = true, want false")
	}
	if sc.PressDebounceStep != 3 || sc.PressDebounceLabel != "4 ms" ||
		sc.ReleaseDebounceStep != 3 || sc.ReleaseDebounceLabel != "4 ms" {
		t.Fatalf("unexpected Gen-A debounce: press=%d(%s) release=%d(%s)",
			sc.PressDebounceStep, sc.PressDebounceLabel, sc.ReleaseDebounceStep, sc.ReleaseDebounceLabel)
	}
	if sc.SleepMinutes != 10 || sc.RippleControl || sc.LinearCorrection || sc.SensorAngleDeg != 0 || sc.GlassMode {
		t.Fatalf("unexpected sensor flags: %+v", sc)
	}

	// Exact 56-byte dump of 0x0600..0x0637 from live RTX5050 mouse:
	mem600 := []byte{
		0x03, 0x00, 0x01, 0x00, // 0x0600: Key 0 Left
		0x03, 0x00, 0x04, 0x00, // 0x0604: Key 2 Middle
		0x03, 0x00, 0x02, 0x00, // 0x0608: Key 1 Right
		0x07, 0x00, 0x00, 0x00, // 0x060C: Key 5 DPI+ (Disabled)
		0x07, 0x00, 0x00, 0x00, // 0x0610: Key 6 DPI- (Disabled)
		0x03, 0x00, 0x10, 0x00, // 0x0614: Key 7 Forward
		0x03, 0x00, 0x08, 0x00, // 0x0618: Key 8 Back
		0x07, 0x00, 0x08, 0x00, // 0x061C: unused
		0x07, 0x00, 0x08, 0x00, // 0x0620: unused
		0xFF, 0xFF, 0xFF, 0xFF, // 0x0624: Key 11 Wheel Up
		0xFF, 0xFF, 0xFF, 0xFF, // 0x0628: Key 12 Wheel Down
		0xFF, 0xFF, 0xFF, 0xFF, // 0x062C: unused
		0xFF, 0xFF, 0xFF, 0xFF, // 0x0630: unused
		0x07, 0x00, 0x00, 0x00, // 0x0634: Key 13 Bottom DIY (Disabled)
	}
	btns, err := DecodeButtonTable(mem600)
	if err != nil {
		t.Fatalf("DecodeButtonTable: %v", err)
	}
	if len(btns) != 10 {
		t.Fatalf("len(btns) = %d, want 10", len(btns))
	}

	expected := map[ButtonID]ButtonActionType{
		ButtonLeft:      ActionLeftClick,
		ButtonRight:     ActionRightClick,
		ButtonMiddle:    ActionMiddleClick,
		ButtonDPIPlus:   ActionDisabled,
		ButtonDPIMinus:  ActionDisabled,
		ButtonForward:   ActionForward,
		ButtonBack:      ActionBack,
		ButtonWheelUp:   ActionWheelDefault,
		ButtonWheelDown: ActionWheelDefault,
		ButtonBottomDIY: ActionDisabled,
	}
	for _, b := range btns {
		if want := expected[b.Slot.ID]; b.Action.Type != want {
			t.Fatalf("button %d (%s): got %s, want %s", b.Slot.ID, b.Slot.NameEN, b.Action.Type, want)
		}
	}
}

func TestKeyboardShortcutAndLeftClickGuard(t *testing.T) {
	act := ButtonAction{
		Type:      ActionKeyboard,
		Modifiers: KeyModifiers{Ctrl: true, Shift: true},
		KeyName:   "R",
	}
	raw, err := EncodeButtonAction(act)
	if err != nil {
		t.Fatalf("EncodeButtonAction: %v", err)
	}
	if raw != [4]byte{0x02, 0x00, 0x03, 0x15} {
		t.Fatalf("raw = % x, want 02 00 03 15", raw)
	}
	slot, _ := LookupButtonSlot(ButtonForward)
	dec := DecodeButtonAction(slot, raw)
	if dec.Type != ActionKeyboard || !dec.Modifiers.Ctrl || !dec.Modifiers.Shift || dec.KeyName != "R" {
		t.Fatalf("unexpected decoded keyboard action: %+v", dec)
	}

	// Verify Left-Click safety guard rejects removing the only Left-Click
	defs := DefaultButtonMappings()
	if err := ValidateButtonMappings(defs); err != nil {
		t.Fatalf("default mappings should be valid: %v", err)
	}
	defs[0].Action.Type = ActionRightClick
	if err := ValidateButtonMappings(defs); !errors.Is(err, ErrMissingLeftClick) {
		t.Fatalf("expected ErrMissingLeftClick, got %v", err)
	}
}

func TestEncodeUnknownButtonActionPreservesRaw(t *testing.T) {
	raw := [4]byte{0xEE, 0x01, 0x02, 0x03}
	got, err := EncodeButtonAction(ButtonAction{Type: ActionUnknown, Raw: raw})
	if err != nil {
		t.Fatalf("EncodeButtonAction: %v", err)
	}
	if got != raw {
		t.Fatalf("raw = % x, want % x", got, raw)
	}

	defs := DefaultButtonMappings()
	defs[3].Action = ButtonAction{Type: ActionUnknown, Raw: raw}
	if err := ValidateButtonMappings(defs); err != nil {
		t.Fatalf("unknown macro should be preserved by validation: %v", err)
	}
}

func TestReadWriteMemoryFramingAndDPIEncoder(t *testing.T) {
	readTX, err := BuildReadMemory(TargetWirelessMouse, OpcodesGenA.ReadMemory, AddrDPIStagesX, 24)
	if err != nil {
		t.Fatalf("BuildReadMemory: %v", err)
	}
	if readTX[0] != 0xBA || readTX[1] != 0xA5 || readTX[2] != 0xA4 || readTX[3] != 24 {
		t.Fatalf("unexpected read header: % x", readTX[:8])
	}
	if gotAddr := binary.LittleEndian.Uint32(readTX[4:8]); gotAddr != AddrDPIStagesX {
		t.Fatalf("addr = %#x, want %#x", gotAddr, AddrDPIStagesX)
	}

	stages := []DPIStage{
		{Index: 1, X: 400, Y: 400},
		{Index: 2, X: 800, Y: 750},
		{Index: 3, X: 1624, Y: 1600}, // Should snap 1624 -> 1600
	}
	bx, by, err := EncodeDPIBlocks(stages, 3)
	if err != nil {
		t.Fatalf("EncodeDPIBlocks: %v", err)
	}
	if bx[14] != 2 || by[14] != 2 {
		t.Fatalf("stageCount byte = %d/%d, want 2", bx[14], by[14])
	}
	mem888 := make([]byte, 24)
	copy(mem888, bx[:])
	mem888[16] = 1 // Stage 2 active
	dpiCfg, err := DecodeDPIConfig(mem888, by[:])
	if err != nil {
		t.Fatalf("DecodeDPIConfig: %v", err)
	}
	if dpiCfg.StageCount != 3 || dpiCfg.CurrentStage != 2 || !dpiCfg.IndependentXY ||
		dpiCfg.Stages[1].X != 800 || dpiCfg.Stages[1].Y != 750 || dpiCfg.Stages[2].X != 1600 {
		t.Fatalf("unexpected decoded DPIConfig: %+v", dpiCfg)
	}
}

func TestMatchEndpointsSkipsMI00AndPrefersWiredAndCol06ForGenA(t *testing.T) {
	infos := []hid.DeviceInfo{
		{Path: "mi00_mouse", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 0, UsagePage: 0x0001, Usage: 0x0002},
		{Path: "dongle_col04_cmd", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x000E},
		{Path: "dongle_col05_other", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 7},
		{Path: "dongle_col06_report", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 10},
		{Path: "wired_col04_cmd", VendorID: 0x24AE, ProductID: 0x4411, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x000E},
	}

	pairs := MatchEndpoints(infos)
	if len(pairs) != 2 {
		t.Fatalf("len(pairs) = %d, want 2", len(pairs))
	}
	if pairs[0].Model.ProductID != 0x4411 {
		t.Fatalf("expected wired 0x4411 first, got %#x", pairs[0].Model.ProductID)
	}
	if pairs[1].Report == nil || pairs[1].Report.Path != "dongle_col06_report" {
		t.Fatalf("expected dongle_col06_report, got %+v", pairs[1].Report)
	}
}

func TestValidateButtonMappingsRejectsMalformedTables(t *testing.T) {
	defs := DefaultButtonMappings()
	if err := ValidateButtonMappings(defs); err != nil {
		t.Fatalf("default mappings: %v", err)
	}

	duplicate := append([]ButtonMapping(nil), defs...)
	duplicate[1].Slot.ID = duplicate[0].Slot.ID
	if err := ValidateButtonMappings(duplicate); err == nil {
		t.Fatal("expected duplicate button ID to be rejected")
	}

	unknown := append([]ButtonMapping(nil), defs...)
	unknown[1].Slot.ID = ButtonID(250)
	if err := ValidateButtonMappings(unknown); err == nil {
		t.Fatal("expected unknown button ID to be rejected")
	}

	if err := ValidateButtonMappings(defs[:9]); err == nil {
		t.Fatal("expected incomplete table to be rejected")
	}

	badAction := append([]ButtonMapping(nil), defs...)
	badAction[1].Action = ButtonAction{Type: ButtonActionType("not-a-real-action")}
	if err := ValidateButtonMappings(badAction); err == nil {
		t.Fatal("expected unsupported action to be rejected")
	}
}

func TestMatchEndpointsKeepsPhysicalDevicesSeparate(t *testing.T) {
	infos := []hid.DeviceInfo{
		{Path: "a_cmd", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x000E, PhysicalID: "usb-a"},
		{Path: "a_report", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 10, PhysicalID: "usb-a"},
		{Path: "b_cmd", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x000E, PhysicalID: "usb-b"},
		{Path: "b_report", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 10, PhysicalID: "usb-b"},
	}
	pairs := MatchEndpoints(infos)
	if len(pairs) != 2 {
		t.Fatalf("len(pairs) = %d, want 2", len(pairs))
	}
	for _, pair := range pairs {
		if pair.Command == nil || pair.Report == nil {
			t.Fatalf("pair missing command/report: %+v", pair)
		}
		if pair.Command.PhysicalID != pair.Report.PhysicalID {
			t.Fatalf("mixed physical devices: %q vs %q", pair.Command.PhysicalID, pair.Report.PhysicalID)
		}
	}
}

func TestMatchEndpointsMarksMissingPhysicalIDAmbiguous(t *testing.T) {
	infos := []hid.DeviceInfo{
		{Path: "cmd-a", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x000E},
		{Path: "rep-a", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 10},
		{Path: "rep-b", VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 10},
	}
	pairs := MatchEndpoints(infos)
	if len(pairs) != 1 || !pairs[0].Ambiguous {
		t.Fatalf("pairs = %+v, want one ambiguous pair", pairs)
	}
}

func TestMatchEndpointsRejectsWindowsCommandAndReportFromDifferentDevices(t *testing.T) {
	infos := []hid.DeviceInfo{
		{Path: `\\?\hid#vid_24ae&pid_1211&mi_01&col04#7&aaaa&0&0004#{guid}`, VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x000E},
		{Path: `\\?\hid#vid_24ae&pid_1211&mi_01&col06#7&bbbb&0&0006#{guid}`, VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 10},
	}
	pairs := MatchEndpoints(infos)
	if len(pairs) != 1 || !pairs[0].Ambiguous {
		t.Fatalf("pairs = %+v, want one ambiguous pair", pairs)
	}
}

func TestMatchEndpointsKeepsSameWindowsDeviceCollectionsTogether(t *testing.T) {
	infos := []hid.DeviceInfo{
		{Path: `\\?\hid#vid_24ae&pid_1211&mi_01&col04#7&aaaa&0&0004#{guid}`, VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x000E},
		{Path: `\\?\hid#vid_24ae&pid_1211&mi_01&col06#7&aaaa&0&0006#{guid}`, VendorID: 0x24AE, ProductID: 0x1211, InterfaceNumber: 1, UsagePage: 0xFF00, Usage: 0x0002, InputReportLen: 10},
	}
	pairs := MatchEndpoints(infos)
	if len(pairs) != 1 || pairs[0].Ambiguous || pairs[0].Command == nil || pairs[0].Report == nil {
		t.Fatalf("pairs = %+v, want one paired non-ambiguous device", pairs)
	}
}
