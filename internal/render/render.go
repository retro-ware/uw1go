// Package render draws Ultima Underworld's 3D view.
//
// The game draws a frame square by square, farthest first: it starts the frame (Begin), and
// places the corners of floors, ceilings and walls (Point), which it fills flat or with a
// texture; it stands sprites at points and calls the 3D models (doors, tables, pillars ...).
// The models come from UW.EXE, decoded once into Go values (model.go, decode.go); the
// package draws with floating point view geometry, clipping and perspective-correct
// texture mapping into a frame buffer of its own.
package render

import (
	"errors"
	"math"
)

// The screen's shape: the height a pixel row stands for against a column's width.
const pixelAspect = 27197.0 / 32768

// Sprites stand by the original's sprite records: an object's anchor (its picture's
// hot spot), and the scales for objects' and critters' pictures.
const (
	objectHotX, objectHotY = 8, 16
	spriteScaleX           = 4
	spriteScaleY           = 5
)

// textureSlot is one of the places the game puts the textures it draws with.
type textureSlot struct {
	width   int
	vlimit  int // the last byte offset quads map to (lower walls map less)
	rowMask int // the byte offsets rows wrap to (the hallucination effect scrambles it)
	pix     []byte
}

// slotCount is how many texture slots there are: two each of 16, 32 and 64 wide for the
// walls and floors, and one for an object's picture.
const slotCount = 7

var initialSlots = [slotCount]textureSlot{
	{width: 16, vlimit: 16*16 - 1, rowMask: 16 * 15},
	{width: 16, vlimit: 16*16 - 1, rowMask: 16 * 15},
	{width: 32, vlimit: 32*32 - 1, rowMask: 32 * 31},
	{width: 32, vlimit: 32*32 - 1, rowMask: 32 * 31},
	{width: 64, vlimit: 64*64 - 1, rowMask: 64 * 63},
	{width: 64, vlimit: 64*64 - 1, rowMask: 64 * 63},
	{width: 64, vlimit: 64*96 - 1, rowMask: 64 * 127},
}

// Pic is a picture: one byte per pixel, rows top first, 0 transparent.
type Pic struct {
	Pix        []byte
	W, H       int
	HotX, HotY int // the anchor, for critters (objects use the sprite record's)
}

// Camera is where the view looks from: grid coordinates (x across, y ahead, z height,
// 256 to a square) and 16-bit angles (heading right, pitch up, bank).
type Camera struct {
	X, Y, Z              int16
	Heading, Pitch, Bank int16
}

type vec3 [3]float64

// mat3 is a view matrix: m[a][b] is input axis a's part of view axis b (x right, y up,
// z the depth), at the original's scale (its 1.15 entries over 65536).
type mat3 [3][3]float64

// Renderer is the 3D view's state.
type Renderer struct {
	models  []*Model
	vars    [numVars]int16
	slots   [slotCount]textureSlot
	context int // the slot ContextTextured faces take

	// The frame buffer, W by H, top row first.
	FB   []byte
	W, H int

	zoom      int
	cam       Camera
	lightabs  [16 * 256]byte
	xfer      [6 * 256]byte
	smoothDiv int
	smoothAdd int // smooth_base plus smooth_lowpass when that is positive
	pixMode   int // 1 normal, 2 a pick frame (sprites in bmcolor)
	bmcolor   byte

	// per frame
	eye      [3]float64 // the eye, x, y (up), z (ahead)
	base     mat3       // the frame's view matrix
	projX    float64    // the projection scales and the screen centre
	projY    float64
	cx, cy   float64
	spriteX  float64 // a horizontal unit's screen-x scale for sprites (26D0h)
	m        mat3
	obj      vec3 // the eye in the current object's frame
	objShift int
	pts      [nPoints]point
	colour   byte
	fill     fillMode
	gourBase byte
}

type point struct {
	v     vec3
	codes uint8
	shade uint8
}

type fillMode int

const (
	fillFlat fillMode = iota
	fillGouraud
	fillTrans
	fillConcave // flat, for polygons that may be concave
)

// Load reads the models from UW.EXE, whose data lies shift bytes past where the release
// the listing was made from has it.
func Load(exe []byte, shift int) (*Renderer, error) {
	if len(exe) < 32 || exe[0] != 'M' || exe[1] != 'Z' {
		return nil, errors.New("render: UW.EXE is not an MZ executable")
	}
	header := (int(exe[8]) | int(exe[9])<<8) * 16
	start := header + dataSegmentParagraph*16 + shift
	if start < 0 || start+dataSegmentSize > len(exe) {
		return nil, errors.New("render: UW.EXE is too short")
	}
	models, err := decodeModels(exe[start : start+dataSegmentSize])
	if err != nil {
		return nil, err
	}
	r := &Renderer{models: models, slots: initialSlots, context: noSlot, pixMode: 1, zoom: defaultZoom, smoothDiv: 1}
	for i := 0; i < 16; i++ {
		for c := 0; c < 256; c++ {
			r.lightabs[i<<8|c] = byte(c)
		}
	}
	r.Resize(defaultWidth, defaultHeight)
	return r, nil
}

// The original's view: its window's size and zoom (1.15).
const (
	defaultWidth, defaultHeight = 172, 113
	defaultZoom                 = 24674
)

// nPoints is the size of the point table (doors keep deltas as points 128 and 256).
const nPoints = 8192

