//go:build windows

package updaterguard

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const mutexName = `Local\RapooVT9Pro4K_FirmwareUpdater`

// Held reports whether a firmware-updater guard process currently owns the
// device. The guard outlives the bridge/UI that launched the official updater.
func Held() bool {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return false
	}
	handle, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(handle)
	return true
}

// Run is the guard process entry point: it holds the cross-process mutex for
// the complete lifetime of the official updater child.
func Run(updaterPath string) error {
	if !filepath.IsAbs(updaterPath) {
		return errors.New("the updater path must be absolute")
	}
	switch strings.ToLower(filepath.Base(updaterPath)) {
	case "rapoogamedev.exe", "firmwareexe.exe":
	default:
		return errors.New("select RapooGameDev.exe or FirmwareExe.exe")
	}

	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return err
	}
	handle, err := windows.CreateMutex(nil, true, name)
	if err != nil {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return errors.New("firmware update is already in progress")
		}
		return fmt.Errorf("CreateMutex: %w", err)
	}
	defer func() {
		_ = windows.ReleaseMutex(handle)
		_ = windows.CloseHandle(handle)
	}()

	cmd := exec.Command(updaterPath)
	cmd.Dir = filepath.Dir(updaterPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}
