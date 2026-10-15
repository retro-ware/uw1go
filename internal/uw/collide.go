package uw

// Port of COLLIDE.C: terrain and object collision for anything that moves or is placed:
// the floor height under the four corners of a mover's footprint, the walls between
// them, the objects it overlaps sorted by height, and the placing of objects in the map.
//
// All of it works on the collision record curP points at: the caller sets x, y, z (1/8
// tiles, z in object units), radius, height and the object's index, and the checks fill
// hits0, hits1, floor, top, slope, open, found, count and first.

// MotionCalc is the motion calculation record.
type MotionCalc struct {
	x, y, z      int16
	heading      uint16
	radius       byte
	height       byte
	index        int16
	hits0, hits1 int16
	floor, top   byte
	slope, open  byte
	found, count byte
	first        int8
}

// Collision is what the object checks found in a mover's way.
type Collision struct {
	top, bottom byte
	link        Link
	offset      int16
}

// pnt is a corner of the footprint, or (the fifth) its centre: which of the nine tiles
// around the centre tile it is in, and where in that tile.
type colPnt struct {
	tile, x, y byte
	flags      uint16
}

var (
	xlow, xhigh, ypos, ylow, yhigh int8
	// oCollisions has a ninth record: obj_coll_check's bound lets the original write one
	// past its eight.
	oCollisions [9]Collision
	colTiles    [9]int16
	curP        *MotionCalc
	pnt         [5]colPnt
	centIdx     int // the centre tile's index in the map (centptr)
	nvokHgt     int16
	firstsolve  bool
	posX        int8
	nvokTerr    int16

	tileOff = [9]int{-65, -64, -63, -1, 0, 1, 63, 64, 65}
)

// tileTerr is a tile's terrain word: type 0-3, height 4-7, the floor texture's terrain
// byte from bit 4. Outside the map it reads as a solid tile.
func tileTerr(i int) int16 {
	if i < 0 || i >= len(Level.Tiles) {
		return TILE_SOLID
	}
	t := &Level.Tiles[i]
	return int16(t.Type() + t.Height()<<4 + int(TxmTerr[t.Floor()]&0xFF)<<4)
}

// GetHgt is the floor height at point n (0 to 3 the corners, 4 the centre).
func GetHgt(n int, steep *bool) byte {
	t := colTiles[pnt[n].tile]
	h := byte((t & 0xF0) >> 1)
	p := &pnt[n]
	*steep = false
	switch t & 0xF {
	case TILE_SOLID:
		h = 0x80
	case TILE_DIAG_SE:
		if p.y >= p.x {
			h = 0x80
		}
		*steep = true
	case TILE_DIAG_SW:
		if int(p.x)+int(p.y) >= 7 {
			h = 0x80
		}
		*steep = true
	case TILE_DIAG_NE:
		if int(p.x)+int(p.y) <= 7 {
			h = 0x80
		}
		*steep = true
	case TILE_DIAG_NW:
		if p.y <= p.x {
			h = 0x80
		}
		*steep = true
	case TILE_SLOPE_N:
		h += p.y & 7
	case TILE_SLOPE_S:
		h += 7 - p.y&7
	case TILE_SLOPE_E:
		h += p.x & 7
	case TILE_SLOPE_W:
		h += 7 - p.x&7
	}
	return h
}

// GetSlopeHgt is the floor height under x, y (1/256 tiles) in the centre tile, in the
// physics record's z units, following a slope exactly.
func GetSlopeHgt(x, y int) int {
	h := 0
	y &= 0xFF
	x &= 0xFF
	switch colTiles[4] & 0xF {
	case TILE_SLOPE_N:
		h = y
	case TILE_SLOPE_S:
		h = 0xFF - y
	case TILE_SLOPE_E:
		h = x
	case TILE_SLOPE_W:
		h = 0xFF - x
	}
	h >>= 2
	h += int(colTiles[4]&0xF0) << 2
	return h
}

// SolvePnt sets pnt[n].flags from the floor at corner n against curP.z, within range.
// It returns false if the corner is in a diagonal tile.
func SolvePnt(n int, rng byte) bool {
	var steep bool
	h := GetHgt(n, &steep)
	var flags uint16
	switch {
	case h == 0x80:
		flags |= 0x200
	case int(curP.z)+int(rng) < int(h):
		flags |= 0x100
	case int(curP.z)-int(rng) > int(h):
		flags |= 0x800
	default:
		flags |= 8 << ((colTiles[pnt[n].tile] & 0x300) >> 8)
	}
	pnt[n].flags = flags
	if curP.top < h {
		curP.top = h
	}
	return !steep
}

