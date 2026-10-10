package uw

// Port of PATHFIND.C (seg006_1477): getting a critter to a square. critHeadForLoc, called
// by AI.C's goals, follows a stored path, walks straight when beeline finds the tiles
// between walkable, searches for a path with floodPath (breadth first, at most 32 steps in
// a box 5 tiles around the start and destination) into one of 16 shared path slots, or
// gives up and wanders. A critter that bumps into a door tries to open it. It also has the
// collision handlers the physics engine calls for walking, flying and swimming critters,
// moveMeJoe for mobile objects that are not critters, lineOfSight and buildCorpse.

// PathSq is one square of a critter's path; beeline keeps the floor height in unused,
// floodPath marks a step needing a jump in flag.
type PathSq struct{ x, y, unused, flag byte }

// PathRec is a stored path: two bits of direction and one jump bit per step, the steps
// taken (count, the low 7 bits of flag) and the length (index).
type PathRec struct {
	x, y       byte
	flag       byte // count:7, slope:1
	index      byte
	directions [16]byte
	slopes     [8]byte
}

func (p *PathRec) count() int { return int(p.flag & 0x7F) }
func (p *PathRec) slope() int { return int(p.flag >> 7) }
func (p *PathRec) setCount(v int) {
	p.flag = p.flag&0x80 | byte(v&0x7F)
}
func (p *PathRec) setSlope(v int) { p.flag = p.flag&0x7F | byte(v&1)<<7 }

// StaticTile is the flood search's record of a square.
type StaticTile struct {
	pathx, pathy byte // the square the path came from
	height       byte
	flagDist     byte // pathflag:1, dist:7
	step         byte
}

func (s *StaticTile) pathflag() byte     { return s.flagDist & 1 }
func (s *StaticTile) dist() byte         { return s.flagDist >> 1 }
func (s *StaticTile) setPathflag(v byte) { s.flagDist = s.flagDist&^1 | v&1 }
func (s *StaticTile) setDist(v byte)     { s.flagDist = s.flagDist&1 | v<<1 }

type pathPt struct{ x, y byte }

var (
	critTerr       int16
	doorx, doory   int
	tdx, tdy       int16
	control        byte
	collobject     *Object
	tpAct          *PhysHandler
	pnAct          *Phys
	txpos, typos   byte
	tzpos          int8
	txpost, typost uint16
	aligned        byte
	dontchangedz   byte
	myid           byte
	jump           byte
	tdistsqr       uint16
	tdisttsqr      uint32
	failed         byte
	mycst          *Creature
	meptr          *Object
	pathlen        int
	myxpos, myypos int
	myzpos         int
	hitwall        byte
	mytarget       *Object
	myxhome        int
	myyhome        int
	myheight       byte
	didhitobj      byte
	myxpost        int16
	myypost        int16
	didmove        byte
	hitadoor       byte
	curDanger      byte
	myoldspeed     byte
	XP, YP         int
	myoldfacing    byte
	myoldheading   byte

	// one more than the original's 63 squares (and a few spare), which add_to_beeline_path
	// and store_path run past
	pathsq    [68]PathSq
	paths     [16]PathRec
	freepaths uint16 = 0xFFFF

	floodList0, floodList1 [64]pathPt

	// stdat as 64 by 64 squares, flat and with room either side, as the search reaches a
	// square past the map's edge
	stiles [68 * 64]StaticTile
)

var pathingOffset = [4]struct{ x, y int8 }{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}

var pathTurns = [3][3]byte{{0xFF, 3, 0xFF}, {2, 0xFF, 0}, {0xFF, 1, 0xFF}}

var slopeForDir = [4]byte{TILE_SLOPE_N, TILE_SLOPE_E, TILE_SLOPE_S, TILE_SLOPE_W}

func stile(x, y int) *StaticTile { return &stiles[(x+2)*64+y] }

// pathTurn is path_turns[dx + 1][dy + 1]; stale squares past a path's end can give other
// differences, read as 0.
func pathTurn(dx, dy int) byte {
	if dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		return 0
	}
	return pathTurns[dx+1][dy+1]
}

var nullMapTile Tile

// tileAt is Map_GetAddr with a blank tile for a square off the map.
func mapTileAt(x, y int) *Tile {
	if t := MapGetAddr(x, y); t != nil {
		return t
	}
	return &nullMapTile
}

// setHtx sets the current critter's heading (0-7) and clears its fine heading.
func setHtx(h byte) {
	meptr.Heading = h << 5
	meptr.SetHeading(int(h))
	meptr.SetFineHead(0)
}

// buildCorpse leaves a dead critter's remains: fluids (item 0xD8 + fluids) where it lay,
// and, 7 times in 16, its remains (item 0xC0 + corpse) near it.
func buildCorpse(obj *Object, fluids, corpse int) {
	home := mapTileAt(obj.HomeX(), obj.HomeY())
	if fluids != 0 {
		if remains := CreateObj(0xD8+int(byte(fluids)), false); remains != nil {
			remains.SetFineX(obj.FineX())
			remains.SetFineY(obj.FineY())
			remains.SetZ(obj.Z())
			remains.SetQuality(0x28)
			ObjAdd(&home.Objects, remains)
			objDeal(remains, XP, YP, 1)
		}
	}
	if corpse != 0 {
		if rand()%16 >= 7 {
			return
		}
		if remains := CreateObj(0xC0+int(byte(corpse)), false); remains != nil {
			remains.SetOwner(obj.InMajor())
			nearMobPutAt(obj, remains, 4, false)
		}
	}
}

