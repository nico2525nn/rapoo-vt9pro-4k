//go:build linux

package hid

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type linuxProvider struct{}

func newNativeProvider() Provider {
	return &linuxProvider{}
}

func (p *linuxProvider) Enumerate(filter Filter) ([]DeviceInfo, error) {
	entries, err := os.ReadDir("/sys/class/hidraw")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read /sys/class/hidraw: %w", err)
	}

	var out []DeviceInfo
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "hidraw") {
			continue
		}
		sysBase := filepath.Join("/sys/class/hidraw", name, "device")
		vid, pid, product, ok := parseLinuxUevent(filepath.Join(sysBase, "uevent"))
		if !ok || !filter.MatchesVIDPID(vid, pid) {
			continue
		}

		iface := readLinuxUSBInterfaceNumber(sysBase)
		if filter.InterfaceNumber >= 0 && iface != filter.InterfaceNumber {
			continue
		}

		var summary DescriptorSummary
		if rawDesc, err := os.ReadFile(filepath.Join(sysBase, "report_descriptor")); err == nil {
			summary = ParseReportDescriptor(rawDesc)
		}

		out = append(out, DeviceInfo{
			Path:             filepath.Join("/dev", name),
			VendorID:         vid,
			ProductID:        pid,
			InterfaceNumber:  iface,
			UsagePage:        summary.UsagePage,
			Usage:            summary.Usage,
			InputReportLen:   summary.InputReportLen,
			OutputReportLen:  summary.OutputReportLen,
			FeatureReportLen: summary.FeatureReportLen,
			Product:          product,
		})
	}
	return out, nil
}

func parseLinuxUevent(path string) (vid, pid uint16, product string, ok bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "HID_ID="):
			parts := strings.Split(strings.TrimPrefix(line, "HID_ID="), ":")
			if len(parts) == 3 {
				v, err1 := strconv.ParseUint(parts[1], 16, 32)
				p, err2 := strconv.ParseUint(parts[2], 16, 32)
				if err1 == nil && err2 == nil {
					vid = uint16(v)
					pid = uint16(p)
					ok = true
				}
			}
		case strings.HasPrefix(line, "HID_NAME="):
			product = strings.TrimPrefix(line, "HID_NAME=")
		}
	}
	return vid, pid, product, ok
}

func readLinuxUSBInterfaceNumber(hidSysPath string) int {
	resolved, err := filepath.EvalSymlinks(hidSysPath)
	if err != nil {
		resolved = hidSysPath
	}
	cur := resolved
	for depth := 0; depth < 5; depth++ {
		cand := filepath.Join(cur, "bInterfaceNumber")
		if b, err := os.ReadFile(cand); err == nil {
			if n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 16, 32); err == nil {
				return int(n)
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return -1
}

func (p *linuxProvider) Open(info DeviceInfo, writeAccess bool) (Device, error) {
	flags := os.O_RDONLY
	if writeAccess {
		flags = os.O_RDWR
	}
	fd, err := syscall.Open(info.Path, flags|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", info.Path, err)
	}
	f := os.NewFile(uintptr(fd), info.Path)
	if f == nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("os.NewFile(%s) returned nil", info.Path)
	}
	return &linuxDevice{
		info: info,
		file: f,
	}, nil
}

type linuxDevice struct {
	info      DeviceInfo
	file      *os.File
	closeOnce sync.Once
	mu        sync.RWMutex
	closed    bool
}

func (d *linuxDevice) Info() DeviceInfo {
	return d.info
}

// hidIoc computes _IOWR('H', nr, length) on standard Linux architectures
// (nr = 0x0A for HIDIOCGINPUT, nr = 0x0B for HIDIOCSOUTPUT).
func hidIocRW(nr, length int) uintptr {
	const (
		iocWrite = 1
		iocRead  = 2
	)
	return uintptr(((iocRead | iocWrite) << 30) | (length << 16) | (int('H') << 8) | nr)
}

func (d *linuxDevice) SetOutputReport(data []byte, timeout time.Duration) error {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return ErrClosed
	}
	f := d.file
	d.mu.RUnlock()

	outLen := len(data)
	if d.info.OutputReportLen > outLen {
		outLen = d.info.OutputReportLen
	}
	buf := make([]byte, outLen)
	copy(buf, data)

	rawConn, err := f.SyscallConn()
	if err != nil {
		return ErrClosed
	}

	var ioctlErr syscall.Errno
	ctrlErr := rawConn.Control(func(fd uintptr) {
		_, _, e1 := syscall.Syscall(
			syscall.SYS_IOCTL,
			fd,
			hidIocRW(0x0B, len(buf)), // HIDIOCSOUTPUT(len) — USB Control Pipe SET_REPORT (Output)
			uintptr(unsafe.Pointer(&buf[0])),
		)
		if e1 != 0 {
			ioctlErr = e1
		}
	})
	if ctrlErr != nil {
		return ctrlErr
	}
	if ioctlErr == 0 {
		return nil
	}
	// Fallback to interrupt OUT write only on older kernels without HIDIOCSOUTPUT (< 5.11)
	if ioctlErr == syscall.EINVAL || ioctlErr == syscall.ENOTTY {
		if timeout > 0 {
			_ = f.SetWriteDeadline(time.Now().Add(timeout))
			defer f.SetWriteDeadline(time.Time{})
		}
		if _, werr := f.Write(buf); werr != nil {
			return fmt.Errorf("hidraw write fallback: %w", werr)
		}
		return nil
	}
	return fmt.Errorf("HIDIOCSOUTPUT: %w", ioctlErr)
}

