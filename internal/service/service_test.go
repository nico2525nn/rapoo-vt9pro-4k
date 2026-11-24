package service

import (
	"context"
	"encoding/binary"
	"errors"
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
	cmdDev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		buf := make([]byte, length)
		if sleeping.Load() {
			buf[0] = 0x02
			return buf, nil
		}
		buf[0] = 0x01
		buf[1] = 0x01
		buf[2] = 78
		return buf, nil
	})

	prov.Register(cmdDev)
	prov.Register(repDev)

	svc := NewMouseService(prov, Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
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
		SleepGrace:        time.Nanosecond,
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

func TestStartupDoesNotWaitForRealHid(t *testing.T) {
	prov := hid.NewMockProvider()
	cmd := hid.NewMockDevice(hid.DeviceInfo{
		Path:            "cmd",
		VendorID:        0x24AE,
		ProductID:       0x1211,
		InterfaceNumber: 1,
		UsagePage:       0xFF00,
		Usage:           0x000E,
		OutputReportLen: 32,
	})
	cmd.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		time.Sleep(300 * time.Millisecond)
		return nil, errors.New("slow response")
	})
	prov.Register(cmd)
	svc := NewMouseService(prov, Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	started := time.Now()
	svc.Startup(ctx)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		cancel()
		svc.Shutdown()
		t.Fatalf("Startup blocked for %v on real HID", elapsed)
	}
	cancel()
	svc.Shutdown()
}

func TestEnsureDemoReadyBeforeSnapshot(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
		StartInDemo:       true,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()
	svc.EnsureDemoReady()

	snap, err := svc.ReadSnapshot()
	if err != nil {
		t.Fatalf("ReadSnapshot after EnsureDemoReady: %v", err)
	}
	if !snap.DemoMode || snap.DPI == nil || snap.Sensor == nil || len(snap.Buttons) != 10 {
		t.Fatalf("incomplete demo snapshot: %+v", snap)
	}
}

func TestMouseServiceWriteGuardRejectsStaleGenerationAndDeadline(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	generation := svc.GetState().Generation
	if generation == 0 {
		t.Fatal("demo generation is zero")
	}

	if _, err := svc.SetPollingRateForGeneration(4000, generation+1, time.Time{}); !errors.Is(err, ErrDeviceChanged) {
		t.Fatalf("stale generation error = %v, want ErrDeviceChanged", err)
	}
	if _, err := svc.SetPollingRateForGeneration(4000, generation, time.Now().Add(-time.Second)); !errors.Is(err, ErrDeadlineExceeded) {
		t.Fatalf("expired deadline error = %v, want ErrDeadlineExceeded", err)
	}
	if _, err := svc.SetPollingRateForGeneration(4000, generation, time.Time{}); err != nil {
		t.Fatalf("current generation write: %v", err)
	}
}

func TestDPIPartialErrorPreservesAppliedSteps(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	before, err := svc.ReadSnapshot()
	if err != nil || before.DPI == nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}

	original := svc.demoDevice.CmdDev.OnSetOutput
	svc.demoDevice.CmdDev.SetOnSetOutput(func(data []byte) error {
		if len(data) >= 8 && binary.LittleEndian.Uint32(data[4:8]) == rapoo.AddrDPIStagesY {
			return errors.New("injected Y write failure")
		}
		return original(data)
	})

	cfg := *before.DPI
	cfg.Stages[0].X = 450
	_, err = svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{})
	var partial *PartialWriteError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %v (%T), want PartialWriteError", err, err)
	}
	foundX := false
	for _, step := range partial.Applied {
		if step == "dpiX" {
			foundX = true
		}
	}
	if !foundX {
		t.Fatalf("applied steps = %v, want dpiX", partial.Applied)
	}
}

