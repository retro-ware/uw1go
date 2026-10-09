package gr

// Video memory blocks: VALLOC.ASM's spare video memory, where the game saves screen
// rectangles (save_rect, restore_rect) and LOADGR.C keeps the screen furniture's pictures.
// A block is named by a handle, 1 and up; 0 is "none".

// Block is a w by h picture in video memory.
type Block struct {
	W, H   int
	Pix    []byte
	X, Y   int // where save_rect took it from (top left, y up)
	SW, SH int // and its size
}

var blocks = []*Block{nil}

// VBlock returns the block of a handle, or nil.
func VBlock(h int) *Block {
	if h <= 0 || h >= len(blocks) {
		return nil
	}
	return blocks[h]
}

// Valloc reserves a w by h block and returns its handle.
func Valloc(w, h int) int {
	b := &Block{W: w, H: h, Pix: make([]byte, w*h+64)}
	for i := 1; i < len(blocks); i++ {
		if blocks[i] == nil {
			blocks[i] = b
			return i
		}
	}
	blocks = append(blocks, b)
	return len(blocks) - 1
}

// Vfree releases a block.
func Vfree(h int) {
	if h > 0 && h < len(blocks) {
		blocks[h] = nil
	}
}

// VallocPicture puts a picture into video memory and returns its handle.
func VallocPicture(w, h int, pix []byte) int {
	hd := Valloc(w, h)
	copy(blocks[hd].Pix, pix)
	return hd
}

// SaveRect saves the w by h rectangle at (x, y) of the drawing page into block h.
func SaveRect(hd, x, y, w, h int) {
	b := VBlock(hd)
	if b == nil {
		return
	}
	if w*h > len(b.Pix) {
		b.Pix = make([]byte, w*h)
	}
	b.X, b.Y, b.SW, b.SH = x, y, w, h
	b.W = w
	Grab(b.Pix, x, y, w, h)
}

// RestoreRect puts a saved rectangle back where it was taken from.
func RestoreRect(hd int) {
	b := VBlock(hd)
	if b == nil || b.SW == 0 {
		return
	}
	t, c := Transparency, ShowClip
	Transparency, ShowClip = false, false
	Show(b.X, b.Y, b.Pix, b.SH, b.SW, 0, 0)
	Transparency, ShowClip = t, c
}

// ShowBlock draws block hd (seg003_581E): w by h of it with its top left at (x, y),
// starting at the block's (sx, sy).
func ShowBlock(hd, x, y, w, h, sx, sy int) {
	b := VBlock(hd)
	if b == nil {
		return
	}
	pitch := b.W
	if b.SW != 0 {
		pitch = b.SW
	}
	// the block is pitch pixels wide; draw w-sx by h-sy of it
	sub := b.Pix
	cw := w - sx
	ch := h - sy
	if cw <= 0 || ch <= 0 {
		return
	}
	tmp := make([]byte, cw*ch)
	for r := 0; r < ch; r++ {
		for c := 0; c < cw; c++ {
			i := (sy+r)*pitch + sx + c
			if i < len(sub) {
				tmp[r*cw+c] = sub[i]
			}
		}
	}
	Show(x, y, tmp, ch, cw, 0, 0)
}

// BlockSize returns a block's picture size (MODEX.ASM's seg015_1F9B_325).
func BlockSize(hd int) (int, int) {
	b := VBlock(hd)
	if b == nil {
		return 0, 0
	}
	return b.W, b.H
}
