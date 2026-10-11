package uw

import (
	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/host"
)

// The game screen without its frame: the 3D view fills the screen down to the message
// scroll, and what is left of the furniture lies over it on gr's HUD layer, cut out of its
// pictures: the gargoyle at the top in the middle, his eyes the weapon's charge (the power
// gem's job), the compass at the bottom in the middle, the flasks at the bottom right, the
// rune shelf and the active spells either side of the compass. The right-hand panel shows
// while mouse look is off, over the view's right side. The icon bar, the dragons, the power
// gem and the view's frame are gone.

const (
	hudShift = 0x16 // the furniture under and over the old view moves right with its middle

	// MAIN.BYT's gargoyle with his wings and claws and the bar he holds: columns
	// gargX0 to gargX0+gargW-1 of the top gargH rows.
	gargX0 = 0x52
	gargW  = 0x72
	gargH  = 0x13

	// eyesX, eyesY are the eyes' place in MAIN.BYT (EYES.GR's pictures are 20 by 3).
	eyesX = 0x80
	eyesY = 4

	compassY = 0x39 // the plate's top (y up)
	flaskY   = 0x41

	// viewH is the view's height: the screen down to the message scroll.
	viewH = 0xA8

	panelX, panelY, panelW, panelH = 0xEC, 0xC0, 0x53, 0x72

	// PANEL_HIDDEN is RightPanel while the panel is hidden: no panel draws then.
	PANEL_HIDDEN = 5
)

var (
	hudActive  bool // the game screen is up
	panelShown bool
	gargPix    []byte
	gargKeep   []bool
	eyePics    [6][]byte // dark, slight yellow, bright yellow, orange, red, red dimmer
	eyeState   = -1
	eyeGlow    bool
	flaskKeep  []bool
	flX        = [2]int{0x106, 0x122}
	compassX   int // the plate's left (layoutHUD)
	runeX      [3]int
	spellX     [3]int
	runeY      = flaskY + 0x10 // the stones' 14 rows end 2 above the flasks
	spellY     = 0x33
)

// layoutHUD places the furniture whose place depends on the screen's width: the compass
// in the bottom left corner (its star 4 from the edge) with the active spells beside it,
// the rune shelf centred over the flasks, which stay at the right with the panel.
func layoutHUD() {
	compassX = leftX(3)
	for i := range spellX {
		spellX[i] = compassX + 0x38 + (2-i)*0x11
	}
	for i := range runeX {
		runeX[i] = 0x108 + i*0x0F
	}
}

// widenScroll draws the message scroll's art across a wide screen: MAIN.BYT's left end at
// the left edge, its paper (columns scrollA to scrollB-1) repeated up to where the 320 wide
// screen's, drawn already, goes on.
func widenScroll() {
	const scrollA, scrollB, rows = 0x18, 0x118, 0x20
	if gr.OX == 0 {
		return
	}
	buf := make([]byte, 0xFA00)
	if !bltfromdrive("DATA\\main.byt", buf) {
		return
	}
	w := gr.OX + scrollA
	strip := make([]byte, w*rows)
	for r := 0; r < rows; r++ {
		src := buf[(0xC8-rows+r)*0x140:]
		for p := 0; p < w; p++ {
			c := p
			if p >= scrollA {
				c = scrollA + (p-scrollA)%(scrollB-scrollA)
			}
			strip[r*w+p] = src[c]
		}
	}
	t := gr.Transparency
	gr.Transparency = false
	gr.Show(leftX(0), rows-1, strip, rows, w, 0, 0)
	gr.Transparency = t
}

// iconPix is an icon's picture: its pixels, rows top first, and its size.
func iconPix(icon int) ([]byte, int, int) {
	s := grsWhich1(icon)
	if s >= firstVram {
		b := slotVram(s)
		if b == nil {
			return nil, 0, 0
		}
		if b.SW != 0 {
			return b.Pix, b.SW, b.SH
		}
		return b.Pix, b.W, b.H
	}
	bm := slotPic(s)
	w, h := bmSize(bm)
	return grsUnpack(bm), w, h
}

// blit copies a w by h picture into a cw wide canvas at (x, y), opaque or skipping 0.
func blit(canvas []byte, cw int, pix []byte, w, h, x, y int, trans bool) {
	ch := len(canvas) / cw
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if x+c < 0 || x+c >= cw || y+r < 0 || y+r >= ch || r*w+c >= len(pix) {
				continue
			}
			if p := pix[r*w+c]; p != 0 || !trans {
				canvas[(y+r)*cw+x+c] = p
			}
		}
	}
}

