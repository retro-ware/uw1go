package uw

import (
	"strconv"
	"strings"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of AUTOMAP.C: the map screen: the player's map of each level (PlayersMap, one byte
// a tile: type in the low nibble, terrain in bits 4..5, kind in 6..7), its notes, and
// moving between levels' maps.

// tile_pattern: how each solid tile type is shaded, 1 with the terrain, 2 darkened.
var tilePattern = [5][3][3]byte{
	{{1, 1, 1}, {1, 1, 1}, {1, 1, 1}},
	{{2, 1, 1}, {0, 2, 1}, {0, 0, 2}},
	{{1, 1, 2}, {1, 2, 0}, {2, 0, 0}},
	{{0, 0, 2}, {0, 2, 1}, {2, 1, 1}},
	{{2, 0, 0}, {1, 2, 0}, {1, 1, 2}},
}

// tileWalls[type]: which sides of a tile of each type are wall (TRIGGER.C's tile_walls).
var tileWalls = [16]byte{30, 0, 19, 21, 11, 13, 32, 32, 32, 32, 0, 0, 0, 0, 0, 30}

// ATM is an automap note.
type ATM struct {
	text string
	x, y int // x -1 once erased
}

var (
	automapRegistered bool
	diagSide          = [4]int{1, 2, 0, 3}
	doorDy            = [4]int{-1, 0, -1, 1}
	doorDx            = [4]int{0, -1, -1, -1}

	notesDirty     bool
	mapLevel       int
	PlayersMap     [MAP_SIZE][MAP_SIZE]byte
	oldStrings     int
	numWords       int
	mapMouse       int
	ATMStrings     [101]ATM
	automapEnabled = true
)

// AutoMap opens the map screen.
func AutoMap() {
	if !automapRegistered {
		inputAddkey(KEY_ESC, MODE_GAME, MODE_MAP, func(a any) { newscr(a.(int)) })
		automapRegistered = true
	}
	setNewMusic(MUSIC_MAPS)
	changeMusicMaybe()
	SaveAutoMapLevel(nil, PlayerLevel)
	ShowAutoMapLevel(PlayerLevel)
	mapMouse = inputAddmouse(0, 0, 0x13F, 0xC7, nil, MODE_MAP, func(any) { ManageDungeonMap() })
	mouseConstrain(0x16, 7, 0x13F, 0xC7)
	mouseHide()
	forceMouseCursor(ICON_CURSORS + 0xC)
	mouseShow()
	notesDirty = false
}

func playersMapBytes() []byte {
	b := make([]byte, MAP_TILES)
	for y := range PlayersMap {
		copy(b[y*MAP_SIZE:], PlayersMap[y][:])
	}
	return b
}

// SaveAutoMapLevel writes PlayersMap as level lev's automap block.
func SaveAutoMapLevel(arc *Arc, lev int) bool {
	var a Arc
	if arc == nil {
		if !openArc(&a, "SAVE0\\lev.ark") {
			return false
		}
		arc = &a
		defer closeArc(&a)
	}
	return putArc(arc, lev+0x1A, playersMapBytes())
}

// GetAutoMapLevel reads level lev's automap into PlayersMap.
func GetAutoMapLevel(arc *Arc, lev int) bool {
	b := getArc(arc, lev+0x1A)
	if len(b) != 0 && len(b) != MAP_TILES {
		return false
	}
	for y := range PlayersMap {
		if len(b) >= (y+1)*MAP_SIZE {
			copy(PlayersMap[y][:], b[y*MAP_SIZE:])
		}
	}
	return true
}

// ExitAutoMap leaves the map screen.
func ExitAutoMap() {
	mouseHide()
	inputDel(mapMouse)
	unforceMouseCursor(0)
	SaveTheWords(mapLevel)
	var arc Arc
	if mapLevel != PlayerLevel && openArc(&arc, "SAVE0\\lev.ark") {
		GetAutoMapLevel(&arc, PlayerLevel)
		closeArc(&arc)
	}
	seg014_1DC5_15C5()
	grfxClear()
	grfxQuikpal(PAL_GAME)
	mouseFreereign()
	mouseShow()
}

func ClearAutoMap() { PlayersMap = [MAP_SIZE][MAP_SIZE]byte{} }

// ShowDungeonMap draws every seen tile, three pixels a tile, shading its sides.
func ShowDungeonMap() {
	for y := 1; y < 0x3F; y++ {
		for x := 1; x < 0x3F; x++ {
			t := int(PlayersMap[y][x] & 0xF)
			if t <= 0 || t >= 10 {
				continue
			}
			DoTile(t, x, y)
			var side [4]bool
			if tileWalls[t]&TW_DIAG != 0 {
				d := diagSide[t-2]
				side[d] = ShadeSide(d, x, y)
				d = (d + 1) & 3
				side[d] = ShadeSide(d, x, y)
			} else {
				for i := 0; i < 4; i++ {
					side[i] = ShadeSide(i, x, y)
				}
			}
			px, py := x*3+10, y*3+7
			for i := 0; i < 4; i++ {
				if side[i] && side[(i+1)&3] {
					PixelDarken(px-((i&2)<<1), py-((i&2)<<1), 3, 2)
				}
			}
		}
	}
}

// ShadeSide shades one side of the tile at x, y if the neighbour there is not open.
func ShadeSide(side, x, y int) bool {
	px := (x-1)*3 + 10
	py := (y-1)*3 + 7
	switch side {
	case 0:
		y++
	case 1:
		x++
	case 2:
		y--
	case 3:
		x--
	}
	t := int(PlayersMap[y&0x3F][x&0x3F] & 0xF)
	if t > 0 && t < 10 {
		return false
	}
	a, b := 6, 2
	if t == 11 {
		a, b = 0, 4
	}
	switch side {
	case 0, 2:
		if side == 0 {
			py += 4
		}
		py--
		for i := 0; i < 3; i++ {
			PixelDarken(px+i, py, a, b)
		}
	case 1, 3:
		if side == 1 {
			px += 4
		}
		px--
		for i := 0; i < 3; i++ {
			PixelDarken(px, py+i, a, b)
		}
	}
	return true
}

// grPixel plots one pixel of colour c (seg015_1F9B_25A, UW2's gr_pixel).
func grPixel(x, y, c int) {
	if x < 0 || x >= gr.W || y < 0 || y >= gr.H {
		return
	}
	gr.Draw()[gr.RowOf(y)*gr.W+x] = byte(c)
}

// PixelDarken adds base plus a random 0..n-1 to the colour at x, y.
func PixelDarken(x, y, base, n int) {
	d := 0x7FFF / n
	rand() // the original also sets the pen colour with one draw
	grPixel(x, y, base+rand()/d+int(gr.ReadPixel(x, y)))
}

// DoTile draws one tile from its PlayersMap byte.
func DoTile(typ, x, y int) {
	if typ == 0 {
		return
	}
	b := PlayersMap[y][x] >> 4
	shade := b & 3
	kind := b & 0xFC
	px := (x-1)*3 + 10
	py := (y-1)*3 + 7
	if typ >= 6 {
		typ = 1
	}
	typ--
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			switch tilePattern[typ][j][i] {
			case 1:
				switch shade {
				case 0:
					PixelDarken(px+i, py+j, 2, 3)
				case 1:
					grPixel(px+i, py+j, rand()%2+0xB1)
				case 2:
					grPixel(px+i, py+j, rand()%2+0xB5)
				}
			case 2:
				PixelDarken(px+i, py+j, 6, 2)
			}
		}
	}
	switch kind {
	case 0xC, 0x10:
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				PixelDarken(px+i, py+j, 6, 3)
			}
		}
	case 8:
		for j := 0; j < 3; j++ {
			for i := 0; i < 3; i++ {
				grPixel(px+i, py+j, rand()*3/0x8000+0xE9)
			}
		}
	case 4:
		DoDoorTile(x, y, px, py)
	}
}

