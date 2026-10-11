package uw

// The minimap: the player's map (PlayersMap, as the map screen draws it) around the
// player, north up, in the top left corner of the game screen's HUD. Each tile is three
// pixels, shaped as the map screen shapes it (tilePattern): the floor seen, its water and
// lava, doors and bridges; the player is a dot with a tick the way they face. What is not
// mapped lies in fog, but for the solid squares beside the floor seen, which are walls
// (the map has no mark for a wall seen): where the fog meets the floor, there is more to
// explore.

const (
	miniTiles  = 21 // across
	miniRows   = 15 // up
	miniCell   = 3
	miniW      = miniTiles*miniCell + 2 // with the border
	miniH      = miniRows*miniCell + 2
	miniMargin = 2
)

// The minimap's colours in the game palette, picked once: the border, unseen and wall,
// the floor, its wall edge, water, lava, a door, a bridge, the player and their tick, and
// the fog's two shades.
const (
	mmBorder = iota
	mmDark
	mmFloor
	mmEdge
	mmWater
	mmLava
	mmDoor
	mmBridge
	mmPlayer
	mmTick
	mmFog
	mmFog2
	mmCount
)

var (
	miniCols [mmCount]byte
	miniPix  [miniW * miniH]byte
	miniKeep [miniW * miniH]bool
	miniKey  [miniTiles*miniRows + 3]int // what the minimap drawn shows
	miniOn   bool
)

func miniColours() {
	if miniCols[mmFloor] != 0 {
		return
	}
	var pal [0x300]byte
	readQuikpal(PAL_GAME, pal[:])
	for i, want := range [mmCount][3]int{
		{36, 28, 16}, {3, 3, 4}, {34, 28, 18}, {14, 10, 6}, {8, 16, 40},
		{52, 14, 2}, {44, 22, 6}, {22, 14, 8}, {63, 63, 40}, {63, 20, 8},
		{17, 17, 20}, {22, 22, 26},
	} {
		miniCols[i] = nearestColour(pal[:], want, 1)
	}
}

// miniX, miniY are the minimap's top left (y up).
func miniX() int { return leftX(miniMargin) }
func miniY() int { return 0xC7 - miniMargin }

// updateMinimap redraws the minimap when what it shows changed: the player moved a pixel's
// worth or turned, or a tile in it was mapped; none while the options panel is open.
func updateMinimap() {
	if !hudActive || inplist.mode != MODE_GAME || ThePlayer == nil || LeftPanel == 1 {
		return
	}
	px, py := int(uint16(PN.X))*miniCell>>8, int(uint16(PN.Y))*miniCell>>8
	tx, ty := px/miniCell, py/miniCell
	key := miniKey
	key[0], key[1], key[2] = px, py, int(uint16(PlayerFacing)+0x1000)>>13
	n := 3
	for r := 0; r < miniRows; r++ {
		for c := 0; c < miniTiles; c++ {
			key[n] = int(miniTile(tx-miniTiles/2+c, ty-miniRows/2+r))
			n++
		}
	}
	if miniOn && key == miniKey {
		return
	}
	miniKey, miniOn = key, true
	drawMinimap(px, py, tx, ty)
}

// hideMinimap takes the minimap off while the options panel is open (LeftPanel 1).
func hideMinimap() {
	if miniOn {
		hudClear(miniX(), miniY(), miniW, miniH)
		miniOn = false
	}
}

// miniTile is PlayersMap at x, y, 0 off the map.
func miniTile(x, y int) byte {
	if x < 0 || y < 0 || x >= MAP_SIZE || y >= MAP_SIZE {
		return 0
	}
	return PlayersMap[y][x]
}

// miniOpen is whether a map byte is a tile seen and walkable (the map screen's 1 to 9).
func miniOpen(b byte) bool { return b&0xF > 0 && b&0xF < 10 }

// miniWall is whether the unmapped square at x, y is a wall beside the floor seen: solid,
// with a mapped square among its eight neighbours.
func miniWall(x, y int) bool {
	if x < 0 || y < 0 || x >= MAP_SIZE || y >= MAP_SIZE || tileAt(y*MAP_SIZE+x).Type() != 0 {
		return false
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if miniOpen(miniTile(x+dx, y+dy)) {
				return true
			}
		}
	}
	return false
}

