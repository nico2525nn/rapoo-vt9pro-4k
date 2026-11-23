// Package rapoo implements the vendor HID protocol and domain models for
// Rapoo Realtek 4K gaming mice (VT9 Pro 4K wired & wireless, Gen-A & Gen-B).
package rapoo

import (
	"errors"
	"fmt"
	"math"
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
	// ErrMissingLeftClick prevents saving a button map where no physical button is bound to Left Click.
	ErrMissingLeftClick = errors.New("rapoo: at least one physical button must remain assigned to Left Click")
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

// EstimateBatteryHours estimates remaining active wireless battery life (hours)
// for the VT9 Pro 4K's 800mAh Li-Po cell (~160h @ 1000Hz, ~50h @ 4000Hz).
func EstimateBatteryHours(percentage, pollingRateHz int) float64 {
	if percentage <= 0 {
		return 0
	}
	if percentage > 100 {
		percentage = 100
	}
	var fullHours float64
	switch {
	case pollingRateHz >= 4000:
		fullHours = 48.0
	case pollingRateHz >= 2000:
		fullHours = 95.0
	case pollingRateHz >= 1000:
		fullHours = 160.0
	case pollingRateHz >= 500:
		fullHours = 185.0
	default:
		fullHours = 200.0
	}
	h := fullHours * (float64(percentage) / 100.0)
	return math.Round(h*10) / 10
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

const (
	MinDPI      uint16 = 50
	MaxDPI      uint16 = 26000
	DPIStep     uint16 = 50
	MaxDPIStage int    = 7
)

// NormalizeDPI clamps and snaps a DPI value to PAW3395 hardware increments (50..26000, step 50).
func NormalizeDPI(v uint16) uint16 {
	if v < MinDPI {
		return MinDPI
	}
	if v > MaxDPI {
		return MaxDPI
	}
	rem := v % DPIStep
	if rem >= DPIStep/2 {
		v += DPIStep - rem
	} else {
		v -= rem
	}
	if v > MaxDPI {
		return MaxDPI
	}
	return v
}

// DPIStage represents a single X/Y DPI slot on the mouse (stages 1..7).
type DPIStage struct {
	Index   int    `json:"index"` // 1-based stage index (1..7)
	X       uint16 `json:"x"`
	Y       uint16 `json:"y"`
	Enabled bool   `json:"enabled"`
}

// DPIConfig holds all configured onboard DPI stages and the active stage index.
type DPIConfig struct {
	CurrentStage  int        `json:"currentStage"`  // 1-based (1..StageCount)
	StageCount    int        `json:"stageCount"`    // 1..7
	IndependentXY bool       `json:"independentXY"` // True when any enabled stage has X != Y
	Stages        []DPIStage `json:"stages"`        // Active/enabled stages (1..StageCount)
	AllStages     []DPIStage `json:"allStages"`     // All 7 onboard slots (preserving inactive slot values)
}

// DebounceStep describes one hardware debounce step on Gen-A PAW3395/Realtek firmware.
type DebounceStep struct {
	Index   int     `json:"index"`   // 0..6
	RawByte byte    `json:"rawByte"` // Units of 0.5ms: 1, 2, 4, 8, 16, 24, 32
	Ms      float64 `json:"ms"`      // 0.5, 1, 2, 4, 8, 12, 16
	Label   string  `json:"label"`   // "0.5 ms", "1 ms", ...
}

// GenADebounceSteps lists the exact 7 debounce settings used by QDeviceVT9PRO_4K.
var GenADebounceSteps = []DebounceStep{
	{Index: 0, RawByte: 0x01, Ms: 0.5, Label: "0.5 ms"},
	{Index: 1, RawByte: 0x02, Ms: 1.0, Label: "1 ms"},
	{Index: 2, RawByte: 0x04, Ms: 2.0, Label: "2 ms"},
	{Index: 3, RawByte: 0x08, Ms: 4.0, Label: "4 ms"},
	{Index: 4, RawByte: 0x10, Ms: 8.0, Label: "8 ms"},
	{Index: 5, RawByte: 0x18, Ms: 12.0, Label: "12 ms"},
	{Index: 6, RawByte: 0x20, Ms: 16.0, Label: "16 ms"},
}

// SensorConfig holds onboard sensor, switch debounce, and power-saving parameters.
type SensorConfig struct {
	PollingRateHz      int     `json:"pollingRateHz"`      // 125, 250, 500, 1000, 2000, 4000
	Wireless8KSwitch   byte    `json:"wireless8kSwitch"`   // Raw byte at 0x0881
	SilentHeightLOD    byte    `json:"silentHeightLod"`    // Raw byte at 0x0884 (1..11)
	LiftOffLevel       int     `json:"liftOffLevel"`       // 0..10 (0=1.0mm .. 10=2.0mm)
	LiftOffDistanceMm  float64 `json:"liftOffDistanceMm"`  // 1.0 .. 2.0 (step 0.1)
	MotionSync         bool    `json:"motionSync"`         // mem[0x0885] != 0
	PressDebounceRaw   byte    `json:"pressDebounceRaw"`   // Raw byte at 0x08C0
	PressDebounceStep  int     `json:"pressDebounceStep"`  // 0..6 on Gen-A
	PressDebounceMs    int     `json:"pressDebounceMs"`    // Rounded integer ms (0..32)
	PressDebounceLabel string  `json:"pressDebounceLabel"` // Exact human label e.g. "0.5 ms", "4 ms"
	ReleaseDebounceRaw byte    `json:"releaseDebounceRaw"` // Raw byte at 0x08C1
	ReleaseDebounceStep int    `json:"releaseDebounceStep"`
	ReleaseDebounceMs  int     `json:"releaseDebounceMs"`
	ReleaseDebounceLabel string `json:"releaseDebounceLabel"`
	SleepMinutes       int     `json:"sleepMinutes"`     // mem[0x08C2] (2..120)
	RippleControl      bool    `json:"rippleControl"`    // (~mem[0x08C3]) & 1 != 0
	LinearCorrection   bool    `json:"linearCorrection"` // ((mem[0x08C3] >> 1) ^ 1) & 1 != 0
	SensorAngleDeg     int8    `json:"sensorAngleDeg"`   // int8(mem[0x08C4]) (-30..+30)
	GlassMode          bool    `json:"glassMode"`        // mem[0x08C5] != 0
	LEDLowPowerMode    byte    `json:"ledLowPowerMode"`  // mem[0x08C6] (0 or 1)
}

// SensorPatch specifies user updates to SensorConfig fields.
type SensorPatch struct {
	LiftOffLevel        *int  `json:"liftOffLevel,omitempty"`        // 0..10 (1.0mm..2.0mm)
	MotionSync          *bool `json:"motionSync,omitempty"`          // ON/OFF
	PressDebounceStep   *int  `json:"pressDebounceStep,omitempty"`   // 0..6 (Gen-A)
	ReleaseDebounceStep *int  `json:"releaseDebounceStep,omitempty"` // 0..6 (Gen-A)
	SleepMinutes        *int  `json:"sleepMinutes,omitempty"`        // 2..120
	RippleControl       *bool `json:"rippleControl,omitempty"`       // ON/OFF
	LinearCorrection    *bool `json:"linearCorrection,omitempty"`    // ON/OFF
	SensorAngleDeg      *int  `json:"sensorAngleDeg,omitempty"`      // -30..+30
	GlassMode           *bool `json:"glassMode,omitempty"`           // ON/OFF
	LEDLowPowerMode     *byte `json:"ledLowPowerMode,omitempty"`     // 0..1
}

// DeviceSnapshot aggregates full read-only state for CLI inspection or the Wails UI.
type DeviceSnapshot struct {
	Model          ModelSpec       `json:"model"`
	CommandHID     string          `json:"commandHidPath"`
	ReportHID      string          `json:"reportHidPath,omitempty"`
	Battery        BatteryState    `json:"battery"`
	EstimatedHours float64         `json:"estimatedHours"`
	Firmware       *FirmwareInfo   `json:"firmware,omitempty"`
	DPI            *DPIConfig      `json:"dpi,omitempty"`
	Sensor         *SensorConfig   `json:"sensor,omitempty"`
	Buttons        []ButtonMapping `json:"buttons,omitempty"`
	DemoMode       bool            `json:"demoMode,omitempty"`
	StateRevision  uint64          `json:"stateRevision"`
	Generation     uint64          `json:"generation"`
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
