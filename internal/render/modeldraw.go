package render

// Drawing the models' steps (model.go).

func (b Block) draw(r *Renderer) bool {
	for _, s := range b {
		if s.draw(r) {
			return true
		}
	}
	return false
}

// drawPart draws a part of its own: an end in it ends only it.
func (r *Renderer) drawPart(b Block) { b.draw(r) }

func (s Call) draw(r *Renderer) bool { r.drawPart(s.Body); return false }
func (s Jump) draw(r *Renderer) bool { r.drawPart(s.Body); return true }
func (s Stop) draw(r *Renderer) bool { return true }

func (s Unless) draw(r *Renderer) bool {
	v := int(r.vars[s.Var])
	if (s.AtMost && v <= s.Value) || (!s.AtMost && v >= s.Value) {
		return false
	}
	return s.Body.draw(r)
}

func (s InOrderByVar) draw(r *Renderer) bool {
	first, second := s.First, s.Second
	if int(r.vars[s.Var]) < s.Value {
		first, second = second, first
	}
	r.drawPart(first)
	r.drawPart(second)
	return false
}

func (s InOrder) draw(r *Renderer) bool {
	first, second := s.First, s.Second
	if r.side(s.Plane) < 0 {
		first, second = second, first
	}
	r.drawPart(first)
	r.drawPart(second)
	return false
}

func (s IfFacing) draw(r *Renderer) bool {
	if r.side(s.Plane) < 0 {
		return false
	}
	return s.Body.draw(r)
}

func (s IfFacingAxis) draw(r *Renderer) bool {
	if (wrap16(s.Offset+int(r.obj[s.Axis])) < 0) != (s.Normal < 0) {
		return false
	}
	return s.Body.draw(r)
}

// side is which side of p the eye is on (negative behind).
func (r *Renderer) side(p Plane) float64 {
	sum := 0.0
	for _, t := range p {
		sum += float64(t.Normal) * float64(wrap16(t.Offset+int(r.obj[t.Axis])))
	}
	return sum
}

// wrap16 is v in the original's 16-bit arithmetic.
func wrap16(v int) int { return int(int16(v)) }

func (s MoveOrigin) draw(r *Renderer) bool {
	for i, d := range s.Offset {
		r.obj[i] -= float64(d)
	}
	return false
}

func (s Turned) draw(r *Renderer) bool {
	h := s.Heading
	if s.ByVar {
		h = r.vars[s.Var]
	}
	m, o := r.m, r.obj
	r.turn(h)
	r.drawPart(s.Body)
	r.m, r.obj = m, o
	return false
}

func fvec(p [3]int) vec3 { return vec3{float64(p[0]), float64(p[1]), float64(p[2])} }

func (r *Renderer) point(n int) *point { return &r.pts[n%nPoints] }

func (s PlacePoint) draw(r *Renderer) bool {
	r.point(s.Index).set(r.transform(fvec(s.Pos)))
	return false
}

func (s OffsetPoint) draw(r *Renderer) bool {
	from := r.point(s.From).v
	r.point(s.Index).set(add(from, r.rotate(fvec(s.Offset))))
	return false
}

func (s SumPoint) draw(r *Renderer) bool {
	a, b := r.point(s.A).v, r.point(s.B).v
	if s.Subtract {
		b = vec3{-b[0], -b[1], -b[2]}
	}
	r.point(s.Index).set(add(a, b))
	return false
}

func (s Delta) draw(r *Renderer) bool {
	r.point(s.Index).v = r.rotate(fvec(s.Offset))
	return false
}

func (p *point) set(v vec3) {
	p.v = v
	p.codes = codes(v)
}

