//go:build linux

package hid

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireLinuxDeviceLockIsExclusive(t *testing.T) {
	info := DeviceInfo{
		Path:       filepath.Join(t.TempDir(), "hidraw-test"),
		PhysicalID: fmt.Sprintf("test-%d", time.Now().UnixNano()),
	}

	// TMPDIR must not influence the machine-local lock name.
	t.Setenv("TMPDIR", t.TempDir())
	first, err := acquireLinuxDeviceLock(info)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	t.Setenv("TMPDIR", t.TempDir())
	t.Cleanup(func() {
		_ = first.Close()
	})

	if _, err := acquireLinuxDeviceLock(info); !errors.Is(err, ErrDeviceBusy) {
		t.Fatalf("second lock error = %v, want ErrDeviceBusy", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first lock: %v", err)
	}
	second, err := acquireLinuxDeviceLock(info)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second lock: %v", err)
	}
}
