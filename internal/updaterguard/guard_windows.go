//go:build windows

package updaterguard

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
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
func Run(updaterPath string, vendorID, productID uint16, onStarted func()) error {
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

	var hidMutex windows.Handle
	if productID != 0 {
		name := hid.CommandMutexName(vendorID, productID)
		namePtr, nameErr := windows.UTF16PtrFromString(name)
		if nameErr != nil {
			return nameErr
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			candidate, err := windows.CreateMutex(nil, true, namePtr)
			if err == nil {
				hidMutex = candidate
				break
			}
			if candidate != 0 {
				_ = windows.CloseHandle(candidate)
			}
			if !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
				return fmt.Errorf("CreateMutex(%s): %w", name, err)
			}
			if time.Now().After(deadline) {
				return errors.New("the HID command handle is still in use by another process")
			}
			time.Sleep(50 * time.Millisecond)
		}
		defer func() {
			_ = windows.ReleaseMutex(hidMutex)
			_ = windows.CloseHandle(hidMutex)
		}()
	}

	cmd := exec.Command(updaterPath)
	cmd.Dir = filepath.Dir(updaterPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	if onStarted != nil {
		onStarted()
	}
	return cmd.Wait()
}