// hudPut puts a picture on the HUD with the pointer off the screen meanwhile.
func hudPut(x, y, w, h int, pix []byte, keep []bool) {
	if !hudActive || len(pix) < w*h || len(keep) < w*h {
		return
	}
	mouseHide()
	gr.PutHUD(x, y, w, h, pix, keep)
	mouseShow()
}

// hudClear gives a rectangle back to the view.
func hudClear(x, y, w, h int) {
	hudPut(x, y, w, h, make([]byte, w*h), make([]bool, w*h))
}

// hudStart puts the furniture on the HUD (initScrgr, with the game screen).
func hudStart() {
	hudActive = true
	gr.HUDOn = true
	gr.ClearHUD()
	loadHUD()
	eyeState = -1
	healthOn = false
	drawGargoyle()
	miniOn = false
	panelShown = lookOn // so that showPanel draws it or hides it
	showPanel(!lookOn)
}

// hudStop takes it off when the game screen goes.
func hudStop() {
	showPanel(false)
	hudActive = false
	gr.HUDOn = false
	gr.ClearHUD()
}

// loadHUD cuts the pictures once: the gargoyle out of MAIN.BYT (its black is the
// background), the eyes' colours (the orange is the bright yellow with less green, to the
// nearest of the game palette's colours, where the red eyes differ from the yellow) and the flasks' outline.
func loadHUD() {
	if gargPix != nil {
		return
	}
	buf := make([]byte, 0xFA00)
	if !bltfromdrive("DATA\\main.byt", buf) {
		pfatalCode(ERR_READ | 0xB)
	}
	gargPix = make([]byte, gargW*gargH)
	gargKeep = make([]bool, gargW*gargH)
	for r := 0; r < gargH; r++ {
		for c := 0; c < gargW; c++ {
			p := buf[r*0x140+gargX0+c]
			gargPix[r*gargW+c], gargKeep[r*gargW+c] = p, p != 0
		}
	}

	eye := func(n int) []byte {
		pix, _, _ := iconPix(ICON_EYES + n)
		return append([]byte(nil), pix...)
	}
	eyePics = [6][]byte{eye(0), eye(4), eye(6), nil, eye(9), eye(8)}
	var pal [0x300]byte
	readQuikpal(PAL_GAME, pal[:])
	yellow, red := eyePics[2], eyePics[4]
	orange := make([]byte, len(yellow))
	for i := range orange {
		a := yellow[i]
		if i >= len(red) || red[i] == a {
			orange[i] = a
			continue
		}
		c := pal[int(a)*3:]
		want := [3]int{int(c[0]), int(c[1]) * 55 / 100, int(c[2]) / 2}
		orange[i] = nearestColour(pal[:], want, 0x40)
	}
	eyePics[3] = orange

	empty, w, h := iconPix(ICON_FLASKS + 0x4B)
	sil, _, _ := iconPix(ICON_FLASKS + 0x4C)
	flaskKeep = make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if i >= len(empty) || i >= len(sil) {
				continue
			}
			flaskKeep[i] = flaskPixel(x, y, empty[i], sil[i])
		}
	}
}

// nearestColour is the palette's colour nearest want, from colour from up, of those the
// palette cycles leave alone.
func nearestColour(pal []byte, want [3]int, from int) byte {
	best, bestD := 0, 1<<30
	for c := max(from, 1); c < 0x100; c++ {
		if c >= 0x10 && c < 0x18 || c >= 0x30 && c < 0x40 {
			continue
		}
		d := 0
		for k := 0; k < 3; k++ {
			e := int(pal[c*3+k]) - want[k]
			d += e * e
		}
		if d < bestD {
			best, bestD = c, d
		}
	}
	return byte(best)
}

// flaskPixel is whether the empty flask's pixel at x, y (sil the silhouette's) is the
// flask's: inside the glass, the glass's outline and the gold stand; not the stone under
// and behind it or the chain beside it.
func flaskPixel(x, y int, e, sil byte) bool {
	switch {
	case x >= 23:
		return false
	case sil == 0:
		return true
	case e >= 0xA3 && e <= 0xA9:
		return y >= 24
	case e >= 0x60 && e <= 0x6E:
		return y < 24 || x < 22
	case e == 0x6F:
		return y <= 28 && (x <= 19 || x <= 22 && y >= 18 && y < 24)
	}
	return false
}

