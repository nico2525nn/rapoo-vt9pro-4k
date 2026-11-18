//go:build windows

package hid

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	modHid      = syscall.NewLazyDLL("hid.dll")
	modSetupAPI = syscall.NewLazyDLL("setupapi.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procHidDGetHidGuid           = modHid.NewProc("HidD_GetHidGuid")
	procHidDGetAttributes        = modHid.NewProc("HidD_GetAttributes")
	procHidDGetPreparsedData     = modHid.NewProc("HidD_GetPreparsedData")
	procHidDFreePreparsedData    = modHid.NewProc("HidD_FreePreparsedData")
	procHidDGetProductString     = modHid.NewProc("HidD_GetProductString")
	procHidDSetOutputReport      = modHid.NewProc("HidD_SetOutputReport")
	procHidDGetInputReport       = modHid.NewProc("HidD_GetInputReport")
	procHidPGetCaps              = modHid.NewProc("HidP_GetCaps")
	procSetupDiGetClassDevsW     = modSetupAPI.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterf  = modSetupAPI.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetInterfDetailW  = modSetupAPI.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDevInfoLst = modSetupAPI.NewProc("SetupDiDestroyDeviceInfoList")
	procCreateEventW             = modKernel32.NewProc("CreateEventW")
	procSetEvent                 = modKernel32.NewProc("SetEvent")
	procWaitForMultipleObjects   = modKernel32.NewProc("WaitForMultipleObjects")
	procGetOverlappedResult      = modKernel32.NewProc("GetOverlappedResult")
	procCancelIoEx               = modKernel32.NewProc("CancelIoEx")
)

const (
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010

	hidpStatusSuccess = 0x00110000

	waitObject0 = 0x00000000
	waitTimeout = 0x00000102

	errorIOPending    syscall.Errno = 997
	errorOperationAbt syscall.Errno = 995
)

type winGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type spDeviceInterfaceData struct {
	CbSize             uint32
	InterfaceClassGuid winGUID
	Flags              uint32
	Reserved           uintptr
}

type hiddAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

type hidpCaps struct {
	Usage                     uint16
	UsagePage                 uint16
	InputReportByteLength     uint16
	OutputReportByteLength    uint16
	FeatureReportByteLength   uint16
	Reserved                  [17]uint16
	NumberLinkCollectionNodes uint16
	NumberInputButtonCaps     uint16
	NumberInputValueCaps      uint16
	NumberInputDataIndices    uint16
	NumberOutputButtonCaps    uint16
	NumberOutputValueCaps     uint16
	NumberOutputDataIndices   uint16
	NumberFeatureButtonCaps   uint16
	NumberFeatureValueCaps    uint16
	NumberFeatureDataIndices  uint16
}

type windowsProvider struct{}

func newNativeProvider() Provider {
	return &windowsProvider{}
}

func (p *windowsProvider) Enumerate(filter Filter) ([]DeviceInfo, error) {
	var hidGUID winGUID
	procHidDGetHidGuid.Call(uintptr(unsafe.Pointer(&hidGUID)))

	devInfoSet, _, err := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&hidGUID)),
		0,
		0,
		digcfPresent|digcfDeviceInterface,
	)
	if devInfoSet == uintptr(syscall.InvalidHandle) {
		return nil, fmt.Errorf("SetupDiGetClassDevsW: %w", err)
	}
	defer procSetupDiDestroyDevInfoLst.Call(devInfoSet)

	var results []DeviceInfo
	for idx := uint32(0); ; idx++ {
		var ifData spDeviceInterfaceData
		ifData.CbSize = uint32(unsafe.Sizeof(ifData))

		r1, _, _ := procSetupDiEnumDeviceInterf.Call(
			devInfoSet,
			0,
			uintptr(unsafe.Pointer(&hidGUID)),
			uintptr(idx),
			uintptr(unsafe.Pointer(&ifData)),
		)
		if r1 == 0 {
			break
		}

		var reqSize uint32
		procSetupDiGetInterfDetailW.Call(
			devInfoSet,
			uintptr(unsafe.Pointer(&ifData)),
			0,
			0,
			uintptr(unsafe.Pointer(&reqSize)),
			0,
		)
		if reqSize < 8 {
			continue
		}

		detailBuf := make([]byte, reqSize)
		detailCbSize := uint32(6)
		if unsafe.Sizeof(uintptr(0)) == 8 {
			detailCbSize = 8
		}
		*(*uint32)(unsafe.Pointer(&detailBuf[0])) = detailCbSize

		r2, _, _ := procSetupDiGetInterfDetailW.Call(
			devInfoSet,
			uintptr(unsafe.Pointer(&ifData)),
			uintptr(unsafe.Pointer(&detailBuf[0])),
			uintptr(reqSize),
			0,
			0,
		)
		if r2 == 0 {
			continue
		}

		u16Slice := unsafe.Slice((*uint16)(unsafe.Pointer(&detailBuf[4])), (reqSize-4)/2)
		devPath := utf16ZToString(u16Slice)
		if devPath == "" {
			continue
		}

		// Gaming-safe pre-filtering: inspect VID, PID, and MI_xx from the symbolic
		// path BEFORE opening a metadata handle. MI_00 (the 4000 Hz gaming mouse
		// interface) is never opened even for metadata.
		vid, pid, iface, parsedOK := ParseWindowsDevicePath(devPath)
		if parsedOK {
			if !filter.MatchesVIDPID(vid, pid) {
				continue
			}
			if filter.InterfaceNumber >= 0 && iface >= 0 && iface != filter.InterfaceNumber {
				continue
			}
		}

		info, err := queryWindowsCollectionInfo(devPath, vid, pid, iface)
		if err != nil {
			continue
		}
		if !filter.MatchesVIDPID(info.VendorID, info.ProductID) {
			continue
		}
		if filter.InterfaceNumber >= 0 && info.InterfaceNumber != filter.InterfaceNumber {
			continue
		}
		results = append(results, info)
	}

	return results, nil
}

