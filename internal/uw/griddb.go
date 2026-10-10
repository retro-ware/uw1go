package uw

import "github.com/memmaker/uw1go/internal/render"

// Port of GRIDDB.C: drawing the 3D view from the map grid. subprocess walks the visible
// cells of the vision grid from the farthest row to the eye and grdbElem draws each tile's
// floor, ceiling, walls and diagonal face, then hands its objects to GAMESORT.C.

type flrFn func(pts *[4]render.Vertex, shade, tex byte)
type walFn func(pts *[4]render.Vertex, shade, height, tex byte)

var (
	loopx, loopy  int
	pGloc         int
	qdec          *[4]byte
	AnimObjInPipe byte
	colorToMap    [192]uint16
	colorToObj    [192]uint16
	PickUp        byte
	sqmod         byte
	tmptr         int // the tile index of the cell being emitted
	pipeexp       int
	cTmBm, cTmDm  int
	grFcall       flrFn
	grCcall       flrFn
	grWcall       walFn
	tCacheOK      byte

	dist8       = 4
	distpoly    = 7
	tmapson     = 1
	lighton     = 1
	curautocode byte

	gftab = [2]flrFn{polyflr, txtflr}
	gctab = [2]flrFn{polycie, polyflr}
	gwtab = [2]walFn{polywal, txtwal}
)

var quad_mod = [4][2]int{{1, MAP_SIZE}, {-MAP_SIZE, 1}, {-1, -MAP_SIZE}, {MAP_SIZE, -1}}
var qudecode = [4][4]byte{{0, 1, 3, 2}, {2, 0, 1, 3}, {3, 2, 0, 1}, {1, 3, 2, 0}}
var hgtmodtab = [5][4]byte{{0, 0, 1, 1}, {1, 1, 0, 0}, {0, 1, 0, 1}, {1, 0, 1, 0}, {0, 0, 0, 0}}
var pget1 = [3]byte{2, 0, 1}
var pget2 = [3]byte{0, 1, 3}
var thgt = [6][5]byte{
	{0, 0, 1, 0, 0}, {1, 0, 0, 0, 0}, {0, 0, 0, 1, 0},
	{1, 1, 0, 1, 0}, {0, 1, 1, 1, 0}, {1, 1, 1, 0, 0},
}
var wallmodtab = [3][6]byte{{1, 1, 3, 1, 0, 1}, {0, 1, 2, 1, 1, 3}, {0, 0, 0, 0, 1, 2}}
var dxtab = [4][6]int8{{0, 0, 1, 1, 1, -1}, {0, 1, 1, 0, -1, -1}, {1, 0, 0, 1, 1, 1}, {1, 1, 0, 0, -1, 1}}
var norm = [4][3]int8{{0, 4, -1}, {0, 4, 1}, {-1, 4, 0}, {1, 4, 0}}
var tile_mapcode = [16]byte{
	0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x0B, 0x0B,
	0x0B, 0x0B, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
}
var wall_nbr = [3]int{0, 1, 2}

// setGraphicsLevel texture maps every face: the detail is always very high, the setting
// gone from the options.
func setGraphicsLevel() {
	tmapson = 1
	gctab[1] = txtflr
	gftab[1] = txtflr
	lighton = 1
}

// processGrid draws a normal frame.
func processGrid() { processGridWith(subprocess) }

// processGridWith begins a frame from the current camera and has walk draw the squares
// (wide-pitch's two passes, viewPasses, walk the grid twice).
func processGridWith(walk func()) {
	oldlight := lighton
	AnimObjInPipe = 0
	PickUp = 0
	grFcall = gftab[tmapson]
	grCcall = gctab[tmapson]
	grWcall = gwtab[tmapson]
	if PlayerLevel == 9 {
		lighton = 0
	}
	beginFrame(false, tmapson == 0, PlayerLevel != 9)
	pipeexp = 0
	walk()
	if player.Level != 0 && player.Level < 0x10 && PlayerLevel != 9 {
		if exp := pipeexp * PlayerLevel / 10; exp != 0 {
			playerGetExp(exp)
		}
	}
	if PlayerLevel == 9 {
		lighton = oldlight
	}
}

// do3dPickup draws a pick frame.
func do3dPickup() {
	PickUp = 1
	beginFrame(true, true, false)
	grFcall = gftab[0]
	grCcall = gctab[0]
	grWcall = gwtab[0]
	sdXmod = quad_mod[quad][0]
	sdYmod = quad_mod[quad][1]
	subprocess()
}

func playersMapAt(i int) *byte { return &PlayersMap[i>>6][i&63] }