func TestPresetRestoresUnknownOnboardMacro(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	snap, err := svc.ReadSnapshot()
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	var macroRaw = [4]byte{0xEE, 0x10, 0x20, 0x30}
	var macroSlot rapoo.ButtonID = rapoo.ButtonForward
	for i := range snap.Buttons {
		if snap.Buttons[i].Slot.ID == macroSlot {
			snap.Buttons[i].Action = rapoo.ButtonAction{
				Type: rapoo.ActionUnknown,
				Raw:  macroRaw,
			}
		}
	}
	saved, err := svc.presetStore.SaveSnapshot("", "macro preset", "", snap)
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	if _, err := svc.SetButtonMappingForGeneration(
		int(macroSlot),
		rapoo.ButtonAction{Type: rapoo.ActionRightClick},
		svc.GetState().Generation,
		time.Time{}); err != nil {
		t.Fatalf("change button: %v", err)
	}
	if _, err := svc.ApplyPresetForGeneration(saved.ID, svc.GetState().Generation, time.Time{}); err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	after, err := svc.ReadSnapshot()
	if err != nil {
		t.Fatalf("ReadSnapshot after apply: %v", err)
	}
	for _, mapping := range after.Buttons {
		if mapping.Slot.ID != macroSlot {
			continue
		}
		if mapping.Action.Type != rapoo.ActionUnknown || mapping.Action.Raw != macroRaw {
			t.Fatalf("macro not restored: %+v", mapping.Action)
		}
		return
	}
	t.Fatal("macro slot missing after apply")
}

func TestHigherPriorityEndpointKeepsCurrentEqualDevice(t *testing.T) {
	newProvider := func(physical string) *hid.MockProvider {
		prov := hid.NewMockProvider()
		prov.Register(hid.NewMockDevice(hid.DeviceInfo{
			Path:            "cmd-" + physical,
			VendorID:        0x24AE,
			ProductID:       0x1211,
			InterfaceNumber: 1,
			UsagePage:       0xFF00,
			Usage:           0x000E,
			OutputReportLen: 32,
			PhysicalID:      physical,
		}))
		prov.Register(hid.NewMockDevice(hid.DeviceInfo{
			Path:            "rep-" + physical,
			VendorID:        0x24AE,
			ProductID:       0x1211,
			InterfaceNumber: 1,
			UsagePage:       0xFF00,
			Usage:           0x0002,
			InputReportLen:  10,
			PhysicalID:      physical,
		}))
		return prov
	}

	provA := newProvider("container:a")
	provB := newProvider("container:b")
	// The service normally sees both devices through one provider. Build a
	// combined view for the endpoint lookup.
	combined := hid.NewMockProvider()
	for _, prov := range []*hid.MockProvider{provA, provB} {
		infos, err := prov.Enumerate(rapoo.DefaultHIDFilter())
		if err != nil {
			t.Fatalf("enumerate: %v", err)
		}
		for _, info := range infos {
			dev, err := prov.Open(info, false)
			if err != nil {
				t.Fatalf("open mock: %v", err)
			}
			_ = dev.Close()
			combined.Register(hid.NewMockDevice(info))
		}
	}
	infos, err := combined.Enumerate(rapoo.DefaultHIDFilter())
	if err != nil {
		t.Fatalf("combined enumerate: %v", err)
	}
	pairs := rapoo.MatchEndpoints(infos)
	var pairA rapoo.EndpointPair
	for _, pair := range pairs {
		if pair.Command != nil && pair.Command.PhysicalID == "container:a" {
			pairA = pair
		}
	}
	if pairA.Command == nil || pairA.Report == nil {
		t.Fatalf("pair A not found: %+v", pairs)
	}
	client, err := rapoo.OpenEndpoint(combined, pairA, rapoo.ClientOptions{})
	if err != nil {
		t.Fatalf("open pair A: %v", err)
	}
	defer client.Close()

	svc := NewMouseService(combined, Config{PresetDir: t.TempDir()})
	svc.client = client
	svc.state.Connected = true
	svc.state.Generation = 1
	if svc.higherPriorityEndpointAvailable() {
		t.Fatal("current equal-priority endpoint should not be replaced")
	}

	svc.realProvider = provB
	if !svc.higherPriorityEndpointAvailable() {
		t.Fatal("missing current endpoint should trigger reconnect")
	}
}

