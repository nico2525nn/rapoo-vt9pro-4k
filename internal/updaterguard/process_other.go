//go:build !windows

package updaterguard

import "os/exec"

// ConfigureCommand is a no-op on non-Windows platforms.
func ConfigureCommand(_ *exec.Cmd) {}
