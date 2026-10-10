package render

// The interpreter (INTERP.ASM, TMAPOPS.ASM, PGCACHE.ASM, SPHERE.ASM's object header). A
// program is a stream of words: an opcode, then its operands. Point operands are point
// numbers times 8 (offsets into the original's point table); offsets are relative to the
// end of their operand; addresses are data segment offsets of model variables.

// Debugf, when set, gets the interpreter's complaints about malformed programs.
var Debugf func(format string, args ...any)

func (r *Renderer) complain(format string, args ...any) {
	if Debugf != nil {
		Debugf(format, args...)
	}
}

type vm struct {
	r  *Renderer
	pc int
}

func (c *vm) word() int {
	v := c.r.R16(c.pc & 0xFFFF)
	c.pc += 2
	return int(v)
}

func (c *vm) sword() int { return int(int16(c.word())) }

func (c *vm) byte_() int {
	v := c.r.Mem[c.pc&0xFFFF]
	c.pc++
	return int(v)
}

func (c *vm) sbyte() int { return int(int8(c.byte_())) }

// nPoints is the size of the point table: point operands are byte offsets (8 to a point)
// into the original's, which runs on past 256 (doors keep a delta at 800h).
const nPoints = 0x10000 / 8

// pt is a point operand.
func (c *vm) pt() *point { return &c.r.pts[(c.word()>>3)%nPoints] }

// target reads an offset operand and is where it leads.
func (c *vm) target() int {
	o := c.sword()
	return (c.pc + o) & 0xFFFF
}

// maxDepth bounds sub-program nesting, against malformed programs.
const maxDepth = 64

// run runs the program at pc until its end (opcode 0, or a test that ends it).
func (r *Renderer) run(pc int) { r.runAt(pc, 0) }