// SolveCenter is SolvePnt for the centre, into hits0.
func SolveCenter(rng byte) bool {
	var steep bool
	curP.hits0 = (colTiles[4] & 0x300) >> 8
	curP.floor = GetHgt(4, &steep)
	switch {
	case curP.floor == 0x80:
		curP.hits0 |= 0x200
	case int(curP.z)+int(rng) < int(curP.floor):
		curP.hits0 |= 0x100
	case int(curP.z)-int(rng) > int(curP.floor):
		curP.hits0 |= 0x800
	default:
		curP.hits0 |= 4
		curP.hits0 |= 8 << ((colTiles[4] & 0x300) >> 8)
	}
	if colTiles[4]&0xF >= TILE_SLOPE_N {
		curP.hits0 |= 0x2000
	}
	return !steep
}

func loadColTile(n int) {
	if colTiles[n] == 0x1111 {
		colTiles[n] = tileTerr(centIdx + tileOff[n])
	}
}

// TerrainCheck finds the terrain under curP's footprint.
func TerrainCheck(rng byte) {
	for i := range colTiles {
		colTiles[i] = 0x1111
	}
	centIdx = int(curP.x>>3) + int(curP.y>>3)<<6
	x := int(curP.x & 7)
	y := int(curP.y & 7)
	off := 4
	pnt[4] = colPnt{tile: byte(off), x: byte(x), y: byte(y), flags: pnt[4].flags}
	loadColTile(4)
	SolveCenter(rng)
	curP.hits1 = curP.hits0
	curP.top = curP.floor
	if curP.radius == 0 {
		return
	}
	r := int(curP.radius)
	for y -= r; y < 0; y += 8 {
		off -= 3
	}
	for x -= r; x < 0; x += 8 {
		off--
	}
	pnt[0].tile, pnt[0].x, pnt[0].y = byte(off), byte(x), byte(y)
	for x += r * 2; x > 7; x -= 8 {
		off++
	}
	pnt[1].tile, pnt[1].x, pnt[1].y = byte(off), byte(x), byte(y)
	for y += r * 2; y > 7; y -= 8 {
		off += 3
	}
	pnt[2].tile, pnt[2].x, pnt[2].y = byte(off), byte(x), byte(y)
	for x -= r * 2; x < 0; x += 8 {
		off--
	}
	pnt[3].tile, pnt[3].x, pnt[3].y = byte(off), byte(x), byte(y)
	for i := 0; i < 4; i++ {
		loadColTile(int(pnt[i].tile))
	}
	firstsolve = true
	walls := [5]byte{4, 0x10, 2, 8, 4}
	for i := 0; i < 4; i++ {
		if !SolvePnt(i, rng) && pnt[i].flags&0x300 == 0 {
			for j := 0; j < 2; j++ {
				if pnt[(i+j<<1+1)&3].tile != pnt[i].tile &&
					tileWalls[colTiles[pnt[i].tile]&0xF]&walls[j+i] != 0 {
					pnt[i].flags = 0x200
					curP.top = 0x80
					j = 2
				}
			}
			firstsolve = false
		}
	}
	curP.hits1 |= int16(pnt[0].flags | pnt[1].flags | pnt[2].flags | pnt[3].flags)
}

var (
	colSide = [4]int8{1, -1, -1, 1}
	colDirs = [3][3]byte{{5, 4, 3}, {6, 9, 2}, {7, 0, 1}}
)

// ComputeHeading finds from the last TerrainCheck curP.slope, the direction of the
// corners that hit a wall or a high floor, and curP.open, that of the clear corners.
func ComputeHeading() {
	var ox, oy, sx, sy int8
	nopen, nsteep := 0, 0
	for i := 0; i < 4; i++ {
		if pnt[i].flags&0xF8 == 0 {
			oy += colSide[i]
			ox += colSide[(i+3)&3]
			nopen++
		}
		if pnt[i].flags&0x300 != 0 {
			sx += colSide[i]
			sy += colSide[(i+3)&3]
			nsteep++
		}
	}
	if nsteep != 0 {
		curP.slope = colDirs[int(sx)/nsteep+1][int(sy)/nsteep+1]
		if nsteep == 1 && curP.slope%2 != 0 && firstsolve {
			want := (9 - curP.slope) & 7
			head := byte(curP.heading >> 13)
			switch (head - want) & 7 {
			case 0, 1:
				curP.slope = 9
			case 2, 3:
				curP.slope = (curP.slope + 1) & 7
			case 4, 5:
				k := (curP.slope - 1) >> 1
				curP.slope--
				var a, b byte
				switch k {
				case 0:
					a, b = pnt[k].x, pnt[k].y
				case 1:
					a, b = pnt[k].x, 8-pnt[k].y
				case 2:
					a, b = pnt[k].y, pnt[k].x
				case 3:
					a, b = 8-pnt[k].x, pnt[k].y
				}
				if a < b {
					curP.slope = (curP.slope + 2) & 7
				}
				if a == b {
					curP.slope++
				}
			case 6, 7:
				curP.slope = (curP.slope + 7) & 7
			}
		}
	} else {
		curP.slope = 9
	}
	if nopen == 1 || nopen == 2 {
		curP.open = colDirs[int(oy)/-nopen+1][int(ox)/-nopen+1]
	} else {
		curP.open = 9
	}
}

