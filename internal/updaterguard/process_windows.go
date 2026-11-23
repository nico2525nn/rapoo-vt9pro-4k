//go:build windows

package updaterguard

import (
	"os/exec"
	"syscall"
)

// ConfigureCommand hides the guard process's console. The guard must not be
// attached to a closable console window while it holds the updater mutex.
func ConfigureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
