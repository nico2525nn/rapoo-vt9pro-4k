package rapoo

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nico/rapoo-vt9pro-4k/pkg/hid"
)

const (
	// VendorIDRapoo is Shenzhen Rapoo Technology Co., Ltd.
	VendorIDRapoo uint16 = 0x24AE

	// VendorControlInterface is USB MI_01 (MI_00 is the 4000Hz HID Mouse interface).
	VendorControlInterface = 1

	// UsageCommandRealtek is the top-level Usage for the 0xBA command/response collection (COL04).
	UsageCommandRealtek uint16 = 0x000E
	// UsageAsyncReport is the top-level Usage for the async notification collections (COL05/COL06).
	UsageAsyncReport uint16 = 0x0002
)

// OpcodeSet groups firmware-generation-specific command opcodes.
type OpcodeSet struct {
	Name          string `json:"name"`
	DongleVersion byte   `json:"-"`
	MouseVersion  byte   `json:"-"`
	ReadMemory    byte   `json:"-"`
	WriteMemory   byte   `json:"-"`
	GetProfile    byte   `json:"-"`
	SetProfile    byte   `json:"-"`
	GetBattery    byte   `json:"-"`
	FactoryReset  byte   `json:"-"`
	ResetMagic    []byte `json:"-"`
	GenADebounce  bool   `json:"genADebounce"`
}

var (
	// OpcodesGenA is used by VT9 Pro 4K (0x1211 / 0x4411, RapooGameDev QDeviceVT9PRO_4K).
	OpcodesGenA = OpcodeSet{
		Name:          "realtek_0xAx",
		DongleVersion: 0xA3,
		MouseVersion:  0xA3,
		ReadMemory:    0xA4,
		WriteMemory:   0xA5,
		GetProfile:    0xA7,
		SetProfile:    0xA8,
		GetBattery:    0xAA,
		FactoryReset:  0xAD,
		ResetMagic:    []byte{0x52, 0x3D},
		GenADebounce:  true,
	}

	// OpcodesGenB is used by VT9 Pro 4K Rev B / A-HUB firmware (0x1227 / 0x4427, MouseForRealtek).
	OpcodesGenB = OpcodeSet{
		Name:          "realtek_0xBx",
		DongleVersion: 0xB1,
		MouseVersion:  0xB3,
		ReadMemory:    0xB4,
		WriteMemory:   0xB5,
		GetProfile:    0xB7,
		SetProfile:    0xB8,
		GetBattery:    0xBA,
		FactoryReset:  0xD0,
		ResetMagic:    nil,
		GenADebounce:  false,
	}
)

func alternateOpcodes(cur OpcodeSet) OpcodeSet {
	if cur.GetBattery == OpcodesGenA.GetBattery {
		return OpcodesGenB
	}
	return OpcodesGenA
}

// ModelSpec defines static protocol properties for a supported Rapoo mouse PID.
type ModelSpec struct {
	Name          string         `json:"name"`
	VendorID      uint16         `json:"vendorId"`
	ProductID     uint16         `json:"productId"`
	Connection    ConnectionMode `json:"connection"`
	ProtocolGen   int            `json:"protocolGen"`
	TargetByte    byte           `json:"targetByte"`
	Opcodes       OpcodeSet      `json:"opcodes"`
	PreferColName string         `json:"-"` // "col06" (10B) on GenA, "col05" (7B) on GenB
	PreferRepLen  int            `json:"-"`
	StepDelay     time.Duration  `json:"-"`
	MaxRateHz     int            `json:"maxRateHz"`
	Priority      int            `json:"-"` // Higher wins when both wired & dongle are connected
}

// SupportedProductIDs returns the unique PIDs of supported models.
func SupportedProductIDs() []uint16 {
	seen := make(map[uint16]bool)
	ids := make([]uint16, 0, len(SupportedModels))
	for _, model := range SupportedModels {
		if model.ProductID == 0 || seen[model.ProductID] {
			continue
		}
		seen[model.ProductID] = true
		ids = append(ids, model.ProductID)
	}
	return ids
}