// moveMeJoe is one step for a mobile object that is not a critter. A spent object of
// quality class below 3 is deleted (returning false), or kept with 1 hit point.
func moveMeJoe() bool {
	if meptr.HP == 0 && ComObjData[meptr.Item()].QualClass() < 3 {
		if ObjPunt(&mapTileAt(meptr.HomeX(), meptr.HomeY()).Objects, meptr, false) != nil {
			meptr.HP = 1
		} else {
			return false
		}
	}
	if ComObjData[meptr.Item()].NoHit() != 0 {
		CT3.ignore = 0x1000
	} else {
		CT3.ignore = 0
	}
	getPhysData(meptr, &CN3)
	doCritPhys(&CN3, &CT3)
	XP = meptr.HomeX()
	YP = meptr.HomeY()
	result := setPhysData(meptr, &CN3)
	if result {
		meptr.SetBin(meptr.Bin() + meptr.Rate())
	}
	return result
}

// initAi sets up the critter physics records and handlers: CN1/CT1 walkers, CN2/CT2
// fliers, CN3/CT3 other mobile objects, CN4/CT4 swimmers.
func initAi() {
	CN1.Acc[0], CN1.Acc[1], CN1.Flags = 0, 0, 0x80
	CN2.Acc[0], CN2.Acc[1], CN2.Flags = 0, 0, 0x80
	CN3.Acc[0], CN3.Acc[1], CN3.Flags = 0, 0, 0
	CN4.Acc[0], CN4.Acc[1], CN4.Flags = 0, 0, 0x80
	CT1 = PhysHandler{mask: 0x1F30, noclimb: 0x1010, w6: 0x20, ignore: 0, special: critHndlrWalk}
	CT2 = PhysHandler{mask: 0x700, noclimb: 0x80, w6: 0, ignore: 0x1000, special: critHndlrFly}
	CT3 = PhysHandler{mask: 0, noclimb: 0, w6: 0, ignore: 0, special: critHndlrObj}
	CT4 = PhysHandler{mask: 0x1728, noclimb: 0x10A8, w6: 0, ignore: 0x10, special: critHndlrSwim}
}

// getTerrain is the collision bits for obj where it stands.
func getTerrain(obj *Object) int {
	var calc MotionCalc
	curP = &calc
	curP.index = int16(ObjMemTPtr(obj))
	curP.radius = byte(ComObjData[obj.Item()].Radius())
	curP.height = byte(ComObjData[obj.Item()].Height())
	curP.x = int16(obj.HomeX()<<3 + obj.FineX())
	curP.y = int16(obj.HomeY()<<3 + obj.FineY())
	curP.z = int16(obj.Z())
	TerrainCheck(8)
	return int(uint16(curP.hits0 | curP.hits1))
}

// critHndlrWalk is the walking critter's collision handler.
func critHndlrWalk(state *uint16) bool {
	motion := *state
	if motion&0x1000 != 0 {
		if CN1.Acc[2] == 0 {
			CN1.Acc[2] = -4
		}
		meptr.SetRate(1)
		failed = 1
		control = 0
		return false
	}
	if motion&0x10 != 0 {
		if motion&0xF8 == 0x10 {
			failed = 1
			control = 0
			putEffect(meptr, 6, 3, 0, 0, int(CN1.X>>8), int(CN1.Y>>8))
			meptr.SetSeq(SEQ_DYING)
			meptr.SetFrame(3)
			meptr.SetRate(1)
			return true
		}
		if meptr.B157() == 0 {
			failed = 1
			CN1.Vel[0], CN1.Vel[1] = 0, 0
			return true
		}
	}
	if motion&0x800 != 0 && critTerr&0x800 == 0 {
		if meptr.B157() != 0 {
			return false
		}
		failed = 1
		CN1.Vel[0], CN1.Vel[1] = 0, 0
		return true
	}
	if motion&0x20 != 0 && critTerr&0x20 == 0 {
		if meptr.B157() != 0 {
			return false
		}
		failed = 1
		CN1.Vel[0], CN1.Vel[1] = 0, 0
		return true
	}
	if motion&0x300 != 0 {
		failed = 1
		return false
	}
	if motion&0x400 != 0 {
		if door := IsaDoor(&doorx, &doory); door != nil {
			collobject = door
			hitadoor = 1
			failed = 1
			didhitobj = 1
		} else {
			failed = 1
			didhitobj = 1
			collobject = CollObject()
		}
	}
	return failed != 0 && control != 0
}

// critHndlrObj is the moving object's handler; its mask is 0, so it is never called.
func critHndlrObj(state *uint16) bool { return false }

// critHndlrFly is the flying critter's handler.
func critHndlrFly(state *uint16) bool {
	control = 1
	if *state&0x200 != 0 {
		failed = 1
		return false
	}
	if *state&0x100 != 0 {
		CN2.Vel[2] = 0x80
		hitwall = 1
	}
	if *state&0x400 != 0 {
		failed = 1
		didhitobj = 1
		collobject = CollObject()
	}
	return failed != 0 && control != 0
}

