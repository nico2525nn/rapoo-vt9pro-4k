package rapoo

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
)

func TestClientBusyPollingAndAsyncNotifications(t *testing.T) {
	prov := hid.NewMockProvider()

	cmdDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            `\\?\hid#vid_24ae&pid_1211&mi_01&col04`,
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		InputReportLen:  32,
		OutputReportLen: 32,
	})
	repDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            `\\?\hid#vid_24ae&pid_1211&mi_01&col06`,
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x0002,
		InputReportLen:  10,
	})

	var polls atomic.Int32
	cmdDev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		n := polls.Add(1)
		buf := make([]byte, length)
		if n < 3 {
			buf[0] = 0x02
			return buf, nil
		}
		buf[0] = 0x01
		buf[1] = 0x01
		buf[2] = 92
		return buf, nil
	})

	prov.Register(cmdDev)
	prov.Register(repDev)

	asyncCh := make(chan AsyncEvent, 4)
	client, err := OpenFirst(prov, ClientOptions{
		BusyPollInterval: time.Millisecond,
		MaxBusyPolls:     10,
		OnAsyncEvent: func(ev AsyncEvent) {
			asyncCh <- ev
		},
	})
	if err != nil {
		t.Fatalf("OpenFirst: %v", err)
	}
	defer client.Close()

	bat, err := client.ReadBattery()
	if err != nil {
		t.Fatalf("ReadBattery: %v", err)
	}
	if bat.Percentage != 92 || bat.Charging || !bat.Online {
		t.Fatalf("unexpected battery: %+v", bat)
	}
	if polls.Load() != 3 {
		t.Fatalf("polls = %d, want 3", polls.Load())
	}

	repDev.PushAsyncPacket([]byte{0xBB, 0x00, 0x00, 0x01, 0x00, 0x02, 95, 0x00})
	select {
	case ev := <-asyncCh:
		if ev.Battery == nil || ev.Battery.Percentage != 95 || !ev.Battery.Charging {
			t.Fatalf("unexpected async event: %+v", ev.Battery)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async 0xBB event")
	}
}

func TestVirtualDeviceFullReadModifyWriteRoundTrip(t *testing.T) {
	vd := NewVirtualDevice()
	pairs := MatchEndpoints([]hid.DeviceInfo{vd.CmdDev.DeviceInfo, vd.RepDev.DeviceInfo})
	if len(pairs) != 1 {
		t.Fatalf("len(pairs) = %d, want 1", len(pairs))
	}
	// Speed up RF step delay in unit tests
	pairs[0].Model.StepDelay = time.Millisecond

	client, err := OpenEndpoint(vd.Provider, pairs[0], ClientOptions{
		BusyPollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("OpenEndpoint: %v", err)
	}
	defer client.Close()

	// 1. Update Polling Rate to 4000 Hz
	if err := client.SetPollingRate(4000); err != nil {
		t.Fatalf("SetPollingRate(4000): %v", err)
	}

	// 2. Update DPI X/Y stages (independent X=800, Y=700 on Stage 1, 3 stages active)
	err = client.SetDPIConfig(DPIConfig{
		CurrentStage: 1,
		StageCount:   3,
		AllStages: []DPIStage{
			{Index: 1, X: 800, Y: 700},
			{Index: 2, X: 1600, Y: 1600},
			{Index: 3, X: 3200, Y: 3200},
			{Index: 4, X: 4800, Y: 4800},
			{Index: 5, X: 6400, Y: 6400},
			{Index: 6, X: 12800, Y: 12800},
			{Index: 7, X: 26000, Y: 26000},
		},
	})
	if err != nil {
		t.Fatalf("SetDPIConfig: %v", err)
	}

	// 3. Update Sensor Parameters (LOD 1.5mm = level 5, MotionSync OFF, Debounce 0.5ms = step 0, Angle -12°, Glass ON, Ripple ON, Linear ON, Sleep 30m)
	lod := 5
	ms := false
	deb := 0
	sleep := 30
	rip := true
	lin := true
	ang := -12
	glass := true
	err = client.UpdateSensorConfig(SensorPatch{
		LiftOffLevel:        &lod,
		MotionSync:          &ms,
		PressDebounceStep:   &deb,
		ReleaseDebounceStep: &deb,
		SleepMinutes:        &sleep,
		RippleControl:       &rip,
		LinearCorrection:    &lin,
		SensorAngleDeg:      &ang,
		GlassMode:           &glass,
	})
	if err != nil {
		t.Fatalf("UpdateSensorConfig: %v", err)
	}

	// 4. Button safety guard: cannot disable Left Click while no other button has Left Click
	err = client.SetButtonMapping(ButtonLeft, ButtonAction{Type: ActionDisabled})
	if !errors.Is(err, ErrMissingLeftClick) {
		t.Fatalf("expected ErrMissingLeftClick when disabling Left Click, got %v", err)
	}

	// 5. Remap Bottom DIY button to Ctrl+C and DPI+ to Sniper DPI
	err = client.SetButtonMapping(ButtonBottomDIY, ButtonAction{
		Type:      ActionKeyboard,
		Modifiers: KeyModifiers{Ctrl: true},
		KeyName:   "C",
	})
	if err != nil {
		t.Fatalf("SetButtonMapping(BottomDIY): %v", err)
	}
	if err := client.SetButtonMapping(ButtonDPIPlus, ButtonAction{Type: ActionSniperDPI}); err != nil {
		t.Fatalf("SetButtonMapping(DPIPlus): %v", err)
	}

	// 6. Verify full ReadSnapshot matches every single change
	snap, err := client.ReadSnapshot()
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	if snap.Sensor.PollingRateHz != 4000 || snap.Sensor.LiftOffDistanceMm != 1.5 ||
		snap.Sensor.MotionSync || snap.Sensor.PressDebounceLabel != "0.5 ms" ||
		snap.Sensor.SensorAngleDeg != -12 || !snap.Sensor.GlassMode ||
		!snap.Sensor.RippleControl || !snap.Sensor.LinearCorrection ||
		snap.Sensor.SleepMinutes != 30 {
		t.Fatalf("unexpected Sensor snapshot: %+v", snap.Sensor)
	}
	if snap.DPI.StageCount != 3 || snap.DPI.CurrentStage != 1 || !snap.DPI.IndependentXY ||
		snap.DPI.Stages[0].X != 800 || snap.DPI.Stages[0].Y != 700 {
		t.Fatalf("unexpected DPI snapshot: %+v", snap.DPI)
	}

	foundDIY := false
	for _, b := range snap.Buttons {
		if b.Slot.ID == ButtonBottomDIY {
			foundDIY = true
			if b.Action.Type != ActionKeyboard || !b.Action.Modifiers.Ctrl || b.Action.KeyName != "C" {
				t.Fatalf("unexpected DIY button action: %+v", b.Action)
			}
		}
	}
	if !foundDIY {
		t.Fatal("BottomDIY button missing from snapshot")
	}

	// 7. FactoryReset restores defaults
	if err := client.FactoryReset(); err != nil {
		t.Fatalf("FactoryReset: %v", err)
	}
	afterReset, err := client.ReadSnapshot()
	if err != nil {
		t.Fatalf("ReadSnapshot after reset: %v", err)
	}
	if afterReset.Sensor.PollingRateHz != 1000 || afterReset.DPI.StageCount != 7 || afterReset.DPI.CurrentStage != 4 {
		t.Fatalf("unexpected state after FactoryReset: sensor=%+v dpi=%+v", afterReset.Sensor, afterReset.DPI)
	}
}

func TestDeadlineAfterSendReturnsResultUnknown(t *testing.T) {
	dev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            "cmd",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		OutputReportLen: 32,
	})
	client := &Client{
		endpoint: EndpointPair{Model: ModelSpec{StepDelay: 40 * time.Millisecond}},
		cmdDev:   dev,
	}
	client.opts.applyDefaults()
	_, err := client.sendAndRecvDeadline([]byte{0xBA, 0xA5, 0xA0, 4, 0, 0, 0, 0}, time.Now().Add(10*time.Millisecond))
	if !errors.Is(err, ErrResultUnknown) {
		t.Fatalf("error = %v, want ErrResultUnknown", err)
	}
}

