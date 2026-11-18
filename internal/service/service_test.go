package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
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
		Path:            "rep_col05",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x0002,
		InputReportLen:  32,
	})

	var sleeping atomic.Bool
	cmdDev.OnGetInput = func(reportID byte, length int) ([]byte, error) {
		buf := make([]byte, length)
		if sleeping.Load() {
			buf[0] = 0x02 // Mouse asleep
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
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	waitForState(t, svc, func(st State) bool {
		return st.Connected && st.HasBattery && st.Percentage == 78 && !st.Sleeping
	})

	// Put mouse to sleep and force refresh: should keep Percentage == 78 and mark Sleeping == true!
	sleeping.Store(true)
	svc.RefreshNow()
	waitForState(t, svc, func(st State) bool {
		return st.Connected && st.HasBattery && st.Percentage == 78 && st.Sleeping && !st.MouseOnline
	})

	// Switch to PassiveOnly mode and push an async 0xBB wakeup with 81% charging
	svc.SetPollingMode(ModePassiveOnly)
	time.Sleep(220 * time.Millisecond) // allow passive reopen
	repDev.PushAsyncPacket([]byte{0xBB, 0x00, 0x00, 0x01, 0x00, 0x02, 81, 0x00})

	waitForState(t, svc, func(st State) bool {
		return st.Mode == ModePassiveOnly && st.Percentage == 81 && st.Charging && !st.Sleeping
	})
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
