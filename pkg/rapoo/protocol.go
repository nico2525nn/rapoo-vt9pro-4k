package rapoo

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	// ReportIDCommand is the HID Report ID for request/response commands on Usage 0x000E (COL04).
	ReportIDCommand byte = 0xBA
	// ReportIDAsyncGenA is the 10-byte async status/wake Report ID on COL06 (Gen-A firmware).
	ReportIDAsyncGenA byte = 0x06
	// ReportIDAsyncBattery is the unsolicited battery/status Report ID on COL05 (Gen-B / A-HUB firmware).
	ReportIDAsyncBattery byte = 0xBB
	// ReportIDAsyncEvent is the unsolicited profile/DPI/key event Report ID on COL05.
	ReportIDAsyncEvent byte = 0xBD

	// PacketLength is the logical frame size for Report ID 0xBA commands.
	PacketLength = 32

	// TargetWirelessMouse addresses the remote wireless mouse across the 4K dongle RF link.
	TargetWirelessMouse byte = 0xA5
	// TargetLocalOrWired addresses a USB-wired mouse or the local 4K dongle MCU itself.
	TargetLocalOrWired byte = 0xFF

	// Response status codes in normalized resp[0].
	StatusUnsupported byte = 0x00 // Unrecognized opcode or uninitialized report buffer
	StatusOK          byte = 0x01 // Valid command response
	StatusPending     byte = 0x02 // Dongle busy polling RF or wireless mouse asleep

	// Onboard memory addresses (shared across Gen-A and Gen-B Realtek 4K firmwares).
	AddrPollingRate      uint32 = 0x0880
	AddrSilentHeight     uint32 = 0x0884
	AddrMotionSync       uint32 = 0x0885
	AddrDPIStagesX       uint32 = 0x0888
	AddrCurrentDPIGear   uint32 = 0x0898
	AddrSensorParameters uint32 = 0x08C0
	AddrSleepTimeout     uint32 = 0x08C2
	AddrDPIStagesY       uint32 = 0x08C8
)

var pollingCodeToHz = map[byte]int{
	0x08: 125,
	0x04: 250,
	0x02: 500,
	0x01: 1000,
	0x84: 2000,
	0x82: 4000,
	0x81: 8000,
}

var hzToPollingCode = map[int]byte{
	125:  0x08,
	250:  0x04,
	500:  0x02,
	1000: 0x01,
	2000: 0x84,
	4000: 0x82,
	8000: 0x81,
}

// NormalizeCommandResponse strips a leading 0xBA Report ID byte if present so
// Windows (which strips Report ID in HidD_GetInputReport payload) and Linux
// hidraw (which keeps buf[0] == 0xBA) produce identical slices.
func NormalizeCommandResponse(raw []byte) []byte {
	if len(raw) > 0 && raw[0] == ReportIDCommand {
		return raw[1:]
	}
	return raw
}

// BuildCommand constructs a 32-byte 0xBA command frame.
func BuildCommand(target, opcode byte, payload ...byte) ([]byte, error) {
	if 3+len(payload) > PacketLength {
		return nil, fmt.Errorf("rapoo: payload length %d exceeds %d-byte frame", len(payload), PacketLength)
	}
	pkt := make([]byte, PacketLength)
	pkt[0] = ReportIDCommand
	pkt[1] = target
	pkt[2] = opcode
	copy(pkt[3:], payload)
	return pkt, nil
}

// BuildBatteryQuery constructs the battery & charging status command packet
// (opcode 0xAA on Gen-A 0x1211/0x4411, 0xBA on Gen-B 0x1227/0x4427).
func BuildBatteryQuery(target, opcode byte) []byte {
	pkt, _ := BuildCommand(target, opcode)
	return pkt
}