// subprocess walks the vision grid from the farthest row to the eye.
func subprocess() {
	tCacheOK = 0xE0
	qdec = &qudecode[quad]
	dx := chgtable[quad][0]
	dy := chgtable[quad][1]
	gl := glocAt(mxY, 0)
	row := mapptr + mxY*dy - dx<<4
	idx := row
	var i int
	sortSetup(-10)
	for loopy = mxY; loopy >= 0; loopy-- {
		sortSetup(2)
		loopx = 0
		pGloc = gl
		tmptr = row
		i = idx
		for ; loopx < 0x10; loopx, pGloc, tmptr, i = loopx+1, pGloc+1, tmptr+dx, i+dx {
			if i&^0xFFF == 0 {
				grdbElem(playersMapAt(i))
			}
		}
		sortSetup(1)
		loopx = 0x20
		pGloc = gl + loopx
		tmptr = row + loopx*dx
		i = idx + loopx*dx
		for ; loopx > 0x10; loopx, pGloc, tmptr, i = loopx-1, pGloc-1, tmptr-dx, i-dx {
			if i&^0xFFF == 0 {
				grdbElem(playersMapAt(i))
			}
		}
		sortSetup(0)
		if i&^0xFFF == 0 {
			grdbElem(playersMapAt(i))
		}
		mousQUp(true)
		gl -= 33
		row -= dy
		idx -= dy
	}
	if drawingBehind { // the back pass leaves the automap alone
		return
	}
	tmptr, i = row, idx
	for loopx = 0; loopx < 0x21; loopx, tmptr, i = loopx+1, tmptr+dx, i+dx {
		if i&^0xFFF == 0 {
			if am := playersMapAt(i); *am == 0 {
				*am = tile_mapcode[tileAt(tmptr).Type()]
			}
		}
	}
}

// flatFace fills a quad with colour c.
func flatFace(c int, pts *[4]render.Vertex) { rend.Polygon(byte(c), pts[:]...) }

func shadedTexColor(tex int, shade byte) int {
	return int(lightabs[(int(shade)*lighton<<8+int(texFirstPixel(tex)))&0xFFF])
}

func polyflr(pts *[4]render.Vertex, shade, tex byte) {
	if PickUp != 0 {
		flatFace(int(tex)+PICK_FLOOR, pts)
	} else {
		flatFace(shadedTexColor(int(tex)+0x6A, shade), pts)
	}
}

func polycie(pts *[4]render.Vertex, shade, tex byte) {
	if PickUp != 0 {
		flatFace(PICK_CEILING, pts)
	} else {
		flatFace(shadedTexColor(int(tex)+0x6A, shade), pts)
	}
}

func polywal(pts *[4]render.Vertex, shade, height, tex byte) {
	if PickUp != 0 {
		flatFace(int(tex)+PICK_WALL, pts)
	} else {
		flatFace(shadedTexColor(int(tex)+0x3A, shade), pts)
	}
}

// txtflr draws a texture-mapped floor or ceiling (with pts nil, only puts its texture in
// slot cTmBm).
func txtflr(pts *[4]render.Vertex, shade, tex byte) {
	if int(shade) >= distpoly && pts != nil {
		polyflr(pts, shade, tex)
		return
	}
	t := int(tex)
	vlimit := 0x3FF
	if int(shade) >= dist8 {
		t += 0x6A
		cTmBm = lighton
		vlimit = 0xFF
		cTmDm = 0x10
	} else {
		cTmBm = 2
		if shade != 0 {
			cTmBm += lighton
		}
		cTmDm = 0x20
		t += 0x30
	}
	rend.SetTexture(cTmBm, levelTex(t&0xFF))
	rend.SetTextureHeight(cTmBm, vlimit)
	if pts != nil {
		rend.TexQuad(cTmBm, *pts, [4]int{int(qdec[3]), int(qdec[2]), int(qdec[1]), int(qdec[0])})
	}
}

// txtwal draws a texture-mapped wall height tiles high (with pts nil, only puts its
// texture in slot cTmBm). tCacheOK counts the walls a tile has drawn: from the second on
// the slot already holds the texture.
func txtwal(pts *[4]render.Vertex, shade, height, tex byte) {
	if int(shade) >= distpoly && pts != nil {
		polywal(pts, shade, height, tex)
		return
	}
	t := int(tex)
	var vlimit int
	if int(shade) >= dist8 {
		cTmBm = 0
		if shade != 0 {
			cTmBm += lighton
		}
		cTmDm = 0x10
		vlimit = int(height)<<2<<4 - 1
		t += 0x3A
	} else {
		cTmBm = 4
		if shade != 0 {
			cTmBm += lighton
		}
		cTmDm = 0x40
		vlimit = int(height)<<4<<6 - 1
	}
	c := tCacheOK
	tCacheOK++
	if int8(c) < 1 {
		rend.SetTexture(cTmBm, levelTex(t&0xFF))
	}
	rend.SetTextureHeight(cTmBm, vlimit)
	if pts != nil {
		rend.WallQuad(cTmBm, *pts)
	}
}

// gridPoint places a corner of the grid: x, y a grid cell (the eye's column 16, its row
// 0), z a tile height (16 the ceiling).
func gridPoint(x, y, z int) render.Vertex {
	if drawingBehind { // wide-pitch's back pass: turned back into the forward frame
		x, y = 33-x, 1-y
	}
	return rend.Point((x-0x10)<<8, z<<6, y<<8)
}

