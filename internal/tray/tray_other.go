//go:build !windows

package tray

import (
	"context"
	"errors"

	"github.com/nico/rapoo-vt9pro-4k/internal/service"
)

// ErrAlreadyRunning is declared for cross-platform signature parity.
var ErrAlreadyRunning = errors.New("vt9tray is already running")

// Run returns an error on non-Windows platforms.
func Run(ctx context.Context, svc *service.MouseService) error {
	return RunWithOptions(ctx, svc, Options{})
}

// RunWithOptions returns an error on non-Windows platforms.
func RunWithOptions(_ context.Context, _ *service.MouseService, _ Options) error {
	return errors.New("vt9tray Win32 system tray requires Windows (GOOS=windows)")
}

// IsAutoStartEnabled reports false on non-Windows platforms.
func IsAutoStartEnabled(_ string) bool {
	return false
}

// SetAutoStartEnabled is a no-op on non-Windows platforms.
func SetAutoStartEnabled(valueName string, enable bool) error {
	return SetAutoStartEnabledWithArgs(valueName, enable)
}

// SetAutoStartEnabledWithArgs is a no-op on non-Windows platforms.
func SetAutoStartEnabledWithArgs(_ string, _ bool, _ ...string) error {
	return errors.New("Windows startup registry requires GOOS=windows")
}

// AutoStartCommand returns an empty command on non-Windows platforms.
func AutoStartCommand(_ string) string {
	return ""
}

// DeleteAutoStart is a no-op on non-Windows platforms.
func DeleteAutoStart(_ string) error {
	return errors.New("Windows startup registry requires GOOS=windows")
}