// DoDoorTile draws a door turned to face the open neighbours.
func DoDoorTile(x, y, px, py int) {
	px++
	py++
	PixelDarken(px, py, 6, 3)
	for d := 0; d < 4; d++ {
		if PlayersMap[(y+doorDy[d])&0x3F][(x+doorDx[d])&0x3F]&0xF == 1 ||
			PlayersMap[(y-doorDy[d])&0x3F][(x-doorDx[d])&0x3F]&0xF == 1 {
			PixelDarken(px+doorDy[d], py+doorDx[d], 6, 3)
			PixelDarken(px-doorDy[d], py-doorDx[d], 6, 3)
			break
		}
	}
}

// DetermineBetterHit is, of two notes under the mouse, the one whose middle is nearer.
func DetermineBetterHit(a, b *ATM, x, y int) *ATM {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	w := gr.StringWidth(a.text)
	ax := abs(a.x + w/2 - x)
	ay := abs(a.y - 2 - y)
	w = gr.StringWidth(b.text)
	bx := abs(b.x + w/2 - x)
	by := abs(b.y - 2 - y)
	if by < ay || bx < ax {
		return b
	}
	return a
}

// ManageDungeonMap is the map screen's mouse handler: notes, erasing, the level arrows
// and leaving.
func ManageDungeonMap() {
	gr.ForegroundColor, gr.BackgroundColor = 0x2D, 0x2D
	mx := inplist.x - 0x16
	my := inplist.y - 7
	cmd := inplist.cmd
	if cmd >= 4 {
		return
	}
	mouseRelease(true)
	switch {
	case mx > 0x104 && mx < 0x140 && my > 0x13 && my < 0x33:
		cmd = 0xFF
		newscr(MODE_GAME)
	case mx > 0x104 && mx < 0x140 && my > 0x33 && my < 0x52:
		cmd = 0xFD
		mouseConstrain(0, 0, 0x13F, 0xC7)
		mouseHide()
		forceMouseCursor(ICON_CURSORS + 0xD)
		mouseShow()
		for mouseGetInput() != 1 {
		}
		mx, my = mouseGetxy()
		mouseConstrain(0x16, 7, 0x13F, 0xC7)
		unforceMouseCursor(1)
	case mx > 0x113 && mx < 0x140 && my > 0xB7 && my < 0xC7:
		cmd = 0xFC
	case mx > 0x113 && mx < 0x140 && my > 0 && my < 0x12:
		cmd = 0xFB
	default:
		cmd = 0xFE
	}
	switch cmd {
	case 0xFE:
		curx := 0
		if numWords != 100 {
			note := &ATMStrings[numWords]
			grfxLoadFont("font4x5p.sys")
			forceMouseCursor(ICON_CURSORS + 0xE)
			note.x = mx
			note.y = my + 4
			var text []byte
			curx = note.x - 1
			mousePutxy(curx+10, my+0x12)
			for {
				for {
					if cmd = doKeyboardInput(false); cmd >= 0 {
						break
					}
					if cmd = mouseGetbut(); cmd > 0 {
						break
					}
				}
				if cmd == KEY_ENTER || cmd == KEY_ESC || cmd < 4 {
					break
				}
				if cmd >= 0x20 && cmd <= 0x7A {
					ch := strings.ToUpper(string(rune(cmd)))
					w := gr.StringWidth(string(text)) + gr.StringWidth(ch)
					if note.x+w > 0x13B || len(text) > 0x2D {
						doBeep(0x12C, 0xA)
					} else {
						text = append(text, ch[0])
					}
				} else if cmd == KEY_BACKSPACE {
					if len(text) > 0 {
						w := gr.StringWidth(string(text))
						text = text[:len(text)-1]
						gr.SetTheColor(0x106)
						gr.Rectangle(note.x, note.y, note.x+w, note.y-6)
					}
				}
				curx = note.x + gr.StringWidth(string(text)) - 1
				mousePutxy(curx+10, my+0x12)
				gr.StringToScreen(string(text), note.x, note.y)
			}
			grfxLoadFont("font5x6p.sys")
			if len(text) > 0 {
				notesDirty = true
				note.text = string(text)
				numWords++
			}
		}
		mouseHide()
		RedisplayStrings()
		mousePutxy(curx+0x16, my+7)
		unforceMouseCursor(2)
	case 0xFD:
		if numWords > 0 {
			var best *ATM
			hit := 0
			for i := 0; i < numWords; i++ {
				note := &ATMStrings[i]
				w := gr.StringWidth(note.text)
				if note.x <= mx && note.x+w >= mx && note.y-5 <= my && note.y >= my {
					best = DetermineBetterHit(best, note, mx, my)
					if best == note {
						hit = i
					}
				}
			}
			if best != nil {
				notesDirty = true
				w := gr.StringWidth(best.text)
				gr.SetTheColor(0x106)
				gr.Rectangle(best.x, best.y, best.x+w, best.y-5)
				if numWords-1 == hit {
					numWords--
				} else {
					ATMStrings[hit].x = -1
				}
				RedisplayStrings()
			}
		}
		mouseShow()
	case 0xFC:
		if mapLevel > 1 {
			ChangeAutoMapLevel(mapLevel - 1)
		}
	case 0xFB:
		if mapLevel < 0x63 {
			ChangeAutoMapLevel(mapLevel + 1)
		}
	}
	mouseRelease(true)
}

