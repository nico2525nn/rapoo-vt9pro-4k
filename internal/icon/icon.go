// Package icon renders crisp, pixel-aligned HiDPI battery percentage icons
// (DIB resources and .ico files) for the Windows system tray with zero external
// font or image library dependencies.
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
)

// VisualSpec describes what to render on the tray icon.
type VisualSpec struct {
	Connected  bool
	HasBattery bool
	Percentage int // 0..100
	Charging   bool
	Sleeping   bool
}

var (
	colOutline      = color.NRGBA{R: 10, G: 15, B: 26, A: 245}
	colBarTrack     = color.NRGBA{R: 15, G: 23, B: 42, A: 230}
	colTextNormal   = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colTextCharging = color.NRGBA{R: 125, G: 211, B: 252, A: 255} // Sky-300
	colTextWarn     = color.NRGBA{R: 253, G: 224, B: 71, A: 255}  // Yellow-300
	colTextLow      = color.NRGBA{R: 252, G: 165, B: 165, A: 255} // Red-300
	colTextSleep    = color.NRGBA{R: 203, G: 213, B: 225, A: 225} // Slate-300
	colTextOffline  = color.NRGBA{R: 148, G: 163, B: 184, A: 220} // Slate-400

	colBarGood     = color.NRGBA{R: 34, G: 197, B: 94, A: 255}  // Green-500
	colBarCharging = color.NRGBA{R: 56, G: 189, B: 248, A: 255} // Sky-400
	colBarWarn     = color.NRGBA{R: 250, G: 204, B: 21, A: 255} // Yellow-400
	colBarLow      = color.NRGBA{R: 239, G: 68, B: 68, A: 255}  // Red-500
	colBarSleep    = color.NRGBA{R: 100, G: 116, B: 139, A: 230}
)

// RenderImage draws a crisp pixel-aligned tray icon of size x size pixels.
func RenderImage(size int, spec VisualSpec) *image.NRGBA {
	if size < 16 {
		size = 16
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	text := "--"
	if spec.Connected && spec.HasBattery {
		pct := spec.Percentage
		if pct < 0 {
			pct = 0
		} else if pct > 100 {
			pct = 100
		}
		text = formatInt(pct)
	}

	fg, barCol := choosePalette(spec)

	// Layout bottom battery bar
	barH := maxInt(2, size/8)
	barY := size - barH
	barPadX := maxInt(1, size/16)
	barW := size - 2*barPadX

	// Draw dark track behind bottom bar for contrast on light taskbars
	fillRect(img, barPadX, barY-1, barW, barH+1, colBarTrack)
	if spec.Connected && spec.HasBattery {
		pct := clampInt(spec.Percentage, 0, 100)
		fillW := (barW * pct) / 100
		if pct > 0 && fillW < 1 {
			fillW = 1
		}
		if fillW > 0 {
			fillRect(img, barPadX, barY, fillW, barH, barCol)
		}
	}

	// Available vertical area above the bar for text + outline
	availH := barY - 1
	drawOutlinedDigits(img, size, availH, text, fg)

	return img
}

func choosePalette(spec VisualSpec) (fg, bar color.NRGBA) {
	if !spec.Connected || !spec.HasBattery {
		return colTextOffline, colBarSleep
	}
	if spec.Charging {
		return colTextCharging, colBarCharging
	}
	if spec.Sleeping {
		if spec.Percentage <= 15 {
			return colTextLow, colBarLow
		}
		return colTextSleep, colBarSleep
	}
	switch {
	case spec.Percentage <= 15:
		return colTextLow, colBarLow
	case spec.Percentage <= 30:
		return colTextWarn, colBarWarn
	default:
		return colTextNormal, colBarGood
	}
}

func drawOutlinedDigits(dst *image.NRGBA, canvasW, availH int, text string, fg color.NRGBA) {
	mask := make([]bool, canvasW*canvasW)

	// Pick integer glyph metrics that guarantee uniform stroke width and crisp holes
	var digitW, digitH, oneW, stroke, gap int
	isThree := len(text) >= 3

	switch {
	case canvasW <= 17: // 16x16 (100% DPI)
		digitH = 10
		if isThree {
			oneW, digitW, stroke, gap = 2, 4, 1, 1
		} else {
			oneW, digitW, stroke, gap = 3, 6, 2, 1
		}
	case canvasW <= 21: // 20x20 (125% DPI)
		digitH = 12
		if isThree {
			oneW, digitW, stroke, gap = 2, 5, 2, 1
		} else {
			oneW, digitW, stroke, gap = 3, 7, 2, 2
		}
	case canvasW <= 26: // 24x24 (150% DPI)
		digitH = 15
		if isThree {
			oneW, digitW, stroke, gap = 3, 6, 2, 1
		} else {
			oneW, digitW, stroke, gap = 4, 9, 3, 2
		}
	default: // 32x32+ (200%+ DPI)
		scale := canvasW / 16
		digitH = minInt(availH-2*scale, 10*scale)
		if isThree {
			oneW = 2 * scale
			digitW = 4*scale + 1
			stroke = maxInt(2, scale)
			gap = maxInt(1, scale)
		} else {
			oneW = 3 * scale
			digitW = 6 * scale
			stroke = 2 * scale
			gap = maxInt(2, scale)
		}
	}

	if digitH > availH-2 {
		digitH = availH - 2
	}

	totalW := 0
	for i, ch := range text {
		if i > 0 {
			totalW += gap
		}
		if ch == '1' {
			totalW += oneW
		} else {
			totalW += digitW
		}
	}

	startX := (canvasW - totalW) / 2
	startY := (availH - digitH) / 2
	if startY < 1 {
		startY = 1
	}

	curX := startX
	for _, ch := range text {
		w := digitW
		if ch == '1' {
			w = oneW
		}
		rasterizeGlyph(mask, canvasW, curX, startY, w, digitH, stroke, byte(ch))
		curX += w + gap
	}

	// Render 8-neighbor dark outline halo first so digits stay legible on any background
	outlineR := 1
	if canvasW >= 32 {
		outlineR = 2
	}
	for y := 0; y < canvasW; y++ {
		for x := 0; x < canvasW; x++ {
			if mask[y*canvasW+x] {
				continue
			}
			if hasNeighbor(mask, canvasW, x, y, outlineR) {
				dst.SetNRGBA(x, y, colOutline)
			}
		}
	}

	// Render crisp foreground glyph pixels
	for y := 0; y < canvasW; y++ {
		for x := 0; x < canvasW; x++ {
			if mask[y*canvasW+x] {
				dst.SetNRGBA(x, y, fg)
			}
		}
	}
}

func hasNeighbor(mask []bool, stride, cx, cy, r int) bool {
	for dy := -r; dy <= r; dy++ {
		ny := cy + dy
		if ny < 0 || ny >= stride {
			continue
		}
		for dx := -r; dx <= r; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx := cx + dx
			if nx < 0 || nx >= stride {
				continue
			}
			if mask[ny*stride+nx] {
				return true
			}
		}
	}
	return false
}