func queryWindowsCollectionInfo(devPath string, fallbackVID, fallbackPID uint16, iface int) (DeviceInfo, error) {
	pathPtr, err := syscall.UTF16PtrFromString(devPath)
	if err != nil {
		return DeviceInfo{}, err
	}

	h, err := syscall.CreateFile(
		pathPtr,
		0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return DeviceInfo{}, err
	}
	defer syscall.CloseHandle(h)

	info := DeviceInfo{
		Path:            devPath,
		VendorID:        fallbackVID,
		ProductID:       fallbackPID,
		InterfaceNumber: iface,
	}

	var attrs hiddAttributes
	attrs.Size = uint32(unsafe.Sizeof(attrs))
	if r, _, _ := procHidDGetAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&attrs))); r != 0 {
		info.VendorID = attrs.VendorID
		info.ProductID = attrs.ProductID
		info.VersionNumber = attrs.VersionNumber
	}

	var preparsed uintptr
	if r, _, _ := procHidDGetPreparsedData.Call(uintptr(h), uintptr(unsafe.Pointer(&preparsed))); r != 0 && preparsed != 0 {
		var caps hidpCaps
		status, _, _ := procHidPGetCaps.Call(preparsed, uintptr(unsafe.Pointer(&caps)))
		procHidDFreePreparsedData.Call(preparsed)
		if status == hidpStatusSuccess {
			info.UsagePage = caps.UsagePage
			info.Usage = caps.Usage
			info.InputReportLen = int(caps.InputReportByteLength)
			info.OutputReportLen = int(caps.OutputReportByteLength)
			info.FeatureReportLen = int(caps.FeatureReportByteLength)
		}
	}

	var prodBuf [128]uint16
	if r, _, _ := procHidDGetProductString.Call(uintptr(h), uintptr(unsafe.Pointer(&prodBuf[0])), uintptr(len(prodBuf)*2)); r != 0 {
		info.Product = utf16ZToString(prodBuf[:])
	}

	return info, nil
}

func (p *windowsProvider) Open(info DeviceInfo, writeAccess bool) (Device, error) {
	pathPtr, err := syscall.UTF16PtrFromString(info.Path)
	if err != nil {
		return nil, err
	}

	access := uint32(syscall.GENERIC_READ)
	flags := uint32(syscall.FILE_FLAG_OVERLAPPED)
	if writeAccess {
		access |= syscall.GENERIC_WRITE
		// Control-pipe HidD_SetOutputReport / HidD_GetInputReport are synchronous
		// IOCTLs; opening command handles without FILE_FLAG_OVERLAPPED matches
		// RapooGameDev.exe and avoids NULL-OVERLAPPED hazards inside hid.dll.
		flags = 0
	}

	h, err := syscall.CreateFile(
		pathPtr,
		access,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		flags,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateFile(%s): %w", info.Path, err)
	}

	closeEvt, err := createManualResetEvent(false)
	if err != nil {
		syscall.CloseHandle(h)
		return nil, err
	}

	return &windowsDevice{
		info:       info,
		handle:     h,
		closeEvt:   closeEvt,
		overlapped: flags&syscall.FILE_FLAG_OVERLAPPED != 0,
	}, nil
}

type windowsDevice struct {
	info       DeviceInfo
	handle     syscall.Handle
	closeEvt   syscall.Handle
	overlapped bool
	closeOnce  sync.Once
	mu         sync.RWMutex
	closed     bool
}

func (d *windowsDevice) Info() DeviceInfo {
	return d.info
}

func (d *windowsDevice) SetOutputReport(data []byte, _ time.Duration) error {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return ErrClosed
	}
	h := d.handle
	d.mu.RUnlock()

	outLen := len(data)
	if d.info.OutputReportLen > outLen {
		outLen = d.info.OutputReportLen
	}
	buf := make([]byte, outLen)
	copy(buf, data)

	r, _, err := procHidDSetOutputReport.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r == 0 {
		return fmt.Errorf("HidD_SetOutputReport: %w", err)
	}
	return nil
}

