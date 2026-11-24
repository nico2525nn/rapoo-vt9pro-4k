//go:build linux

package hid

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxDeviceFileLockIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hidraw-test")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("create test device: %v", err)
	}
	info := DeviceInfo{Path: path}
	provider := &linuxProvider{}

	first, err := provider.Open(info, true)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	t.Cleanup(func() {
		_ = first.Close()
	})

	if _, err := provider.Open(info, true); !errors.Is(err, ErrDeviceBusy) {
		t.Fatalf("second open error = %v, want ErrDeviceBusy", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}
	second, err := provider.Open(info, true)
	if err != nil {
		t.Fatalf("open after release: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second: %v", err)
	}
}