// critHndlrSwim is the swimming critter's handler.
func critHndlrSwim(state *uint16) bool {
	if *state&0x300 != 0 {
		CN4.Vel[0], CN4.Vel[1] = 0, 0
		failed = 1
		return false
	}
	if *state&0x400 != 0 {
		CN4.Vel[0], CN4.Vel[1] = 0, 0
		failed = 1
		didhitobj = 1
		collobject = CollObject()
	}
	if *state&8 != 0 {
		CN4.Vel[0], CN4.Vel[1] = 0, 0
		failed = 1
	}
	return failed != 0 && control != 0
}

// doCritPhys runs pn under tp for the critter's rate in time units.
func doCritPhys(pn *Phys, tp *PhysHandler) bool {
	pn.Time = int16(meptr.Rate() << 4)
	doPhysics(pn, tp)
	return true
}

func maxb(a, b byte) byte {
	if a > b {
		return a
	}
	return b
}

func isSlope(t byte) bool { return t >= TILE_SLOPE_N && t <= TILE_SLOPE_W }

// hypMove says whether a critter can move from square (x2, y2) to (x3, y3), having come
// from (x1, y1); x1 == 0 means (x2, y2) is the start, x3 == 0 that it is the
// destination. height is the floor height on entering (x2, y2); *out gets the height
// after the move and *dist collects danger, which must stay within curDanger. Sets jump
// when the move needs one.
func hypMove(x1, y1, x2, y2, x3, y3 byte, flags, costflags int, height byte, out, dist *byte) bool {
	blocked, onobj, tested := false, false, false
	jump = 0
	tile2 := mapTileAt(int(x2), int(y2))
	tile1 := mapTileAt(int(x1), int(y1))
	tile3 := mapTileAt(int(x3), int(y3))
	type2 := byte(tile2.Type())
	type3 := byte(tile3.Type())
	terr2 := uint(TxmTerr[tile2.Floor()] >> 4)
	terr3 := uint(TxmTerr[tile3.Floor()] >> 4)
	tw2, tw3 := tileWalls[type2], tileWalls[type3]
	if x1 == 0 {
		*out = height
		h23 := byte(tile3.Height())
		if x3 > x2 && tw3&TW_WEST != 0 || x3 < x2 && tw3&TW_EAST != 0 ||
			y3 > y2 && tw3&TW_SOUTH != 0 || y3 < y2 && tw3&TW_NORTH != 0 ||
			x3 > x2 && tw2&TW_EAST != 0 || x3 < x2 && tw2&TW_WEST != 0 ||
			y3 > y2 && tw2&TW_NORTH != 0 || y3 < y2 && tw2&TW_SOUTH != 0 {
			return false
		}
		if flags&0x1000 == 0 {
			return true
		}
		if isSlope(type3) && slopeForDir[pathTurn(int(x3)-int(x2), int(y3)-int(y2))&3] != type3 {
			h23++
		}
		return int(h23) <= int(height)+1
	} else if x3 == 0 {
		*out = height
		if flags&0x1000 == 0 {
			return true
		}
		objh := byte(0)
		for link := &tile2.Objects; link.Index() != 0 && objh == 0; {
			obj := ObjPtrTMem(link)
			com := &ComObjData[obj.Item()]
			if com.Solid() != 0 {
				objh = byte((obj.Z() + com.Height()) >> 3)
			}
			link = &obj.QN
		}
		h23 := byte(tile2.Height())
		if objh > h23 {
			h23 = objh
			onobj = true
		}
		if tile1.Height() > int(h23)+1 {
			*out = h23
			*dist += height - *out - 1
			if *dist > curDanger {
				return false
			}
		}
		if !onobj {
			if flags&(8<<terr2) != 0 {
				return false
			}
			if costflags&(8<<terr2) != 0 {
				*dist += 2
				if *dist > curDanger {
					return false
				}
			}
		}
		return true
	}
	*out = height
	switch {
	case x3 > x2:
		if tw3&TW_WEST != 0 || tw2&TW_EAST != 0 {
			return false
		}
	case x3 < x2:
		if tw3&TW_EAST != 0 || tw2&TW_WEST != 0 {
			return false
		}
	case y3 > y2:
		if tw3&TW_SOUTH != 0 || tw2&TW_NORTH != 0 {
			return false
		}
	case y3 < y2:
		if tw3&TW_NORTH != 0 || tw2&TW_SOUTH != 0 {
			return false
		}
	}
	objh := byte(0)
	dir := 0
	for link := &tile2.Objects; link.Index() != 0 && objh == 0; {
		obj := ObjPtrTMem(link)
		com := &ComObjData[obj.Item()]
		if obj.Major() == MAJOR_RECT && obj.Minor() == MINOR_DOOR && obj.InClass() < 8 {
			dhead := obj.HeadingBits() & 3
			dxp := obj.FineX()
			dyp := obj.FineY()
			if !tested {
				switch {
				case x1 < x2:
					if y2 < y3 {
						dir = 0
					} else if y2 > y3 {
						dir = 2
					} else {
						dir = 1
					}
				case x1 > x2:
					if y2 < y3 {
						dir = 3
					} else if y2 > y3 {
						dir = 5
					} else {
						dir = 4
					}
				case y1 < y2:
					if x2 < x3 {
						dir = 8
					} else if x2 > x3 {
						dir = 6
					} else {
						dir = 7
					}
				default:
					if x2 < x3 {
						dir = 11
					} else if x2 > x3 {
						dir = 9
					} else {
						dir = 10
					}
				}
				tested = true
			}
			switch dir {
			case 7, 10:
				if dhead != 2 {
					return false
				}
			case 1, 4:
				if dhead != 0 {
					return false
				}
			case 0, 9:
				switch dhead {
				case 0:
					if dyp > 3 {
						return false
					}
				case 1:
					return false
				case 2:
					if dxp < 4 {
						return false
					}
				}
			case 2, 6:
				switch dhead {
				case 0:
					if dyp < 4 {
						return false
					}
				case 2:
					if dxp < 4 {
						return false
					}
				case 3:
					return false
				}
			case 3, 11:
				switch dhead {
				case 0:
					if dyp > 3 {
						return false
					}
				case 2:
					if dxp > 3 {
						return false
					}
				case 3:
					return false
				}
			case 5, 8:
				switch dhead {
				case 0:
					if dyp < 4 {
						return false
					}
				case 1:
					return false
				case 2:
					if dxp > 3 {
						return false
					}
				}
			}
		} else if com.Solid() != 0 {
			objh = byte((obj.Z() + com.Height()) >> 3)
		}
		link = &obj.QN
	}
	if flags&0x1000 == 0 {
		*out = 0x10 - (myheight+3)>>2
		return true
	}
	h12 := maxb(byte(tile2.Height()), byte(tile1.Height()))
	h23 := maxb(byte(tile2.Height()), byte(tile3.Height()))
	h23b := h23
	if height > h12 {
		h12 = height
	}
	if isSlope(type3) && slopeForDir[pathTurn(int(x3)-int(x2), int(y3)-int(y2))&3] != type3 {
		h23++
	}
	if int(maxb(h12, h23))<<3+int(myheight) > 0x7F {
		return false
	}
	if int(h12) > int(h23)+1 {
		if int(h12) > int(objh)+1 {
			blocked = true
			*out = maxb(h23, objh)
			*dist += h12 - *out - 1
			if *dist > curDanger {
				return false
			}
		} else {
			h23 = objh
			h23b = objh
			onobj = true
		}
	} else if objh != 0 && h12 >= objh && int(h12) <= int(objh)+1 {
		onobj = true
	}
	if int(h23) > int(h12)+1 {
		return false
	}
	if int(h12) > tile2.Height()+1 && !blocked && !onobj {
		if flags&(8<<terr2) != 0 {
			return false
		}
		if costflags&(8<<terr3) != 0 {
			*dist += 2
			if *dist >= curDanger {
				return false
			}
		}
		if int(h12) > int(objh)+1 {
			*out = h12
			if h12 >= h23 && mycst.B0A&0x20 != 0 {
				*dist++
				if *dist >= curDanger {
					return false
				}
				jump = 1
				return true
			}
			return false
		}
		*out = h23b
		return true
	} else if flags&(8<<terr2) != 0 && !onobj {
		if flags&(8<<terr3) != 0 {
			return false
		}
		if costflags&(8<<terr3) != 0 {
			*dist += 2
			if *dist > curDanger {
				return false
			}
		}
		if mycst.B0A&0x20 != 0 {
			*dist++
			if *dist >= curDanger {
				return false
			}
			jump = 1
			return true
		}
		return false
	}
	*out = h23b
	return true
}