// seg bitmask constants for 7-segment crisp rasterizer:
//
//	 --A--
//	|     |
//	F     B
//	|--G--|
//	E     C
//	|     |
//	 --D--
const (
	segA = 1 << iota
	segB
	segC
	segD
	segE
	segF
	segG
)

var digitSegments = map[byte]uint8{
	'0': segA | segB | segC | segD | segE | segF,
	'2': segA | segB | segG | segE | segD,
	'3': segA | segB | segG | segC | segD,
	'4': segF | segG | segB | segC,
	'5': segA | segF | segG | segC | segD,
	'6': segA | segF | segG | segE | segC | segD,
	'7': segA | segB | segC,
	'8': segA | segB | segC | segD | segE | segF | segG,
	'9': segA | segF | segB | segG | segC | segD,
}

func rasterizeGlyph(mask []bool, stride, x0, y0, w, h, t int, ch byte) {
	set := func(px, py int) {
		if px >= 0 && px < stride && py >= 0 && py < stride {
			mask[py*stride+px] = true
		}
	}
	box := func(bx, by, bw, bh int) {
		for yy := by; yy < by+bh; yy++ {
			for xx := bx; xx < bx+bw; xx++ {
				set(xx, yy)
			}
		}
	}

	if ch == '-' {
		midY := y0 + (h-t)/2
		box(x0, midY, w, t)
		return
	}

	if ch == '1' {
		stemX := x0 + w - t
		if stemX < x0 {
			stemX = x0
		}
		box(stemX, y0, minInt(t, w), h)
		// Subtle top nose when width permits
		if w > t {
			box(x0, y0+minInt(1, h/6), w-t, t)
		}
		return
	}

	segs, ok := digitSegments[ch]
	if !ok {
		return
	}

	midY := y0 + (h-t)/2
	topH := midY - y0 + t
	botH := (y0 + h) - midY

	if segs&segA != 0 {
		box(x0, y0, w, t)
	}
	if segs&segG != 0 {
		box(x0, midY, w, t)
	}
	if segs&segD != 0 {
		box(x0, y0+h-t, w, t)
	}
	if segs&segF != 0 {
		box(x0, y0, t, topH)
	}
	if segs&segB != 0 {
		box(x0+w-t, y0, t, topH)
	}
	if segs&segE != 0 {
		box(x0, midY, t, botH)
	}
	if segs&segC != 0 {
		box(x0+w-t, midY, t, botH)
	}
}