func TestDeadlineIsCheckedAfterReadBeforeWrite(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	original := svc.demoDevice.CmdDev.OnGetInput
	svc.demoDevice.CmdDev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return original(reportID, length)
	})

	generation := svc.GetState().Generation
	_, err := svc.SetPollingRateForGeneration(4000, generation, time.Now().Add(10*time.Millisecond))
	if !errors.Is(err, ErrDeadlineExceeded) {
		t.Fatalf("error = %v, want ErrDeadlineExceeded", err)
	}
}

func TestProfileApplyKeepsPriorStepsWhenNextStepFails(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	snap, err := svc.ReadSnapshot()
	if err != nil || snap.Sensor == nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	snap.Sensor.PollingRateHz = 2000
	saved, err := svc.presetStore.SaveSnapshot("", "rate then dpi fail", "", snap)
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	original := svc.demoDevice.CmdDev.OnSetOutput
	svc.demoDevice.CmdDev.SetOnSetOutput(func(data []byte) error {
		if len(data) >= 8 && binary.LittleEndian.Uint32(data[4:8]) == rapoo.AddrDPIStagesY {
			return errors.New("injected Y write failure")
		}
		return original(data)
	})

	_, err = svc.ApplyPresetForGeneration(saved.ID, svc.GetState().Generation, time.Time{})
	var partial *PartialWriteError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %v (%T), want PartialWriteError", err, err)
	}
	foundRate := false
	for _, step := range partial.Applied {
		if step == "pollingRate" {
			foundRate = true
		}
	}
	if !foundRate {
		t.Fatalf("applied steps = %v, want pollingRate", partial.Applied)
	}
}

func TestDPIReadbackNormalizesAndChecksAllStages(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	snap, err := svc.ReadSnapshot()
	if err != nil || snap.DPI == nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}

	// Normalization must not turn a valid write into a false partial: 451/0
	// is encoded as 450/450.
	cfg := *snap.DPI
	cfg.Stages[0].X = 451
	cfg.Stages[0].Y = 0
	cfg.AllStages = append([]rapoo.DPIStage(nil), snap.DPI.AllStages...)
	cfg.AllStages[0].X = 451
	cfg.AllStages[0].Y = 0
	if _, err := svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{}); err != nil {
		t.Fatalf("normalized DPI write: %v", err)
	}

	// The verifier must compare every encoded slot, including slots outside
	// StageCount. ACK a DPI write without changing memory and expect a partial.
	original := svc.demoDevice.CmdDev.OnSetOutput
	svc.demoDevice.CmdDev.SetOnSetOutput(func(data []byte) error {
		if len(data) >= 8 {
			addr := binary.LittleEndian.Uint32(data[4:8])
			if addr == rapoo.AddrDPIStagesX || addr == rapoo.AddrDPIStagesY {
				return nil
			}
		}
		return original(data)
	})
	cfg = *snap.DPI
	cfg.AllStages = append([]rapoo.DPIStage(nil), snap.DPI.AllStages...)
	cfg.AllStages[6].X = 500
	cfg.AllStages[6].Y = 500
	_, err = svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{})
	var partial *PartialWriteError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %v (%T), want PartialWriteError", err, err)
	}
}

func TestDPIConfigFailsBeforeWritingWhenHiddenStagesCannotBeRead(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()
	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}

	svc.demoDevice.CmdDev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		return nil, errors.New("hidden DPI read failure")
	})
	cfg := rapoo.DPIConfig{
		CurrentStage: 1,
		StageCount:   1,
		Stages:       []rapoo.DPIStage{{Index: 1, X: 450, Y: 450}},
	}
	if _, err := svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{}); err == nil {
		t.Fatal("expected hidden DPI read failure to fail before writing")
	}
}

