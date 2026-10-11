package gr

// The 3D view's frame buffer (GRENTRY.ASM): a linear picture the renderer draws into, which
// cFBtoScreen copies to the screen at the place cPlaceFB gave it. Its y also counts up from
// its bottom row; it is stored top row first.

var (
	FB       []byte
	FBW, FBH int
	FBX, FBY int // the view's top left on the screen (y up)
	fbClipL  int
	fbClipR  int
	fbClipB  int
	fbClipT  int
)

// PlaceFB sets the frame buffer's place on the screen and its size.
func PlaceFB(x, y, w, h int) {
	FBX, FBY, FBW, FBH = x, y, w, h
	if len(FB) != w*h {
		FB = make([]byte, w*h)
	}
	fbClipL, fbClipR, fbClipB, fbClipT = 0, w-1, 0, h-1
}

// FillFB fills the frame buffer with a colour.
func FillFB(c byte) {
	for i := range FB {
		FB[i] = c
	}
}

// FBToScreen copies the frame buffer to the drawing page, under the HUD while it is on:
// its pixels stay, but for the pointer drawn into the frame over them.
func FBToScreen() {
	if !HUDOn {
		t, c := Transparency, ShowClip
		Transparency, ShowClip = false, false
		Show(FBX, FBY, FB, FBH, FBW, 0, 0)
		Transparency, ShowClip = t, c
		return
	}
	RefreshRect(FBX, FBY, FBW, FBH)
	if cursPix == nil {
		return
	}
	p := Pages[DrawPage][:]
	for r := 0; r < cursH; r++ {
		fy := cursY - r
		ly := FBY - (FBH - 1 - fy)
		if fy < 0 || fy >= FBH || ly < 0 || ly >= H {
			continue
		}
		for c := 0; c < cursW; c++ {
			fx := cursX + c
			lx := FBX + fx + OX
			if fx < 0 || fx >= FBW || lx < 0 || lx >= W {
				continue
			}
			if i := RowOf(ly)*W + lx; Layer[i] == LayerHUD && cursorAt(fx, fy) {
				p[i] = FB[(FBH-1-fy)*FBW+fx]
			}
		}
	}
	cursPix = nil
}

// FBShow draws a bitmap into the frame buffer with its top left at (x, y) (y up from the
// buffer's bottom), transparent while Transparency is set, clipped to the buffer.
func FBShow(bm []byte, x, y, w, h int) {
	if FB == nil {
		return
	}
	for r := 0; r < h; r++ {
		ly := y - r
		if ly < 0 || ly >= FBH {
			continue
		}
		drow := (FBH - 1 - ly) * FBW
		for c := 0; c < w; c++ {
			lx := x + c
			if lx < 0 || lx >= FBW {
				continue
			}
			i := r*w + c
			if i >= len(bm) {
				return
			}
			p := bm[i]
			if Transparency && p == 0 {
				continue
			}
			FB[drow+lx] = p
		}
	}
}

// CopyFB copies a rectangle of the frame buffer (x, y relative to it) into the block. The
// part past the buffer keeps what the block holds: the pointer straddles the view, and the
// screen save made where it moved there (mousReSave) has the frame under it, not black. Under
// the HUD it takes what the page shows there without the pointer.
func (b *Block) CopyFB(x, y, w, h int) {
	if b == nil || FB == nil {
		return
	}
	if len(b.Pix) < w*h {
		b.Pix = make([]byte, w*h)
	}
	for r := 0; r < h; r++ {
		ly := y - r
		for c := 0; c < w; c++ {
			lx := x + c
			if lx < 0 || lx >= FBW || ly < 0 || ly >= FBH {
				continue
			}
			v := FB[(FBH-1-ly)*FBW+lx]
			if HUDOn {
				i := RowOf(FBY-(FBH-1-ly))*W + FBX + lx + OX
				if FBX+lx+OX < 0 || FBX+lx+OX >= W {
					continue
				}
				switch Layer[i] {
				case LayerHUD:
					v = HUDPix[i]
				case LayerPage:
					continue
				}
			}
			b.Pix[r*w+c] = v
		}
	}
}

