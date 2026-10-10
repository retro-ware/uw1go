package gr

// Video memory blocks: VALLOC.ASM's spare video memory, where the game saves screen
// rectangles (save_rect, restore_rect) and LOADGR.C keeps the screen furniture's pictures.
// A nil block is "none": its methods do nothing.

// Block is a w by h picture in video memory.
type Block struct {
	W, H   int
	Pix    []byte
	X, Y   int // where Save took it from (top left, y up)
	SW, SH int // and its size
}

// NewBlock reserves a w by h block (valloc).
func NewBlock(w, h int) *Block {
	return &Block{W: w, H: h, Pix: make([]byte, w*h+64)}
}

// Save saves the w by h rectangle at (x, y) of the drawing page into the block.
func (b *Block) Save(x, y, w, h int) {
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

// Restore puts a saved rectangle back where it was taken from.
func (b *Block) Restore() {
	if b == nil || b.SW == 0 {
		return
	}
	t, c := Transparency, ShowClip
	Transparency, ShowClip = false, false
	Show(b.X, b.Y, b.Pix, b.SH, b.SW, 0, 0)
	Transparency, ShowClip = t, c
}

// Show draws w-sx by h-sy of the block, from its (sx, sy), with its top left at (x, y).
func (b *Block) Show(x, y, w, h, sx, sy int) {
	if b == nil {
		return
	}
	pitch := b.W
	if b.SW != 0 {
		pitch = b.SW
	}
	cw := w - sx
	ch := h - sy
	if cw <= 0 || ch <= 0 {
		return
	}
	tmp := make([]byte, cw*ch)
	for r := 0; r < ch; r++ {
		for c := 0; c < cw; c++ {
			if i := (sy+r)*pitch + sx + c; i < len(b.Pix) {
				tmp[r*cw+c] = b.Pix[i]
			}
		}
	}
	Show(x, y, tmp, ch, cw, 0, 0)
}

// Size is the block's picture size.
func (b *Block) Size() (int, int) {
	if b == nil {
		return 0, 0
	}
	return b.W, b.H
}