func TestShortAllStagesDoesNotProduceFalsePartial(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	cfg := rapoo.DPIConfig{
		CurrentStage: 1,
		StageCount:   1,
		AllStages: []rapoo.DPIStage{
			{Index: 1, X: 450, Y: 450, Enabled: true},
		},
	}
	if _, err := svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{}); err != nil {
		t.Fatalf("short AllStages write: %v", err)
	}
}

func TestCachedSnapshotWithOldGenerationIsNotReturned(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{PresetDir: t.TempDir()})
	svc.connectMu.Lock()
	svc.cachedSnap = &rapoo.DeviceSnapshot{Generation: 1}
	svc.state.Generation = 2
	_, err := svc.ReadSnapshot()
	svc.connectMu.Unlock()
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("error = %v, want ErrBusy", err)
	}
}

func TestReadSnapshotReturnsBusyWithoutWaitingForTheConnectionLock(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{PresetDir: t.TempDir()})
	svc.connectMu.Lock()
	started := time.Now()
	_, err := svc.ReadSnapshot()
	svc.connectMu.Unlock()
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("error = %v, want ErrBusy", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("ReadSnapshot blocked for %v", elapsed)
	}
}

func TestShortAllStagesChecksExplicitExtraSlots(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()
	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	snap, err := svc.ReadSnapshot()
	if err != nil || snap.DPI == nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}

	original := svc.demoDevice.CmdDev.OnSetOutput
	svc.demoDevice.CmdDev.SetOnSetOutput(func(data []byte) error {
		if len(data) >= 8 {
			addr := binary.LittleEndian.Uint32(data[4:8])
			if addr == rapoo.AddrDPIStagesX || addr == rapoo.AddrDPIStagesY {
				return nil
			}
		}
		return original(data)
	})
	first := snap.DPI.AllStages[0]
	second := snap.DPI.AllStages[1]
	second.X = 500
	second.Y = 500
	cfg := rapoo.DPIConfig{
		CurrentStage: 1,
		StageCount:   1,
		AllStages:    []rapoo.DPIStage{first, second},
	}
	_, err = svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{})
	var partial *PartialWriteError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %v (%T), want PartialWriteError", err, err)
	}
}

func TestStagesWithSevenEntriesChecksAllSlots(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()
	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	snap, err := svc.ReadSnapshot()
	if err != nil || snap.DPI == nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	original := svc.demoDevice.CmdDev.OnSetOutput
	svc.demoDevice.CmdDev.SetOnSetOutput(func(data []byte) error {
		if len(data) >= 8 {
			addr := binary.LittleEndian.Uint32(data[4:8])
			if addr == rapoo.AddrDPIStagesX || addr == rapoo.AddrDPIStagesY {
				return nil
			}
		}
		return original(data)
	})
	stages := append([]rapoo.DPIStage(nil), snap.DPI.AllStages...)
	stages[6].X = 550
	stages[6].Y = 550
	cfg := rapoo.DPIConfig{CurrentStage: 1, StageCount: 1, Stages: stages}
	_, err = svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{})
	var partial *PartialWriteError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %v (%T), want PartialWriteError", err, err)
	}
}

func TestSavePresetReloadsSettingsAfterBatteryWake(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()
	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	waitForState(t, svc, func(st State) bool { return st.Connected && st.DemoMode })
	// Make the live device differ from the last cached snapshot.
	binary.LittleEndian.PutUint16(svc.demoDevice.Mem[0x0888:0x088A], 1600)
	binary.LittleEndian.PutUint16(svc.demoDevice.Mem[0x08C8:0x08CA], 1600)

	original := svc.demoDevice.CmdDev.OnGetInput
	pending := 6
	svc.demoDevice.CmdDev.SetOnGetInput(func(reportID byte, length int) ([]byte, error) {
		if pending > 0 {
			pending--
			return []byte{0xBA, 0x02, 0x00, 0x00}, nil
		}
		return original(reportID, length)
	})

	saved, err := svc.SavePreset("", "after wake", "")
	if err != nil {
		t.Fatalf("SavePreset: %v", err)
	}
	if saved.DPI.Stages[0].X != 1600 {
		t.Fatalf("saved stage 1 X = %d, want 1600", saved.DPI.Stages[0].X)
	}
}