// Resize sets the frame buffer's size.
func (r *Renderer) Resize(w, h int) {
	if w == r.W && h == r.H && r.FB != nil {
		return
	}
	r.W, r.H = w, h
	r.FB = make([]byte, w*h)
}

// SetZoom sets the zoom (1.15; the larger, the wider the view).
func (r *Renderer) SetZoom(z int) { r.zoom = z }

// SetCamera sets the next frame's camera.
func (r *Renderer) SetCamera(c Camera) { r.cam = c }

// SetLightTabs sets the 16 distance shading maps.
func (r *Renderer) SetLightTabs(b []byte) { copy(r.lightabs[:], b) }

// Lightabs is the shading maps.
func (r *Renderer) Lightabs() []byte { return r.lightabs[:] }

// SetXfer sets XFER.DAT's colour tables, through which translucent sprite pixels remap
// what is behind them.
func (r *Renderer) SetXfer(b []byte) { copy(r.xfer[:], b) }

// SetSmooth sets the distance shading: a vertex at view distance d gets shade
// min(d/32*div/64 + max(lowpass, 0) + base, 15).
func (r *Renderer) SetSmooth(div, base, lowpass int) {
	r.smoothDiv = div
	r.smoothAdd = base + max(lowpass, 0)
}

// SetPickMode makes the next frames pick frames (sprites drawn in their pick colour).
func (r *Renderer) SetPickMode(on bool) {
	r.pixMode = 1
	if on {
		r.pixMode = 2
	}
}

// Fill fills the frame buffer with colour c.
func (r *Renderer) Fill(c byte) {
	for i := range r.FB {
		r.FB[i] = c
	}
}

// Dim darkens the frame buffer through shading map n.
func (r *Renderer) Dim(n int) {
	row := r.lightabs[(n&15)<<8:]
	for i, c := range r.FB {
		r.FB[i] = row[c]
	}
}

// angle is a 16-bit angle's sine and cosine.
func angle(a int16) (float64, float64) {
	return math.Sincos(float64(a) * (2 * math.Pi / 65536))
}

func mul(a, b mat3) (r mat3) {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				r[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return
}

// setupView builds the frame's view (INSTANCE.ASM's render_3d: create_matrix and
// scale_matrix). The view is the camera's heading, pitch and bank in that order, the
// depth scaled by the zoom and x by the pixels' shape.
func (r *Renderer) setupView() {
	sh, ch := angle(r.cam.Heading)
	sp, cp := angle(r.cam.Pitch)
	sb, cb := angle(r.cam.Bank)
	rot := mul(mul(mat3{{ch, 0, sh}, {0, 1, 0}, {-sh, 0, ch}},
		mat3{{1, 0, 0}, {0, cp, sp}, {0, -sp, cp}}),
		mat3{{cb, -sb, 0}, {sb, cb, 0}, {0, 0, 1}})

	// The eye is 8 units left of and 10 above the camera, in the camera's frame.
	off := vec3{-8, 10, 0}
	r.eye = [3]float64{float64(r.cam.X), float64(r.cam.Z), float64(r.cam.Y)}
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			r.eye[j] += off[i] * rot[j][i]
		}
	}

	// scale_matrix: the half sizes of the window, x squeezed so that the screen's 4:3
	// becomes the frustum's square, and the zoom on the depth.
	r.projX = float64(r.W >> 1)
	r.projY = float64(r.H >> 1)
	scale := vec3{1, 1, 1}
	k := pixelAspect
	if t := float64(int(k * r.projX)); t > r.projY {
		scale[0] = r.projY / t
	} else if t < r.projY {
		scale[1] = k * r.projX / r.projY
	}
	if z := float64(r.zoom) / 32768; r.zoom >= 0 {
		scale[2] = z
	} else {
		scale[0] *= -z
		scale[1] *= -z
	}
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			r.base[a][b] = rot[a][b] * scale[b] / 2
		}
	}
	r.cx, r.cy = float64(r.W)/2, float64(r.H)/2

	// A horizontal unit across the view, for sprites: the view's direction on the ground
	// turned a quarter.
	m := &r.base
	cc, ce := m[0][2], m[2][2]
	if l := math.Hypot(cc, ce); l > 1e-9 {
		cc, ce = cc/l, ce/l
	} else {
		cc, ce = 0, 1
	}
	r.spriteX = ce*m[0][0] - cc*m[2][0]
}

// codes is a view-space point's clip codes: 1 left of the view, 2 right, 4 above,
// 8 below, 80h behind the eye.
func codes(v vec3) uint8 {
	var c uint8
	if v[0] < -v[2] {
		c |= 1
	}
	if v[0] > v[2] {
		c |= 2
	}
	if v[1] > v[2] {
		c |= 4
	}
	if v[1] < -v[2] {
		c |= 8
	}
	if v[2] <= 0 {
		c |= 0x80
	}
	return c
}

// rotate turns an offset in the current object's frame into view space.
func (r *Renderer) rotate(d vec3) (v vec3) {
	s := math.Ldexp(1, r.objShift)
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			v[b] += d[a] * s * r.m[a][b]
		}
	}
	return
}

// transform is a point of the current object in view space.
func (r *Renderer) transform(p vec3) vec3 {
	return r.rotate(vec3{p[0] - r.obj[0], p[1] - r.obj[1], p[2] - r.obj[2]})
}

// project is a view-space point's place on the screen, in pixels from the top left.
func (r *Renderer) project(v vec3) (float64, float64) {
	return r.cx + v[0]*r.projX/v[2], r.cy - v[1]*r.projY/v[2]
}