// hypSee is lineOfSight's step test: can a sight line at height (fine units) pass from
// square (x1, y1) into (x2, y2)?
func hypSee(oldx, oldy, x1, y1, x2, y2, height byte) bool {
	tile1 := mapTileAt(int(x1), int(y1))
	tile2 := mapTileAt(int(x2), int(y2))
	tw1, tw2 := tileWalls[tile1.Type()], tileWalls[tile2.Type()]
	if !(oldx == 0 || oldx == x1 && oldy == y1) && x2 == 0 {
		return true
	}
	if x2 > x1 && tw2&TW_WEST != 0 || x2 < x1 && tw2&TW_EAST != 0 ||
		y2 > y1 && tw2&TW_SOUTH != 0 || y2 < y1 && tw2&TW_NORTH != 0 ||
		x2 > x1 && tw1&TW_EAST != 0 || x2 < x1 && tw1&TW_WEST != 0 ||
		y2 > y1 && tw1&TW_NORTH != 0 || y2 < y1 && tw1&TW_SOUTH != 0 {
		return false
	}
	return int(height)>>3 >= tile2.Height()
}

// floodPath searches for a walkable path from (x, y) at floor height height0 to (destx,
// desty), with rng as the danger allowance. On success the path is in pathsq and pathlen.
func floodPath(x, y, height0, destx, desty, destz, rng int) bool {
	cx, cy := int(int8(x)), int(int8(y))
	dx, dy, dz := int(int8(destx)), int(int8(desty)), int(int8(destz))
	curDanger = byte(rng)
	ncur, nnext := 0, 0
	cur, next := &floodList0, &floodList1
	stiles = [68 * 64]StaticTile{}
	lo := func(a, b int) int {
		if a < b {
			a = a - 5
		} else {
			a = b - 5
		}
		if a > 1 {
			return a
		}
		return 1
	}
	hi := func(a, b int) int {
		if a > b {
			a = a + 5
		} else {
			a = b + 5
		}
		if a < MAP_SIZE {
			return a
		}
		return MAP_SIZE
	}
	minx, miny := lo(cx, dx), lo(cy, dy)
	maxx, maxy := hi(cx, dx), hi(cy, dy)
	s := stile(cx, cy)
	s.pathx = byte(cx)
	s.height = byte(height0)
	s.step = 0
	for d := 0; d < 4; d++ {
		nx := int(int8(cx + int(pathingOffset[d].x)))
		ny := int(int8(cy + int(pathingOffset[d].y)))
		np := stile(nx, ny)
		dist := byte(0)
		if hypMove(0, 0, byte(cx), byte(cy), byte(nx), byte(ny), int(tpAct.noclimb), int(tpAct.w6), byte(height0), &np.height, &dist) {
			if nx == dx && ny == dy {
				pathlen = 1
				pathsq[0] = PathSq{x: byte(cx), y: byte(cy), unused: pathsq[0].unused}
				pathsq[1].x = byte(nx)
				pathsq[1].y = byte(ny)
				return true
			}
			np.pathx = byte(cx)
			np.pathy = byte(cy)
			np.step = 1
			np.setDist(dist)
			cur[ncur] = pathPt{byte(nx), byte(ny)}
			ncur++
		}
	}
	for step := 1; step < 0x20 && ncur > 0; step++ {
		nnext = 0
		for i := 0; i < ncur && nnext < 0x40; i++ {
			tx := int(int8(cur[i].x))
			ty := int(int8(cur[i].y))
			rec := stile(tx, ty)
			for d := 0; d < 4; d++ {
				nx := int(int8(tx + int(pathingOffset[d].x)))
				ny := int(int8(ty + int(pathingOffset[d].y)))
				if nx < minx || nx > maxx || ny < miny || ny > maxy {
					continue
				}
				np := stile(nx, ny)
				dist := rec.dist()
				if int(rec.pathx) == nx&0xFF && int(rec.pathy) == ny&0xFF {
					continue
				}
				var height byte
				ok := hypMove(rec.pathx, rec.pathy, byte(tx), byte(ty), byte(nx), byte(ny), int(tpAct.noclimb), int(tpAct.w6), rec.height, &height, &dist)
				isnew := np.pathx == 0
				if !ok {
					continue
				}
				if isnew || (rec.step < np.step && iabs(int(height)-dz) < iabs(int(np.height)-dz)) {
					np.pathx = byte(tx)
					np.pathy = byte(ty)
					np.height = height
					np.step = byte(step + 1)
					np.setDist(dist)
					rec.setPathflag(jump)
					if isnew && nnext < 64 {
						next[nnext] = pathPt{byte(nx), byte(ny)}
						nnext++
					}
					if nx == dx && ny == dy &&
						hypMove(byte(tx), byte(ty), byte(nx), byte(ny), 0, 0, int(tpAct.noclimb), int(tpAct.w6), np.height, &np.height, &dist) {
						makePathFromFloodData(step, byte(dx), byte(dy))
						return true
					}
				}
			}
		}
		cur, next = next, cur
		ncur = nnext
	}
	return false
}

