//go:build !windows

package tray

import (
	"context"
	"errors"

	"github.com/nico/rapoo-vt9pro-4k/internal/service"
)

// ErrAlreadyRunning is declared for cross-platform signature parity.
var ErrAlreadyRunning = errors.New("vt9tray is already running")

// Run returns an error on non-Windows platforms; use cmd/vt9ctl or a Wails GUI
// frontend bound to service.MouseService on Linux.
func Run(_ context.Context, _ *service.MouseService) error {
	return errors.New("vt9tray Win32 system tray requires Windows (GOOS=windows); use vt9ctl on Linux")
}
