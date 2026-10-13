package render

import "math"

// Drawing a frame. Positions are in the frame's coordinates: x across (0 the eye's
// column's left edge, 256 to a square), y up (the grid's height units) and z ahead (0 the
// eye's row's near edge), the camera turned into the first quadrant as the game's grid is.

// Vertex is a point placed in the frame, in view space.
type Vertex struct {
	v     vec3
	codes uint8
}

// The grid's extents (SPHERE.ASM's object header for the frame): the point shift keeps
// the most precision for points this far from the eye.
var gridExtent = [3]int{0x2200, 0x400, 0x1100}

// Begin starts a frame from the camera SetCamera set: the frame buffer cleared, the view
// built and the model variables a frame starts with set (pick a pick frame, flat texture
// mapped faces drawn flat, smooth distance shading).
func (r *Renderer) Begin(pick, flat, smooth bool) {
	r.setupView()
	r.Fill(0)
	r.m = r.base
	r.obj = r.eye
	bits := 0
	for i, e := range gridExtent {
		bits |= int(math.Abs(math.Floor(r.eye[i]))) | e
	}
	r.objShift = max(15-bitLen(bits), 0)
	r.fill = fillFlat
	r.colour = 0
	r.SetPickMode(pick)
	r.SetVar(VarPick, b2i(pick))
	r.SetVar(VarFlat, b2i(flat))
	r.SetVar(VarSmooth, b2i(smooth))
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Point places a point of the frame.
func (r *Renderer) Point(x, y, z int) Vertex {
	v := r.transform(vec3{float64(x), float64(y), float64(z)})
	return Vertex{v, codes(v)}
}

func vtxs(vs []Vertex) []vtx {
	poly := make([]vtx, len(vs))
	for i, p := range vs {
		poly[i] = vtx{v: p.v, codes: p.codes}
	}
	return poly
}

// Polygon fills a polygon with colour c.
func (r *Renderer) Polygon(c byte, vs ...Vertex) {
	r.setColour(int(c))
	r.polygon(vtxs(vs))
}

// SetVar sets model variable n (the Var constants), which the models read.
func (r *Renderer) SetVar(n, v int) {
	if n >= 0 && n < numVars {
		r.vars[n] = int16(v)
	}
}

// SetTexture puts a texture's pixels into a slot.
func (r *Renderer) SetTexture(slot int, pix []byte) {
	if slot >= 0 && slot < slotCount {
		r.slots[slot].pix = pix
	}
}

// SetTextureHeight sets a slot's v limit: the last byte offset its quads map to, which is
// shorter than the texture for walls lower than a square.
func (r *Renderer) SetTextureHeight(slot, vlimit int) {
	if slot >= 0 && slot < slotCount {
		r.slots[slot].vlimit = int(uint16(vlimit))
	}
}

// SetRowMask sets a slot's row mask, the byte offsets a texture's rows wrap to (the
// hallucination effect scrambles them).
func (r *Renderer) SetRowMask(slot, mask int) {
	if slot >= 0 && slot < slotCount {
		r.slots[slot].rowMask = int(uint16(mask))
	}
}

// SetContext makes slot the one the models' context mapped faces take.
func (r *Renderer) SetContext(slot int) { r.context = slot }

// TexQuad draws a quad textured from slot, each vertex at a corner of the texture (bit 0
// the right edge, 2 the bottom).
func (r *Renderer) TexQuad(slot int, vs [4]Vertex, corners [4]int) {
	cs := make([]Corner, 4)
	for i, c := range corners {
		cs[i] = Corner{Right: c%2 != 0, Bottom: c >= 2}
	}
	r.cornerPoly(slot, vtxs(vs[:]), cs)
}

// WallQuad draws a quad textured from slot going round the texture from its top left.
func (r *Renderer) WallQuad(slot int, vs [4]Vertex) {
	r.cornerPoly(slot, vtxs(vs[:]), compactCorners)
}

// SetPickColour sets the colour pick frames draw the next object in.
func (r *Renderer) SetPickColour(c byte) { r.bmcolor = c }

// ObjectSprite stands an object's picture at v, by the object sprite record's hot spot
// and scales.
func (r *Renderer) ObjectSprite(v Vertex, pic Pic) {
	pic.HotX, pic.HotY = objectHotX, objectHotY
	r.sprite(v.v, pic, spriteScaleX, spriteScaleY)
}

// CritterSprite stands a critter's frame at v, by its own hot spot.
func (r *Renderer) CritterSprite(v Vertex, pic Pic) {
	r.sprite(v.v, pic, spriteScaleX, spriteScaleY)
}

// SetDelta keeps an offset of y up as point n, which the door models add to their own.
func (r *Renderer) SetDelta(n, y int) {
	r.pts[n%nPoints].v = r.rotate(vec3{0, float64(y), 0})
}

// Model draws 3D model n standing at x, y, z, turned by heading.
func (r *Renderer) Model(n, x, y, z int, heading int16) {
	if n < 0 || n >= len(r.models) || r.models[n] == nil {
		r.complain("render: no model %d\n", n)
		return
	}
	m, o := r.m, r.obj
	r.obj = vec3{r.eye[0] - float64(x), r.eye[1] - float64(y), r.eye[2] - float64(z)}
	if heading != 0 {
		r.turn(heading)
	}
	r.drawPart(r.models[n].Body)
	r.m, r.obj = m, o
}