// makePathFromFloodData reads the path back from the flood records, from the
// destination (x, y) to the start, into pathsq.
func makePathFromFloodData(length int, x, y byte) {
	pathlen = length + 1
	pathsq[length+1].x = x
	pathsq[length+1].y = y
	pathsq[0].flag = 0
	for i := length + 1; i > 0; i-- {
		t := stile(int(pathsq[i].x), int(pathsq[i].y))
		pathsq[i-1].x = t.pathx
		pathsq[i-1].y = t.pathy
		pathsq[i].flag = t.pathflag()
	}
}

// beeline says whether the straight line of tiles from (x1, y1) to (x2, y2) is walkable
// with no jumps: 1 if so, 0 if not, -1 for the same tile.
func beeline(x1, y1, x2, y2 int) int {
	acc := byte(0x40)
	dx := int8(x2 - x1)
	dy := int8(y2 - y1)
	x, y := byte(x1), byte(y1)
	tile := mapTileAt(x1, y1)
	curDanger = 0
	if dx == 0 && dy == 0 {
		return -1
	}
	var major, minor *byte
	var step, side, slope int8
	sign := func(v int8) int8 {
		if v > 0 {
			return 1
		}
		return -1
	}
	if dx >= dy {
		if int(dx) >= -int(dy) {
			major, minor = &x, &y
			slope = int8((int32(dy) << 7) / int32(dx))
			step, side = 1, sign(dy)
		} else {
			major, minor = &y, &x
			slope = int8((int32(dx) << 7) / int32(dy))
			step, side = -1, sign(dx)
		}
	} else {
		if int(dx) >= -int(dy) {
			major, minor = &y, &x
			slope = int8((int32(dx) << 7) / int32(dy))
			step, side = 1, sign(dx)
		} else {
			major, minor = &x, &y
			slope = int8((int32(dy) << 7) / int32(dx))
			step, side = -1, sign(dy)
		}
	}
	pathsq[0].x = byte(x1)
	pathsq[0].y = byte(y1)
	pathlen = 1
	pathsq[0].unused = byte(tile.Height())
	for {
		*major += byte(step)
		if !addToBeelinePath(x, y) {
			return 0
		}
		acc += byte(slope)
		if acc&0x80 != 0 {
			acc &= 0x7F
			*minor += byte(side)
			if !addToBeelinePath(x, y) {
				return 0
			}
		}
		if int(x) == x2 && int(y) == y2 {
			break
		}
	}
	var unused byte
	a, b := &pathsq[pathlen-2], &pathsq[pathlen-1]
	return b2int(hypMove(a.x, a.y, b.x, b.y, 0, 0, int(tpAct.noclimb), int(tpAct.w6), a.unused, &a.unused, &unused))
}

