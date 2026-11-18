package rapoo

import (
	"encoding/binary"
	"sync"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
)

// VirtualDevice emulates a Rapoo VT9 Pro 4K (PID 0x1211, Gen-A) onboard flash
// and USB control-pipe command responder for deterministic testing and UI Demo Mode.
type VirtualDevice struct {
	mu       sync.Mutex
	Provider *hid.MockProvider
	CmdDev   *hid.MockDevice
	RepDev   *hid.MockDevice

	BatteryPct int
	Charging   bool
	Sleeping   bool
	MouseFW    uint32
	DongleFW   uint32
	Mem        [0x1000]byte
	lastReply  [PacketLength]byte
}

// NewVirtualDevice creates a memory-backed virtual Rapoo VT9 Pro 4K (0x24AE:0x1211).
func NewVirtualDevice() *VirtualDevice {
	prov := hid.NewMockProvider()
	cmdDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            `virtual://rapoo/vt9pro4k/mi_01/col04`,
		VendorID:        VendorIDRapoo,
		ProductID:       0x1211,
		Product:         "Rapoo VT9 Pro 4K (Demo)",
		InterfaceNumber: VendorControlInterface,
		UsagePage:       0xFF00,
		Usage:           UsageCommandRealtek,
		InputReportLen:  PacketLength,
		OutputReportLen: PacketLength,
	})
	repDev := hid.NewMockDevice(hid.DeviceInfo{
		Path:            `virtual://rapoo/vt9pro4k/mi_01/col06`,
		VendorID:        VendorIDRapoo,
		ProductID:       0x1211,
		Product:         "Rapoo VT9 Pro 4K (Demo)",
		InterfaceNumber: VendorControlInterface,
		UsagePage:       0xFF00,
		Usage:           UsageAsyncReport,
		InputReportLen:  10,
	})

	vd := &VirtualDevice{
		Provider:   prov,
		CmdDev:     cmdDev,
		RepDev:     repDev,
		BatteryPct: 82,
		Charging:   false,
		Sleeping:   false,
		MouseFW:    125, // 1.25
		DongleFW:   222, // 2.22
	}
	vd.resetMemoryLocked()

	cmdDev.OnSetOutput = vd.handleSetOutput
	cmdDev.OnGetInput = vd.handleGetInput

	prov.Register(cmdDev)
	prov.Register(repDev)
	return vd
}

func (v *VirtualDevice) resetMemoryLocked() {
	for i := range v.Mem {
		v.Mem[i] = 0xFF
	}

	// Default 10-button layout at 0x0600..0x0637
	for _, slot := range VT9Pro4KButtons {
		raw, _ := EncodeButtonAction(ButtonAction{Type: slot.DefaultAction})
		copy(v.Mem[slot.Addr:slot.Addr+4], raw[:])
	}

	// 0x0880: 1000 Hz wireless (0x01), 8K off (0x00), 1000 Hz wired (0x01)
	copy(v.Mem[0x0880:0x0884], []byte{0x01, 0x00, 0x01, 0xFF})
	// 0x0884: LOD 1.1mm (level 1 -> raw 0x02), MotionSync ON (0x01)
	copy(v.Mem[0x0884:0x0888], []byte{0x02, 0x01, 0x01, 0x00})

	// 0x0888 & 0x08C8: 7 DPI stages [400, 800, 1200, 1600, 3200, 6400, 1000], stageCount=7 (6)
	dpis := [7]uint16{400, 800, 1200, 1600, 3200, 6400, 1000}
	for i, d := range dpis {
		binary.LittleEndian.PutUint16(v.Mem[0x0888+i*2:0x0888+i*2+2], d)
		binary.LittleEndian.PutUint16(v.Mem[0x08C8+i*2:0x08C8+i*2+2], d)
	}
	v.Mem[0x0896] = 6
	v.Mem[0x0897] = 0
	v.Mem[0x08D6] = 6
	v.Mem[0x08D7] = 0

	// 0x0898: Active stage 4 (index 3 -> 1600 DPI)
	copy(v.Mem[0x0898:0x089C], []byte{0x03, 0x00, 0x02, 0x01})

	// 0x08C0: Press=4ms (0x08), Release=4ms (0x08), Sleep=10m (0x0A), Ripple/Linear OFF (0x03), Angle=0, Glass=0, LED=1
	copy(v.Mem[0x08C0:0x08C8], []byte{0x08, 0x08, 0x0A, 0x03, 0x00, 0x00, 0x01, 0x00})
}

func (v *VirtualDevice) handleSetOutput(tx []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	var resp [PacketLength]byte
	v.lastReply = resp
	if len(tx) < 3 || tx[0] != ReportIDCommand {
		return nil
	}

	target := tx[1]
	opcode := tx[2]

	// Dongle firmware version works even while wireless mouse is asleep.
	if target == TargetLocalOrWired && opcode == OpcodesGenA.DongleVersion {
		resp[0] = StatusOK
		binary.LittleEndian.PutUint32(resp[1:5], v.DongleFW)
		v.lastReply = resp
		return nil
	}

	if v.Sleeping {
		resp[0] = StatusPending
		v.lastReply = resp
		return nil
	}

	switch opcode {
	case OpcodesGenA.GetBattery:
		resp[0] = StatusOK
		if v.Charging {
			resp[1] = 2
		} else {
			resp[1] = 1
		}
		resp[2] = byte(v.BatteryPct)

	case OpcodesGenA.MouseVersion:
		resp[0] = StatusOK
		binary.LittleEndian.PutUint32(resp[1:5], v.MouseFW)

	case OpcodesGenA.ReadMemory:
		if len(tx) >= 8 {
			count := int(tx[3])
			addr := int(binary.LittleEndian.Uint32(tx[4:8]))
			if count > 0 && count <= PacketLength-4 && addr >= 0 && addr+count <= len(v.Mem) {
				resp[0] = StatusOK
				copy(resp[4:4+count], v.Mem[addr:addr+count])
			}
		}

	case OpcodesGenA.WriteMemory:
		if len(tx) >= 8 {
			count := int(tx[3])
			addr := int(binary.LittleEndian.Uint32(tx[4:8]))
			if count > 0 && 8+count <= len(tx) && addr >= 0 && addr+count <= len(v.Mem) {
				copy(v.Mem[addr:addr+count], tx[8:8+count])
				resp[0] = StatusOK
			}
		}

	case OpcodesGenA.FactoryReset:
		if len(tx) >= 5 && tx[3] == 0x52 && tx[4] == 0x3D {
			v.resetMemoryLocked()
			resp[0] = StatusOK
		}
	}

	v.lastReply = resp
	return nil
}

func (v *VirtualDevice) handleGetInput(_ byte, length int) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]byte, length)
	copy(out, v.lastReply[:])
	return out, nil
}