// eyesFor is the eyes' colour for power gem frame p: 0 resting, 1 to 8 charging, 9 full.
func eyesFor(p int) int {
	switch {
	case p <= 0:
		return 0
	case p <= 3:
		return 1
	case p <= 6:
		return 2
	case p <= 8:
		return 3
	}
	return 4
}

// drawGargoyle draws the gargoyle with his eyes the colour the charge gives them.
func drawGargoyle() {
	st := eyesFor(int(goal[SCR_POWER]))
	if st == 4 && eyeGlow {
		st = 5
	}
	if st == eyeState {
		return
	}
	eyeState = st
	pix := append([]byte(nil), gargPix...)
	blit(pix, gargW, eyePics[st], 20, 3, eyesX-gargX0, eyesY, false)
	hudPut(midX(gargX0+hudShift), 0xC7, gargW, gargH, pix, gargKeep)
}

// drawCompass draws the compass turned to dir: the star and the needle on it, without the
// plate's ring and stone.
func drawCompass(dir int) {
	plate, w, h := iconPix(dir&3 + ICON_COMPASS)
	if plate == nil || len(plate) < w*h {
		return
	}
	pix := append([]byte(nil), plate[:w*h]...)
	if n, nw, nh := iconPix(dir + ICON_COMPASS + 4); n != nil {
		blit(pix, w, n, nw, nh, nedlX[dir]-0x70, 0x44-nedlY[dir], false)
	}
	hudPut(compassX, compassY, w, h, pix, compassKeep(pix, w, h))
}

// compassKeep is the star: the largest patch of its gold and the needle's red, with the
// black outline round it.
func compassKeep(pix []byte, w, h int) []bool {
	gold := func(p byte) bool { return p >= 0xA4 && p <= 0xAC || p == 0xEB || p >= 0xB0 && p <= 0xBF }
	label := make([]int, w*h)
	best, bestN, next := 0, 0, 0
	for i := range pix {
		if !gold(pix[i]) || label[i] != 0 {
			continue
		}
		next++
		n := 0
		stack := []int{i}
		label[i] = next
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			n++
			x, y := j%w, j/w
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					continue
				}
				if k := ny*w + nx; label[k] == 0 && gold(pix[k]) {
					label[k] = next
					stack = append(stack, k)
				}
			}
		}
		if n > bestN {
			best, bestN = next, n
		}
	}
	keep := make([]bool, w*h)
	for i := range keep {
		keep[i] = best != 0 && label[i] == best
	}
	for i := range keep {
		if keep[i] || pix[i] != 3 {
			continue
		}
		x, y := i%w, i/w
		for dy := -1; dy <= 1 && !keep[i]; dy++ {
			for dx := -1; dx <= 1; dx++ {
				nx, ny := x+dx, y+dy
				if nx >= 0 && nx < w && ny >= 0 && ny < h && label[ny*w+nx] == best && best != 0 {
					keep[i] = true
					break
				}
			}
		}
	}
	return keep
}

// drawFlask draws flask f filled to level n with liquid pictures from base (ICON_FLASKS'
// numbering), bubbles picture bub showing on top when not negative.
func drawFlask(f, base, n, bub int) {
	empty, w, h := iconPix(ICON_FLASKS + 0x4B)
	sil, _, _ := iconPix(ICON_FLASKS + 0x4C)
	if empty == nil || len(empty) < w*h || len(sil) < w*h || flaskKeep == nil {
		return
	}
	pix := append([]byte(nil), empty[:w*h]...)
	for i := 1; i <= n; i++ {
		if l, lw, lh := iconPix(base + i - 1); l != nil {
			blit(pix, w, l, lw, lh, 0, 0x4A-liquidY[i], false)
		}
	}
	if bub >= 0 && n > 0 {
		if b, bw, bh := iconPix(bub); b != nil {
			blit(pix, w, b, bw, bh, 0, 0x4A-liquidY[n], false)
		}
	}
	for i := range pix {
		if sil[i] != 0 {
			pix[i] = empty[i]
		}
	}
	hudPut(flX[f], flaskY, w, h, pix, flaskKeep)
}

