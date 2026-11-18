package hid

import (
	"strconv"
	"strings"
)

// ParseWindowsDevicePath extracts VID, PID, and MI_xx interface number from a
// Windows SetupAPI device interface path without touching the device handle.
// Example path: \\?\hid#vid_24ae&pid_1211&mi_01&col04#8&1a2b3c4d&0&0003#{4d1e55b2-...}
func ParseWindowsDevicePath(path string) (vid uint16, pid uint16, iface int, ok bool) {
	lower := strings.ToLower(path)
	iface = -1

	vidIdx := strings.Index(lower, "vid_")
	pidIdx := strings.Index(lower, "pid_")
	if vidIdx >= 0 && len(lower) >= vidIdx+8 {
		if v, err := strconv.ParseUint(lower[vidIdx+4:vidIdx+8], 16, 16); err == nil {
			vid = uint16(v)
		}
	}
	if pidIdx >= 0 && len(lower) >= pidIdx+8 {
		if p, err := strconv.ParseUint(lower[pidIdx+4:pidIdx+8], 16, 16); err == nil {
			pid = uint16(p)
		}
	}
	if miIdx := strings.Index(lower, "&mi_"); miIdx >= 0 && len(lower) >= miIdx+6 {
		if m, err := strconv.ParseInt(lower[miIdx+4:miIdx+6], 16, 32); err == nil {
			iface = int(m)
		}
	} else if miIdx := strings.Index(lower, "#mi_"); miIdx >= 0 && len(lower) >= miIdx+6 {
		if m, err := strconv.ParseInt(lower[miIdx+4:miIdx+6], 16, 32); err == nil {
			iface = int(m)
		}
	}

	return vid, pid, iface, vid != 0 && pid != 0
}

// DescriptorSummary holds top-level UsagePage, Usage, and maximum report byte
// lengths (including 1 byte for Report ID when Report IDs are present) parsed
// from a raw USB HID Report Descriptor.
type DescriptorSummary struct {
	UsagePage        uint16
	Usage            uint16
	InputReportLen   int
	OutputReportLen  int
	FeatureReportLen int
}

// ParseReportDescriptor parses a binary USB HID Report Descriptor to extract
// the first top-level Application Collection's UsagePage and Usage, as well as
// Input/Output/Feature report byte lengths. Used primarily on Linux hidraw.
func ParseReportDescriptor(desc []byte) DescriptorSummary {
	var (
		out            DescriptorSummary
		gotTopUsage    bool
		collectionLvl  int
		curUsagePage   uint16
		curUsage       uint16
		hasUsage       bool
		reportSizeBits int
		reportCount    int
		hasReportID    bool
		inBitsByID     = map[byte]int{}
		outBitsByID    = map[byte]int{}
		featBitsByID   = map[byte]int{}
		curReportID    byte
	)

	for i := 0; i < len(desc); {
		prefix := desc[i]
		if prefix == 0xFE {
			// Long item: [0xFE, dataSize, longTag, data...]
			if i+2 >= len(desc) {
				break
			}
			sz := int(desc[i+1])
			i += 3 + sz
			continue
		}

		sizeCode := prefix & 0x03
		var size int
		switch sizeCode {
		case 0:
			size = 0
		case 1:
			size = 1
		case 2:
			size = 2
		case 3:
			size = 4
		}
		if i+1+size > len(desc) {
			break
		}

		var val uint32
		for b := 0; b < size; b++ {
			val |= uint32(desc[i+1+b]) << (8 * b)
		}

		itemType := (prefix >> 2) & 0x03
		tag := (prefix >> 4) & 0x0F

		switch itemType {
		case 0: // Main
			switch tag {
			case 0x0A: // Collection
				if collectionLvl == 0 && !gotTopUsage {
					out.UsagePage = curUsagePage
					if hasUsage {
						out.Usage = curUsage
					}
					gotTopUsage = true
				}
				collectionLvl++
			case 0x0C: // End Collection
				if collectionLvl > 0 {
					collectionLvl--
				}
			case 0x08: // Input
				inBitsByID[curReportID] += reportSizeBits * reportCount
			case 0x09: // Output
				outBitsByID[curReportID] += reportSizeBits * reportCount
			case 0x0B: // Feature
				featBitsByID[curReportID] += reportSizeBits * reportCount
			}
			// Local items reset after every Main item.
			curUsage = 0
			hasUsage = false

		case 1: // Global
			switch tag {
			case 0x00: // Usage Page
				curUsagePage = uint16(val)
			case 0x07: // Report Size
				reportSizeBits = int(val)
			case 0x08: // Report ID
				hasReportID = true
				curReportID = byte(val)
			case 0x09: // Report Count
				reportCount = int(val)
			}

		case 2: // Local
			switch tag {
			case 0x00: // Usage
				if !hasUsage {
					curUsage = uint16(val)
					hasUsage = true
				}
			}
		}

		i += 1 + size
	}

	extraIDByte := 0
	if hasReportID {
		extraIDByte = 1
	}
	out.InputReportLen = maxReportBytes(inBitsByID, extraIDByte)
	out.OutputReportLen = maxReportBytes(outBitsByID, extraIDByte)
	out.FeatureReportLen = maxReportBytes(featBitsByID, extraIDByte)
	return out
}

func maxReportBytes(bitsByID map[byte]int, extra int) int {
	maxB := 0
	for _, bits := range bitsByID {
		if bits <= 0 {
			continue
		}
		b := (bits+7)/8 + extra
		if b > maxB {
			maxB = b
		}
	}
	return maxB
}