func (r *Renderer) runAt(pc, depth int) {
	if depth > maxDepth {
		r.complain("render: sub-programs nested too deep at %04X\n", pc)
		return
	}
	c := &vm{r: r, pc: pc}
	call := func(at int) { r.runAt(at, depth+1) }
	for steps := 0; ; steps++ {
		if steps > 1<<20 {
			r.complain("render: program at %04X does not end\n", pc)
			return
		}
		at := c.pc
		switch op := c.word(); op {
		case 0x00: // eof
			return
		case 0x02: // movec: [address] = value
			a := c.word()
			r.W16(a, uint16(c.word()))
		case 0x04: // movem: [b] = [a]
			a := c.word()
			r.W16(c.word(), r.R16(a))
		case 0xBE: // movei: [[b]] = [a]
			a := c.word()
			r.W16(int(r.R16(c.word())), r.R16(a))

		case 0x12: // sfcal: run the sub-program at the offset
			call(c.target())
		case 0x14, 0x16: // ifle, ifge: skip when [address] <= (>=) value
			skip, a, v := c.sword(), c.word(), c.sword()
			x := r.s16(a)
			if (op == 0x14 && x <= v) || (op == 0x16 && x >= v) {
				c.pc += skip
			}
		case 0x48: // ijmp
			c.pc = c.target()
		case 0x32: // ijmp_i: to the address in a variable
			c.pc = int(r.R16(c.word()))
		case 0x6C: // sortvar: two sub-programs, in the order a variable picks
			a, v := c.word(), c.sword()
			first, second := c.target(), c.target()
			if r.s16(a) < v {
				first, second = second, first
			}
			call(first)
			call(second)
		case 0x06, 0x0C, 0x0E, 0x10: // sortnorm: two sub-programs, the nearer half last
			s := c.plane(op)
			first, second := c.target(), c.target()
			if s < 0 {
				first, second = second, first
			}
			call(first)
			call(second)
		case 0x58, 0x5E, 0x60, 0x62: // jnorm: skip the face when the eye is behind it
			skip := c.sword()
			if c.plane(op) < 0 {
				c.pc += skip
			}
		case 0x64, 0x66, 0x68: // jnorm along one axis
			skip, n, p := c.sword(), c.sword(), c.sword()
			axis := map[int]int{0x64: 0, 0x66: 1, 0x68: 2}[op]
			if (int16(p+int(r.obj[axis])) < 0) != (n < 0) {
				c.pc += skip
			}

		case 0x4A: // rorg: move the origin
			for i := 0; i < 3; i++ {
				r.obj[i] -= float64(c.sword())
			}
		case 0x50, 0xBA: // ihcall: a sub-program turned by a heading
			a := c.word()
			if op == 0xBA {
				a = int(r.R16(a))
			}
			to := c.target()
			m, o := r.m, r.obj
			r.turn(int16(a))
			call(to)
			r.m, r.obj = m, o

		case 0x7A: // defres: a point, (x, z, y)
			x, z, y := c.sword(), c.sword(), c.sword()
			c.setPoint(c.pt(), r.transform(vec3{float64(x), float64(y), float64(z)}))
		case 0xB6: // minires: the same with bytes, each times 32
			x, z, y := c.sbyte()*32, c.sbyte()*32, c.sbyte()*32
			n := c.byte_()
			c.setPoint(&r.pts[n], r.transform(vec3{float64(x), float64(y), float64(z)}))
		case 0x82: // multires: count points from one on
			n, first := c.word(), c.word()>>3
			for i := 0; i < n; i++ {
				x, z, y := c.sword(), c.sword(), c.sword()
				c.setPoint(&r.pts[(first+i)%nPoints], r.transform(vec3{float64(x), float64(y), float64(z)}))
			}
		case 0xB8: // minimulti
			n, first := c.byte_(), c.byte_()
			for i := 0; i < n; i++ {
				x, z, y := c.sbyte()*32, c.sbyte()*32, c.sbyte()*32
				c.setPoint(&r.pts[(first+i)%nPoints], r.transform(vec3{float64(x), float64(y), float64(z)}))
			}
		case 0x86, 0x2A, 0x88, 0x1C, 0x8A, 0x2C: // x_rel, y_rel, z_rel
			src := *c.pt()
			d := float64(c.sword())
			var off vec3
			off[map[int]int{0x86: 0, 0x2A: 0, 0x88: 1, 0x1C: 1, 0x8A: 2, 0x2C: 2}[op]] = d
			c.setPoint(c.pt(), add(src.v, r.rotate(off)))
		case 0x90, 0x92, 0x94: // xy_rel, xz_rel, yz_rel
			a, b := float64(c.sword()), float64(c.sword())
			off := map[int]vec3{0x90: {a, b, 0}, 0x92: {a, 0, b}, 0x94: {0, b, a}}[op]
			src := *c.pt()
			c.setPoint(c.pt(), add(src.v, r.rotate(off)))
		case 0x4C: // defdelta: a rotated offset, kept as a point
			x, z, y := c.sword(), c.sword(), c.sword()
			c.pt().v = r.rotate(vec3{float64(x), float64(y), float64(z)})
		case 0x8C, 0xC6: // addres, subres
			a, b := *c.pt(), *c.pt()
			if op == 0xC6 {
				b.v = vec3{-b.v[0], -b.v[1], -b.v[2]}
			}
			c.setPoint(c.pt(), add(a.v, b.v))
		case 0xCC: // setshade: points' shades, for Gouraud faces
			n := c.word()
			for i := 0; i < n; i++ {
				p := c.pt()
				p.shade = byte(c.byte_())
			}
			c.pc = (c.pc + 1) &^ 1
		case 0xD4: // uwshade: the base colour from a variable, and points' shades
			n := c.word()
			r.gourBase = r.Mem[c.word()]
			bias := int(r.Mem[varShade])
			for i := 0; i < n; i++ {
				p := c.pt()
				p.shade = byte(min(c.byte_()+bias&0xFF, 0x0E))
			}
			c.pc = (c.pc + 1) &^ 1

		case 0x84: // hull: end the program when every point is outside one plane
			n := c.word()
			and := uint8(0xFF)
			for i := 0; i < n; i++ {
				and &= c.pt().codes
			}
			if and != 0 {
				return
			}
		case 0x78: // parapiped: a bounding box test
			if !c.box() {
				return
			}

		case 0x2E, 0x5C: // setcolor, setcolv
			col := c.word()
			if op == 0x5C {
				col += int(r.R16(c.word()))
			}
			r.setColour(col)
		case 0xBC: // uwcolv: a colour from a variable, shaded
			if r.Mem[varPick] != 0 {
				c.pc += 4
				r.setColour(int(r.bmcolor))
				break
			}
			a, s := c.word(), c.word()
			row := min((s&0xFF)+int(r.Mem[varShade]), 15)
			r.setColour(int(r.lightabs[(row<<8+s>>8+int(r.R16(a)))&0xFFF]))
		case 0x44: // vexsurf: flat, colour kept
			r.fill = fillFlat
		case 0xC8, 0xCA: // goursurf, goursurfv
			c.word()
			r.fill = fillGouraud
		case 0xD6: // uwsurf: Gouraud, or the pick colour
			if r.Mem[varPick] != 0 {
				r.setColour(int(r.bmcolor))
			} else {
				r.fill = fillGouraud
			}
		case 0xD8: // transsurf
			r.fill = fillTrans
		case 0x22, 0x7E: // polyres: a polygon
			n := c.word()
			poly := make([]vtx, n)
			for i := range poly {
				p := c.pt()
				poly[i] = vtx{v: p.v, codes: p.codes, s: float64(p.shade)}
			}
			r.polygon(poly)

		case 0xAE: // setbmcol: the pick colour
			r.bmcolor = byte(c.word())
		case 0xB2: // set_tmctxt
			r.setCtxt(c.word() & 0xFF)

		case 0x36, 0x34, 0x9E: // tmap, wtmap, tmap_tri: corners given
			n := 4
			if op == 0x9E {
				n = 3
			}
			slot := c.word() & 0xFF
			poly := make([]vtx, n)
			corners := make([]int, n)
			for i := range poly {
				p := c.pt()
				poly[i] = vtx{v: p.v, codes: p.codes}
				corners[i] = c.word()
			}
			r.cornerPoly(slot, poly, corners)
		case 0xA0, 0xA2, 0xA4, 0xA6: // compact_tmap, compact_wtmap: a quad, corners implied
			slot := c.word() & 0xFF
			poly := make([]vtx, 4)
			for i := range poly {
				p := &r.pts[c.byte_()]
				poly[i] = vtx{v: p.v, codes: p.codes}
			}
			corners := compactCorners
			if op >= 0xA4 {
				corners = mirroredCorners
			}
			r.cornerPoly(slot, poly, corners[:])
		case 0xA8, 0xAA, 0xB4, 0xCE: // gtmap, gwtmap, ctxt_gtmap, ctxt_gmap: fractions
			slot := r.ctxtSlot()
			if op == 0xA8 || op == 0xAA {
				slot = c.word() & 0xFF
				r.setCtxt(slot)
			}
			w, vl := r.slotSize(slot)
			n := c.word()
			poly := make([]vtx, n)
			for i := range poly {
				p := c.pt()
				fu, fv := float64(c.word())/65536, float64(c.word())/65536
				poly[i] = vtx{v: p.v, codes: p.codes, u: fu * w, t: fv * vl}
			}
			r.texPoly(slot, poly)
		case 0xAC: // gtri: nothing

		default:
			r.complain("render: opcode %02X at %04X\n", op, at)
			return
		}
	}
}