// ParseBatteryResponse parses a normalized response from GetBattery (0xAA / 0xBA).
func ParseBatteryResponse(raw []byte, now time.Time) (BatteryState, error) {
	resp := NormalizeCommandResponse(raw)
	if len(resp) < 3 {
		return BatteryState{}, fmt.Errorf("%w: short battery response (%d bytes)", ErrInvalidResponse, len(resp))
	}
	switch resp[0] {
	case StatusOK:
		pct := int(resp[2])
		if pct > 100 {
			pct = 100
		}
		return BatteryState{
			Percentage: pct,
			Charging:   resp[1] > 1,
			Online:     true,
			Sleeping:   false,
			Source:     BatterySourceCommand,
			UpdatedAt:  now,
		}, nil

	case StatusPending:
		return BatteryState{
			Online:    false,
			Sleeping:  true,
			Source:    BatterySourceCommand,
			UpdatedAt: now,
		}, ErrMouseAsleep

	default:
		return BatteryState{}, fmt.Errorf("%w: unexpected status %#02x (raw=% x)", ErrInvalidResponse, resp[0], raw)
	}
}

// ParseAsyncReport decodes an unsolicited interrupt Input report from COL06 (Gen-A)
// or COL05 (Gen-B). Returns (nil, false) for unrelated or empty packets.
func ParseAsyncReport(raw []byte, now time.Time) (*AsyncEvent, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	switch raw[0] {
	case ReportIDAsyncBattery:
		if len(raw) < 7 {
			return nil, false
		}
		online := raw[3] != 0
		pct := int(raw[6])
		if pct > 100 {
			pct = 100
		}
		st := &BatteryState{
			Percentage: pct,
			Charging:   raw[5] > 1,
			Online:     online,
			Sleeping:   !online,
			Source:     BatterySourceAsync,
			UpdatedAt:  now,
		}
		return &AsyncEvent{
			Kind:    EventBattery,
			Battery: st,
			Raw:     raw,
		}, true

	case ReportIDAsyncEvent:
		if len(raw) >= 3 && raw[1] == 0x01 {
			profileID := int((raw[2] >> 4) & 0x0F)
			return &AsyncEvent{
				Kind:      EventProfile,
				ProfileID: profileID,
				Raw:       raw,
			}, true
		}
		return nil, false

	case ReportIDAsyncGenA:
		// 10-byte COL06 state notification emitted by VT9 Pro 4K on wake / DPI / cable change.
		return &AsyncEvent{
			Kind: EventWake,
			Raw:  raw,
		}, true

	default:
		return nil, false
	}
}

// BuildFirmwareQuery builds a mouse (0xA3/0xB3) or dongle (0xA1/0xB1) firmware query.
func BuildFirmwareQuery(target, opcode byte) []byte {
	pkt, _ := BuildCommand(target, opcode)
	return pkt
}

// ParseFirmwareResponse parses a 32-bit little-endian firmware version from resp[1:5].
func ParseFirmwareResponse(raw []byte) (uint32, error) {
	resp := NormalizeCommandResponse(raw)
	if len(resp) < 5 {
		return 0, fmt.Errorf("%w: short firmware response (%d bytes)", ErrInvalidResponse, len(resp))
	}
	if resp[0] == StatusPending {
		return 0, ErrMouseAsleep
	}
	if resp[0] != StatusOK {
		return 0, fmt.Errorf("%w: firmware status %#02x", ErrInvalidResponse, resp[0])
	}
	return binary.LittleEndian.Uint32(resp[1:5]), nil
}

// BuildReadMemory constructs an onboard memory read request (Opcode 0xA4 / 0xB4).
func BuildReadMemory(target, opcode byte, addr uint32, count byte) ([]byte, error) {
	if count == 0 || int(count) > PacketLength-4 {
		return nil, fmt.Errorf("rapoo: invalid memory read count %d", count)
	}
	var args [5]byte
	args[0] = count
	binary.LittleEndian.PutUint32(args[1:5], addr)
	return BuildCommand(target, opcode, args[:]...)
}

// ParseReadMemory extracts count data bytes starting at normalized resp[4].
func ParseReadMemory(raw []byte, count int) ([]byte, error) {
	resp := NormalizeCommandResponse(raw)
	if len(resp) == 0 {
		return nil, fmt.Errorf("%w: empty memory read response", ErrInvalidResponse)
	}
	if resp[0] == StatusPending {
		return nil, ErrMouseAsleep
	}
	if resp[0] != StatusOK {
		return nil, fmt.Errorf("%w: memory read status %#02x", ErrInvalidResponse, resp[0])
	}
	if len(resp) < 4+count {
		return nil, fmt.Errorf("%w: memory read response too short (%d < %d)", ErrInvalidResponse, len(resp), 4+count)
	}
	out := make([]byte, count)
	copy(out, resp[4:4+count])
	return out, nil
}

