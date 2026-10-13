package render

import "math"

// vtx is a polygon's vertex: the view-space point and what is interpolated across the
// polygon (s a shade, u and t the texture's column and byte offset).
type vtx struct {
	v       vec3
	codes   uint8
	s, u, t float64
}

// nearZ is the near plane polygons are clipped to.
const nearZ = 1.0

// clipNear cuts away the part of a polygon in front of the near plane. The cut vertices
// are interpolated in view space, the same way whichever way round an edge is walked, so
// that polygons sharing an edge share the cut.
func clipNear(in []vtx) []vtx {
	out := make([]vtx, 0, len(in)+2)
	for i := range in {
		a, b := in[i], in[(i+1)%len(in)]
		if a.v[2] >= nearZ {
			out = append(out, a)
		}
		if (a.v[2] >= nearZ) != (b.v[2] >= nearZ) {
			p, q := a, b
			if p.v[2] > q.v[2] {
				p, q = q, p
			}
			k := (nearZ - p.v[2]) / (q.v[2] - p.v[2])
			lerp := func(x, y float64) float64 { return x + (y-x)*k }
			out = append(out, vtx{
				v: vec3{lerp(p.v[0], q.v[0]), lerp(p.v[1], q.v[1]), nearZ},
				s: lerp(p.s, q.s), u: lerp(p.u, q.u), t: lerp(p.t, q.t),
			})
		}
	}
	return out
}

// sv is a projected vertex: its place and its attributes over z (and 1/z), which are
// linear on the screen.
type sv struct {
	x, y float64
	a    [4]float64 // 1/z, s/z, u/z, t/z
}

// span is a polygon's row: from x0 to x1 (pixel centres x0 <= x+0.5 < x1), with the
// attributes at each end.
type span struct {
	x0, x1 float64
	a0, a1 [4]float64
}

// scan projects a polygon and calls row for each of its screen rows.
func (r *Renderer) scan(poly []vtx, row func(y int, s span)) {
	if len(poly) < 3 {
		return
	}
	and := uint8(0xFF)
	for _, p := range poly {
		and &= p.codes
	}
	if and != 0 {
		return
	}
	poly = clipNear(poly)
	if len(poly) < 3 {
		return
	}
	ps := make([]sv, len(poly))
	top, bot := math.Inf(1), math.Inf(-1)
	for i, p := range poly {
		x, y := r.project(p.v)
		iz := 1 / p.v[2]
		ps[i] = sv{x, y, [4]float64{iz, p.s * iz, p.u * iz, p.t * iz}}
		top, bot = min(top, y), max(bot, y)
	}
	y0 := max(int(math.Ceil(top-0.5)), 0)
	y1 := min(int(math.Ceil(bot-0.5)), r.H)
	for y := y0; y < y1; y++ {
		yc := float64(y) + 0.5
		s := span{x0: math.Inf(1), x1: math.Inf(-1)}
		for i := range ps {
			a, b := &ps[i], &ps[(i+1)%len(ps)]
			if a.y == b.y || yc < min(a.y, b.y) || yc >= max(a.y, b.y) {
				continue
			}
			if a.y > b.y {
				a, b = b, a
			}
			k := (yc - a.y) / (b.y - a.y)
			x := a.x + (b.x-a.x)*k
			var at [4]float64
			for j := range at {
				at[j] = a.a[j] + (b.a[j]-a.a[j])*k
			}
			if x < s.x0 {
				s.x0, s.a0 = x, at
			}
			if x > s.x1 {
				s.x1, s.a1 = x, at
			}
		}
		if s.x1 > s.x0 {
			row(y, s)
		}
	}
}

// pixels calls px for each pixel of a span with the attributes there (divided back by
// 1/z: s, u, t).
func (r *Renderer) pixels(y int, s span, px func(i, x int, s, u, t float64)) {
	x0 := max(int(math.Ceil(s.x0-0.5)), 0)
	x1 := min(int(math.Ceil(s.x1-0.5)), r.W)
	d := s.x1 - s.x0
	for x := x0; x < x1; x++ {
		k := (float64(x) + 0.5 - s.x0) / d
		iz := s.a0[0] + (s.a1[0]-s.a0[0])*k
		z := 1 / iz
		px(y*r.W+x, x,
			(s.a0[1]+(s.a1[1]-s.a0[1])*k)*z,
			(s.a0[2]+(s.a1[2]-s.a0[2])*k)*z,
			(s.a0[3]+(s.a1[3]-s.a0[3])*k)*z)
	}
}

// shadeRow is a shade's lightabs row, dithered between neighbouring rows on a
// checkerboard.
func shadeRow(s float64, x, y int) int {
	if (x^y)&1 != 0 {
		s += 0.25
	}
	return min(max(int(s), 0), 15) << 8
}