// drawSmallIcon puts a 16 by h icon on the HUD at x, y, its 0 the view; a negative icon
// clears the place.
func drawSmallIcon(icon, x, y, h int) {
	if icon < 0 {
		hudClear(x, y, 0x10, h)
		return
	}
	pix, w, ih := iconPix(icon)
	if pix == nil {
		return
	}
	canvas := make([]byte, 0x10*h)
	blit(canvas, 0x10, pix, w, ih, 0, 0, false)
	keep := make([]bool, len(canvas))
	for i, p := range canvas {
		keep[i] = p != 0
	}
	hudPut(x, y, 0x10, h, canvas, keep)
}

// panelSet is the panel (PANEL_*) a key or the rune bag asks for: shown at once while the
// panel is hidden, turned to otherwise.
func panelSet(panel int) {
	if !panelShown {
		RightPanel = panel
		setting[SCR_PANEL], goal[SCR_PANEL] = int8(panel), int8(panel)
		return
	}
	setScreenFrame(SCR_PANEL, panel)
}

// showPanel shows or hides the right-hand panel over the view.
func showPanel(on bool) {
	if !hudActive || on == panelShown {
		return
	}
	mouseHide()
	if on {
		RightPanel = int(setting[SCR_PANEL])
		panelShown = true
		setRunes(player.Shelf[:]) // the shelf goes under the panel
		gr.SetLayer(panelX, panelY, panelW, panelH, gr.LayerPage)
		if pic, ok := readGrFar("panels", RightPanel); ok {
			gr.Show(panelX, panelY, pic, panelH, panelW, 0, 0)
		}
		panelDispatch[RightPanel]()
		panelCorners()
	} else {
		holdScrgr()
		panelShown = false
		RightPanel = PANEL_HIDDEN
		gr.SetLayer(panelX, panelY, panelW, panelH, gr.LayerFrame)
		gr.RefreshRect(panelX, panelY, panelW, panelH)
		setRunes(player.Shelf[:])
		editchng(2)
	}
	inputEnable(panelInput, on)
	mouseShow()
}

// keyPanel: i, r, k show the inventory (0), the rune bag (1) or the statistics (2) with
// mouse look off; the panel showing already goes back to mouse look.
func keyPanel(panel int) {
	if panelShown && RightPanel == panel {
		mouseLookSet(true)
		return
	}
	panelSet(panel)
	mouseLookSet(false)
	showPanel(true)
}

// panelTurnKey keys the panel's place while it turns: the dark it turns on (colour F1h)
// shows the view; done, the whole place is the panel's again.
func panelTurnKey(done bool) {
	if !hudActive {
		return
	}
	page := gr.Draw()
	for r := 0; r <= panelH; r++ {
		row := gr.RowOf(panelY - r)
		for c := 0; c <= panelW; c++ {
			i := row*gr.W + panelX + c
			if done && (c == panelW || r == panelH) {
				gr.Layer[i] = gr.LayerFrame
			} else if done || page[i] != 0xF1 {
				gr.Layer[i] = gr.LayerPage
			} else {
				gr.Layer[i] = gr.LayerFrame
			}
		}
	}
	gr.RefreshRect(panelX, panelY, panelW+1, panelH+1)
	if done {
		panelCorners()
	}
}

// panelCorners gives the black outside the panel's rounded corners to the view: from each
// corner the pixels of the corner's colour joined to it, within a few pixels of it.
func panelCorners() {
	if !hudActive || !panelShown {
		return
	}
	const box = 6
	page := gr.Draw()
	at := func(c, r int) int { return gr.RowOf(panelY-r)*gr.W + panelX + c }
	for _, k := range [4][2]int{{0, 0}, {panelW - 1, 0}, {0, panelH - 1}, {panelW - 1, panelH - 1}} {
		black := page[at(k[0], k[1])]
		seen := map[[2]int]bool{k: true}
		stack := [][2]int{k}
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			gr.Layer[at(p[0], p[1])] = gr.LayerFrame
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				q := [2]int{p[0] + d[0], p[1] + d[1]}
				if q[0] < 0 || q[0] >= panelW || q[1] < 0 || q[1] >= panelH || seen[q] ||
					iabs(q[0]-k[0]) >= box || iabs(q[1]-k[1]) >= box || page[at(q[0], q[1])] != black {
					continue
				}
				seen[q] = true
				stack = append(stack, q)
			}
		}
	}
	gr.RefreshRect(panelX, panelY, panelW, panelH)
}

