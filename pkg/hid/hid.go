// Package hid provides a zero-CGO, cross-platform HID abstraction tailored for
// vendor-specific gaming peripheral control channels on Windows and Linux.
package hid

import (
	"errors"
	"time"
)

var (
	// ErrTimeout indicates an I/O operation exceeded its deadline.
	ErrTimeout = errors.New("hid: operation timed out")
	// ErrClosed indicates the device handle has already been closed.
	ErrClosed = errors.New("hid: device closed")
	// ErrUnsupported indicates HID access is not implemented on the current OS.
	ErrUnsupported = errors.New("hid: unsupported platform")
)

// DeviceInfo describes a single top-level HID collection discovered on the host.
type DeviceInfo struct {
	Path             string `json:"path"`
	VendorID         uint16 `json:"vendorId"`
	ProductID        uint16 `json:"productId"`
	VersionNumber    uint16 `json:"versionNumber"`
	InterfaceNumber  int    `json:"interfaceNumber"` // -1 if unknown
	UsagePage        uint16 `json:"usagePage"`
	Usage            uint16 `json:"usage"`
	InputReportLen   int    `json:"inputReportLen"`
	OutputReportLen  int    `json:"outputReportLen"`
	FeatureReportLen int    `json:"featureReportLen"`
	Product          string `json:"product,omitempty"`
}

// Filter restricts which HID collections are probed during Enumerate.
// Pre-filtering by VendorID and InterfaceNumber avoids opening unrelated
// devices or high-frequency gaming mouse input interfaces (such as MI_00).
type Filter struct {
	VendorID        uint16
	ProductIDs      []uint16 // Empty matches any ProductID for VendorID
	InterfaceNumber int      // -1 matches any interface; >= 0 requires exact match
}

// MatchesVIDPID checks whether vid and pid satisfy the filter.
func (f Filter) MatchesVIDPID(vid, pid uint16) bool {
	if f.VendorID != 0 && vid != f.VendorID {
		return false
	}
	if len(f.ProductIDs) > 0 {
		found := false
		for _, want := range f.ProductIDs {
			if pid == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Device represents an open HID top-level collection.
type Device interface {
	// Info returns the metadata discovered for this HID collection.
	Info() DeviceInfo

	// SetOutputReport sends a control-pipe SET_REPORT (Output) frame
	// (HidD_SetOutputReport on Windows, HIDIOCSOUTPUT on Linux).
	// Byte 0 must be the Report ID.
	SetOutputReport(data []byte, timeout time.Duration) error

	// GetInputReport issues a control-pipe GET_REPORT (Input) for reportID
	// (HidD_GetInputReport on Windows, HIDIOCGINPUT on Linux).
	GetInputReport(reportID byte, length int, timeout time.Duration) ([]byte, error)

	// ReadInputPacket reads a single asynchronous interrupt Input report.
	// Returns ErrTimeout if no packet arrives within timeout.
	ReadInputPacket(timeout time.Duration) ([]byte, error)

	// Close releases the underlying OS handle and unblocks pending reads.
	Close() error
}

// Provider enumerates and opens HID collections.
type Provider interface {
	Enumerate(filter Filter) ([]DeviceInfo, error)
	Open(info DeviceInfo, writeAccess bool) (Device, error)
}

// NewProvider returns the native OS HID provider (Win32 SetupAPI/HidD on Windows,
// hidraw/sysfs on Linux).
func NewProvider() Provider {
	return newNativeProvider()
}
