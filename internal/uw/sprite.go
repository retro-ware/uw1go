package uw

import "github.com/felix/uw1go/internal/gr"

// Port of SPRITE.ASM: pictures redrawn in place over what is under them, on four layers.
// The calls that change a sprite only record the change and queue it (spChanged);
// updateSprites puts back what the queued sprites covered, top layer first, then draws
// them again, bottom layer first, saving what each will cover. Layer 0 sprites save
// nothing and are never restored.

const (
	spInUse     = 1
	spVisible   = 2
	spNoSave    = 4
	spDestroyed = 8
	spTrans     = 0x10
)

type sprite struct {
	flags int
	x     int
	y     byte
	w     int
	h     byte
	yoff  byte
	layer int
	pic   int
	block int
}

var (
	spInf   [0x40]sprite
	spRedo  [4][]int // the sprites to redraw, per layer
	spMap   [4][]int // the sprites on screen, per layer
	spDirty bool
)

func spValid(n int) bool { return n >= 0 && n < len(spInf) }

// createSprite returns a free sprite on layer, with a w by h video memory block for
// layers 1 to 3, or -1.
func createSprite(layer int, wh ...int) int {
	for i := range spInf {
		s := &spInf[i]
		if s.flags&spInUse != 0 {
			continue
		}
		s.flags = spInUse | spNoSave
		if gr.Transparency {
			s.flags |= spTrans
		}
		s.layer = layer
		s.block = 0
		if layer != 0 {
			if s.block = gr.Valloc(wh[0], wh[1]); s.block == 0 {
				return -1
			}
		}
		s.yoff = 0
		return i
	}
	return -1
}

func destroySprite(n int) {
	if spValid(n) {
		spInf[n].flags = (spInf[n].flags | spDestroyed) &^ spVisible
		spChanged(n)
	}
}

func changeSprite(n, x, y, w, h int) {
	if spValid(n) {
		s := &spInf[n]
		s.x, s.y, s.w, s.h = x, byte(y), w, byte(h)
		spChanged(n)
	}
}

func drawSprite(n, pic int) {
	if spValid(n) {
		spInf[n].pic = pic
		spInf[n].flags |= spVisible
		spChanged(n)
	}
}

// drawMask is drawSprite, drawn transparent.
func drawMask(n, pic int) {
	if spValid(n) {
		spInf[n].pic = pic
		spInf[n].flags |= spVisible | spTrans
		spChanged(n)
	}
}

func eraseSprite(n int) {
	if spValid(n) && spInf[n].flags&spVisible != 0 {
		spInf[n].flags &^= spVisible
		spChanged(n)
	}
}

func moveSprite(n, x, y int) {
	if spValid(n) {
		spInf[n].x, spInf[n].y = x, byte(y)
		spChanged(n)
	}
}

func resizeSprite(n, w, h int) {
	if spValid(n) {
		spInf[n].w, spInf[n].h = w, byte(h)
		spChanged(n)
	}
}

func setYoff(n, yoff int) {
	if spValid(n) {
		spInf[n].yoff = byte(yoff)
		spChanged(n)
	}
}

func spIndex(l []int, n int) int {
	for i, v := range l {
		if v == n {
			return i
		}
	}
	return -1
}

func spQueue(n int) {
	l := spInf[n].layer & 3
	if spIndex(spRedo[l], n) < 0 {
		spRedo[l] = append(spRedo[l], n)
	}
}

// spChanged queues sprite n, keeps its layer's on-screen list, and queues every visible
// sprite of a higher layer that overlaps it. The y and height compare as signed bytes.
func spChanged(n int) {
	spDirty = true
	spQueue(n)
	s := &spInf[n]
	l := s.layer & 3
	if i := spIndex(spMap[l], n); i < 0 {
		if s.flags&spVisible != 0 {
			spMap[l] = append(spMap[l], n)
		}
	} else if s.flags&spVisible == 0 {
		last := len(spMap[l]) - 1
		spMap[l][i] = spMap[l][last]
		spMap[l] = spMap[l][:last]
	}
	for hl := l + 1; hl < 4; hl++ {
		for _, o := range spMap[hl] {
			d := &spInf[o]
			if d.x > s.x+s.w || d.x+d.w < s.x {
				continue
			}
			if int8(s.y-s.h) > int8(d.y) || int8(s.y) < int8(d.y-d.h) {
				continue
			}
			spQueue(o)
		}
	}
}

// updateSprites redraws the queued sprites.
func updateSprites() {
	if !spDirty {
		return
	}
	mouseHide()
	for l := 3; l >= 1; l-- {
		for _, n := range spRedo[l] {
			s := &spInf[n]
			if s.flags&spNoSave != 0 {
				s.flags &^= spNoSave
			} else {
				gr.RestoreRect(s.block)
			}
			if s.flags&spDestroyed != 0 {
				s.flags &^= spInUse
				if s.block != 0 {
					gr.Vfree(s.block)
				}
			}
		}
	}
	for l := 0; l < 4; l++ {
		if l != 0 {
			for _, n := range spRedo[l] {
				s := &spInf[n]
				if s.flags&spVisible == 0 {
					s.flags |= spNoSave
				} else {
					gr.SaveRect(s.block, s.x, int(s.y), s.w, int(s.h))
				}
			}
		}
		redo := spRedo[l]
		spRedo[l] = spRedo[l][:0]
		for _, n := range redo {
			s := &spInf[n]
			if s.flags&spVisible == 0 {
				continue
			}
			gr.Transparency = s.flags&spTrans != 0
			if s.yoff != 0 {
				maskToScreen(s.pic, s.x, int(s.y), int(s.h), s.w, int(s.yoff))
			} else {
				picToScreen(s.pic, s.x, int(s.y))
			}
			gr.Transparency = false
		}
	}
	mouseShow()
	spDirty = false
}
