package render

// Debugf, when set, gets the renderer's complaints (a model the game asks for that is
// not there).
var Debugf func(format string, args ...any)

func (r *Renderer) complain(format string, args ...any) {
	if Debugf != nil {
		Debugf(format, args...)
	}
}

// turn enters a part turned by heading a: the matrix's x and z rows turn, and the eye
// with them.
func (r *Renderer) turn(a int16) {
	s, co := angle(a)
	m := r.m
	for b := 0; b < 3; b++ {
		r.m[0][b] = co*m[0][b] - s*m[2][b]
		r.m[2][b] = s*m[0][b] + co*m[2][b]
	}
	x, z := r.obj[0], r.obj[2]
	r.obj[0] = x*co - z*s
	r.obj[2] = x*s + z*co
}

func bitLen(v int) int {
	n := 0
	for ; v > 0; v >>= 1 {
		n++
	}
	return n
}

func (r *Renderer) setColour(col int) {
	r.fill = fillFlat
	r.colour = byte(col)
}

// slotSize is a slot's width and v limit.
func (r *Renderer) slotSize(slot int) (w, vlim float64) {
	if slot < 0 || slot >= slotCount {
		return 1, 1
	}
	return float64(r.slots[slot].width), float64(r.slots[slot].vlimit)
}

// objectSlot is the slot an object's picture goes in, for the models textured with it.
const objectSlot = 6

// SetObjectTexture puts a picture in its stored form (format, width, height, a 16-bit
// size, then a byte a pixel) into the object slot, for the models that are textured with
// an object's picture.
func (r *Renderer) SetObjectTexture(b []byte) {
	if len(b) < 5 {
		return
	}
	w, h, size := int(b[1]), int(b[2]), int(b[3])|int(b[4])<<8
	if w == 0 || h == 0 {
		return
	}
	shift := bitLen(w) - 1
	px := make([]byte, size)
	copy(px, b[5:])
	r.slots[objectSlot] = textureSlot{
		width:   w,
		vlimit:  int(uint16(size - 1)),
		rowMask: int(uint16((h - 1) << shift)),
		pix:     px,
	}
}

// cornerPoly draws a polygon textured from slot, each vertex at a corner of the texture,
// and makes slot the context slot.
func (r *Renderer) cornerPoly(slot int, poly []vtx, corners []Corner) {
	r.context = slot
	w, vl := r.slotSize(slot)
	for i := range poly {
		poly[i].u, poly[i].t = 0, 0
		if corners[i].Right {
			poly[i].u = w
		}
		if corners[i].Bottom {
			poly[i].t = vl
		}
	}
	r.texPoly(slot, poly)
}

// noSlot is no texture slot: faces textured from it are drawn flat.
const noSlot = -1