// lineOfSight says whether a point at (x1, y1, z1) can see (x2, y2, z2); positions in
// fine units, heights in object units. A line longer than 10 tiles is never clear.
func lineOfSight(x1, y1, z1, x2, y2, z2 int) bool {
	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	x := byte(x1 >> 3)
	y := byte(y1 >> 3)
	oldx, lastx := x, x
	oldy, lasty := y, y
	destx := byte(x2 >> 3)
	desty := byte(y2 >> 3)
	if dx == 0 && dy == 0 {
		return true
	}
	var major, minor *byte
	var diff, step, dir, slope, frac byte
	dirOf := func(v int) byte {
		if v > 0 {
			return 1
		}
		return 0xFF
	}
	if dx >= dy {
		if -dy <= dx {
			major, minor = &x, &y
			diff = byte(dx >> 3)
			step = 1
			dir = dirOf(dy)
			if dir == 1 {
				slope = byte((int32(dy) << 7) / int32(dx))
				frac = byte((y1&7)<<4 + int(slope)*(7-x1&7)/8)
			} else {
				slope = byte((int32(-dy) << 7) / int32(dx))
				frac = byte((7-y1&7)<<4 + int(slope)*(7-x1&7)/8)
			}
		} else {
			major, minor = &y, &x
			diff = byte(-dy >> 3)
			step = 0xFF
			dir = dirOf(dx)
			if dir == 1 {
				slope = byte((int32(dx) << 7) / int32(-dy))
				frac = byte((7-x1&7)<<4 + int(slope)*(y1&7)/8)
			} else {
				slope = byte((int32(-dx) << 7) / int32(-dy))
				frac = byte((x1&7)<<4 + int(slope)*(y1&7)/8)
			}
		}
	} else {
		if -dy <= dx {
			major, minor = &y, &x
			diff = byte(dy >> 3)
			step = 1
			dir = dirOf(dx)
			if dir == 1 {
				slope = byte((int32(dx) << 7) / int32(dy))
				frac = byte((x1&7)<<4 + int(slope)*(7-y1&7)/8)
			} else {
				slope = byte((int32(-dx) << 7) / int32(dy))
				frac = byte((7-x1&7)<<4 + int(slope)*(7-y1&7)/8)
			}
		} else {
			major, minor = &x, &y
			diff = byte(-dx >> 3)
			step = 0xFF
			dir = dirOf(dy)
			if dir == 1 {
				slope = byte((int32(dy) << 7) / int32(-dx))
				frac = byte((7-y1&7)<<4 + int(slope)*(x1&7)/8)
			} else {
				slope = byte((int32(-dy) << 7) / int32(-dx))
				frac = byte((y1&7)<<4 + int(slope)*(x1&7)/8)
			}
		}
	}
	z := byte(z1)
	count := 0
	for {
		if frac&0x80 != 0 {
			frac &= 0x7F
			*minor += dir
			if !hypSee(oldx, oldy, lastx, lasty, x, y, z) {
				return false
			}
			if x == destx && y == desty {
				return hypSee(lastx, lasty, x, y, 0, 0, z)
			}
			oldx, oldy = lastx, lasty
			lastx, lasty = x, y
		}
		*major += step
		count++
		if count > 10 {
			return false
		}
		if diff != 0 {
			z = byte(z1 + count*dz/int(diff))
		}
		if !hypSee(oldx, oldy, lastx, lasty, x, y, z) {
			return false
		}
		if x == destx && y == desty {
			return hypSee(lastx, lasty, x, y, 0, 0, z)
		}
		oldx, oldy = lastx, lasty
		lastx, lasty = x, y
		frac += slope
	}
}

// addToBeelinePath is beeline's step: append (x, y) to pathsq and test the move into it.
func addToBeelinePath(x, y byte) bool {
	var unused byte
	curDanger = 0
	pathsq[pathlen].x = x
	pathsq[pathlen].y = y
	pathlen++
	if pathlen > 0x3F {
		return false
	}
	var ok bool
	if pathlen == 2 {
		ok = hypMove(0, 0, pathsq[0].x, pathsq[0].y, pathsq[1].x, pathsq[1].y,
			int(tpAct.noclimb), int(tpAct.w6), pathsq[0].unused, &pathsq[1].unused, &unused)
	} else {
		a, b, c := &pathsq[pathlen-3], &pathsq[pathlen-2], &pathsq[pathlen-1]
		ok = hypMove(a.x, a.y, b.x, b.y, c.x, c.y,
			int(tpAct.noclimb), int(tpAct.w6), a.unused, &b.unused, &unused)
	}
	return ok && jump == 0
}

// findFreePath finds the first free path slot.
func findFreePath(found *int) bool {
	for i := 0; i < 16; i++ {
		if freepaths&(1<<i) != 0 {
			*found = i
			return true
		}
	}
	return false
}

// clearPaths frees all 16 stored paths and clears every critter's has-a-path bit.
func clearPaths() {
	for i := 2; i < NUM_MOBILE; i++ {
		ObjIntTMem(i).SetB157(0)
	}
	freepaths = 0xFFFF
}