// grdbElem draws one tile, the one at tmptr in grid cell pGloc.
func grdbElem(automap *byte) {
	t := tileAt(tmptr)
	flags := int(glocs[pGloc].flags)
	if flags&GLOC_SEEN == 0 {
		if *automap == 0 && !drawingBehind {
			*automap = tile_mapcode[t.Type()]
			pipeexp++
		}
		clearObjsort()
		return
	}
	ht := byte(t.Height())
	sqmod = glocs[pGloc].shade & 0xF
	var code byte
	if sqmod < 8 {
		code = byte(t.Type()) | byte(TxmTerr[t.Floor()])
	} else if code = *automap; code == 0 {
		code = tile_mapcode[t.Type()]
	}
	hq := 4
	if flags&(GLOC_DIAG|GLOC_SHAPED) == GLOC_SHAPED {
		hq = flags & 3
	}
	hm := &hgtmodtab[hq]
	var vis bool
	if hq == 4 {
		vis = cPlayer.z > hgtVal[ht]
	} else {
		s := int16(int16((loopx-0x10)<<8)-cPlayer.x)*int16(norm[hq][0]) +
			(int16(loopy<<8)-cPlayer.y)*int16(norm[hq][2]) +
			(hgtVal[int(ht)+int(hm[0])]-cPlayer.z)*int16(norm[hq][1])
		vis = s < 0
	}
	var pts [4]render.Vertex
	pt := gridPoint
	tCacheOK = 0xE0
	if vis {
		pts[0] = pt(loopx, loopy+1, int(ht+hm[2]))
		pts[1] = pt(loopx+1, loopy+1, int(ht+hm[3]))
		pts[2] = pt(loopx+1, loopy, int(ht+hm[1]))
		pts[3] = pt(loopx, loopy, int(ht+hm[0]))
		grFcall(&pts, sqmod, byte(t.Floor()))
	}
	if cPlayer.z <= 0x3F4 {
		pts[0] = pt(loopx, loopy, 0x10)
		pts[1] = pt(loopx+1, loopy, 0x10)
		pts[2] = pt(loopx+1, loopy+1, 0x10)
		pts[3] = pt(loopx, loopy+1, 0x10)
		grCcall(&pts, sqmod, 9)
	}
	bit := GLOC_STEP_RIGHT
	tCacheOK = 0
	for w := 0; w < 3; w++ {
		bit2 := bit
		bit >>= 1
		if flags&bit == 0 {
			continue
		}
		wm := &wallmodtab[w]
		var nh, h2, sh byte
		if int(glocs[pGloc].shade)&bit2 != 0 {
			n := tileAt(tmptr + chgtable[quad][wall_nbr[w]])
			nh = byte(n.Height())
			nt := trans_grid[quad][n.Type()]
			wi := byte(4)
			if tileWalls[nt]&TW_SLOPE == TW_SLOPE {
				wi = nt - 6
			}
			sh = nh + thgt[w+3][wi] - ht - thgt[w][hq]
			h2 = nh + hgtmodtab[wi][pget2[w]]
			nh = nh + hgtmodtab[wi][pget1[w]]
		} else {
			h2, nh = 0x10, 0x10
			sh = 0x10 - ht - thgt[w][hq]
		}
		pts[0] = pt(loopx+int(wm[0]), loopy+int(wm[1]), int(nh))
		pts[1] = pt(loopx+int(wm[3]), loopy+int(wm[4]), int(h2))
		pts[2] = pt(loopx+int(wm[3]), loopy+int(wm[4]), int(ht+hm[wm[5]]))
		pts[3] = pt(loopx+int(wm[0]), loopy+int(wm[1]), int(ht+hm[wm[2]]))
		grWcall(&pts, sqmod, sh, byte(t.Wall()))
	}
	if flags&(GLOC_DIAG|GLOC_SHAPED) == GLOC_DIAG|GLOC_SHAPED {
		d := &dxtab[flags&3]
		s := (int16((loopx+int(d[0])-0x10)<<8)-cPlayer.x)*int16(d[4]) +
			(int16((loopy+int(d[1]))<<8)-cPlayer.y)*int16(d[5])
		if s < 0 {
			pts[0] = pt(loopx+int(d[0]), loopy+int(d[1]), 0x10)
			pts[1] = pt(loopx+int(d[2]), loopy+int(d[3]), 0x10)
			pts[2] = pt(loopx+int(d[2]), loopy+int(d[3]), int(ht))
			pts[3] = pt(loopx+int(d[0]), loopy+int(d[1]), int(ht))
			grWcall(&pts, sqmod, 0x10-ht, byte(t.Wall()))
		}
	}
	if drawingBehind { // no objects behind the player, and the automap left alone
		return
	}
	doObjsort(&t.Objects)
	if curautocode != 0 {
		if sqmod < 8 {
			code |= curautocode << 6
		}
		curautocode = 0
	}
	if automapEnabled {
		*automap = code
	}
}

// UsPtr is the object the eye is attached to (the player, or none for a detached camera).
var UsPtr *Object