// SupportedModels lists all Rapoo VT9 Pro 4K family PIDs.
var SupportedModels = []ModelSpec{
	{
		Name:          "Rapoo VT9 Pro 4K",
		VendorID:      VendorIDRapoo,
		ProductID:     0x4411,
		Connection:    ConnectionWired,
		ProtocolGen:   1,
		TargetByte:    TargetLocalOrWired,
		Opcodes:       OpcodesGenA,
		PreferColName: "col06",
		PreferRepLen:  10,
		StepDelay:     5 * time.Millisecond,
		MaxRateHz:     1000,
		Priority:      20, // Prefer wired USB-C when plugged in for charging/wired play
	},
	{
		Name:          "Rapoo VT9 Pro 4K",
		VendorID:      VendorIDRapoo,
		ProductID:     0x1211,
		Connection:    ConnectionWireless4K,
		ProtocolGen:   1,
		TargetByte:    TargetWirelessMouse,
		Opcodes:       OpcodesGenA,
		PreferColName: "col06",
		PreferRepLen:  10,
		StepDelay:     35 * time.Millisecond,
		MaxRateHz:     4000,
		Priority:      10,
	},
	{
		Name:          "Rapoo VT9 Pro 4K (Rev B)",
		VendorID:      VendorIDRapoo,
		ProductID:     0x4427,
		Connection:    ConnectionWired,
		ProtocolGen:   2,
		TargetByte:    TargetLocalOrWired,
		Opcodes:       OpcodesGenB,
		PreferColName: "col05",
		PreferRepLen:  7,
		StepDelay:     5 * time.Millisecond,
		MaxRateHz:     1000,
		Priority:      19,
	},
	{
		Name:          "Rapoo VT9 Pro 4K (Rev B)",
		VendorID:      VendorIDRapoo,
		ProductID:     0x1227,
		Connection:    ConnectionWireless4K,
		ProtocolGen:   2,
		TargetByte:    TargetWirelessMouse,
		Opcodes:       OpcodesGenB,
		PreferColName: "col05",
		PreferRepLen:  7,
		StepDelay:     35 * time.Millisecond,
		MaxRateHz:     4000,
		Priority:      9,
	},
}

// LookupModel returns the ModelSpec for a given VID/PID pair.
func LookupModel(vid, pid uint16) (ModelSpec, bool) {
	for _, m := range SupportedModels {
		if m.VendorID == vid && m.ProductID == pid {
			return m, true
		}
	}
	return ModelSpec{}, false
}

// EndpointPair pairs the 0xBA command collection with the optional async
// notification collection for a single physical Rapoo device.
type EndpointPair struct {
	Model   ModelSpec
	Command *hid.DeviceInfo
	Report  *hid.DeviceInfo
}

// DefaultHIDFilter returns a gaming-safe HID enumeration filter restricted to
// Rapoo's vendor interface MI_01 (never touching MI_00 mouse input).
func DefaultHIDFilter() hid.Filter {
	return hid.Filter{
		VendorID:        VendorIDRapoo,
		ProductIDs:      SupportedProductIDs(),
		InterfaceNumber: VendorControlInterface,
	}
}

// MatchEndpoints groups enumerated HID collections by PID and selects the
// Command (Usage 0x000E) and Async Report (Usage 0x0002) collections, ordered
// by priority (Wired first, then 4K Wireless Dongle).
func MatchEndpoints(infos []hid.DeviceInfo) []EndpointPair {
	byDevice := make(map[string]*EndpointPair)

	for i := range infos {
		info := infos[i]
		model, ok := LookupModel(info.VendorID, info.ProductID)
		if !ok {
			continue
		}
		if info.InterfaceNumber == 0 {
			continue
		}
		if info.InterfaceNumber > 0 && info.InterfaceNumber != VendorControlInterface {
			continue
		}
		if info.UsagePage != 0 && info.UsagePage < 0xFF00 {
			continue
		}

		key := fmt.Sprintf("%04x:%04x", info.VendorID, info.ProductID)
		if info.PhysicalID != "" {
			key += "|" + info.PhysicalID
		}
		pair, exists := byDevice[key]
		if !exists {
			pair = &EndpointPair{Model: model}
			byDevice[key] = pair
		}

		switch info.Usage {
		case UsageCommandRealtek:
			cp := info
			pair.Command = &cp
		case UsageAsyncReport:
			cp := info
			if pair.Report == nil || prefersReportEndpoint(model, cp, *pair.Report) {
				pair.Report = &cp
			}
		}
	}

	var out []EndpointPair
	for _, pair := range byDevice {
		if pair.Command == nil && pair.Report == nil {
			continue
		}
		out = append(out, *pair)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Model.Priority > out[j].Model.Priority
	})
	return out
}

func prefersReportEndpoint(model ModelSpec, cand, curr hid.DeviceInfo) bool {
	if model.PreferColName != "" {
		candMatch := strings.Contains(strings.ToLower(cand.Path), model.PreferColName)
		currMatch := strings.Contains(strings.ToLower(curr.Path), model.PreferColName)
		if candMatch != currMatch {
			return candMatch
		}
	}
	if model.PreferRepLen > 0 {
		candLen := cand.InputReportLen == model.PreferRepLen
		currLen := curr.InputReportLen == model.PreferRepLen
		if candLen != currLen {
			return candLen
		}
	}
	return cand.Path < curr.Path
}