// storePath packs pathsq into a stored path.
func storePath(path *PathRec) {
	path.setCount(0)
	path.x = pathsq[0].x
	path.y = pathsq[0].y
	path.index = byte(pathlen)
	for i := 0; i < pathlen; i += 4 {
		direction := byte(0)
		for j := 0; j < 4; j++ {
			a, b := &pathsq[i+j], &pathsq[i+j+1]
			direction += (pathTurn(int(b.x)-int(a.x), int(b.y)-int(a.y)) & 3) << (j * 2)
		}
		path.directions[i/4] = direction
	}
	for i := 0; i < pathlen; i += 8 {
		slope := byte(0)
		for j := 0; j < 8 && i+j+1 < len(pathsq); j++ {
			slope += (pathsq[i+j+1].flag & 1) << j
		}
		path.slopes[i/8] = slope
	}
}

// setNextSquareOnPath advances a stored path to its next square; false at its end.
func setNextSquareOnPath(path *PathRec) bool {
	c := path.count()
	if c >= int(path.index) {
		return false
	}
	direction := (path.directions[c/4] >> ((c % 4) << 1)) & 3
	path.x += byte(pathingOffset[direction].x)
	path.y += byte(pathingOffset[direction].y)
	path.setSlope(int(path.slopes[c/8]>>(c%8)) & 1)
	path.setCount(c + 1)
	return true
}

// closeToSquare says whether a critter on tile (xhome, yhome) at fine position (xpos,
// ypos) is at the path square (pathx, pathy).
func closeToSquare(flag, xhome, yhome, xpos, ypos, pathx, pathy int) bool {
	if flag == 0 {
		if xpos >= 6 && pathx > xhome {
			xhome++
		} else if xpos <= 1 && pathx < xhome {
			xhome--
		}
		if ypos >= 6 && pathy > yhome {
			yhome++
		} else if ypos <= 1 && pathy < yhome {
			yhome--
		}
	}
	return xhome == pathx && yhome == pathy
}

// moveAlongPath follows a stored path; false when it is done.
func moveAlongPath(path *PathRec) bool {
	tilex, tiley := int(path.x), int(path.y)
	if closeToSquare(path.slope(), myxpos, myypos, int(myxpost&7), int(myypost&7), tilex, tiley) {
		if !setNextSquareOnPath(path) {
			return false
		}
	} else {
		tilex, tiley = myxpos, myypos
	}
	if path.slope() != 0 {
		doThatJumpKindaThing(path)
	} else {
		if mycst.Flier() != 0 {
			adjustHeight(meptr.DestX(), meptr.DestY())
		}
		x := int(path.x) << 3
		if int(path.x) == tilex {
			x += 4
		} else if int(path.x) < tilex {
			x += 7
		}
		y := int(path.y) << 3
		if int(path.y) == tiley {
			y += 4
		} else if int(path.y) < tiley {
			y += 7
		}
		setHtx(deltatotheta(int8(int8(x)-int8(myxpost)), int8(int8(y)-int8(myypost))))
	}
	return true
}

// doThatJumpKindaThing is a jump step on a path: walk to the edge of the square, then
// leap towards the next.
func doThatJumpKindaThing(path *PathRec) {
	x := int(path.x)<<3 - 2
	if int(path.x) == myxpos {
		x += 6
	} else if int(path.x) < myxpos {
		x += 11
	}
	y := int(path.y)<<3 - 2
	if int(path.y) == myypos {
		y += 6
	} else if int(path.y) < myypos {
		y += 11
	}
	if iabs(x-int(myxpost))+iabs(y-int(myypost)) < 3 {
		c := path.count()
		direction := (path.directions[c/4] >> ((c % 4) << 1)) & 3
		x = int(path.x) + int(pathingOffset[direction].x)
		y = int(path.y) + int(pathingOffset[direction].y)
		heading := deltatotheta(int8(int8(x)-int8(meptr.HomeX())), int8(int8(y)-int8(meptr.HomeY())))
		didmove = 1
		setHtx(heading)
		meptr.SetRate(1)
		meptr.SetPitch(0x16)
		meptr.SetSpeed(11)
	} else {
		setHtx(deltatotheta(int8(int8(x)-int8(myxpost)), int8(int8(y)-int8(myypost))))
	}
}

// deltatotheta is the nearest of the 8 headings (0 is +y, clockwise towards +x) for the
// vector (x, y).
func deltatotheta(x, y int8) byte {
	dx := x << 1
	dy := y << 1
	if y > dx {
		if int(x) > -int(dy) {
			if int(y) > -int(dx) {
				return 0
			}
			return 7
		}
		if x > dy {
			return 5
		}
		return 6
	}
	if int(x) > -int(dy) {
		if x > dy {
			return 2
		}
		return 1
	}
	if int(y) > -int(dx) {
		return 3
	}
	return 4
}

// setLoc sets the current critter's destination square and height.
func setLoc(x, y, z int) {
	x, y, z = x&0xFF, y&0xFF, z&0xFF
	if meptr.DestX() != x || meptr.DestY() != y || meptr.TargetZ() != z {
		meptr.SetDestX(x)
		meptr.SetDestY(y)
		meptr.SetTargetZ(z)
		meptr.SetB185(1)
		meptr.SetB186(0)
	}
}

func freeMyPath() {
	freepaths |= 1 << meptr.Path()
	meptr.SetB157(0)
}

