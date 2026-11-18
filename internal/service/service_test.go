package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

func TestMouseServicePreservesBatteryDuringWirelessSleepAndPassiveMode(t *testing.T) {
	prov := hid.NewMockProvider()
	cmdDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            "cmd_col04",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		InputReportLen:  32,
		OutputReportLen: 32,
	})
	repDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            "rep_col06",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x0002,
		InputReportLen:  10,
	})

	var sleeping atomic.Bool
	cmdDev.OnGetInput = func(reportID byte, length int) ([]byte, error) {
		buf := make([]byte, length)
		if sleeping.Load() {
			buf[0] = 0x02
			return buf, nil
		}
		buf[0] = 0x01
		buf[1] = 0x01
		buf[2] = 78
		return buf, nil
	}

	prov.Register(cmdDev)
	prov.Register(repDev)

	svc := NewMouseService(prov, Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		PresetDir:         t.TempDir(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	waitForState(t, svc, func(st State) bool {
		return st.Connected && st.HasBattery && st.Percentage == 78 && !st.Sleeping
	})

	sleeping.Store(true)
	svc.RefreshNow()
	waitForState(t, svc, func(st State) bool {
		return st.Connected && st.HasBattery && st.Percentage == 78 && st.Sleeping && !st.MouseOnline
	})

	svc.SetPollingMode(ModePassiveOnly)
	time.Sleep(220 * time.Millisecond)
	repDev.PushAsyncPacket([]byte{0xBB, 0x00, 0x00, 0x01, 0x00, 0x02, 81, 0x00})

	waitForState(t, svc, func(st State) bool {
		return st.Mode == ModePassiveOnly && st.Percentage == 81 && st.Charging && !st.Sleeping
	})

	// Verify gaming lock rejects writes while in PassiveOnly mode
	if _, err := svc.SetPollingRate(4000); err == nil {
		t.Fatal("expected error mutating polling rate while in ModePassiveOnly")
	}
}

func TestMouseServiceDemoModeAndPresets(t *testing.T) {
	emptyProv := hid.NewMockProvider()
	svc := NewMouseService(emptyProv, Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		PresetDir:         t.TempDir(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	snap, err := svc.SetDemoMode(true)
	if err != nil {
		t.Fatalf("SetDemoMode(true): %v", err)
	}
	if !snap.DemoMode || snap.Battery.Percentage != 82 || len(snap.Buttons) != 10 {
		t.Fatalf("unexpected demo snapshot: %+v", snap)
	}

	// Save the demo snapshot, change a setting, then verify ApplyPreset restores it.
	saved, err := svc.SavePreset("", "Test preset", "")
	if err != nil {
		t.Fatalf("SavePreset: %v", err)
	}
	if _, err := svc.SetPollingRate(4000); err != nil {
		t.Fatalf("SetPollingRate: %v", err)
	}
	applied, err := svc.ApplyPreset(saved.ID)
	if err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if applied.Sensor.PollingRateHz != 1000 || applied.DPI.StageCount != 7 || applied.DPI.Stages[1].X != 800 {
		t.Fatalf("unexpected state after ApplyPreset: sensor=%+v dpi=%+v", applied.Sensor, applied.DPI)
	}

	// Verify Left-Click safety guard via service
	_, err = svc.SetButtonMapping(int(rapoo.ButtonLeft), rapoo.ButtonAction{Type: rapoo.ActionDisabled})
	if err == nil {
		t.Fatal("expected Japanese Left-Click guard error")
	}
}

func waitForState(t *testing.T, svc *MouseService, pred func(State) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pred(svc.GetState()) {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("condition not met; latest state = %+v", svc.GetState())
}
