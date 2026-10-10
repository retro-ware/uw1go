// Package gr is the graphics library: a Go reimplementation of UW1's seg003 (GRCORE.ASM,
// VIDMODE.ASM, GRLIBF.ASM, GRLIBI.ASM and the rest), the routines the game's C draws the
// screen with. The semantics are the original's:
//
//   - The screen is 320 by 200 with two pages; drawing goes to the drawing page, and in the
//     normal state the drawing page is the one shown. grSoftPageFlip switches the drawing page
//     only, grPageFlip shows the drawing page and switches to the other.
//   - y counts up from the bottom of the screen (row 0 is the bottom line), and the clip
//     window's top is above its bottom. Rectangles are given by corners, inclusive.
//   - The primitives draw with the current pen (set_the_color): a colour below 0x100, or one
//     of the special pens that save to, restore from, or copy from the other page.
//   - Bitmaps (show) are drawn opaque, or with colour 0 transparent while Transparency is set,
//     and clipped to the window only while ShowClip is set.
//
// Pages are kept top row first; RowOf turns a library y into a row index.
package gr

const (
	W = 320
	H = 200
)

// RowOf returns the storage row of library y (which counts up from the bottom).
func RowOf(y int) int { return H - 1 - y }

var (
	Pages     [2][W * H]byte
	DrawPage  int // the page drawing goes to
	ShownPage int // the page on the screen

	// The clip window (y counts up, so WinBottom <= WinTop).
	WinLeft, WinTop, WinRight, WinBottom = 0, H - 1, W - 1, 0

	// ShowClip clips bitmaps to the window; Transparency skips colour 0 in them.
	ShowClip     bool
	Transparency bool

	// The pen: set_the_color.
	pen      int
	penColor byte
	penFB    bool // fbuf_setcolor: the primitives draw into the frame buffer

	// PenBlock is the video memory block the save and restore pens (0x100, 0x101) use
	// (Color_data_ptr's words for them).
	PenBlock *Block

	// Palette is the palette being built (6-bit RGB); DAC is what the screen shows.
	// LocalDoPalette copies one to the other.
	Palette [768]byte
	DAC     [768]byte
)

// Draw returns the drawing page.
func Draw() []byte { return Pages[DrawPage][:] }

// Other returns the page that is not the drawing page.
func Other() []byte { return Pages[DrawPage^1][:] }

// Shown returns the page on the screen.
func Shown() []byte { return Pages[ShownPage][:] }

// InitGraphics resets the screen: both pages cleared, the window the whole screen.
func InitGraphics() {
	for i := range Pages {
		clear(Pages[i][:])
	}
	DrawPage, ShownPage = 0, 0
	SetTheWindow(0, H-1, W-1, 0)
}

// GrPageFlip shows the drawing page and makes the other one the drawing page.
func GrPageFlip() {
	ShownPage = DrawPage
	DrawPage ^= 1
	if FlipHook != nil {
		FlipHook()
	}
}

// FlipHook runs after each page flip (the host yields there).
var FlipHook func()

// GrSoftPageFlip switches the drawing page without changing what is shown.
func GrSoftPageFlip() {
	DrawPage ^= 1
}

// CopyHiddenToVisible copies the drawing page onto the other page (the library calls the
// drawing page hidden and the other visible, whichever is on the screen).
func CopyHiddenToVisible() {
	Pages[DrawPage^1] = Pages[DrawPage]
}

// CopyVisibleToHidden copies the other page onto the drawing page.
func CopyVisibleToHidden() {
	Pages[DrawPage] = Pages[DrawPage^1]
}

// SetTheWindow sets the clip window: left, top, right, bottom (y counting up).
func SetTheWindow(x0, y0, x1, y1 int) {
	WinLeft, WinTop, WinRight, WinBottom = x0, y0, x1, y1
}

// SetTheColor selects the pen: a colour, or 0x100 save to PenBlock, 0x101 restore from
// it, 0x106 copy from the other page.
func SetTheColor(c int) {
	pen = c
	penColor = byte(c)
	penFB = false
}