// polygon draws a polygon in the current fill.
func (r *Renderer) polygon(poly []vtx) {
	fb := r.FB
	switch r.fill {
	case fillFlat:
		c := r.colour
		r.scan(poly, func(y int, s span) {
			r.pixels(y, s, func(i, _ int, _, _, _ float64) { fb[i] = c })
		})
	case fillGouraud:
		base := int(r.gourBase)
		r.scan(poly, func(y int, s span) {
			r.pixels(y, s, func(i, x int, sh, _, _ float64) { fb[i] = r.lightabs[shadeRow(sh, x, y)+base] })
		})
	case fillTrans:
		r.scan(poly, func(y int, s span) {
			r.pixels(y, s, func(i, x int, sh, _, _ float64) { fb[i] = r.lightabs[shadeRow(sh, x, y)+int(fb[i])] })
		})
	}
}

// texPoly draws a polygon textured from a slot: flat in the current colour when the
// model variable polym is set, and shaded by distance when smooth_over is.
func (r *Renderer) texPoly(slot int, poly []vtx) {
	if r.vars[VarFlat] != 0 || slot < 0 || slot >= slotCount || r.slots[slot].pix == nil {
		c := r.colour
		r.scan(poly, func(y int, s span) {
			r.pixels(y, s, func(i, _ int, _, _, _ float64) { fb := r.FB; fb[i] = c })
		})
		return
	}
	tex := r.slots[slot].pix
	w := r.slots[slot].width
	mask := r.slots[slot].rowMask
	if w <= 0 {
		return
	}
	smooth := r.vars[VarSmooth] != 0
	if smooth {
		for i := range poly {
			v := poly[i].v
			d := math.Sqrt(v[0]*v[0]+v[1]*v[1]+v[2]*v[2]) / 32
			poly[i].s = min(d*float64(r.smoothDiv)/64+float64(r.smoothAdd), 15)
		}
	}
	fb := r.FB
	r.scan(poly, func(y int, s span) {
		r.pixels(y, s, func(i, x int, sh, u, t float64) {
			ui := min(max(int(u), 0), w-1)
			o := (int(t)&mask + ui) % len(tex)
			if o < 0 {
				o += len(tex)
			}
			c := tex[o]
			if smooth {
				c = r.lightabs[shadeRow(sh, x, y)+int(c)]
			}
			fb[i] = c
		})
	})
}

// sprite draws a picture standing at view-space point p, facing the eye (in_scalebm):
// its hot spot at p, scaled by the record's scales sx and sy, horizontally through the
// view's ground direction and vertically by half a unit, as the original's full-size
// sprites are.
func (r *Renderer) sprite(p vec3, pic Pic, sx, sy int) {
	if pic.W <= 0 || pic.H <= 0 || len(pic.Pix) < pic.W*pic.H {
		return
	}
	s := math.Ldexp(1, r.objShift)
	kx := float64(sx) * s * r.spriteX
	ky := float64(sy) * s * 0.5
	left := vec3{p[0] - float64(pic.HotX)*kx, p[1] + float64(pic.HotY)*ky, p[2]}
	right := vec3{left[0] + float64(pic.W)*kx, left[1] - float64(pic.H)*ky, p[2]}
	// Wholly right of the view or behind the eye, judged at the bottom left corner; left of
	// it, below it or behind, at the top right.
	if codes(vec3{left[0], right[1], p[2]})&0x82 != 0 || codes(vec3{right[0], left[1], p[2]})&0x89 != 0 {
		return
	}
	x0, y0 := r.project(left)
	x1, y1 := r.project(right)
	if x1-x0 > 0x200 || x1 <= x0 || y1 <= y0 {
		return
	}
	px0 := max(int(math.Ceil(x0-0.5)), 0)
	px1 := min(int(math.Ceil(x1-0.5)), r.W)
	py0 := max(int(math.Ceil(y0-0.5)), 0)
	py1 := min(int(math.Ceil(y1-0.5)), r.H)
	fx := float64(pic.W) / (x1 - x0)
	fy := float64(pic.H) / (y1 - y0)
	for y := py0; y < py1; y++ {
		sy := min(int((float64(y)+0.5-y0)*fy), pic.H-1)
		src := pic.Pix[sy*pic.W:]
		dst := r.FB[y*r.W:]
		for x := px0; x < px1; x++ {
			c := src[min(int((float64(x)+0.5-x0)*fx), pic.W-1)]
			switch {
			case c == 0:
			case r.pixMode == 2:
				dst[x] = r.bmcolor
			case c >= 0xFB:
				dst[x] = r.xfer[int(c-0xFB)<<8+int(dst[x])]
			default:
				dst[x] = c
			}
		}
	}
}
