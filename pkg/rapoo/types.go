// Package rapoo implements the vendor HID protocol and domain models for
// Rapoo Realtek 4K gaming mice (VT9 Pro 4K wired & wireless, Gen-A & Gen-B).
package rapoo

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrDeviceNotFound indicates no supported Rapoo device is currently connected.
	ErrDeviceNotFound = errors.New("rapoo: no supported device connected")
	// ErrMouseAsleep indicates the 4K wireless receiver is connected, but the
	// wireless mouse did not answer (powered off or in sleep mode).
	ErrMouseAsleep = errors.New("rapoo: wireless mouse is asleep or offline")
	// ErrInvalidResponse indicates a malformed or unexpected HID response frame.
	ErrInvalidResponse = errors.New("rapoo: invalid protocol response")
)

// ConnectionMode distinguishes USB-C wired operation from 4K wireless receiver operation.
type ConnectionMode string

const (
	ConnectionWired      ConnectionMode = "wired"
	ConnectionWireless4K ConnectionMode = "wireless_4k"
)

// BatterySource describes how a battery reading was obtained.
type BatterySource string

const (
	BatterySourceCommand BatterySource = "command"
	BatterySourceAsync   BatterySource = "async_0xBB"
	BatterySourceCached  BatterySource = "cached_sleep"
)

// BatteryState holds the mouse's battery level and charging/online state.
type BatteryState struct {
	Percentage int           `json:"percentage"` // 0..100
	Charging   bool          `json:"charging"`
	Online     bool          `json:"online"`   // False when wireless mouse is asleep/off
	Sleeping   bool          `json:"sleeping"` // True when dongle is present but mouse timed out
	Source     BatterySource `json:"source"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

// FirmwareInfo holds mouse and optional 4K receiver firmware versions.
type FirmwareInfo struct {
	MouseRaw    uint32 `json:"mouseRaw"`
	MouseString string `json:"mouseString"`
	DongleRaw   uint32 `json:"dongleRaw,omitempty"`
	DongleStr   string `json:"dongleString,omitempty"`
}

// FormatFirmwareVersion formats a raw Rapoo uint32 version (e.g. 125 -> "1.25", 222 -> "2.22").
func FormatFirmwareVersion(raw uint32) string {
	if raw == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d.%02d", raw/100, raw%100)
}

// DPIStage represents a single X/Y DPI slot on the mouse (stages 1..7).
type DPIStage struct {
	Index int    `json:"index"` // 1-based stage index (1..7)
	X     uint16 `json:"x"`
	Y     uint16 `json:"y"`
}

// DPIConfig holds all configured onboard DPI stages and the active stage index.
type DPIConfig struct {
	CurrentStage int        `json:"currentStage"` // 1-based (1..StageCount)
	StageCount   int        `json:"stageCount"`   // 1..7
	Stages       []DPIStage `json:"stages"`
}

// SensorConfig holds onboard sensor and power-saving parameters.
type SensorConfig struct {
	PollingRateHz     int  `json:"pollingRateHz"`     // 125, 250, 500, 1000, 2000, 4000, 8000
	Wireless8KSwitch  byte `json:"wireless8kSwitch"`  // Raw byte at 0x0881
	SilentHeightLOD   byte `json:"silentHeightLod"`   // Raw byte at 0x0884 (0..2)
	MotionSync        bool `json:"motionSync"`        // 0x0885 != 0
	PressDebounceMs   int  `json:"pressDebounceMs"`   // mem[0x08C0] - 2
	ReleaseDebounceMs int  `json:"releaseDebounceMs"` // mem[0x08C1] - 2
	SleepMinutes      int  `json:"sleepMinutes"`      // mem[0x08C2]
	RippleControl     bool `json:"rippleControl"`     // (~mem[0x08C3]) & 1
	LinearCorrection  bool `json:"linearCorrection"`  // ((mem[0x08C3] >> 1) ^ 1) & 1
	SensorAngleDeg    int8 `json:"sensorAngleDeg"`    // int8(mem[0x08C4])
	GlassMode         bool `json:"glassMode"`         // mem[0x08C5] != 0
	LEDLowPowerMode   byte `json:"ledLowPowerMode"`   // mem[0x08C6]
}

// DeviceSnapshot aggregates full read-only state for CLI inspection or a Wails UI.
type DeviceSnapshot struct {
	Model      ModelSpec     `json:"model"`
	CommandHID string        `json:"commandHidPath"`
	ReportHID  string        `json:"reportHidPath,omitempty"`
	Battery    BatteryState  `json:"battery"`
	Firmware   *FirmwareInfo `json:"firmware,omitempty"`
	DPI        *DPIConfig    `json:"dpi,omitempty"`
	Sensor     *SensorConfig `json:"sensor,omitempty"`
}

// EventKind identifies asynchronous telemetry events pushed by the mouse/dongle.
type EventKind string

const (
	EventBattery EventKind = "battery"
	EventProfile EventKind = "profile"
	EventWake    EventKind = "wake" // COL06 status/wake/DPI notification
)

// AsyncEvent represents an unsolicited notification decoded from COL05 / COL06.
type AsyncEvent struct {
	Kind      EventKind     `json:"kind"`
	Battery   *BatteryState `json:"battery,omitempty"`
	ProfileID int           `json:"profileId,omitempty"`
	Raw       []byte        `json:"-"`
}