// RedisplayStrings draws the notes that have not been erased.
func RedisplayStrings() {
	grfxLoadFont("font4x5p.sys")
	gr.ForegroundColor, gr.BackgroundColor = 0x2D, 0x2D
	for i := 0; i < numWords; i++ {
		if ATMStrings[i].x >= 0 {
			gr.StringToScreen(ATMStrings[i].text, ATMStrings[i].x, ATMStrings[i].y)
		}
	}
	grfxLoadFont("font5x6p.sys")
}

const atmSize = 0x36

// SaveTheWords writes the level's notes if they changed, dropping the erased ones.
func SaveTheWords(lev int) {
	if !notesDirty || numWords == 0 {
		return
	}
	for i := 0; i < numWords; i++ {
		if ATMStrings[i].x < 0 {
			copy(ATMStrings[i:], ATMStrings[i+1:])
			numWords--
		}
	}
	b := make([]byte, numWords*atmSize)
	for i := 0; i < numWords; i++ {
		r := b[i*atmSize:]
		copy(r[:0x31], ATMStrings[i].text)
		put16(r, 0x32, uint16(ATMStrings[i].x))
		put16(r, 0x34, uint16(ATMStrings[i].y))
	}
	var arc Arc
	if openArc(&arc, "SAVE0\\lev.ark") {
		putArc(&arc, lev+0x23, b)
		closeArc(&arc)
	}
}