func TestDPIWritePreservesHiddenRawBytes(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()
	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}

	// Stage 7 X/Y live at 0x0894/0x08D4. 0xffff is a vendor sentinel that the
	// decoder normalizes; the raw write path must leave it untouched.
	svc.demoDevice.Mem[0x0894], svc.demoDevice.Mem[0x0895] = 0xFF, 0xFF
	svc.demoDevice.Mem[0x08D4], svc.demoDevice.Mem[0x08D5] = 0xFF, 0xFF
	cfg := rapoo.DPIConfig{
		CurrentStage: 1,
		StageCount:   1,
		Stages:       []rapoo.DPIStage{{Index: 1, X: 450, Y: 450}},
	}
	if _, err := svc.SetDPIConfigForGeneration(cfg, svc.GetState().Generation, time.Time{}); err != nil {
		t.Fatalf("SetDPIConfig: %v", err)
	}
	if svc.demoDevice.Mem[0x0894] != 0xFF || svc.demoDevice.Mem[0x0895] != 0xFF ||
		svc.demoDevice.Mem[0x08D4] != 0xFF || svc.demoDevice.Mem[0x08D5] != 0xFF {
		t.Fatalf("hidden raw DPI bytes were rewritten: % x % x",
			svc.demoDevice.Mem[0x0894:0x0896], svc.demoDevice.Mem[0x08D4:0x08D6])
	}
}

func TestReadbackMismatchIsNotSuccess(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	original := svc.demoDevice.CmdDev.OnSetOutput
	svc.demoDevice.CmdDev.SetOnSetOutput(func(data []byte) error {
		if len(data) >= 8 && binary.LittleEndian.Uint32(data[4:8]) == rapoo.AddrPollingRate {
			// Acknowledge the write without changing the virtual memory.
			return nil
		}
		return original(data)
	})

	_, err := svc.SetPollingRateForGeneration(4000, svc.GetState().Generation, time.Time{})
	var partial *PartialWriteError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %v (%T), want PartialWriteError", err, err)
	}
}

func TestSensorPatchValidatesBeforeWriting(t *testing.T) {
	svc := NewMouseService(hid.NewMockProvider(), Config{
		InitialMode:       ModeNormal,
		NormalInterval:    10 * time.Second,
		ReconnectInterval: 10 * time.Second,
		SleepGrace:        time.Nanosecond,
		PresetDir:         t.TempDir(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Startup(ctx)
	defer svc.Shutdown()

	if _, err := svc.SetDemoMode(true); err != nil {
		t.Fatalf("SetDemoMode: %v", err)
	}
	before, err := svc.ReadSnapshot()
	if err != nil {
		t.Fatalf("ReadSnapshot before: %v", err)
	}
	generation := svc.GetState().Generation
	invalidSleep := 1
	motionSync := !before.Sensor.MotionSync
	_, err = svc.UpdateSensorConfigForGeneration(
		rapoo.SensorPatch{MotionSync: &motionSync, SleepMinutes: &invalidSleep},
		generation,
		time.Time{})
	if err == nil {
		t.Fatal("expected invalid sleep value to fail")
	}

	after, err := svc.ReadSnapshot()
	if err != nil {
		t.Fatalf("ReadSnapshot after: %v", err)
	}
	if after.Sensor.MotionSync != before.Sensor.MotionSync {
		t.Fatalf("Motion Sync changed despite invalid trailing field: before=%v after=%v",
			before.Sensor.MotionSync, after.Sensor.MotionSync)
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