func add(a, b vec3) vec3 { return vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

func (s SetShades) draw(r *Renderer) bool {
	for _, ps := range s.Shades {
		r.point(ps.Point).shade = byte(ps.Shade)
	}
	return false
}

func (s SetShadesFromVar) draw(r *Renderer) bool {
	r.gourBase = byte(r.vars[s.Var])
	bias := int(byte(r.vars[VarShade]))
	for _, ps := range s.Shades {
		r.point(ps.Point).shade = byte(min(ps.Shade+bias, maxPointShade))
	}
	return false
}

// maxPointShade is the darkest shade SetShadesFromVar gives a point.
const maxPointShade = 14

func (s EndUnlessVisible) draw(r *Renderer) bool {
	outside := allOutside
	for _, n := range s.Points {
		outside &= r.point(n).codes
	}
	return outside != 0
}

// allOutside is every clip code: the AND of points' codes starts from it.
const allOutside = ^uint8(0)

func (s EndUnlessBoxVisible) draw(r *Renderer) bool { return !r.boxVisible(s.Center, s.Extent) }

// boxVisible is whether any of the eight corners of the box round point n, with half
// extents e along the model's axes, is inside each side of the view.
func (r *Renderer) boxVisible(n int, e [3]int) bool {
	p := *r.point(n)
	var axes [3]vec3
	for a := range axes {
		var d vec3
		d[a] = float64(e[a])
		axes[a] = r.rotate(d)
	}
	outside := p.codes
	for corner := 0; corner < 8 && outside != 0; corner++ {
		v := p.v
		for a := 0; a < 3; a++ {
			sign := 1.0
			if corner>>a&1 != 0 {
				sign = -1
			}
			for b := 0; b < 3; b++ {
				v[b] += sign * axes[a][b]
			}
		}
		outside &= codes(v)
	}
	return outside == 0
}

func (s SetColour) draw(r *Renderer) bool {
	c := s.Colour
	if s.FromVar {
		c += int(uint16(r.vars[s.Var]))
	}
	r.setColour(c)
	return false
}

func (s ShadedColour) draw(r *Renderer) bool {
	if r.vars[VarPick] != 0 {
		r.setColour(int(r.bmcolor))
		return false
	}
	row := min(s.Shade+int(byte(r.vars[VarShade])), 15)
	r.setColour(int(r.lightabs[(row*256+s.Offset+int(uint16(r.vars[s.Var])))%len(r.lightabs)]))
	return false
}

func (s SetFill) draw(r *Renderer) bool {
	switch s.Style {
	case FillFlat:
		r.fill = fillFlat
	case FillShaded:
		r.fill = fillGouraud
	case FillShadedOrPick:
		if r.vars[VarPick] != 0 {
			r.setColour(int(r.bmcolor))
		} else {
			r.fill = fillGouraud
		}
	case FillTranslucent:
		r.fill = fillTrans
	}
	return false
}

func (s Polygon) draw(r *Renderer) bool {
	poly := make([]vtx, len(s.Points))
	for i, n := range s.Points {
		p := r.point(n)
		poly[i] = vtx{v: p.v, codes: p.codes, s: float64(p.shade)}
	}
	r.polygon(poly)
	return false
}

func (s SetPickColour) draw(r *Renderer) bool { r.bmcolor = s.Colour; return false }
func (s SetContext) draw(r *Renderer) bool    { r.context = s.Slot; return false }

func (s CornerTextured) draw(r *Renderer) bool {
	poly := make([]vtx, len(s.Points))
	for i, n := range s.Points {
		p := r.point(n)
		poly[i] = vtx{v: p.v, codes: p.codes}
	}
	r.cornerPoly(s.Slot, poly, s.Corners)
	return false
}

func (s Textured) draw(r *Renderer) bool {
	slot := r.context
	if !s.Context {
		slot = s.Slot
		r.context = slot
	}
	w, vl := r.slotSize(slot)
	poly := make([]vtx, len(s.Points))
	for i, n := range s.Points {
		p := r.point(n)
		poly[i] = vtx{v: p.v, codes: p.codes, u: s.UVs[i].U * w, t: s.UVs[i].V * vl}
	}
	r.texPoly(slot, poly)
	return false
}

func (s SetSlotHeight) draw(r *Renderer) bool {
	r.SetTextureHeight(int(r.vars[VarSlot]), int(uint16(r.vars[VarSlotHeight])))
	return false
}