// GetTheWords reads the level's notes and draws them.
func GetTheWords(lev int) {
	oldStrings, numWords = 0, 0
	var arc Arc
	if openArc(&arc, "SAVE0\\lev.ark") {
		b := getArc(&arc, lev+0x23)
		n := min(len(b)/atmSize, 100)
		for i := 0; i < n; i++ {
			r := b[i*atmSize:]
			ATMStrings[i] = ATM{cstr(r[:0x32]), int(int16(le16(r, 0x32))), int(int16(le16(r, 0x34)))}
		}
		oldStrings, numWords = n, n
		RedisplayStrings()
		closeArc(&arc)
	}
}

// ShowAutoMapLevel draws the map screen for level lev.
func ShowAutoMapLevel(lev int) {
	buf := make([]byte, 0xFA00)
	mouseHide()
	if bltfromdrive("DATA\\blnkmap.byt", buf) {
		gr.GrSoftPageFlip()
		gr.SetTheWindow(0, 0xC7, 0x13F, 0)
		gr.Show(0, 0xC7, buf, 0xC8, 0x140, 0, 0)
		ShowDungeonMap()
		if lev == PlayerLevel && lev != 9 {
			px := (ThePlayer.HomeX()-1)*3 + 10 - 1
			py := (ThePlayer.HomeY()-1)*3 + 12
			gr.Transparency = true
			picToScreen(ICON_BUTTONS+0x3F, px, py)
			gr.Transparency = false
		}
		mapLevel = lev
		grfxQuikpal(PAL_MAP)
		gr.GrPageFlip()
		gr.GrSoftPageFlip()
		gr.CopyHiddenToVisible()
		GetTheWords(lev)
		gr.ForegroundColor, gr.BackgroundColor = 0x2D, 0x2D
		grfxLoadFont("fontbig.sys")
		num := strconv.Itoa(lev)
		gr.StringToScreen(num, 0x121-gr.StringWidth(num)/2, 0xC2)
		grfxLoadFont("font5x6p.sys")
	} else {
		mouseShow()
		ExitAutoMap()
	}
	mouseShow()
}

// ChangeAutoMapLevel shows another level's map (levels 1 to 8).
func ChangeAutoMapLevel(lev int) {
	SaveTheWords(mapLevel)
	ClearAutoMap()
	var arc Arc
	if lev < 9 && openArc(&arc, "SAVE0\\lev.ark") {
		GetAutoMapLevel(&arc, lev)
		closeArc(&arc)
	}
	ShowAutoMapLevel(lev)
}

func automapScr() { loopMusicMaybe() }
