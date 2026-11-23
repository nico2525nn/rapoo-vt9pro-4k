//go:build !windows

package updaterguard

import "errors"

// Held is always false on non-Windows platforms.
func Held() bool {
	return false
}

// Run is unavailable on non-Windows platforms.
func Run(_ string) error {
	return errors.New("the firmware updater guard requires Windows")
}