func (d *linuxDevice) GetInputReport(reportID byte, length int, _ time.Duration) ([]byte, error) {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return nil, ErrClosed
	}
	f := d.file
	d.mu.RUnlock()

	bufLen := length
	if d.info.InputReportLen > bufLen {
		bufLen = d.info.InputReportLen
	}
	if bufLen < 33 {
		bufLen = 33
	}
	buf := make([]byte, bufLen)
	buf[0] = reportID

	rawConn, err := f.SyscallConn()
	if err != nil {
		return nil, ErrClosed
	}

	var ioctlErr syscall.Errno
	var retLen int
	ctrlErr := rawConn.Control(func(fd uintptr) {
		r1, _, e1 := syscall.Syscall(
			syscall.SYS_IOCTL,
			fd,
			hidIocRW(0x0A, len(buf)), // HIDIOCGINPUT(len)
			uintptr(unsafe.Pointer(&buf[0])),
		)
		if e1 != 0 {
			ioctlErr = e1
			return
		}
		retLen = int(r1)
	})
	if ctrlErr != nil {
		return nil, ctrlErr
	}
	if ioctlErr != 0 {
		return nil, fmt.Errorf("HIDIOCGINPUT: %w", ioctlErr)
	}
	if retLen > 0 && retLen <= len(buf) {
		return buf[:retLen], nil
	}
	return buf, nil
}

func (d *linuxDevice) ReadInputPacket(timeout time.Duration) ([]byte, error) {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return nil, ErrClosed
	}
	f := d.file
	d.mu.RUnlock()

	bufLen := d.info.InputReportLen
	if bufLen < 64 {
		bufLen = 64
	}
	buf := make([]byte, bufLen)

	if timeout > 0 {
		_ = f.SetReadDeadline(time.Now().Add(timeout))
	} else {
		_ = f.SetReadDeadline(time.Time{})
	}
	n, err := f.Read(buf)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, ErrTimeout
		}
		if errors.Is(err, os.ErrClosed) {
			return nil, ErrClosed
		}
		return nil, err
	}
	return buf[:n], nil
}

func (d *linuxDevice) Close() error {
	var err error
	d.closeOnce.Do(func() {
		d.mu.Lock()
		d.closed = true
		d.mu.Unlock()
		err = d.file.Close()
	})
	return err
}