// FbufSetcolor selects a pen that draws into the 3D view's frame buffer.
func FbufSetcolor(c int) {
	pen = c
	penColor = byte(c)
	penFB = true
}

// span writes one span (library y, x0..x1 inclusive) with the current pen.
func span(y, x0, x1 int, st *spanState) {
	if penFB {
		fbSpan(y, x0, x1, st)
		return
	}
	if y < 0 || y >= H {
		return
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 >= W {
		x1 = W - 1
	}
	if x0 > x1 {
		return
	}
	row := RowOf(y) * W
	dst := Pages[DrawPage][row+x0 : row+x1+1]
	switch {
	case pen < 0x100:
		for i := range dst {
			dst[i] = penColor
		}
	case pen == 0x100: // save the spans, in order, to a block
		st.save(dst)
	case pen == 0x101: // restore them
		st.restore(dst)
	default: // 0x106 and the other copy pens: from the other page
		src := Pages[DrawPage^1][row+x0 : row+x1+1]
		copy(dst, src)
	}
}

// spanState carries the save and restore pens' position through one primitive.
type spanState struct {
	blk *Block
	pos int
}

func newSpanState() *spanState {
	st := &spanState{}
	if pen == 0x100 || pen == 0x101 {
		st.blk = PenBlock
	}
	return st
}

func (st *spanState) save(src []byte) {
	if st.blk == nil {
		return
	}
	for _, c := range src {
		if st.pos < len(st.blk.Pix) {
			st.blk.Pix[st.pos] = c
		}
		st.pos++
	}
}

func (st *spanState) restore(dst []byte) {
	if st.blk == nil {
		return
	}
	for i := range dst {
		if st.pos < len(st.blk.Pix) {
			dst[i] = st.blk.Pix[st.pos]
		}
		st.pos++
	}
}

// clipRect orders and clips a rectangle to the window; ok is false when it is wholly outside.
func clipRect(x0, y0, x1, y1 int) (int, int, int, int, bool) {
	if y0 < y1 {
		y0, y1 = y1, y0
	}
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if x1 < WinLeft || x0 > WinRight || y0 < WinBottom || y1 > WinTop {
		return 0, 0, 0, 0, false
	}
	if x0 < WinLeft {
		x0 = WinLeft
	}
	if x1 > WinRight {
		x1 = WinRight
	}
	if y1 < WinBottom {
		y1 = WinBottom
	}
	if y0 > WinTop {
		y0 = WinTop
	}
	return x0, y0, x1, y1, true
}

// Rectangle fills the rectangle with corners (x0, y0) and (x1, y1), clipped to the window.
func Rectangle(x0, y0, x1, y1 int) {
	x0, y0, x1, y1, ok := clipRect(x0, y0, x1, y1)
	if !ok {
		return
	}
	URectangle(x0, y0, x1, y1)
}

// URectangle fills a rectangle without clipping (corners in either order).
func URectangle(x0, y0, x1, y1 int) {
	if y0 < y1 {
		y0, y1 = y1, y0
	}
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	st := newSpanState()
	for y := y0; y >= y1; y-- {
		span(y, x0, x1, st)
	}
}

// ClearWindow fills the whole clip window with the pen.
func ClearWindow() {
	URectangle(WinLeft, WinTop, WinRight, WinBottom)
}

// Box draws the outline of a rectangle, clipped to the window.
func Box(x0, y0, x1, y1 int) {
	x0, y0, x1, y1, ok := clipRect(x0, y0, x1, y1)
	if !ok {
		return
	}
	st := newSpanState()
	span(y0, x0, x1, st)
	span(y1, x0, x1, st)
	for y := y0; y >= y1; y-- {
		span(y, x0, x0, st)
		span(y, x1, x1, st)
	}
}

// UHLine draws a horizontal line, unclipped.
func UHLine(x0, y, x1 int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	span(y, x0, x1, newSpanState())
}

// HLine draws a horizontal line clipped to the window.
func HLine(x0, y, x1 int) {
	if y > WinTop || y < WinBottom {
		return
	}
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if x1 < WinLeft || x0 > WinRight {
		return
	}
	x0 = max(x0, WinLeft)
	x1 = min(x1, WinRight)
	span(y, x0, x1, newSpanState())
}

// UVLine draws a vertical line, unclipped.
func UVLine(x, y0, y1 int) {
	if y0 < y1 {
		y0, y1 = y1, y0
	}
	st := newSpanState()
	for y := y0; y >= y1; y-- {
		span(y, x, x, st)
	}
}

// PlotPixel sets one pixel with the pen, clipped to the window.
func PlotPixel(x, y int) {
	if x < WinLeft || x > WinRight || y < WinBottom || y > WinTop {
		return
	}
	span(y, x, x, newSpanState())
}

// ReadPixel returns the pixel at (x, y) of the drawing page.
func ReadPixel(x, y int) byte {
	if x < 0 || x >= W || y < 0 || y >= H {
		return 0
	}
	return Pages[DrawPage][RowOf(y)*W+x]
}

// Show draws a bitmap of w by h pixels (rows top first, w bytes a row) with its top left
// corner at (x, y), starting at the bitmap's (sx, sy): (w - sx) by (h - sy) pixels are drawn.
func Show(x, y int, bm []byte, h, w, sx, sy int) {
	showInto(Pages[DrawPage][:], W, H, x, y, bm, w, h, sx, sy, Transparency)
}

// showInto is show's common path for a target of tw by th (y up from its bottom).
func showInto(dst []byte, tw, th int, x, y int, bm []byte, w, h, sx, sy int, trans bool) {
	cw := w - sx
	ch := h - sy
	if cw <= 0 || ch <= 0 {
		return
	}
	// clip
	if ShowClip {
		if x+cw-1 < WinLeft || x > WinRight || y-ch+1 > WinTop || y < WinBottom {
			return
		}
		if x < WinLeft {
			d := WinLeft - x
			x += d
			cw -= d
			sx += d
		}
		if x+cw-1 > WinRight {
			cw = WinRight - x + 1
		}
		if y > WinTop {
			d := y - WinTop
			y -= d
			ch -= d
			sy += d
		}
		if y-ch+1 < WinBottom {
			ch = y - WinBottom + 1
		}
	}
	for r := 0; r < ch; r++ {
		ly := y - r
		if ly < 0 || ly >= th {
			continue
		}
		srow := (sy+r)*w + sx
		if srow < 0 || srow >= len(bm) {
			break
		}
		drow := (th - 1 - ly) * tw
		for c := 0; c < cw; c++ {
			lx := x + c
			if lx < 0 || lx >= tw {
				continue
			}
			if srow+c >= len(bm) {
				break
			}
			p := bm[srow+c]
			if trans && p == 0 {
				continue
			}
			dst[drow+lx] = p
		}
	}
}

// Grab copies the w by h rectangle at (x, y) (top left, y up) of the drawing page into dst.
func Grab(dst []byte, x, y, w, h int) {
	p := Pages[DrawPage][:]
	for r := 0; r < h; r++ {
		ly := y - r
		for c := 0; c < w; c++ {
			lx := x + c
			var v byte
			if lx >= 0 && lx < W && ly >= 0 && ly < H {
				v = p[RowOf(ly)*W+lx]
			}
			if r*w+c < len(dst) {
				dst[r*w+c] = v
			}
		}
	}
}

// VCopy copies a w by h screen rectangle from (sx, sy) to (x, y), within the drawing page.
func VCopy(sx, sy, w, h, x, y int) {
	tmp := make([]byte, w*h)
	Grab(tmp, sx, sy, w, h)
	t := Transparency
	Show(x, y, tmp, h, w, 0, 0)
	Transparency = t
}

// LocalDoPalette loads count entries of Palette from first into the DAC.
func LocalDoPalette(count int, first int) {
	if count <= 0 {
		count = 256
	}
	for i := first; i < first+count && i < 256; i++ {
		copy(DAC[i*3:i*3+3], Palette[i*3:i*3+3])
	}
}
