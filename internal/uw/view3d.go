package uw

import (
	"time"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of VIEW3D.C: setting up and drawing a frame of the 3D view. setupVars turns the
// camera into the first quadrant, viewPasses builds the vision grid and has GRIDDB.C's
// processGridWith draw the frame with the renderer (internal/render), and sendDb copies its
// frame buffer to gr.FB and the screen.

// gvec is an edge of the vision arc walking the grid. mapi is a tile index (it may leave
// the map: tileAt guards it), loc and loc2 indices into glocs.
type gvec struct {
	link      byte
	dx, dy    int16
	x         int8
	fx        uint8
	y         int8
	fy        uint8
	mapi      int
	loc, loc2 int
}

// gloc is one cell of the vision grid.
type gloc struct{ flags, shade byte }

// The grid is kept flat with spare cells either side, as the edges may step one cell
// past a row's ends (into the neighbouring row, as in the original's array).
const (
	glocPad  = 40
	glocRow  = 33
	glocSize = glocPad*2 + 17*glocRow
)

var glocs [glocSize]gloc

// glocAt is the index of row j, column i.
func glocAt(j, i int) int { return glocPad + j*glocRow + i }

var chgtable = [4][3]int{
	{1, MAP_SIZE, -1}, {-MAP_SIZE, 1, MAP_SIZE}, {-1, -MAP_SIZE, 1}, {MAP_SIZE, -1, -MAP_SIZE},
}
var headmod = [4]uint16{0, 0x4000, 0x8000, 0xC000}
var trans_grid = [4][16]byte{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	{0, 1, 4, 2, 5, 3, 9, 8, 6, 7},
	{0, 1, 5, 4, 3, 2, 7, 6, 9, 8},
	{0, 1, 3, 5, 2, 4, 8, 9, 7, 6},
}
var enc_dat = [16][7]byte{
	{0},
	{0xB8, 0x98, 0xB0, 0x98, 0xB0, 0x98, 0xB0},
	{0xE4, 0xC4, 0xE4, 0xC4, 0xE0, 0xC4, 0xE4},
	{0xCD, 0xCD, 0xC5, 0xC9, 0xC5, 0xCD, 0xC5},
	{0xD6, 0xD2, 0x00, 0xD6, 0x00, 0xD6, 0x00},
	{0xD7, 0x00, 0xD3, 0x00, 0xD7, 0x00, 0xD7},
	{0xBC, 0x9C, 0xB4, 0x9C, 0xB4, 0x9C, 0xB4},
	{0xBD, 0x9D, 0xB5, 0x9D, 0xB5, 0x9D, 0xB5},
	{0xBE, 0x9E, 0xB6, 0x9E, 0xB6, 0x9E, 0xB6},
	{0xBF, 0x9F, 0xB7, 0x9F, 0xB7, 0x9F, 0xB7},
}
var (
	side_dark = [2]byte{TW_NORTH, 0}
	side_wall = [2]byte{TW_WEST, TW_EAST}
	side_tile = [2]byte{2, 3}
	side_step = [2]int{-1, 1}
)

var (
	cybOn    bool
	xwid     int
	xhgt     int
	curZoom  uint16
	quad     int
	loct     int
	strtime  uint32
	mapptr   int // the eye's tile index
	mxY      int
	gvecs    [16]gvec // [15] stands for gvechead (only its link is used)
	lastXeye int
	lastYeye int
)

const gvechead = 15

// solidTile stands for any tile off the map.
var solidTile Tile

// tileAt is the tile at index i, or a solid tile off the map.
func tileAt(i int) *Tile {
	if i >= 0 && i < MAP_SIZE*MAP_SIZE {
		return &Level.Tiles[i]
	}
	return &solidTile
}

// place3dView places the view window (w by h, its bottom row at y) and sets the zoom.
func place3dView(x, y, w, h int) {
	demoMode = false
	xwid = w
	xhgt = h
	cPlaceFB(x, y, w, h)
	mous3dSet(x, y, w, h)
	mousPlayer(x, y-h+1, w, h)
	if inplist.mode&8 != 0 {
		curZoom = 0x6062
	} else if inplist.mode&MODE_GAME != 0 {
		demoMode = true
		curZoom = 0x61A8
	} else {
		curZoom = 0x7ED2
	}
	initRenderer()
	rend.SetZoom(int(int16(curZoom)))
}

func cPlaceFB(x, y, w, h int) {
	gr.PlaceFB(x, y, w, h)
	initRenderer()
	rend.Resize(w, h)
}

// setCyb is the hallucination effect: it scrambles the level textures' slots' row masks.
func setCyb(on bool) {
	if cybOn == on {
		return
	}
	cybOn = on
	initRenderer()
	vals := [6]int{0xF0, 0xF0, 0x3E0, 0x3E0, 0xFC0, 0xFC0}
	masks := [6]int{0xFF, 0xFF, 0x3FF, 0x3FF, 0xFFF, 0xFFF}
	for i := 0; i < 6; i++ {
		v := vals[i]
		if on {
			v = rand() & masks[i]
		}
		rend.SetRowMask(i, v)
	}
}

// init3d initialises the renderer and the vision grid's shades.
func init3d() {
	initRenderer()
	presetGrid(8)
}

// sendDb shows the drawn frame.
func sendDb() {
	runRender()
	if demoMode {
		doFbufBms()
	}
	mous3dHide()
	gr.FBToScreen()
	mous3dShow()
}

// runRender copies the frame the renderer has drawn into gr.FB.
func runRender() {
	dprintf("render %v\n", time.Since(frameStart))
	copyFB()
}

// do3dGrab draws a pick frame into the frame buffer without showing it.
func do3dGrab() {
	do3dPickup()
	runRender()
}

// renderFB draws a frame into the frame buffer only.
func renderFB() {
	viewPasses()
	runRender()
}

// establishView draws a frame and shows it.
func establishView() {
	strtime = GameTime()
	viewPasses()
	sendDb()
}

// setupVars turns the camera into the first quadrant.
func setupVars() {
	getEye()
	lastXeye = int(cPlayer.x >> 8)
	lastYeye = int(cPlayer.y >> 8)
	mapptr = lastXeye + lastYeye*MAP_SIZE
	h := int(uint16(cPlayer.heading) >> 13)
	q := ((h + 1) & 7) >> 1
	loct = h % 2
	quad = q
	cPlayer.x &= 0xFF
	cPlayer.y &= 0xFF
	switch quad {
	case 1:
		t := cPlayer.x
		cPlayer.x = 0xFF - cPlayer.y
		cPlayer.y = t
	case 2:
		cPlayer.x = 0xFF - cPlayer.x
		cPlayer.y = 0xFF - cPlayer.y
	case 3:
		t := cPlayer.x
		cPlayer.x = cPlayer.y
		cPlayer.y = 0xFF - t
	}
	cPlayer.heading = int16(uint16(cPlayer.heading) - headmod[quad])
}

// presetGrid fills each cell's distance shade.
func presetGrid(rng int) {
	if rng >= 16 {
		return
	}
	div, base, low := smoothDiv, smoothBase, smoothLowpass
	var shades [16]byte
	for i := 0; i < 16; i++ {
		if i > rng {
			shades[i] = 15
			continue
		}
		t := int16(i << 8 >> 5)
		u := t * t
		t = u * 2
		u = int16(cSqRt(uint32(uint16(t))))
		t = int16(int(u)*int(div)) >> 6
		t += low
		if t < 0 {
			t = 0
		}
		u = t + base
		if u > 14 {
			u = 14
		}
		shades[i] = byte(u)
	}
	for j := 0; j < 17; j++ {
		for i := 0; i < 33; i++ {
			d := cSqRt(uint32((16-i)*(16-i) + j*j))
			if d > rng {
				glocs[glocAt(j, i)].shade = 15
			} else {
				glocs[glocAt(j, i)].shade = shades[d]
			}
		}
	}
}

// initGrid starts the two edges of the vision arc at the eye.
func initGrid() {
	g := glocAt(0, 16)
	if tileAt(mapptr).Type() == TILE_SOLID {
		gvecs[gvechead].link = 15
		return
	}
	gvecs[gvechead].link = 0
	for i := 0; i < 2; i++ {
		v := &gvecs[i]
		v.x, v.y = 0, 0
		v.fx, v.fy = uint8(cPlayer.x), uint8(cPlayer.y)
		v.mapi = mapptr
		v.loc = g
	}
	gvecs[0].link = 0x81
	gvecs[1].link = 15
	gvecs[1].dx, gvecs[1].dy = cSinCos(int(uint16(cPlayer.heading + 0x2040)))
	gvecs[0].dx, gvecs[0].dy = cSinCos(int(uint16(cPlayer.heading - 0x2040)))
	gvecs[0].dx >>= 4
	gvecs[0].dy >>= 4
	gvecs[1].dx >>= 4
	gvecs[1].dy >>= 4
}

func incvec(v *gvec) {
	v.x++
	v.mapi += chgtable[quad][0]
	v.loc++
}

func decvec(v *gvec) {
	v.x--
	v.mapi -= chgtable[quad][0]
	v.loc--
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func b2int(b bool) int {
	if b {
		return 1
	}
	return 0
}

// encNChk marks which faces of the edge's tile are seen; with dir set, it steps the edge
// sideways past a wall of the kind asked for.
func encNChk(v *gvec, dir int, want byte) bool {
	shade := int(glocs[v.loc].shade & 0xF)
	tq := &trans_grid[quad]
	ct := &chgtable[quad]
	m := tileAt(v.mapi)
	here := int(tq[m.Type()])
	var sel int
	if v.x != 0 {
		sel = b2int(v.x > 0) + 1
		ax, ay := iabs(int(v.x)), iabs(int(v.y))
		if ax > ay {
			sel += 2
		}
		if ax == ay {
			sel += 4
		}
	}
	code := int(enc_dat[here][sel])
	if code == 0 {
		glocs[v.loc].flags = 0
		return false
	}
	mh := m.Height()
	if code&GLOC_FAR != 0 {
		n := tileAt(v.mapi + ct[1])
		other := int(tq[n.Type()])
		if tileWalls[other]&TW_SOUTH == 0 {
			if uint16(n.Height()+b2int(tileWalls[other]&TW_SLOPE == TW_SLOPE)-b2int(other == TILE_SLOPE_N)) <=
				uint16(mh+b2int(here == 6)+b2int(other == here && here != 1)) {
				code -= GLOC_FAR
			} else {
				shade += GLOC_STEP_FAR
			}
		}
	}
	if code&GLOC_RIGHT != 0 {
		n := tileAt(v.mapi + ct[0])
		other := int(tq[n.Type()])
		if tileWalls[other]&TW_WEST == 0 {
			if uint16(n.Height()+b2int(tileWalls[other]&TW_SLOPE == TW_SLOPE)-b2int(other == TILE_SLOPE_E)) <=
				uint16(mh+b2int(here == 8)+b2int(other == here && here != 1)) {
				code -= GLOC_RIGHT
			} else {
				shade += GLOC_STEP_RIGHT
			}
		}
	}
	if code&GLOC_LEFT != 0 {
		n := tileAt(v.mapi - ct[0])
		other := int(tq[n.Type()])
		if tileWalls[other]&TW_EAST == 0 {
			if uint16(n.Height()+b2int(tileWalls[other]&TW_SLOPE == TW_SLOPE)-b2int(other == TILE_SLOPE_W)) <=
				uint16(mh+b2int(here == 9)+b2int(other == here && here != 1)) {
				code -= GLOC_LEFT
			} else {
				shade += GLOC_STEP_LEFT
			}
		}
	}
	glocs[v.loc].flags = byte(code)
	glocs[v.loc].shade = byte(shade)
	if dir != 0 {
		far := tileWalls[tq[tileAt(v.mapi+ct[1]).Type()]] & TW_SOUTH
		if far == want && side_dark[b2int(want == TW_SOUTH)] == tileWalls[here]&TW_NORTH ||
			tileWalls[here]&TW_DIAG == TW_DIAG && side_dark[b2int(want == 0)] == tileWalls[here]&TW_NORTH {
			if dir == 1 {
				incvec(v)
				v.fx = 0
			} else {
				decvec(v)
				v.fx = 0xFF
			}
			return true
		}
	}
	return false
}

// newdels moves both edges on to the next row; false when the arc has closed.
func newdels(a, b *gvec) bool {
	a.y++
	if a.y > 16 {
		return false
	}
	for glocs[a.loc+33].shade&0xF == 0xF {
		incvec(a)
		a.fx = 0
		encNChk(a, 0, 0)
		if a.x > b.x {
			return false
		}
	}
	px, py := int(cPlayer.x), int(cPlayer.y)
	if a.y > 1 || iabs(int(a.fx)-px)+iabs(int(a.fy)-py) > 16 {
		a.dx = int16(int(a.x)<<8 + int(a.fx) - px)
		a.dx -= int16(iabs(int(a.dx))/50 + 2)
		a.dy = int16(int(a.y)<<8 - py)
	}
	a.mapi += chgtable[quad][1]
	a.loc += 33
	a.fy = 0
	b.y++
	for glocs[b.loc+33].shade&0xF == 0xF {
		decvec(b)
		b.fx = 0xFF
		encNChk(b, 0, 0)
		if a.x > b.x {
			return false
		}
	}
	if b.y > 1 || iabs(int(b.fx)-px)+iabs(int(b.fy)-py) > 16 {
		b.dx = int16(int(b.x)<<8 + int(b.fx) - px)
		b.dx += int16(iabs(int(b.dx))/50 + 2)
		b.dy = int16(int(b.y)<<8 - py - 1)
	}
	b.fy = 0
	b.loc += 33
	b.mapi += chgtable[quad][1]
	return true
}

// moveToNext walks one edge across its row to the next row boundary.
func moveToNext(v *gvec) {
	side := 1
	if v.dx < 0 {
		side = 0
	}
	other := (side + 1) % 2
	var rem int32
	if side == 1 {
		rem = 0x100 - int32(v.fx)
	} else {
		rem = int32(v.fx)
	}
	ownSide := (int(v.link)&0x80)^(side<<7) == 0
	if ownSide {
		v.loc2 = v.loc
	}
	step := int32(side_step[side])
	goOn := func() bool {
		if v.dy == 0 {
			return true
		}
		if v.dx == 0 {
			return false
		}
		return step*int32(v.dx)*(0x100-int32(v.fy)) > rem*int32(v.dy)
	}
	g := goOn()
	for g {
		t := trans_grid[quad][tileAt(v.mapi).Type()]
		nb := trans_grid[quad][tileAt(v.mapi+side_step[side]*chgtable[quad][0]).Type()]
		if tileWalls[t]&side_wall[side] == 0 && tileWalls[nb]&side_wall[other] == 0 {
			v.fy += uint8(int32(v.dy) * rem / (step * int32(v.dx)))
			v.fx = uint8(other * 0xFF)
			rem = 0x100
			if ownSide {
				encNChk(v, 0, 0)
			}
			if side_step[side] == 1 {
				incvec(v)
			} else {
				decvec(v)
			}
			if glocs[v.loc].shade&0xF == 0xF || iabs(int(v.x)) > 16 {
				if side_step[other] == 1 {
					incvec(v)
				} else {
					decvec(v)
				}
				v.fx = uint8(side * 0xFF)
				v.fy = 0xFF
				if !ownSide {
					v.loc2 = v.loc
				}
				return
			}
		} else {
			v.fy = 0xFF
			if tileWalls[t]&side_wall[side] == side_wall[side] {
				if side_tile[side] == t {
					v.fx = uint8(other * 0xFF)
				} else {
					v.fx = uint8(side * 0xFF)
				}
			} else {
				v.fx = uint8(side * 0xFF)
			}
			if !ownSide {
				v.loc2 = v.loc
			}
			return
		}
		g = goOn()
	}
	q := int16(step * int32(v.dx) * (0xFF - int32(v.fy)) / int32(v.dy))
	v.fx += uint8(int(step) * int(q))
	v.fy = 0xFF
	if !ownSide {
		v.loc2 = v.loc
	}
}

// fillTheGrid fills the cells between the edge after cur and the next one on this row.
func fillTheGrid(cur *int, out *int) {
	c := &gvecs[*cur]
	a := &gvecs[c.link&0xF]
	b := &gvecs[a.link&0xF]
	*out = b.loc2 + 1
	for encNChk(a, 1, 8) {
		if int(a.x)<<8+int(a.fx) > int(b.x)<<8+int(b.fx) {
			c.link = c.link&0xF0 + b.link&0xF
			a.link = 0
			b.link = 0
			return
		}
	}
	if a.x < b.x {
		for encNChk(b, -1, 8) {
		}
	}
	tmp := *a
	if !newdels(a, b) {
		c.link = c.link&0xF0 + b.link&0xF
		a.link = 0
		b.link = 0
		return
	}
	*cur = int(a.link & 0xF)
	if a.x != b.x {
		incvec(&tmp)
		for b.x > tmp.x {
			for encNChk(&tmp, 1, 0) {
				if b.x <= tmp.x {
					break
				}
			}
			if b.x > tmp.x {
				incvec(&tmp)
				for encNChk(&tmp, 1, 8) {
					if b.x <= tmp.x {
						break
					}
				}
				incvec(&tmp)
			}
		}
	}
}

// buildGrid builds the vision grid row by row from the eye outwards.
func buildGrid() {
	mxY = -1
	g := glocAt(0, 0)
	for {
		mxY++
		for cur := gvechead; gvecs[cur].link&0xF != 0xF; {
			v := int(gvecs[cur].link & 0xF)
			moveToNext(&gvecs[v])
			cur = v
		}
		cur := gvechead
		end := g + 33
		for gvecs[cur].link&0xF != 0xF {
			for gvecs[gvecs[cur].link&0xF].loc2 > g {
				glocs[g].flags = 0
				g++
			}
			fillTheGrid(&cur, &g)
		}
		for g < end {
			glocs[g].flags = 0
			g++
		}
		if gvecs[gvechead].link == 15 || mxY >= 16 {
			break
		}
	}
}

// camera is struct Camera (view3d.h): where the view is drawn from.
type camera struct{ x, y, z, heading, pitch, bank int16 }

// cPlayer is the camera of the current frame, the eye turned into the first quadrant.
var cPlayer camera

// The view fills the screen's width, which the original's vision grid does not: it reaches
// 45 degrees either side of the heading (and the squares beside the player) and no further
// back than the eye's row. The frame is drawn in up to two passes (after UltimaHacks'
// setupPerspectiveAndEnqueueDraw, as underworld-exhumed's VIEW3D.C has it), begun from
// the camera looking ahead: at a pitch beyond the original's first the squares behind the
// player, the grid built with the heading turned half round and cut to a small diamond,
// without their objects, each point turned back into the forward frame (gridPoint); then the
// view ahead, its edge rays opened to a right angle either side and the squares beyond the
// view's half width dropped unless within five squares.
var drawingBehind bool // the back pass is drawing

// viewHalfAngle is the view's half width: the full-width view at the game's zoom shows
// 59 degrees either side of the heading (x/z up to 1.67), and a little more is kept.
const viewHalfAngle = 0x2B00

func viewPasses() {
	setupVars() // the frame's camera looks ahead
	processGridWith(func() {
		if PlayerPitch > 0x1000 || PlayerPitch < -0x1000 {
			drawingBehind = true
			PlayerFacing = int16(uint16(PlayerFacing) + 0x8000)
			setupVars()
			initGrid()
			openRays()
			buildGrid()
			cullRearward()
			subprocess()
			PlayerFacing = int16(uint16(PlayerFacing) + 0x8000)
			drawingBehind = false
		}
		setupVars()
		initGrid()
		openRays()
		buildGrid()
		cullForward()
		mousQUp(false)
		subprocess()
	})
}

// openRays turns initGrid's edge rays (heading -+ 0x2040) to a right angle either side: the
// sines -0x7FFF and 0x7FFF and cosines 0, scaled as initGrid scales its own.
func openRays() {
	gvecs[0].dx, gvecs[0].dy = -0x800, 0
	gvecs[1].dx, gvecs[1].dy = 0x7FF, 0
}

// cullRearward keeps row 0 within 3 columns of the eye, row 1 within 5, row 2 within 3.
func cullRearward() {
	for row := 0; row < 17; row++ {
		lim := -1
		switch row {
		case 0, 2:
			lim = 3
		case 1:
			lim = 5
		}
		for col := -16; col <= 16; col++ {
			if col > lim || col < -lim {
				glocs[glocAt(row, col+16)].flags = 0
			}
		}
	}
}

// cullForward drops the squares outside the view's half width of the heading, unless
// |column| + row <= 5.
func cullForward() {
	la, ra := cPlayer.heading-viewHalfAngle, cPlayer.heading+viewHalfAngle
	ls, lc := cSinCos(int(uint16(la)))
	rs, rc := cSinCos(int(uint16(ra)))
	for row := 0; row < 17; row++ {
		left, right := -16, 16
		if la > -0x3F00 {
			left = max(int(int16(int32(row+1)*int32(ls)/int32(lc)))-1, -16)
		}
		if ra < 0x3F00 {
			right = min(int(int16(int32(row+1)*int32(rs)/int32(rc)))+1, 16)
		}
		for col := -16; col <= 16; col++ {
			if iabs(col)+row > 5 && (col < left || col > right) {
				glocs[glocAt(row, col+16)].flags = 0
			}
		}
	}
}