// fbSpan writes a span into the frame buffer with the pen (fbuf_setcolor).
func fbSpan(y, x0, x1 int, st *spanState) {
	if FB == nil || y < 0 || y >= FBH {
		return
	}
	x0 = max(x0, 0)
	x1 = min(x1, FBW-1)
	if x0 > x1 {
		return
	}
	row := (FBH - 1 - y) * FBW
	dst := FB[row+x0 : row+x1+1]
	switch {
	case pen < 0x100:
		for i := range dst {
			dst[i] = penColor
		}
	case pen == 0x100:
		st.save(dst)
	case pen == 0x101:
		st.restore(dst)
	}
}

// The HUD: the game screen's view fills the screen down to the message scroll, and the
// furniture that stays on it (the gargoyle, the compass, the flasks, the panels) lies over
// the frame. Layer says per screen pixel what the frame's copy does there.
const (
	LayerFrame = iota // the frame's pixel
	LayerHUD          // HUDPix's
	LayerPage         // the page's own (a panel drawn on it)
)

var (
	// HUDOn: the copy honours Layer (the game screen); off, it copies the whole frame.
	HUDOn  bool
	Layer  [MaxW * H]byte
	HUDPix [MaxW * H]byte

	// The pointer as drawn into the frame (top left in frame coordinates, y up), so that
	// it shows over the HUD as well.
	cursPix                    []byte
	cursX, cursY, cursW, cursH int
)

// ClearHUD gives the whole screen back to the frame.
func ClearHUD() {
	clear(Layer[:])
}

// SetLayer sets the layer of a w by h rectangle (top left at x, y; y up).
func SetLayer(x, y, w, h int, l byte) {
	for r := 0; r < h; r++ {
		row := RowOf(y - r)
		if row < 0 || row >= H {
			continue
		}
		for c := 0; c < w; c++ {
			if lx := x + c + OX; lx >= 0 && lx < W {
				Layer[row*W+lx] = l
			}
		}
	}
}

// PutHUD puts a w by h picture on the HUD at (x, y): the pixels keep says, the rest of the
// rectangle the frame's. The page is redrawn there.
func PutHUD(x, y, w, h int, pix []byte, keep []bool) {
	for r := 0; r < h; r++ {
		row := RowOf(y - r)
		if row < 0 || row >= H {
			continue
		}
		for c := 0; c < w; c++ {
			lx := x + c + OX
			if lx < 0 || lx >= W {
				continue
			}
			i := row*W + lx
			if keep[r*w+c] {
				Layer[i], HUDPix[i] = LayerHUD, pix[r*w+c]
			} else if Layer[i] == LayerHUD {
				Layer[i] = LayerFrame
			}
		}
	}
	RefreshRect(x, y, w, h)
}

// RefreshRect redraws a rectangle of the drawing page from the frame and the HUD.
func RefreshRect(x, y, w, h int) {
	p := Pages[DrawPage][:]
	for r := 0; r < h; r++ {
		ly := y - r
		if ly < 0 || ly >= H {
			continue
		}
		row := RowOf(ly)
		fy := FBY - ly // the frame's row
		for c := 0; c < w; c++ {
			lx := x + c + OX
			if lx < 0 || lx >= W {
				continue
			}
			i := row*W + lx
			switch Layer[i] {
			case LayerHUD:
				p[i] = HUDPix[i]
			case LayerFrame:
				if fx := lx - OX - FBX; fx >= 0 && fx < FBW && fy >= 0 && fy < FBH && FB != nil {
					p[i] = FB[fy*FBW+fx]
				}
			}
		}
	}
}

// SetFBCursor records the pointer drawn into the frame at (x, y) (frame coordinates, y up).
func SetFBCursor(pix []byte, x, y, w, h int) {
	cursPix, cursX, cursY, cursW, cursH = pix, x, y, w, h
}

// cursorAt is whether the pointer drawn into the frame covers frame pixel (fx, fy) (y up).
func cursorAt(fx, fy int) bool {
	c, r := fx-cursX, cursY-fy
	if cursPix == nil || c < 0 || c >= cursW || r < 0 || r >= cursH || r*cursW+c >= len(cursPix) {
		return false
	}
	return cursPix[r*cursW+c] != 0
}