// objCollCheck adds obj, in tile x, y relative to the centre, to oCollisions if its
// square overlaps the mover's.
func objCollCheck(obj *Object, link int, x, y int8, isnpc bool) {
	if curP.found > 8 {
		return
	}
	com := &ComObjData[obj.Item()]
	var x0, y0, x1, y1 int8
	if com.Radius() == 4 {
		x0 = x << 3
		x1 = x0 + 7
		y0 = y << 3
		y1 = y0 + 7
	} else {
		x0 = x<<3 + int8(obj.FineX())
		y0 = y<<3 + int8(obj.FineY())
		r := com.Radius()
		if obj.Major() == MAJOR_CREATURE && r > 0 && isnpc {
			r--
		}
		x1, y1 = x0+int8(r), y0+int8(r)
		x0, y0 = x0-int8(r), y0-int8(r)
	}
	if x1 < xlow || x0 > xhigh || y1 < ylow || y0 > yhigh {
		return
	}
	c := &oCollisions[curP.found]
	curP.found++
	c.bottom = byte(obj.Z())
	c.top = c.bottom + byte(com.Height())
	if com.Height() == 0 {
		c.top++
	}
	c.link = mkLink(link, 9)
	if posX >= x0 && posX <= x1 && ypos >= y0 && ypos <= y1 {
		c.link.SetLow(c.link.Low() | 0x10)
	}
	c.offset = int16(int(x) + int(y)<<6)
}

// ObjectCheck collects the objects around curP into oCollisions.
func ObjectCheck(flat, useflag bool) {
	base := int(curP.x>>3) + int(curP.y>>3)<<6
	isnpc := false
	if curP.index != 0 {
		isnpc = ObjIntTMem(int(curP.index)).Major() == MAJOR_CREATURE
	}
	curP.found = 0
	posX = int8(curP.x & 7)
	ypos = int8(curP.y & 7)
	var radius int8
	if flat && curP.height == 0 {
		radius = 0
	} else {
		radius = int8(curP.radius)
		if isnpc && radius > 0 {
			radius--
		}
	}
	xhigh, yhigh = posX+radius, ypos+radius
	xlow, ylow = posX-radius, ypos-radius
	x0, y0 := (xlow-11)/8, (ylow-11)/8
	x1, y1 := (xhigh+4)/8, (yhigh+4)/8
	for i := x0; i <= x1; i++ {
		for j := y0; j <= y1; j++ {
			n := 0
			ti := base + int(j)<<6 + int(i)
			if ti < 0 || ti >= len(Level.Tiles) {
				continue
			}
			for link := &Level.Tiles[ti].Objects; link.Index() != 0 && n < 0x40; n++ {
				obj := ObjPtrTMem(link)
				next := &obj.QN
				if link.Index() == int(curP.index) || collideSkip(int(curP.index), link.Index()) {
					link = next
					continue
				}
				com := &ComObjData[obj.Item()]
				static := ObjMemTPtr(obj) >= NUM_MOBILE
				switch {
				case isnpc && com.C32() != 0:
				case com.Height() == 0 && static:
				case !static && obj.Major() != MAJOR_CREATURE && obj.B157() != 0:
				case !useflag || com.Touch() != 0:
					objCollCheck(obj, link.Index(), i, j, isnpc)
				}
				link = next
			}
			if n == 0x40 {
				return
			}
		}
	}
}

func oCswap(i int) { oCollisions[i], oCollisions[i+1] = oCollisions[i+1], oCollisions[i] }