func (d *windowsDevice) GetInputReport(reportID byte, length int, _ time.Duration) ([]byte, error) {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return nil, ErrClosed
	}
	h := d.handle
	d.mu.RUnlock()

	bufLen := length
	if d.info.InputReportLen > bufLen {
		bufLen = d.info.InputReportLen
	}
	if bufLen < 32 {
		bufLen = 32
	}
	buf := make([]byte, bufLen)
	buf[0] = reportID

	r, _, err := procHidDGetInputReport.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if r == 0 {
		return nil, fmt.Errorf("HidD_GetInputReport: %w", err)
	}
	return buf, nil
}

func (d *windowsDevice) ReadInputPacket(timeout time.Duration) ([]byte, error) {
	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()
		return nil, ErrClosed
	}
	h := d.handle
	closeEvt := d.closeEvt
	isOv := d.overlapped
	d.mu.RUnlock()

	bufLen := d.info.InputReportLen
	if bufLen < 64 {
		bufLen = 64
	}
	buf := make([]byte, bufLen)

	if !isOv {
		var bytesRead uint32
		if err := syscall.ReadFile(h, buf, &bytesRead, nil); err != nil {
			return nil, fmt.Errorf("ReadFile: %w", err)
		}
		return buf[:bytesRead], nil
	}

	ioEvt, err := createManualResetEvent(false)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(ioEvt)

	var ov syscall.Overlapped
	ov.HEvent = ioEvt

	var bytesRead uint32
	err = syscall.ReadFile(h, buf, &bytesRead, &ov)
	if err != nil && !errors.Is(err, errorIOPending) {
		return nil, fmt.Errorf("ReadFile: %w", err)
	}

	if errors.Is(err, errorIOPending) {
		n, werr := waitOverlapped(h, &ov, ioEvt, closeEvt, timeout)
		if werr != nil {
			return nil, werr
		}
		bytesRead = n
	}

	out := make([]byte, bytesRead)
	copy(out, buf[:bytesRead])
	return out, nil
}

func (d *windowsDevice) Close() error {
	var closeErr error
	d.closeOnce.Do(func() {
		procSetEvent.Call(uintptr(d.closeEvt))
		d.mu.Lock()
		d.closed = true
		h := d.handle
		d.mu.Unlock()

		procCancelIoEx.Call(uintptr(h), 0)
		closeErr = syscall.CloseHandle(h)
		syscall.CloseHandle(d.closeEvt)
	})
	return closeErr
}

func createManualResetEvent(initialState bool) (syscall.Handle, error) {
	initVal := uintptr(0)
	if initialState {
		initVal = 1
	}
	h, _, err := procCreateEventW.Call(0, 1, initVal, 0)
	if h == 0 {
		return 0, fmt.Errorf("CreateEventW: %w", err)
	}
	return syscall.Handle(h), nil
}

func waitOverlapped(h syscall.Handle, ov *syscall.Overlapped, ioEvt, closeEvt syscall.Handle, timeout time.Duration) (uint32, error) {
	ms := uint32(0xFFFFFFFF)
	if timeout > 0 {
		ms64 := timeout.Milliseconds()
		if ms64 <= 0 {
			ms = 1
		} else if ms64 < 0xFFFFFFFF {
			ms = uint32(ms64)
		}
	}

	handles := [2]syscall.Handle{ioEvt, closeEvt}
	res, _, werr := procWaitForMultipleObjects.Call(
		2,
		uintptr(unsafe.Pointer(&handles[0])),
		0,
		uintptr(ms),
	)

	switch uint32(res) {
	case waitObject0:
		var transferred uint32
		r, _, gerr := procGetOverlappedResult.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(ov)),
			uintptr(unsafe.Pointer(&transferred)),
			0,
		)
		if r == 0 {
			if errors.Is(gerr, errorOperationAbt) {
				return 0, ErrClosed
			}
			return 0, fmt.Errorf("GetOverlappedResult: %w", gerr)
		}
		return transferred, nil

	case waitObject0 + 1:
		procCancelIoEx.Call(uintptr(h), uintptr(unsafe.Pointer(ov)))
		var transferred uint32
		procGetOverlappedResult.Call(uintptr(h), uintptr(unsafe.Pointer(ov)), uintptr(unsafe.Pointer(&transferred)), 1)
		return 0, ErrClosed

	case waitTimeout:
		procCancelIoEx.Call(uintptr(h), uintptr(unsafe.Pointer(ov)))
		var transferred uint32
		procGetOverlappedResult.Call(uintptr(h), uintptr(unsafe.Pointer(ov)), uintptr(unsafe.Pointer(&transferred)), 1)
		return 0, ErrTimeout

	default:
		procCancelIoEx.Call(uintptr(h), uintptr(unsafe.Pointer(ov)))
		return 0, fmt.Errorf("WaitForMultipleObjects: %w", werr)
	}
}

func utf16ZToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			return string(utf16.Decode(s[:i]))
		}
	}
	return string(utf16.Decode(s))
}
