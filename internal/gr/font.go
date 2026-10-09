package gr

import "encoding/binary"

// Font is a FONT*.SYS font (gfx.h struct FontInfo): a 12-byte header, then each character's
// bitmap (Height rows of RowWidth bytes, high bit first) followed by its width.
type Font struct {
	WidthSize  int
	CharSize   int
	SpaceWidth int
	Height     int
	RowWidth   int
	MaxWidth   int
	data       []byte
}

var (
	CurFont *Font
	// ForegroundColor is the text colour, ShadowColor the shadowed text's shadow.
	ForegroundColor byte = 1
	BackgroundColor byte
	ShadowColor     byte
)

// LoadFont parses a FONT*.SYS file.
func LoadFont(b []byte) *Font {
	if len(b) < 12 {
		return nil
	}
	w := func(o int) int { return int(int16(binary.LittleEndian.Uint16(b[o:]))) }
	f := &Font{WidthSize: w(0), CharSize: w(2), SpaceWidth: w(4), Height: w(6), RowWidth: w(8), MaxWidth: w(10)}
	f.data = make([]byte, (f.CharSize+f.WidthSize)*128)
	copy(f.data, b[12:])
	return f
}

func (f *Font) glyph(c byte) ([]byte, int) {
	if c < 0x20 || c > 0x7A {
		c = '?'
	}
	stride := f.CharSize + f.WidthSize
	o := int(c) * stride
	return f.data[o : o+f.CharSize], int(f.data[o+f.CharSize])
}

// StringWidth is the sum of the characters' widths, up to the 0 or 54 characters.
func StringWidth(s string) int {
	if CurFont == nil {
		return 0
	}
	n := 0
	for i := 0; i < len(s) && i < 54; i++ {
		if s[i] == 0 {
			break
		}
		c := s[i]
		if c < 0x20 || c > 0x7A {
			c = '?'
		}
		_, w := CurFont.glyph(c)
		n += w
	}
	return n
}

// StringToScreen draws s in the text colour with its top left at (x, y) (y up), the
// background left as it is. The whole text box must be inside the window, or nothing is drawn.
func StringToScreen(s string, x, y int) {
	if CurFont == nil {
		return
	}
	wd := StringWidth(s)
	if x < WinLeft || x+wd-1 > WinRight || y > WinTop || y-CurFont.Height+1 < WinBottom {
		return
	}
	drawString(Pages[DrawPage][:], W, H, s, x, y, ForegroundColor)
}

// ShadowedString draws s one pixel right and down in the shadow colour, then at (x, y).
func ShadowedString(s string, x, y int) {
	if CurFont == nil {
		return
	}
	drawString(Pages[DrawPage][:], W, H, s, x+1, y-1, ShadowColor)
	drawString(Pages[DrawPage][:], W, H, s, x, y, ForegroundColor)
}

// DrawStringInto draws text into any buffer (y up from its bottom).
func DrawStringInto(dst []byte, tw, th int, s string, x, y int, color byte) {
	drawString(dst, tw, th, s, x, y, color)
}

func drawString(dst []byte, tw, th int, s string, x, y int, color byte) {
	f := CurFont
	px := x
	for i := 0; i < len(s) && i < 54; i++ {
		if s[i] == 0 {
			break
		}
		g, gw := f.glyph(s[i])
		for r := 0; r < f.Height; r++ {
			ly := y - r
			if ly < 0 || ly >= th {
				continue
			}
			row := (th - 1 - ly) * tw
			for c := 0; c < gw; c++ {
				byteI := r*f.RowWidth + c/8
				if byteI >= len(g) {
					continue
				}
				if g[byteI]&(0x80>>(c%8)) == 0 {
					continue
				}
				lx := px + c
				if lx < 0 || lx >= tw {
					continue
				}
				dst[row+lx] = color
			}
		}
		px += gw
	}
}
