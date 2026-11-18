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

func TestParseAsyncReport(t *testing.T) {
	now := time.Unix(1700000000, 0)

	// 0xBB passive battery report: raw[3]=1 (online), raw[5]=2 (charging), raw[6]=64%
	pktBB := []byte{0xBB, 0x00, 0x00, 0x01, 0x03, 0x02, 64, 0x00}
	ev, ok := ParseAsyncReport(pktBB, now)
	if !ok || ev.Kind != EventBattery || ev.Battery == nil {
		t.Fatalf("expected battery event, got ok=%v ev=%+v", ok, ev)
	}
	if ev.Battery.Percentage != 64 || !ev.Battery.Charging || !ev.Battery.Online {
		t.Fatalf("unexpected async battery: %+v", ev.Battery)
	}

	// 0x06 COL06 wake/status notification on Gen-A
	pkt06 := []byte{0x06, 0x02, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	evWake, ok := ParseAsyncReport(pkt06, now)
	if !ok || evWake.Kind != EventWake {
		t.Fatalf("expected wake event, got ok=%v ev=%+v", ok, evWake)
	}

	// 0xBD profile switch notification: high nibble of raw[2]
	pktBD := []byte{0xBD, 0x01, 0x20, 0x00}
	ev2, ok := ParseAsyncReport(pktBD, now)
	if !ok || ev2.Kind != EventProfile || ev2.ProfileID != 2 {
		t.Fatalf("expected profile 2 event, got ok=%v ev=%+v", ok, ev2)
	}
}

func TestReadWriteMemoryFramingAndDecoders(t *testing.T) {
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

	writeTX, err := BuildWriteMemory(TargetLocalOrWired, OpcodesGenA.WriteMemory, AddrPollingRate, []byte{0x82, 0x00, 0x82, 0x01})
	if err != nil {
		t.Fatalf("BuildWriteMemory: %v", err)
	}
	if writeTX[0] != 0xBA || writeTX[1] != 0xFF || writeTX[2] != 0xA5 || writeTX[3] != 4 {
		t.Fatalf("unexpected write header: % x", writeTX[:12])
	}
	if string(writeTX[8:12]) != string([]byte{0x82, 0x00, 0x82, 0x01}) {
		t.Fatalf("unexpected write data: % x", writeTX[8:12])
	}

	mem888 := make([]byte, 24)
	dpis := []uint16{400, 800, 1200, 1600, 3200, 6400, 1000}
	for i, v := range dpis {
		binary.LittleEndian.PutUint16(mem888[i*2:i*2+2], v)
	}
	mem888[14] = 6
	mem888[16] = 6

	dpiCfg, err := DecodeDPIConfig(mem888, nil)
	if err != nil {
		t.Fatalf("DecodeDPIConfig: %v", err)
	}
	if dpiCfg.StageCount != 7 || dpiCfg.CurrentStage != 7 || dpiCfg.Stages[6].X != 1000 {
		t.Fatalf("unexpected DPIConfig: %+v", dpiCfg)
	}

	mem880 := []byte{0x82, 0x00, 0x82, 0x00}
	mem884 := []byte{0x01, 0x00, 0x00, 0x00}
	mem8C0 := []byte{5, 5, 10, 0x03, 0, 0, 0, 0}
	sc, err := DecodeSensorConfig(mem880, mem884, mem8C0)
	if err != nil {
		t.Fatalf("DecodeSensorConfig: %v", err)
	}
	if sc.PollingRateHz != 4000 || sc.SilentHeightLOD != 1 || sc.MotionSync ||
		sc.PressDebounceMs != 3 || sc.ReleaseDebounceMs != 3 || sc.SleepMinutes != 10 ||
		sc.RippleControl || sc.LinearCorrection {
		t.Fatalf("unexpected SensorConfig: %+v", sc)
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
