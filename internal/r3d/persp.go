package r3d

import "os"

// Perspective-correct texture mapping, which the DOS game does not have.
//
// UW1's two texture mappers, gfx_texture_poly_affine (WALLMAP.ASM: floors, ceilings and most
// of what is not a wall) and gfx_texture_poly_wall (POLYFILL.ASM), step u and v linearly
// across the screen, which is why floors swim and walls bend when the view pitches.
// perspWalk does their work with u and v perspective-correct. c2go makes each mapper call it
// at the first instruction of its body (WALLMAP 01F5, POLYFILL 056B, after the original's own
// stack switch) and leave by the original's exit (0363, 0898).
//
// It is underworld-exhumed's port/gfx/perspmap.c (its --enhance perspective) in Go, itself a
// translation of the walk in SiENcE's uwpatch (https://github.com/SiENcE/uwpatch,
// patch/perspective.asm, MIT; built on OpenAbyss's src/uw_texmap.c), keeping its arithmetic.
// It keeps the original's walk, which decides the pixels a face covers, so every face covers
// exactly the pixels it did; what changes is the texel each pixel takes. Along the edges and
// across each span it carries Q = 2^30/z, S = u*Q>>14 and T = v*Q>>14, which are linear on
// the screen, and recovers u and v by division every 16 pixels, stepping them linearly
// between; u and v are clamped to the texture, and a division whose quotient would not fit
// saturates.
//
// Its inputs, where the original mappers read them: CX the vertex count; in the module's data
// segment the projected vertices at 0659 (14 bytes each: +0 screen x, +2 screen y, +4 u,
// +6 v, +8 x, +0A y, +0C z), the frame buffer's segment at 095A and its row offsets at 095E;
// ES:[B07C] the shape record (+0 the width, +2 the height part of v's limit, +4 the
// texture's segment, +6 v's mask). It writes, as the originals do, the ring's end (07AB), its
// last vertex (07A9), the vertices the walk may still use (07D1) and the row (07CF).

// Perspective selects perspWalk over the original affine mappers. UW1GO_AFFINE=1 turns it off.
var Perspective = os.Getenv("UW1GO_AFFINE") == ""

const (
	pRing    = 0x659
	pRingEnd = 0x7AB
	pRingLst = 0x7A9
	pRow     = 0x7CF
	pLeftN   = 0x7D1
	pFBSeg   = 0x95A
	pRowTab  = 0x95E
	pShape   = 0xB07C
	pSpan    = 16
)

type pEdge struct {
	ptr   uint16   // the vertex it heads for
	end   int16    // the row it ends at
	x, dx int32    // x, 16.16, and its step per row
	q, dq [3]int32 // Q, S, T and their steps
}

var pw struct {
	e          [2]pEdge // 0 the left edge, 1 the right
	qc, dqc    [3]int32 // the span's Q, S, T at the current pixel, and their steps
	wall       bool     // the wall mapper's rules
	u1, v1     int32    // u*65536 and v*256 at the current pixel
	ulim, vlim int32    // the largest the texture has
	mask       uint16
	count      int16 // the span's pixels still to draw
}

func pRowNow() int16 { return int16(rw(pDS, pRow)) }

// pDiv is the assembly's idiv: the same quotient wherever it gives one; where x86 would fault
// (MinInt32 by -1) the quotient wraps.
func pDiv(a, b int32) int32 { return int32(int64(a) / int64(b)) }

// pAttr: Q, S, T of the vertex at bx.
func pAttr(bx uint16, out *[3]int32) {
	z := uint32(rw(pDS, bx+0xC))
	if z == 0 { // z = 0 never survives the clip
		z = 1
	}
	q := uint32(0x40000000) / z
	out[0] = int32(q)
	out[1] = int32(uint32(uint64(rw(pDS, bx+4)) * uint64(q) >> 14))
	out[2] = int32(uint32(uint64(rw(pDS, bx+6)) * uint64(q) >> 14))
}

// pSeed: edge i takes its next vertex; keepX, the wall rules' reseed, keeps its x. False when
// the ring is used up.
func pSeed(i int, keepX bool) bool {
	e := &pw.e[i]
	for {
		bx := e.ptr
		if !keepX {
			e.x = int32(uint32(rw(pDS, bx))<<16 | 0x8000) // one half
		}
		keepX = false
		pAttr(bx, &e.q)
		if i != 0 { // right forward, left back
			bx += 14
		} else {
			bx -= 14
		}
		if bx < pRing {
			bx = rw(pDS, pRingLst)
		}
		if bx == rw(pDS, pRingEnd) {
			bx = pRing
		}
		e.ptr = bx
		ww(pDS, pLeftN, rw(pDS, pLeftN)-1)
		if rw(pDS, pLeftN) == 0 {
			return false
		}
		e.end = int16(rw(pDS, bx+2))
		cx := pRowNow() - e.end
		if cx == 0 { // no rows: the next vertex
			continue
		}
		// x's step as the original takes it: the whole part, then the remainder halved into
		// the high word, divided, doubled
		d := int16(rw(pDS, bx) - uint16(uint32(e.x)>>16))
		q1, r1 := d/cx, d%cx
		fr := int16(int32(r1) * 0x8000 / int32(cx))
		e.dx = int32(uint32(fr)*2 + uint32(uint16(q1))<<16)
		var t [3]int32
		pAttr(bx, &t)
		for k := range t {
			e.dq[k] = pDiv(int32(uint32(t[k])-uint32(e.q[k])), int32(cx))
		}
		return true
	}
}

