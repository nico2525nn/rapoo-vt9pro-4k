//go:build windows

package updaterguard

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

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
func Run(updaterPath string, vendorID uint16, productIDs []uint16, onStarted func()) error {
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

	hidMutexes := make([]windows.Handle, 0, len(productIDs))
	defer func() {
		for _, hidMutex := range hidMutexes {
			_ = windows.ReleaseMutex(hidMutex)
			_ = windows.CloseHandle(hidMutex)
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for _, productID := range productIDs {
		if productID == 0 {
			continue
		}
		name := hid.CommandMutexName(vendorID, productID)
		namePtr, nameErr := windows.UTF16PtrFromString(name)
		if nameErr != nil {
			return nameErr
		}
		for {
			candidate, err := windows.CreateMutex(nil, true, namePtr)
			if err == nil {
				hidMutexes = append(hidMutexes, candidate)
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
	}

	job, jobErr := windows.CreateJobObject(nil, nil)
	if jobErr != nil {
		job = 0
	}
	if job != 0 {
		defer windows.CloseHandle(job)
	}

	cmd := exec.Command(updaterPath)
	cmd.Dir = filepath.Dir(updaterPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	jobAssigned := false
	if job != 0 {
		process, openErr := windows.OpenProcess(
			windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
			false,
			uint32(cmd.Process.Pid))
		if openErr == nil {
			assignErr := windows.AssignProcessToJobObject(job, process)
			_ = windows.CloseHandle(process)
			jobAssigned = assignErr == nil
		}
	}
	if onStarted != nil {
		onStarted()
	}
	err = cmd.Wait()
	// If the official launcher hands the update to a child (or grandchild) and
	// exits, keep the HID mutexes until the whole tree has finished.
	waitForChildren(uint32(cmd.Process.Pid), 30*time.Minute)
	if jobAssigned {
		waitForJobEmpty(job, 30*time.Minute)
	}
	return err
}

type jobBasicAccounting struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func waitForJobEmpty(job windows.Handle, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if count, err := jobActiveProcesses(job); err == nil && count == 0 {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func jobActiveProcesses(job windows.Handle) (uint32, error) {
	var info jobBasicAccounting
	if err := windows.QueryInformationJobObject(
		job,
		windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	); err != nil {
		return 0, err
	}
	return info.ActiveProcesses, nil
}

func waitForChildren(parentPID uint32, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !hasChildProcess(parentPID) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func hasChildProcess(parentPID uint32) bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false
	}
	for {
		if entry.ParentProcessID == parentPID && entry.ProcessID != 0 {
			return true
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			return false
		}
	}
}
