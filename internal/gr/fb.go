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

// FBToScreen copies the frame buffer to the drawing page.
func FBToScreen() {
	t, c := Transparency, ShowClip
	Transparency, ShowClip = false, false
	Show(FBX, FBY, FB, FBH, FBW, 0, 0)
	Transparency, ShowClip = t, c
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

// VCopyFB copies a rectangle of the frame buffer (x, y relative to it) into block hd. The
// part past the buffer keeps what the block holds: the pointer straddles the view, and the
// screen save made where it moved there (mousReSave) has the frame under it, not black.
func VCopyFB(x, y, w, h, hd int) {
	b := VBlock(hd)
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
			if lx >= 0 && lx < FBW && ly >= 0 && ly < FBH {
				b.Pix[r*w+c] = FB[(FBH-1-ly)*FBW+lx]
			}
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