func TestLostReplyAfterSendReturnsResultUnknownWithoutDuplicateWrite(t *testing.T) {
	dev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            "cmd",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		OutputReportLen: 32,
	})
	dev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		return nil, errors.New("reply lost")
	})
	client := &Client{
		endpoint: EndpointPair{Model: ModelSpec{StepDelay: time.Millisecond}},
		cmdDev:   dev,
	}
	client.opts.applyDefaults()
	_, err := client.sendAndRecvDeadline([]byte{0xBA, 0xA5, 0xA0, 4, 0, 0, 0, 0}, time.Time{})
	if !errors.Is(err, ErrResultUnknown) {
		t.Fatalf("error = %v, want ErrResultUnknown", err)
	}
	if len(dev.Written) != 1 {
		t.Fatalf("written reports = %d, want 1 (no duplicate write)", len(dev.Written))
	}
}

func TestMalformedReplyAfterWriteIsNotRetried(t *testing.T) {
	dev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            "cmd",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		OutputReportLen: 32,
	})
	dev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		return []byte{0xBA}, nil // report ID only, no normalizable response
	})
	client := &Client{
		endpoint: EndpointPair{Model: ModelSpec{StepDelay: time.Millisecond}},
		cmdDev:   dev,
	}
	client.opts.applyDefaults()
	_, err := client.sendAndRecvDeadline([]byte{0xBA, 0xA5, 0xA0, 4, 0, 0, 0, 0}, time.Time{})
	if !errors.Is(err, ErrResultUnknown) {
		t.Fatalf("error = %v, want ErrResultUnknown", err)
	}
	if len(dev.Written) != 1 {
		t.Fatalf("written reports = %d, want 1", len(dev.Written))
	}
}

func TestPendingAfterWriteReturnsResultUnknown(t *testing.T) {
	dev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            "cmd",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		OutputReportLen: 32,
	})
	dev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		return []byte{0xBA, StatusPending}, nil
	})
	client := &Client{
		endpoint: EndpointPair{Model: ModelSpec{StepDelay: time.Millisecond}},
		cmdDev:   dev,
	}
	client.opts.applyDefaults()
	client.opts.BusyPollInterval = time.Millisecond
	_, err := client.sendAndRecvDeadline([]byte{0xBA, 0xA5, 0xA0, 4, 0, 0, 0, 0}, time.Time{})
	if !errors.Is(err, ErrResultUnknown) {
		t.Fatalf("error = %v, want ErrResultUnknown", err)
	}
	if len(dev.Written) != 1 {
		t.Fatalf("written reports = %d, want 1", len(dev.Written))
	}
}
