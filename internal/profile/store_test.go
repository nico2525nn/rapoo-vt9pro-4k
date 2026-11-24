package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nico/rapoo-vt9pro-4k/pkg/rapoo"
)

func TestStoreSaveExportImportDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	initial, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(initial) != 0 {
		t.Fatalf("len(initial) = %d, want 0", len(initial))
	}

	snap := rapoo.DeviceSnapshot{
		DPI: &rapoo.DPIConfig{
			CurrentStage: 1,
			StageCount:   2,
			Stages: []rapoo.DPIStage{
				{Index: 1, X: 800, Y: 800, Enabled: true},
				{Index: 2, X: 1600, Y: 1600, Enabled: true},
			},
			AllStages: []rapoo.DPIStage{
				{Index: 1, X: 800, Y: 800, Enabled: true},
				{Index: 2, X: 1600, Y: 1600, Enabled: true},
				{Index: 3, X: 3200, Y: 3200, Enabled: false},
				{Index: 4, X: 3200, Y: 3200, Enabled: false},
				{Index: 5, X: 3200, Y: 3200, Enabled: false},
				{Index: 6, X: 3200, Y: 3200, Enabled: false},
				{Index: 7, X: 3200, Y: 3200, Enabled: false},
			},
		},
		Sensor: &rapoo.SensorConfig{
			PollingRateHz:       2000,
			LiftOffLevel:        1,
			MotionSync:          false,
			PressDebounceStep:   2,
			ReleaseDebounceStep: 2,
			SleepMinutes:        15,
			LEDLowPowerMode:     1,
		},
		Buttons: rapoo.DefaultButtonMappings(),
	}

	saved, err := store.SaveSnapshot("", "Valorant Custom", "My 2K setup", snap)
	if err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	if saved.BuiltIn || saved.PollingHz != 2000 {
		t.Fatalf("unexpected saved preset: %+v", saved)
	}
	if saved.Sensor.LEDLowPowerMode == nil || *saved.Sensor.LEDLowPowerMode != 1 {
		t.Fatalf("LED low power not saved: %+v", saved.Sensor)
	}

	exported, err := store.ExportJSON(saved.ID)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	imported, err := store.ImportJSON(exported)
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if imported.ID == saved.ID || imported.Name != "Valorant Custom" {
		t.Fatalf("unexpected imported preset: %+v", imported)
	}

	// Reopen from disk to verify persistence.
	store2, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	list2, err := store2.List()
	if err != nil {
		t.Fatalf("List after reopen: %v", err)
	}
	if len(list2) != 2 {
		t.Fatalf("len(list2) = %d, want 2", len(list2))
	}

	if err := store2.Delete(saved.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list3, err := store2.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(list3) != 1 {
		t.Fatalf("len after delete = %d, want 1", len(list3))
	}

	// A corrupt store must not be reported as an empty library. The last good
	// cache is returned together with the read error.
	if err := os.WriteFile(filepath.Join(dir, "presets.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatalf("corrupt presets: %v", err)
	}
	cached, err := store2.List()
	if err == nil {
		t.Fatal("List on a corrupt store returned nil error")
	}
	if len(cached) != 1 {
		t.Fatalf("cached list after corruption = %d, want 1", len(cached))
	}
}