// BuildWriteMemory constructs an onboard memory write request (Opcode 0xA5 / 0xB5).
func BuildWriteMemory(target, opcode byte, addr uint32, data []byte) ([]byte, error) {
	if len(data) == 0 || len(data) > PacketLength-8 {
		return nil, fmt.Errorf("rapoo: invalid memory write length %d (max %d)", len(data), PacketLength-8)
	}
	args := make([]byte, 5+len(data))
	args[0] = byte(len(data))
	binary.LittleEndian.PutUint32(args[1:5], addr)
	copy(args[5:], data)
	return BuildCommand(target, opcode, args...)
}

// DecodePollingRate translates a firmware polling-rate byte at 0x0880 into Hz.
func DecodePollingRate(code byte) (int, bool) {
	hz, ok := pollingCodeToHz[code]
	return hz, ok
}

// EncodePollingRate translates a polling rate in Hz (125..8000) into its firmware code.
func EncodePollingRate(hz int) (byte, error) {
	code, ok := hzToPollingCode[hz]
	if !ok {
		return 0, fmt.Errorf("rapoo: unsupported polling rate %d Hz", hz)
	}
	return code, nil
}

// DecodeDPIConfig parses the 24-byte X-DPI block at 0x0888 and 20-byte Y-DPI block at 0x08C8.
func DecodeDPIConfig(mem888, mem8C8 []byte) (DPIConfig, error) {
	if len(mem888) < 17 {
		return DPIConfig{}, fmt.Errorf("%w: DPI X block too short (%d)", ErrInvalidResponse, len(mem888))
	}
	stageCount := int(mem888[14]) + 1
	if stageCount < 1 || stageCount > 7 {
		stageCount = 7
	}
	curStage := int(mem888[16]) + 1
	if curStage < 1 || curStage > stageCount {
		curStage = 1
	}

	stages := make([]DPIStage, stageCount)
	for i := 0; i < stageCount; i++ {
		x := binary.LittleEndian.Uint16(mem888[i*2 : i*2+2])
		y := x
		if len(mem8C8) >= (i+1)*2 {
			if yVal := binary.LittleEndian.Uint16(mem8C8[i*2 : i*2+2]); yVal > 0 {
				y = yVal
			}
		}
		stages[i] = DPIStage{
			Index: i + 1,
			X:     x,
			Y:     y,
		}
	}

	return DPIConfig{
		CurrentStage: curStage,
		StageCount:   stageCount,
		Stages:       stages,
	}, nil
}

// DecodeSensorConfig combines memory reads from 0x0880 (4B), 0x0884 (4B), and 0x08C0 (8B).
func DecodeSensorConfig(mem880, mem884, mem8C0 []byte) (SensorConfig, error) {
	if len(mem880) < 2 || len(mem884) < 2 || len(mem8C0) < 7 {
		return SensorConfig{}, fmt.Errorf("%w: sensor memory blocks too short", ErrInvalidResponse)
	}
	hz, _ := DecodePollingRate(mem880[0])
	pressMs := int(mem8C0[0]) - 2
	if pressMs < 0 {
		pressMs = 0
	}
	releaseMs := int(mem8C0[1]) - 2
	if releaseMs < 0 {
		releaseMs = 0
	}
	flags := mem8C0[3]

	return SensorConfig{
		PollingRateHz:     hz,
		Wireless8KSwitch:  mem880[1],
		SilentHeightLOD:   mem884[0],
		MotionSync:        mem884[1] != 0,
		PressDebounceMs:   pressMs,
		ReleaseDebounceMs: releaseMs,
		SleepMinutes:      int(mem8C0[2]),
		RippleControl:     ((^flags) & 0x01) != 0,
		LinearCorrection:  (((flags >> 1) ^ 0x01) & 0x01) != 0,
		SensorAngleDeg:    int8(mem8C0[4]),
		GlassMode:         mem8C0[5] != 0,
		LEDLowPowerMode:   mem8C0[6],
	}, nil
}