func pAdvance(i int) {
	e := &pw.e[i]
	e.x = int32(uint32(e.x) + uint32(e.dx))
	for k := range e.q {
		e.q[k] = int32(uint32(e.q[k]) + uint32(e.dq[k]))
	}
}

// pSample: S or T at the current pixel to u*65536 or v*256, clamped to 0..lim.
func pSample(value, lim int32) int32 {
	hi, half := value>>10, pw.qc[0]>>1
	if hi >= half { // the quotient would not fit
		return lim
	}
	if hi <= -half {
		return 0
	}
	q := int32(int64(value) * (1 << 22) / int64(pw.qc[0]))
	if q < 0 {
		return 0
	}
	if uint32(q) > uint32(lim) {
		return lim
	}
	return q
}

func pUV() {
	pw.u1 = pSample(pw.qc[1], pw.ulim)
	pw.v1 = pSample(pw.qc[2], pw.vlim)
}

func perspWalk(wall bool) {
	n := CX
	pw.wall = wall
	// the ring: its end, its last vertex, the n+1 the walk may use
	ww(pDS, pRingEnd, pRing+n*14)
	ww(pDS, pRingLst, pRing+n*14-14)
	ww(pDS, pLeftN, n+1)
	// the shape record: width, v's limit, the texture's segment, v's mask
	si := rw(pES, pShape)
	pw.ulim = int32(uint32(rw(pES, si))<<16 - 1)
	pw.vlim = int32(uint32(rw(pES, si+2))<<8 | 0xFF)
	tex := int(rw(pES, si+4)) << 4
	pw.mask = rw(pES, si+6)
	// the vertex with the greatest y, the first of equals
	top := uint16(pRing)
	for k, v := uint16(0), uint16(pRing); k < n; k, v = k+1, v+14 {
		if int16(rw(pDS, v+2)) > int16(rw(pDS, top+2)) {
			top = v
		}
	}
	fb := int(rw(pDS, pFBSeg)) << 4
	pw.e[0].ptr, pw.e[1].ptr = top, top
	ww(pDS, pRow, rw(pDS, top+2))
	if !pSeed(0, false) || !pSeed(1, false) {
		return
	}
	for {
		x0 := uint16(uint32(pw.e[0].x) >> 16)
		w := int16(uint16(uint32(pw.e[1].x)>>16) - x0)
		di := rw(pDS, pRowTab+2*uint16(pRowNow())) + x0
		div := int32(w)
		if div == 0 { // a one-pixel span divides by one
			div = 1
		}
		for k := range pw.qc {
			pw.qc[k] = pw.e[0].q[k]
			pw.dqc[k] = pDiv(int32(uint32(pw.e[1].q[k])-uint32(pw.e[0].q[k])), div)
		}
		if count := w + 1; count > 0 { // the width as a pixel count
			pw.count = count
			pUV()
			for pw.count != 0 {
				u0, v0 := pw.u1, pw.v1
				c := min(pw.count, pSpan)
				pw.count -= c
				for k := range pw.qc {
					pw.qc[k] = int32(uint32(pw.qc[k]) + uint32(pw.dqc[k])*uint32(c))
				}
				pUV()
				du := pDiv(int32(uint32(pw.u1)-uint32(u0)), int32(c))
				dv := pDiv(int32(uint32(pw.v1)-uint32(v0)), int32(c))
				u, v := u0, v0
				for ; c > 0; c-- {
					t := uint16(uint32(v)>>8)&pw.mask + uint16(uint32(u)>>16)
					mem[fb+int(di)] = mem[tex+int(t)]
					di++
					u = int32(uint32(u) + uint32(du))
					v = int32(uint32(v) + uint32(dv))
				}
			}
		} else if !pw.wall { // backward: the affine mapper ends the face, the wall's passes the row over
			return
		}
		// the row done: both edges on, a reseed where an edge ends
		ww(pDS, pRow, rw(pDS, pRow)-1)
		pAdvance(0)
		pAdvance(1)
		if pRowNow() <= pw.e[0].end && !pSeed(0, pw.wall) {
			return
		}
		if pRowNow() <= pw.e[1].end && !pSeed(1, pw.wall) {
			return
		}
	}
}

func perspAffine() { perspWalk(false) }
func perspWall()   { perspWalk(true) }