func add(a, b vec3) vec3 { return vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

func (c *vm) setPoint(p *point, v vec3) {
	p.v = v
	p.codes = codes(v)
}

// plane is the side of a plane the eye is on: the sum over the axes of n * (p + eye).
// The jnorm and sortnorm variants leave out the axis their opcode names.
func (c *vm) plane(op int) float64 {
	axes := map[int][]int{0x06: {0, 1, 2}, 0x58: {0, 1, 2}, 0x0C: {1, 2}, 0x5E: {1, 2},
		0x0E: {0, 2}, 0x60: {0, 2}, 0x10: {0, 1}, 0x62: {0, 1}}[op]
	s := 0.0
	for _, a := range axes {
		n, p := c.sword(), c.sword()
		s += float64(n) * float64(int16(p+int(c.r.obj[a])))
	}
	return s
}

// turn enters a sub-object turned by heading a: the matrix's x and z rows turn, and the
// eye with them.
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

// box is do_parapiped: point, three half extents, offset. The eight corners of the box
// round the point outside one plane: skip the offset, or end the program when it is 0.
func (c *vm) box() bool {
	r := c.r
	p := *c.pt()
	var e [3]vec3
	for a := 0; a < 3; a++ {
		var d vec3
		d[a] = float64(c.sword())
		e[a] = r.rotate(d)
	}
	off := c.sword()
	and := p.codes
	for i := 0; i < 8 && and != 0; i++ {
		v := p.v
		for a := 0; a < 3; a++ {
			s := 1.0
			if i>>a&1 != 0 {
				s = -1
			}
			for b := 0; b < 3; b++ {
				v[b] += s * e[a][b]
			}
		}
		and &= codes(v)
	}
	if and == 0 {
		return true
	}
	if off == 0 {
		return false
	}
	c.pc += off
	return true
}

func (r *Renderer) setColour(col int) {
	r.fill = fillFlat
	r.colour = byte(col)
}

// The texture slots. Each is 8 bytes of the data segment: the width, the v limit (the
// texture's last byte offset, which the game sets for walls of other heights), the
// original's texture segment and the row mask (a byte offset mask, which the
// hallucination effect scrambles). The texture itself is r.slots.

func (r *Renderer) slotSize(slot int) (w, vlim float64) {
	if slot >= slotCount {
		return 1, 1
	}
	return float64(r.R16(slotTab + slot*8)), float64(r.R16(slotTab + slot*8 + 2))
}

// setCtxt makes slot the context slot (B07Ch), as the texture opcodes do.
func (r *Renderer) setCtxt(slot int) { r.W16(slotTab-2, uint16(slotTab+slot*8)) }

func (r *Renderer) ctxtSlot() int { return (int(r.R16(slotTab-2)) - slotTab) / 8 & 0xFF }

// SetObjectTexture puts a picture in its stored form (format, width, height, size, then
// one byte a pixel) into texture slot 6, for the models that are textured with an
// object's picture (do_settmobj).
func (r *Renderer) SetObjectTexture(b []byte) {
	if len(b) < 5 {
		return
	}
	w, h, size := int(b[1]), int(b[2]), int(b[3])|int(b[4])<<8
	if w == 0 || h == 0 {
		return
	}
	shift := bitLen(w) - 1
	r.W16(slotTab+6*8, uint16(w))
	r.W16(slotTab+6*8+2, uint16(size-1))
	r.W16(slotTab+6*8+6, uint16((h-1)<<shift))
	px := make([]byte, size)
	copy(px, b[min(5, len(b)):])
	r.slots[6] = px
}

// The corners of a texture: bit 0 the right edge (u = the width), 2 and up the bottom
// (v = the v limit). The compact opcodes' quads go round the texture from its top left; the
// mirrored ones (bcompact) swap left and right.
var (
	compactCorners  = [4]int{0, 1, 3, 2}
	mirroredCorners = [4]int{1, 0, 2, 3}
)

// cornerPoly draws a polygon textured from slot, each vertex at a corner of the texture,
// and makes slot the context slot.
func (r *Renderer) cornerPoly(slot int, poly []vtx, corners []int) {
	r.setCtxt(slot)
	w, vl := r.slotSize(slot)
	for i := range poly {
		poly[i].u, poly[i].t = 0, 0
		if corners[i]&1 != 0 {
			poly[i].u = w
		}
		if corners[i] >= 2 {
			poly[i].t = vl
		}
	}
	r.texPoly(slot, poly)
}
