package hid

import "testing"

func TestParseWindowsDevicePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantVID   uint16
		wantPID   uint16
		wantIface int
		wantOK    bool
	}{
		{
			name:      "vt9pro 4k mi_01 col04",
			path:      `\\?\hid#vid_24ae&pid_1211&mi_01&col04#8&2a9c4b1&0&0003#{4d1e55b2-f16f-11cf-88cb-001111000030}`,
			wantVID:   0x24AE,
			wantPID:   0x1211,
			wantIface: 1,
			wantOK:    true,
		},
		{
			name:      "vt9pro 4k mi_00 mouse",
			path:      `\\?\HID#VID_24AE&PID_1211&MI_00#8&11223344&0&0000#{4d1e55b2-f16f-11cf-88cb-001111000030}`,
			wantVID:   0x24AE,
			wantPID:   0x1211,
			wantIface: 0,
			wantOK:    true,
		},
		{
			name:      "non-composite device",
			path:      `\\?\hid#vid_046d&pid_c547#7&123456&0&0000#{4d1e55b2-f16f-11cf-88cb-001111000030}`,
			wantVID:   0x046D,
			wantPID:   0xC547,
			wantIface: -1,
			wantOK:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vid, pid, iface, ok := ParseWindowsDevicePath(tc.path)
			if ok != tc.wantOK || vid != tc.wantVID || pid != tc.wantPID || iface != tc.wantIface {
				t.Fatalf("got (vid=%#x, pid=%#x, iface=%d, ok=%v), want (%#x, %#x, %d, %v)",
					vid, pid, iface, ok, tc.wantVID, tc.wantPID, tc.wantIface, tc.wantOK)
			}
		})
	}
}

func TestParseReportDescriptor(t *testing.T) {
	// Vendor-defined collection: UsagePage 0xFF00, Usage 0x000E, Report ID 0xBA,
	// 31 bytes Input + 31 bytes Output => total 32 bytes incl. Report ID.
	desc := []byte{
		0x06, 0x00, 0xFF, // Usage Page (Vendor Defined 0xFF00)
		0x09, 0x0E, // Usage (0x0E)
		0xA1, 0x01, // Collection (Application)
		0x85, 0xBA, //   Report ID (0xBA)
		0x09, 0x01, //   Usage (0x01)
		0x75, 0x08, //   Report Size (8)
		0x95, 0x1F, //   Report Count (31)
		0x81, 0x02, //   Input (Data,Var,Abs)
		0x09, 0x02, //   Usage (0x02)
		0x75, 0x08, //   Report Size (8)
		0x95, 0x1F, //   Report Count (31)
		0x91, 0x02, //   Output (Data,Var,Abs)
		0xC0, // End Collection
	}

	got := ParseReportDescriptor(desc)
	if got.UsagePage != 0xFF00 {
		t.Fatalf("UsagePage = %#x, want 0xFF00", got.UsagePage)
	}
	if got.Usage != 0x000E {
		t.Fatalf("Usage = %#x, want 0x000E", got.Usage)
	}
	if got.InputReportLen != 32 {
		t.Fatalf("InputReportLen = %d, want 32", got.InputReportLen)
	}
	if got.OutputReportLen != 32 {
		t.Fatalf("OutputReportLen = %d, want 32", got.OutputReportLen)
	}
}