// miniFog is the fog's shade at a map pixel: a fixed mottle, so that it moves with the map.
func miniFog(wx, wy int) byte {
	h := uint32(wx)*0x9E3779B1 ^ uint32(wy)*0x85EBCA77
	h ^= h >> 15
	h *= 0x2C1B3C6D
	h ^= h >> 13
	if h&3 == 0 {
		return miniCols[mmFog2]
	}
	return miniCols[mmFog]
}

func drawMinimap(px, py, tx, ty int) {
	miniColours()
	for i := range miniKeep {
		miniKeep[i] = true
	}
	set := func(x, y int, c byte) { // x right, y down, within the border
		if x >= 0 && y >= 0 && x < miniW-2 && y < miniH-2 {
			miniPix[(y+1)*miniW+x+1] = c
		}
	}
	// the tile at column c, row r (r up from the bottom) and its map byte
	x0, y0 := tx-miniTiles/2, ty-miniRows/2
	wall := func(c, r int) bool { return miniWall(x0+c, y0+r) }
	for r := 0; r < miniRows; r++ {
		for c := 0; c < miniTiles; c++ {
			b := miniTile(x0+c, y0+r)
			sx, sy := c*miniCell, (miniRows-1-r)*miniCell
			if !miniOpen(b) {
				w := wall(c, r)
				for j := 0; j < 3; j++ {
					for i := 0; i < 3; i++ {
						col := miniCols[mmDark]
						if !w {
							col = miniFog((x0+c)*miniCell+i, (y0+r)*miniCell+2-j)
						}
						set(sx+i, sy+j, col)
					}
				}
				continue
			}
			typ := int(b & 0xF)
			if typ >= 6 {
				typ = 1
			}
			floor := miniCols[mmFloor]
			switch b >> 4 & 3 {
			case 1:
				floor = miniCols[mmWater]
			case 2:
				floor = miniCols[mmLava]
			}
			switch b >> 6 {
			case 3:
				floor = miniCols[mmBridge]
			}
			for j := 0; j < 3; j++ { // tilePattern's rows go up the map
				for i := 0; i < 3; i++ {
					switch tilePattern[typ-1][j][i] {
					case 0:
						set(sx+i, sy+2-j, miniCols[mmDark])
					case 1:
						set(sx+i, sy+2-j, floor)
					case 2:
						set(sx+i, sy+2-j, miniCols[mmEdge])
					}
				}
			}
			if typ == 1 { // the sides against a wall, darker; the fog's open
				if wall(c, r+1) {
					for i := 0; i < 3; i++ {
						set(sx+i, sy, miniCols[mmEdge])
					}
				}
				if wall(c, r-1) {
					for i := 0; i < 3; i++ {
						set(sx+i, sy+2, miniCols[mmEdge])
					}
				}
				if wall(c-1, r) {
					for j := 0; j < 3; j++ {
						set(sx, sy+j, miniCols[mmEdge])
					}
				}
				if wall(c+1, r) {
					for j := 0; j < 3; j++ {
						set(sx+2, sy+j, miniCols[mmEdge])
					}
				}
			}
			if b>>6 == 1 {
				set(sx+1, sy+1, miniCols[mmDoor])
			}
		}
	}

	// the player, at their place within the middle tile, and a tick the way they face
	cx := miniTiles/2*miniCell + px - tx*miniCell
	cy := (miniRows-1-miniRows/2)*miniCell + 2 - (py - ty*miniCell)
	s, co := cSinCos(int(uint16(PlayerFacing)))
	dx := (int(s)*2 + 0x4000*sign(int(s))) / 0x8000
	dy := (int(co)*2 + 0x4000*sign(int(co))) / 0x8000
	set(cx+dx, cy-dy, miniCols[mmTick])
	set(cx, cy, miniCols[mmPlayer])
	set(cx-1, cy, miniCols[mmPlayer])
	set(cx+1, cy, miniCols[mmPlayer])
	set(cx, cy-1, miniCols[mmPlayer])
	set(cx, cy+1, miniCols[mmPlayer])
	set(cx+dx, cy-dy, miniCols[mmTick])

	for x := 0; x < miniW; x++ {
		miniPix[x], miniPix[(miniH-1)*miniW+x] = miniCols[mmBorder], miniCols[mmBorder]
	}
	for y := 0; y < miniH; y++ {
		miniPix[y*miniW], miniPix[y*miniW+miniW-1] = miniCols[mmBorder], miniCols[mmBorder]
	}
	hudPut(miniX(), miniY(), miniW, miniH, miniPix[:], miniKeep[:])
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