// The enemy's health: a bar under the gargoyle, centred, after each blow the player lands on
// a creature. It narrows to its middle, outline and all, as the creature's hit points fall,
// green above two thirds of its average, orange above a third and red below (DOS's three eye
// flashes), and goes after healthShow.
const (
	healthW, healthH = 0x40, 2 // the inside
	healthX          = eyesX + 10 + hudShift - healthW/2 - 1
	healthY          = 0xC7 - gargH - 2
	healthShow       = 4 * 256 // GameTime's ticks
)

var (
	healthCols  [4]byte // outline, green, orange, red
	healthUntil uint32
	healthOn    bool
)

// showEnemyHealth shows hp of most hit points on the bar.
func showEnemyHealth(hp, most int) {
	if !hudActive || most <= 0 {
		return
	}
	if healthCols[0] == 0 {
		var pal [0x300]byte
		readQuikpal(PAL_GAME, pal[:])
		for i, want := range [4][3]int{{0, 0, 0}, {8, 48, 8}, {63, 27, 0}, {52, 4, 4}} {
			healthCols[i] = nearestColour(pal[:], want, 1)
		}
	}
	hp = min(max(hp, 0), most)
	w := (hp*healthW + most - 1) / most
	col := healthCols[3]
	switch {
	case hp*3 > most*2:
		col = healthCols[1]
	case hp*3 > most:
		col = healthCols[2]
	}
	bw, bh := healthW+2, healthH+2
	pix := make([]byte, bw*bh)
	keep := make([]bool, bw*bh)
	x0 := (healthW - w) / 2 // the bar's outline from x0 to x0+w+1
	for r := 0; r < bh; r++ {
		for c := x0; c < x0+w+2 && w > 0; c++ {
			p := healthCols[0]
			if r > 0 && r <= healthH && c > x0 && c <= x0+w {
				p = col
			}
			pix[r*bw+c], keep[r*bw+c] = p, true
		}
	}
	hudPut(midX(healthX), healthY, bw, bh, pix, keep)
	healthUntil, healthOn = GameTime()+healthShow, true
}

// hideEnemyHealth takes the bar off once its time is up.
func hideEnemyHealth() {
	if healthOn && int32(GameTime()-healthUntil) >= 0 {
		healthOn = false
		hudClear(midX(healthX), healthY, healthW+2, healthH+2)
	}
}

// onPanel is whether screen point x, y (y up) is on the right-hand panel while it shows.
func onPanel(x, y int) bool {
	return panelShown && x >= panelX && x < panelX+panelW && y <= panelY && y > panelY-panelH
}

// The game screen goes as wide as the window wants (host.WantW): the 320 wide screen's
// coordinates, which the game draws in, stay, with its right edge on the wide screen's; the
// view, the message scroll and the gargoyle span or centre on the whole width, and the
// furniture on the left (the compass, the options panel) has negative x. The other screens
// (the map, the help, the menus, the cutscenes) are 320 wide, which the host shows 4:3.

// leftX is x from the screen's left edge, midX x of the 320 wide screen moved with its
// middle.
func leftX(x int) int { return x - gr.OX }
func midX(x int) int  { return x - gr.OX/2 }

// wideOn makes the screen as wide as the window wants, for the game screen.
func wideOn() {
	gr.SetWidth(host.WantW, host.WantW-0x140)
	mouseFreereign()
	layoutScroll()
	layoutHUD()
}

// wideOff makes it 320 wide again.
func wideOff() {
	gr.SetWidth(0x140, 0)
	mouseFreereign()
	layoutScroll()
	layoutHUD()
}

// The game screen is laid out again for a new width once the window's has kept still a
// quarter of a second, while nothing is going on (not carrying, aiming or talking).
var (
	wantW     int
	wantSince uint32
)

func checkWidth() {
	if !inGame || scrnum != 0 || !hudActive || inplist.mode != MODE_GAME || GameInputMode != GIM_NONE {
		return
	}
	if host.WantW != wantW {
		wantW, wantSince = host.WantW, GameTime()
		return
	}
	if wantW != gr.W && GameTime()-wantSince >= 0x40 {
		newscr(MODE_GAME)
	}
}