// critHeadForLoc moves the current critter towards square (x, y) at height z.
func critHeadForLoc(x, y, z int) {
	blocked, opening := false, false
	setLoc(x, y, z)
	if meptr.B185() != 0 && meptr.B157() != 0 {
		freeMyPath()
	}
	dx := int8(x - myxpos)
	dy := int8(y - myypos)
	if dx == 0 && dy == 0 {
		if meptr.B157() != 0 {
			freeMyPath()
		}
		if meptr.Goal() == GOAL_GO_HOME {
			critterSetGoal(GOAL_MILL, 0)
		} else if control != 0 {
			meptr.SetSpeed(0)
			meptr.SetB156(1)
			meptr.SetSeq(SEQ_STAND)
			return
		}
	}
	if control == 0 {
		meptr.SetRate(1)
		if meptr.B157() != 0 {
			p := &paths[meptr.Path()]
			if int(p.x) == meptr.HomeX() && int(p.y) == meptr.HomeY() {
				setNextSquareOnPath(p)
			}
		}
		return
	}
	if failed != 0 && aligned == 0 && meptr.B186() == 0 {
		if didhitobj != 0 {
			co := collobject
			if co == nil {
				co = &Level.Objs[0] // FARNULLTRAP
			}
			item := co.Item()
			if hitadoor != 0 {
				meptr.SetSeq(SEQ_STAND)
				if rand()%4 == 0 || meptr.Attitude() != 0 {
					meptr.SetB186(1)
				} else {
					tryToOpenDoor(co)
				}
			} else if item&ID_MAJOR>>6 == MAJOR_CREATURE && item != ITEM_ADVENTURER &&
				meptr.Goal() == GOAL_ATTACK && co.Goal() == GOAL_ATTACK {
			} else if item>>4 == CLASS_DOOR && item&ID_INCLASS >= 8 && mycst.Flier() != 0 {
				meptr.SetPitch(0xE)
				failed = 0
				dontchangedz = 1
				opening = true
			} else {
				meptr.SetB186(1)
			}
		}
		if failed != 0 {
			if meptr.B157() != 0 {
				freeMyPath()
			}
			meptr.SetB187(0)
			blocked = true
		}
	}
	slot := 0
	switch {
	case meptr.B157() != 0:
		if !moveAlongPath(&paths[meptr.Path()]) {
			freeMyPath()
		}
	case meptr.B185() == 0 && meptr.B187() != 0:
		setHtx(deltatotheta(dx, dy))
		if mycst.Flier() != 0 {
			adjustHeight(x, y)
		}
	case meptr.B185() == 0 && meptr.B186() != 0:
		if rand()%8 == 0 {
			meptr.SetB186(0)
		}
		critDrunkwalk()
		return
	case !blocked && beeline(myxpos, myypos, x, y) == 1:
		meptr.SetB187(1)
		setHtx(deltatotheta(dx, dy))
		meptr.SetB186(0)
		if meptr.B157() != 0 {
			freeMyPath()
		}
	case findFreePath(&slot) && floodPath(myxpos, myypos, meptr.Z()>>3, x, y, z, acceptableDanger()):
		freepaths &^= 1 << slot
		storePath(&paths[slot])
		meptr.SetB186(0)
		meptr.SetB157(1)
		meptr.SetPath(slot)
		moveAlongPath(&paths[meptr.Path()])
	default:
		meptr.SetB186(1)
		meptr.SetB187(0)
		critDrunkwalk()
		return
	}
	if didmove == 0 {
		meptr.SetB156(0)
		meptr.SetSeq(SEQ_WALK)
		if opening {
			meptr.SetSpeed(0)
		} else if meptr.Goal() == GOAL_ATTACK {
			meptr.SetSpeed(int(mycst.Run))
		} else {
			meptr.SetSpeed(int(mycst.Speed))
		}
		meptr.SetFrame((meptr.Frame() + 1) % 4)
		meptr.SetRate(4)
	}
}

// adjustHeight sets a flier's pitch on the way to (x, y): about 20 height units above
// the destination's floor, never above 0x78, with a little random bobbing.
func adjustHeight(x, y int) {
	if dontchangedz != 0 {
		return
	}
	t := mapTileAt(x, y)
	z := meptr.Z()
	dh := t.Height()<<3 + 0x14
	if dh > 0x78 {
		dh = 0x78
	}
	var pitch int
	if hitwall != 0 && z < 0x78 || z < dh-8 {
		pitch = 0x12
	} else if z > 0x78 || z > dh+8 {
		pitch = 0xE
	} else {
		pitch = rand()%3 + 0xF
	}
	meptr.B14 = byte(meptr.Rate() | (pitch&0x1F)<<3)
}

// tryToOpenDoor: a critter bumped into a door. One with a locks value uses it and tries
// the lock half the time; otherwise one time in four it bashes the door.
func tryToOpenDoor(door *Object) {
	if door.ID&7 == 7 {
		return
	}
	if mycst.Locks != 0 {
		MapObjX = doorx
		MapObjY = doory
		UseObj(meptr, door, 0)
	}
	if door.Class() != CLASS_DOOR || door.InClass() >= 8 {
		return
	}
	if mycst.Locks != 0 && rand()%2 != 0 {
		checkLock(meptr, door, -int(mycst.Locks))
		return
	}
	if rand()%4 == 0 {
		dmg := 0
		if d := int(mycst.Attacks[0].Damage); d != 0 {
			dmg = rand() % d
		}
		damageItem(door, meptr, doorx, doory, dmg, DMG_PHYSICAL)
	}
}
