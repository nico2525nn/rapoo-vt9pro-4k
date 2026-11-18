package icon

import (
	"encoding/binary"
	"testing"
)

func TestRenderImageAndICOAcrossSizesAndStates(t *testing.T) {
	specs := []VisualSpec{
		{Connected: false},
		{Connected: true, HasBattery: true, Percentage: 100, Charging: true},
		{Connected: true, HasBattery: true, Percentage: 85},
		{Connected: true, HasBattery: true, Percentage: 25},
		{Connected: true, HasBattery: true, Percentage: 9, Sleeping: true},
		{Connected: true, HasBattery: true, Percentage: 0},
	}

	for _, sz := range []int{16, 20, 24, 32} {
		for _, sp := range specs {
			img := RenderImage(sz, sp)
			if img.Bounds().Dx() != sz || img.Bounds().Dy() != sz {
				t.Fatalf("unexpected bounds %v for size %d", img.Bounds(), sz)
			}
			// Verify non-empty visible pixels
			opaque := 0
			for y := 0; y < sz; y++ {
				for x := 0; x < sz; x++ {
					if img.NRGBAAt(x, y).A > 0 {
						opaque++
					}
				}
			}
			if opaque == 0 {
				t.Fatalf("rendered completely transparent icon for size=%d spec=%+v", sz, sp)
			}

			dib := EncodeDIB(img)
			if gotHeader := binary.LittleEndian.Uint32(dib[0:4]); gotHeader != 40 {
				t.Fatalf("BITMAPINFOHEADER size = %d, want 40", gotHeader)
			}
		}
	}

	ico := RenderICO(VisualSpec{Connected: true, HasBattery: true, Percentage: 100}, 16, 20, 24, 32)
	if len(ico) < 6+4*16 {
		t.Fatalf("ICO too small: %d bytes", len(ico))
	}
	if count := binary.LittleEndian.Uint16(ico[4:6]); count != 4 {
		t.Fatalf("ICO entry count = %d, want 4", count)
	}
}