// processObjlist sorts oCollisions: those underfoot first (by top), then the rest by
// bottom; curP.first and curP.count give those in the mover's way.
func processObjlist() {
	flat := 0
	if curP.height == 0 {
		flat = 1
	}
	found := int(curP.found)
	n := 0
	for ; found > n; n++ {
		for k := found - 2; k >= n; k-- {
			if oCollisions[k].top > oCollisions[k+1].top {
				oCswap(k)
			}
		}
		if int(oCollisions[n].top) > int(curP.z) {
			break
		}
	}
	for k := n; found > k; k++ {
		for m := found - 2; m >= k; m-- {
			if oCollisions[m].bottom > oCollisions[m+1].bottom {
				oCswap(m)
			}
		}
	}
	curP.first = int8(n)
	curP.count = 0
	for n+int(curP.count) < found &&
		int(curP.z)+int(curP.height)+flat > int(oCollisions[n+int(curP.count)].bottom) {
		curP.count++
	}
}

// canPlace is whether an object of type item fits at x, y, z (1/8 tiles); it sets
// nvokHgt and nvokTerr.
func canPlace(item, index, x, y, z int, flier bool, rng int) bool {
	oldP := curP
	defer func() { curP = oldP }()
	var calc MotionCalc
	curP = &calc
	curP.index = int16(index)
	curP.radius = byte(ComObjData[item].Radius())
	curP.height = byte(ComObjData[item].Height())
	curP.x, curP.y, curP.z = int16(x), int16(y), int16(z)
	if curP.height != 0x80 && int(curP.height)+int(curP.z) > 0x7F {
		return false
	}
	TerrainCheck(byte(rng))
	if (curP.hits0|curP.hits1)&0x300 != 0 {
		return false
	}
	if int(curP.z)+rng >= int(curP.top) {
		nvokHgt = int16(curP.top)
	} else {
		nvokHgt = int16(curP.floor)
	}
	radius := int(curP.radius)
	if rng > radius {
		radius = rng
	}
	if int(curP.z) <= int(curP.floor)+radius {
		nvokTerr = 1 << (curP.hits0 & 3)
	} else {
		nvokTerr = FOOT_AIR
	}
	ObjectCheck(nvokTerr != FOOT_AIR && index >= NUM_MOBILE, true)
	if curP.found != 0 {
		best := -1
		processObjlist()
		if curP.count != 0 {
			return false
		}
		for i := 0; i < int(curP.first); i++ {
			if int16(oCollisions[i].top) > nvokHgt {
				best = i
				nvokHgt = int16(oCollisions[i].top)
			}
		}
		if best > -1 {
			if ComObjData[ObjPtrTMem(&oCollisions[best].link).Item()].Solid() == 0 {
				return false
			}
			nvokTerr = FOOT_FLOOR
		}
	}
	return flier || (curP.hits0|curP.hits1)&0x800 == 0 || int(curP.z)-rng <= int(nvokHgt)
}

// putAt puts obj near x, y, z with dropAroundPlace, or failing that exactly at x, y
// unless ObjElemFate culls it (nocull skips that); a culled object is freed.
func putAt(x, y, z int, obj *Object, rng int, nocull bool) bool {
	if dropAroundPlace(obj, x, y, z, rng) {
		return true
	}
	if nocull || !ObjElemFate(10, obj) {
		obj.SetFineX(x & 7)
		obj.SetFineY(y & 7)
		if t := MapGetAddr(x>>3, y>>3); t != nil {
			ObjAdd(&t.Objects, obj)
		}
		return true
	}
	ObjFreeLinkChain(nil, obj)
	return false
}

// nearMobPutAt is putAt at src's position.
func nearMobPutAt(src, obj *Object, rng int, nocull bool) bool {
	return putAt(src.HomeX()<<3+src.FineX(), src.HomeY()<<3+src.FineY(), src.Z(), obj, rng, nocull)
}

// stayCentered makes dropAroundPlace try the exact spot first.
var stayCentered bool

// dropAroundPlace tries up to 24 random spots within range of x, y for obj, adding it at
// the end of the first one's tile list.
func dropAroundPlace(obj *Object, x, y, z, rng int) bool {
	span := rng*2 + 1
	for tries := 0; tries < 24; tries++ {
		tx, ty := x, y
		if tries != 0 || !stayCentered {
			tx = x - rng + rand()%span
			ty = y - rng + rand()%span
		}
		if canPlace(obj.Item(), ObjMemTPtr(obj), tx, ty, z, true, 0) {
			tile := MapGetAddr(tx>>3, ty>>3)
			if tile == nil {
				continue
			}
			obj.SetFineX(tx & 7)
			obj.SetFineY(ty & 7)
			obj.SetZ(z)
			ObjAddEnd(&tile.Objects, obj)
			if IsMobElem(obj) {
				obj.SetHomeX(tx >> 3)
				obj.SetHomeY(ty >> 3)
			} else {
				objDeal(obj, tx>>3, ty>>3, 1)
			}
			return true
		}
	}
	return false
}
