package rapoo

import (
	"encoding/binary"
	"fmt"
	"math"
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
	AddrStageCount       uint32 = 0x0896
	AddrCurrentDPIGear   uint32 = 0x0898
	AddrSensorParameters uint32 = 0x08C0
	AddrSleepTimeout     uint32 = 0x08C2
	AddrSensorAngle      uint32 = 0x08C4
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

// BuildFactoryReset constructs the factory reset command packet
// (0xAD 0x52 0x3D on Gen-A, 0xD0 on Gen-B).
func BuildFactoryReset(target byte, ops OpcodeSet) []byte {
	pkt, _ := BuildCommand(target, ops.FactoryReset, ops.ResetMagic...)
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

// ParseFirmwareResponse parses a firmware version from resp[1:5] (supporting both
// BCD resp[2]:resp[1] when resp[2]>0 and raw uint16/uint32 when resp[2]==0).
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
	// If resp[2] > 0 and both resp[1], resp[2] are valid BCD bytes (nibbles 0..9), decode BCD major*100 + minor.
	isBCD := func(b byte) bool { return (b>>4) <= 9 && (b&0x0F) <= 9 }
	fromBCD := func(b byte) uint32 { return uint32(b>>4)*10 + uint32(b&0x0F) }
	if resp[2] > 0 && resp[3] == 0 && resp[4] == 0 && isBCD(resp[2]) && isBCD(resp[1]) {
		return fromBCD(resp[2])*100 + fromBCD(resp[1]), nil
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

// DecodeDPIConfig parses the 24-byte X-DPI block at 0x0888 and 24-byte Y-DPI block at 0x08C8.
func DecodeDPIConfig(mem888, mem8C8 []byte) (DPIConfig, error) {
	if len(mem888) < 17 {
		return DPIConfig{}, fmt.Errorf("%w: DPI X block too short (%d)", ErrInvalidResponse, len(mem888))
	}
	stageCount := int(mem888[14]) + 1
	if stageCount < 1 || stageCount > MaxDPIStage {
		stageCount = MaxDPIStage
	}
	curStage := int(mem888[16]) + 1
	if curStage < 1 || curStage > stageCount {
		curStage = 1
	}

	allStages := make([]DPIStage, MaxDPIStage)
	indepXY := false
	for i := 0; i < MaxDPIStage; i++ {
		x := binary.LittleEndian.Uint16(mem888[i*2 : i*2+2])
		if x == 0 || x == 0xFFFF {
			x = 800
		}
		y := x
		if len(mem8C8) >= (i+1)*2 {
			if yVal := binary.LittleEndian.Uint16(mem8C8[i*2 : i*2+2]); yVal >= MinDPI && yVal <= MaxDPI {
				y = yVal
			}
		}
		enabled := i < stageCount
		if enabled && x != y {
			indepXY = true
		}
		allStages[i] = DPIStage{
			Index:   i + 1,
			X:       x,
			Y:       y,
			Enabled: enabled,
		}
	}

	stages := make([]DPIStage, stageCount)
	copy(stages, allStages[:stageCount])

	return DPIConfig{
		CurrentStage:  curStage,
		StageCount:    stageCount,
		IndependentXY: indepXY,
		Stages:        stages,
		AllStages:     allStages,
	}, nil
}

// EncodeDPIBlocks encodes 7 X/Y DPI slots and stageCount into 16-byte write blocks for 0x0888 and 0x08C8.
func EncodeDPIBlocks(stages []DPIStage, stageCount int) (blockX [16]byte, blockY [16]byte, err error) {
	if stageCount < 1 || stageCount > MaxDPIStage {
		return blockX, blockY, fmt.Errorf("rapoo: DPI stageCount %d out of range (1..7)", stageCount)
	}
	if len(stages) < stageCount {
		return blockX, blockY, fmt.Errorf("rapoo: got %d DPI stages for stageCount %d", len(stages), stageCount)
	}

	defaults := [7]uint16{400, 800, 1200, 1600, 3200, 6400, 26000}
	for i := 0; i < MaxDPIStage; i++ {
		x := defaults[i]
		y := defaults[i]
		if i < len(stages) {
			x = NormalizeDPI(stages[i].X)
			if stages[i].Y > 0 {
				y = NormalizeDPI(stages[i].Y)
			} else {
				y = x
			}
		}
		binary.LittleEndian.PutUint16(blockX[i*2:i*2+2], x)
		binary.LittleEndian.PutUint16(blockY[i*2:i*2+2], y)
	}
	blockX[14] = byte(stageCount - 1)
	blockX[15] = 0x00
	blockY[14] = byte(stageCount - 1)
	blockY[15] = 0x00
	return blockX, blockY, nil
}

// DecodeGenADebounce maps a Gen-A raw debounce byte (1, 2, 4, 8, 16, 24, 32) to step index, ms, and label.
func DecodeGenADebounce(raw byte) (step int, roundedMs int, label string) {
	best := GenADebounceSteps[3] // 4 ms default
	bestDist := 999
	for _, s := range GenADebounceSteps {
		d := int(raw) - int(s.RawByte)
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			best = s
		}
	}
	return best.Index, int(math.Round(best.Ms)), best.Label
}

// EncodeGenADebounceStep maps a step index (0..6) to its exact Gen-A raw byte.
func EncodeGenADebounceStep(step int) (byte, error) {
	if step < 0 || step >= len(GenADebounceSteps) {
		return 0, fmt.Errorf("rapoo: debounce step %d out of range (0..%d)", step, len(GenADebounceSteps)-1)
	}
	return GenADebounceSteps[step].RawByte, nil
}

// DecodeSensorConfig decodes 0x0880 (4B), 0x0884 (4B), and 0x08C0 (8B) using Gen-A defaults.
func DecodeSensorConfig(mem880, mem884, mem8C0 []byte) (SensorConfig, error) {
	return DecodeSensorConfigForGeneration(mem880, mem884, mem8C0, true)
}

// DecodeSensorConfigForGeneration decodes sensor/switch parameters with generation-specific debounce scaling.
func DecodeSensorConfigForGeneration(mem880, mem884, mem8C0 []byte, genADebounce bool) (SensorConfig, error) {
	if len(mem880) < 2 || len(mem884) < 2 || len(mem8C0) < 7 {
		return SensorConfig{}, fmt.Errorf("%w: sensor memory blocks too short", ErrInvalidResponse)
	}
	hz, _ := DecodePollingRate(mem880[0])

	lodRaw := mem884[0]
	lodLevel := int(lodRaw) - 1
	if lodLevel < 0 {
		lodLevel = 0
	}
	if lodLevel > 10 {
		lodLevel = 10
	}
	lodMm := math.Round((1.0+float64(lodLevel)*0.1)*10) / 10

	var pressStep, pressMs int
	var pressLabel string
	var relStep, relMs int
	var relLabel string

	if genADebounce {
		pressStep, pressMs, pressLabel = DecodeGenADebounce(mem8C0[0])
		relStep, relMs, relLabel = DecodeGenADebounce(mem8C0[1])
	} else {
		pressMs = int(mem8C0[0]) - 2
		if pressMs < 0 {
			pressMs = 0
		}
		pressStep = pressMs
		pressLabel = fmt.Sprintf("%d ms", pressMs)

		relMs = int(mem8C0[1]) - 2
		if relMs < 0 {
			relMs = 0
		}
		relStep = relMs
		relLabel = fmt.Sprintf("%d ms", relMs)
	}

	flags := mem8C0[3]
	angle := int8(mem8C0[4])
	if angle < -30 {
		angle = -30
	}
	if angle > 30 {
		angle = 30
	}

	return SensorConfig{
		PollingRateHz:        hz,
		Wireless8KSwitch:     mem880[1],
		SilentHeightLOD:      lodRaw,
		LiftOffLevel:         lodLevel,
		LiftOffDistanceMm:    lodMm,
		MotionSync:           mem884[1] != 0,
		PressDebounceRaw:     mem8C0[0],
		PressDebounceStep:    pressStep,
		PressDebounceMs:      pressMs,
		PressDebounceLabel:   pressLabel,
		ReleaseDebounceRaw:   mem8C0[1],
		ReleaseDebounceStep:  relStep,
		ReleaseDebounceMs:    relMs,
		ReleaseDebounceLabel: relLabel,
		SleepMinutes:         int(mem8C0[2]),
		RippleControl:        ((^flags) & 0x01) != 0,
		LinearCorrection:     (((flags >> 1) ^ 0x01) & 0x01) != 0,
		SensorAngleDeg:       angle,
		GlassMode:            mem8C0[5] != 0,
		LEDLowPowerMode:      mem8C0[6],
	}, nil
}

// EncodeRippleLinearByte encodes RippleControl and LinearCorrection into mem[0x08C3]
// matching QDeviceVT9PRO_4K (0x03 = both OFF, 0x02 = Ripple ON, 0x01 = Linear ON, 0x00 = both ON).
func EncodeRippleLinearByte(ripple, linear bool) byte {
	var b byte
	if !ripple {
		b |= 0x01
	}
	if !linear {
		b |= 0x02
	}
	return b
}