func fillRect(img *image.NRGBA, x0, y0, w, h int, c color.NRGBA) {
	b := img.Bounds()
	for y := y0; y < y0+h; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := x0; x < x0+w; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			img.SetNRGBA(x, y, c)
		}
	}
}

// EncodeDIB encodes an NRGBA image into a Win32 icon DIB resource
// (BITMAPINFOHEADER + bottom-up 32-bit BGRA pixels + 32-bit-aligned 1bpp AND mask),
// directly usable by Win32 CreateIconFromResourceEx and inside .ico files.
func EncodeDIB(img *image.NRGBA) []byte {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()

	xorBytes := w * h * 4
	andRowBytes := ((w + 31) / 32) * 4
	andBytes := andRowBytes * h

	var buf bytes.Buffer
	buf.Grow(40 + xorBytes + andBytes)

	// BITMAPINFOHEADER (40 bytes); biHeight is doubled for XOR + AND masks in icons.
	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&buf, binary.LittleEndian, int32(w))
	_ = binary.Write(&buf, binary.LittleEndian, int32(h*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // biPlanes
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32)) // biBitCount
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))  // BI_RGB
	_ = binary.Write(&buf, binary.LittleEndian, uint32(xorBytes+andBytes))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))

	// Bottom-up 32-bit BGRA XOR bitmap
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(b.Min.X+x, b.Min.Y+y)
			buf.WriteByte(c.B)
			buf.WriteByte(c.G)
			buf.WriteByte(c.R)
			buf.WriteByte(c.A)
		}
	}

	// Bottom-up 1-bpp AND mask (1 = fully transparent pixel, 0 = alpha-blended pixel)
	andRow := make([]byte, andRowBytes)
	for y := h - 1; y >= 0; y-- {
		for i := range andRow {
			andRow[i] = 0
		}
		for x := 0; x < w; x++ {
			if img.NRGBAAt(b.Min.X+x, b.Min.Y+y).A == 0 {
				andRow[x/8] |= 0x80 >> (uint(x) % 8)
			}
		}
		buf.Write(andRow)
	}

	return buf.Bytes()
}

// RenderDIB renders the icon at size x size and returns Win32 DIB bytes for CreateIconFromResourceEx.
func RenderDIB(size int, spec VisualSpec) []byte {
	return EncodeDIB(RenderImage(size, spec))
}

// RenderICO builds a complete multi-resolution Windows .ico file (defaulting to 16, 24, 32 px).
func RenderICO(spec VisualSpec, sizes ...int) []byte {
	if len(sizes) == 0 {
		sizes = []int{16, 24, 32}
	}

	dibs := make([][]byte, len(sizes))
	for i, sz := range sizes {
		dibs[i] = RenderDIB(sz, spec)
	}

	var buf bytes.Buffer
	// ICONDIR (6 bytes)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1)) // Type 1 = Icon
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(sizes)))

	offset := uint32(6 + 16*len(sizes))
	for i, sz := range sizes {
		dimByte := byte(sz)
		if sz >= 256 {
			dimByte = 0
		}
		buf.WriteByte(dimByte)          // bWidth
		buf.WriteByte(dimByte)          // bHeight
		buf.WriteByte(0)                // bColorCount
		buf.WriteByte(0)                // bReserved
		_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // wPlanes
		_ = binary.Write(&buf, binary.LittleEndian, uint16(32)) // wBitCount
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(dibs[i])))
		_ = binary.Write(&buf, binary.LittleEndian, offset)
		offset += uint32(len(dibs[i]))
	}

	for _, dib := range dibs {
		buf.Write(dib)
	}
	return buf.Bytes()
}

func formatInt(n int) string {
	if n == 100 {
		return "100"
	}
	if n >= 10 {
		return string([]byte{byte('0' + n/10), byte('0' + n%10)})
	}
	return string([]byte{byte('0' + n)})
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
