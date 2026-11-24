//go:build windows

package updaterguard

import (
	"errors"
	"fmt"
	"log"
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

	handle, held, err := hid.OpenMutexLease(mutexName)
	if err != nil {
		return fmt.Errorf("CreateMutex: %w", err)
	}
	if held {
		_ = windows.CloseHandle(handle)
		return errors.New("firmware update is already in progress")
	}
	defer windows.CloseHandle(handle)

	hidMutexes := make([]windows.Handle, 0, len(productIDs))
	defer func() {
		for _, hidMutex := range hidMutexes {
			_ = windows.CloseHandle(hidMutex)
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for _, productID := range productIDs {
		if productID == 0 {
			continue
		}
		name := hid.CommandMutexName(vendorID, productID)
		for {
			candidate, held, err := hid.OpenMutexLease(name)
			if err != nil {
				return fmt.Errorf("CreateMutex(%s): %w", name, err)
			}
			if !held {
				hidMutexes = append(hidMutexes, candidate)
				break
			}
			_ = windows.CloseHandle(candidate)
			if time.Now().After(deadline) {
				return errors.New("the HID command handle is still in use by another process")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create updater job: %w", err)
	}
	defer windows.CloseHandle(job)

	process, thread, pid, err := startSuspended(updaterPath, filepath.Dir(updaterPath))
	if err != nil {
		return err
	}
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.TerminateProcess(process, 1)
		_ = windows.CloseHandle(thread)
		_ = windows.CloseHandle(process)
		return fmt.Errorf("assign updater to job: %w", err)
	}
	if _, err := windows.ResumeThread(thread); err != nil {
		_ = windows.TerminateProcess(process, 1)
		_ = windows.CloseHandle(thread)
		_ = windows.CloseHandle(process)
		return fmt.Errorf("resume updater: %w", err)
	}
	_ = windows.CloseHandle(thread)
	if onStarted != nil {
		onStarted()
	}

	waitEvent, waitErr := windows.WaitForSingleObject(process, windows.INFINITE)
	var exitCode uint32
	if waitErr == nil && waitEvent == 0 {
		waitErr = windows.GetExitCodeProcess(process, &exitCode)
	}
	_ = windows.CloseHandle(process)

	// If the official launcher hands the update to a child (or grandchild) and
	// exits, keep the HID mutexes until the whole tree has finished. The job
	// was assigned before the first thread ran, so it covers descendants even
	// when an intermediate process exits.
	waitForChildren(pid, 0)
	waitForJobEmpty(job, 0)
	if waitErr != nil {
		return waitErr
	}
	if exitCode != 0 {
		return fmt.Errorf("updater exited with code %d", exitCode)
	}
	return nil
}

func startSuspended(path, dir string) (windows.Handle, windows.Handle, uint32, error) {
	exe, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, 0, err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{path}))
	if err != nil {
		return 0, 0, 0, err
	}
	currentDir, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, 0, 0, err
	}
	var startup windows.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var info windows.ProcessInformation
	if err := windows.CreateProcess(
		exe,
		commandLine,
		nil,
		nil,
		false,
		windows.CREATE_SUSPENDED,
		nil,
		currentDir,
		&startup,
		&info,
	); err != nil {
		return 0, 0, 0, fmt.Errorf("create updater: %w", err)
	}
	return info.Process, info.Thread, info.ProcessId, nil
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
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for deadline.IsZero() || time.Now().Before(deadline) {
		count, err := jobActiveProcesses(job)
		if err != nil {
			// The job was assigned before the first thread ran. If accounting
			// becomes unavailable, keep holding the HID lock; releasing it
			// while the updater may still be running is worse.
			log.Printf("updater job accounting stopped: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if count == 0 {
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
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for deadline.IsZero() || time.Now().Before(deadline) {
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
