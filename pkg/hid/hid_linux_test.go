//go:build linux

package hid

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireLinuxDeviceLockIsExclusive(t *testing.T) {
	info := DeviceInfo{
		Path:       filepath.Join(t.TempDir(), "hidraw-test"),
		PhysicalID: fmt.Sprintf("test-%d", time.Now().UnixNano()),
	}

	// TMPDIR is intentionally different for the second acquisition; the lock
	// must still be machine-local and shared.
	firstTmp := t.TempDir()
	t.Setenv("TMPDIR", firstTmp)
	first, err := acquireLinuxDeviceLock(info)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	t.Setenv("TMPDIR", t.TempDir())
	lockPath := first.Name()
	t.Cleanup(func() {
		_ = first.Close()
		_ = os.Remove(lockPath)
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
