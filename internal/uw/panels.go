package uw

import "github.com/memmaker/uw1go/internal/gr"

// Port of PANELS.C: the screen furniture over the 3D view (hud.go): the vitality and mana
// flasks, the compass, the gargoyle's eyes (the power gem's charge), the first-person
// weapon, the rune shelf and active spell icons, the turning right-hand panel and the frame
// buffer send. setScreenFrame(which, value) sets an element's goal; updateScreen, each
// frame, steps each element towards its goal through its adjust function: the 'now' ones
// at once, the fast ones every 32 ticks and the slow ones every 64. The dragons (4, 5) and
// the eyes' own blink (7) are gone with the frame.

var (
	flipCh, flipLeft, flipHigh, flipTop, flipPage int
	weapX, weapY                                  [0x1C]byte
	setting, goal                                 [9]int8
	flipWidthNow, pnlWide, pnlSkew                int

	// RightPanel is the panel showing on the right (PANEL_*, PANEL_TURNING while turning,
	// PANEL_HIDDEN while mouse look hides it).
	RightPanel                        int
	slowAdjust, fastAdjust, nowAdjust int
	weapid, lastWeap                  = -1, -1
	wlstate                           = WEAP_SHEATHED
	weapFrame                         = -1
	wxo, wfo                          = 0, 1
	liquidY                           = [14]int{0x2C, 0x30, 0x32, 0x34, 0x36, 0x38, 0x3A, 0x3B, 0x3C, 0x3D, 0x3F, 0x41, 0x43, 0x45}
	bubbling                          [2]byte
	nedlX                             = [16]int{0x88, 0x80, 0x78, 0x74, 0x70, 0x70, 0x74, 0x7C, 0x88, 0x90, 0x9C, 0xA0, 0xA0, 0x9C, 0x98, 0x90}
	nedlY                             = [16]int{0x44, 0x42, 0x41, 0x3E, 0x3A, 0x36, 0x34, 0x31, 0x2F, 0x31, 0x34, 0x36, 0x3A, 0x3E, 0x41, 0x42}
	flipHpct                          = [8]int{0x64, 0x67, 0x69, 0x6A, 0x6A, 0x6A, 0x69, 0x67}
	flipWpct                          = [8]int{0x64, 0x5C, 0x47, 0x26, 0x00, 0x26, 0x47, 0x5C}
	// The turning panel's pictures (EMS handles in the original): 0 the old panel and,
	// at 0x2800, the edge; 1 the new panel; 2 the work buffer.
	flipBuf                     [3][]byte
	pbufFrame                   int
	pbufFlag                    bool
	flipFlags                   = 1
	panelDispatch               = [3]func(){RedispInv, RedispRune, RedispStat}
	weapPics                    [0x1C][]byte // the weapon's pictures (EMS in the original)
	ShowStupidFirstPersonWeapon = true
	demoMode                    bool
)

func adjustElement(i int) {
	switch i {
	case 0, 1:
		adjustFlasks(i)
	case 2:
		adjustCompass()
	case 3:
		adjustPower()
	case 6:
		adjustPanel()
	case 8:
		adjustWeapon()
	}
}

// flaskFrames is flask f's liquid pictures: the first level's and the bubbles' first and
// last.
func flaskFrames(f int) (base, bubFirst, bubLast int) {
	switch {
	case f == 1:
		return ICON_FLASKS + 0x19, ICON_FLASKS + 0x26, ICON_FLASKS + 0x31
	case player.Poison() != 0:
		return ICON_FLASKS + 0x32, ICON_FLASKS + 0x3F, ICON_FLASKS + 0x4A
	}
	return ICON_FLASKS, ICON_FLASKS + 0xD, ICON_FLASKS + 0x18
}

// setFlask draws flask which (0 vitality, 1 mana) at its goal level at once.
func setFlask(which int) {
	if which != 0 && which != 1 {
		return
	}
	base, _, _ := flaskFrames(which)
	setting[which] = goal[which]
	drawFlask(which, base, int(goal[which]), -1)
}

// setCompass draws the compass at its goal heading at once.
func setCompass() {
	setting[SCR_COMPASS] = goal[SCR_COMPASS]
	drawCompass(int(goal[SCR_COMPASS]) & 0xF)
}

// resetScrgr resets every display element.
func resetScrgr() {
	for i := range setting {
		setting[i], goal[i] = 0, 0
	}
	RightPanel = PANEL_INV
	if hudActive && !panelShown {
		RightPanel = PANEL_HIDDEN
	}
	setting[SCR_WEAPON], goal[SCR_WEAPON] = WEAP_SHEATHED, WEAP_SHEATHED
	wlstate = WEAP_SHEATHED
	weapFrame = 0
}

var scrgrInited bool

// initScrgr draws the furniture.
func initScrgr() {
	if !scrgrInited {
		goal[SCR_WEAPON], setting[SCR_WEAPON] = WEAP_SHEATHED, WEAP_SHEATHED
		scrgrInited = true
	}
	hudStart()
	for i := 0; i < 2; i++ {
		setFlask(i)
	}
	setCompass()
	setRunes(player.Shelf[:])
}

// holdScrgr stops a panel turn where it is.
func holdScrgr() {
	RightPanel = int(setting[SCR_PANEL])
	goal[SCR_PANEL] = setting[SCR_PANEL]
	pbufFrame = 0
	pbufFlag = false
}

func freeScrgr() { freePanelflip() }

// setScreenFrame sets display element which's goal to val.
func setScreenFrame(which, val int) {
	switch which {
	case SCR_VITALITY, SCR_MANA:
		max := int(playerdat.Avghit)
		if which == SCR_MANA {
			max = int(player.MaxMana)
		}
		if max != 0 {
			goal[which] = int8(val * 12 / max)
		} else {
			goal[which] = 0
		}
		if goal[which] >= 12 {
			goal[which] = 12
		}
		slowAdjust |= 1 << which
	case SCR_COMPASS:
		if int(setting[SCR_COMPASS]) != val {
			slowAdjust |= 1 << which
			goal[SCR_COMPASS] = int8(val)
		}
	case SCR_POWER:
		if val != 9 {
			nowAdjust |= 1 << which
		} else {
			slowAdjust |= 1 << which
		}
		goal[which] = int8(val)
	case SCR_DRAGON, SCR_DRAGON2, SCR_EYES: // gone with the frame
	case SCR_WEAPON:
		if weapid != lastWeap && goal[SCR_WEAPON] == WEAP_SHEATHED {
			break
		}
		fastAdjust |= 1 << which
		goal[SCR_WEAPON] = int8(val)
	case SCR_PANEL:
		if hudActive && !panelShown {
			panelSet(val)
			break
		}
		if RightPanel == PANEL_TURNING {
			break
		}
		fallthrough
	default:
		slowAdjust |= 1 << which
		goal[which] = int8(val)
	}
}

var updOldTime byte

// updateScreen steps the display elements whose redraw bits are set.
func updateScreen() {
	hideEnemyHealth()
	now := byte(GameTime())
	did := false
	if nowAdjust != 0 {
		for i, bit := 0, 1; i < 9; i, bit = i+1, bit<<1 {
			if nowAdjust&bit != 0 {
				adjustElement(i)
				nowAdjust &^= bit
				did = true
			}
		}
	}
	if now>>5 != updOldTime>>5 {
		for i, bit := 0, 1; i < 9; i, bit = i+1, bit<<1 {
			if fastAdjust&bit != 0 {
				adjustElement(i)
			}
		}
		did = true
	}
	if now>>6 != updOldTime>>6 {
		if r := rand(); r < 0x666 && bubbling[r&1] == 0 {
			bubbling[r&1] = 1
			slowAdjust |= 1 << (r & 1)
		}
		rand() // the dragons' tail swish's, which keeps the random numbers as they were
		for i, bit := 0, 1; i < 9; i, bit = i+1, bit<<1 {
			if slowAdjust&bit != 0 {
				adjustElement(i)
			}
		}
		did = true
	}
	if did {
		updateSprites()
		updOldTime = now
	}
}

var bubStep [2]int

// adjustFlasks moves flask f one step towards its goal and animates its bubbles.
func adjustFlasks(f int) {
	if f != 0 && f != 1 {
		return
	}
	base, bubFirst, bubLast := flaskFrames(f)
	switch diff := goal[f] - setting[f]; {
	case diff > 0:
		setting[f]++
	case diff < 0:
		setting[f]--
	case bubbling[f] == 0:
		slowAdjust &^= 1 << f
	}
	n := int(setting[f])
	bub := -1
	if bubbling[f] == 1 {
		if n > 0 && n < 8 && bubFirst+bubStep[f] < bubLast {
			bubStep[f]++
			bub = bubFirst + bubStep[f]
		} else {
			bubStep[f] = 0
			bubbling[f] = 0
		}
	}
	drawFlask(f, base, n, bub)
}

// adjustCompass turns the compass one step the short way towards its goal.
func adjustCompass() {
	cur := int(setting[SCR_COMPASS])
	diff := int(goal[SCR_COMPASS]) - cur
	if diff == 0 {
		slowAdjust &^= 1 << SCR_COMPASS
		return
	}
	if diff < 0 {
		diff += 16
	}
	if diff <= 8 {
		cur++
	} else {
		cur--
	}
	cur &= 0xF
	setting[SCR_COMPASS] = int8(cur)
	drawCompass(cur)
}

// adjustPower shows the charge in the gargoyle's eyes: dark at rest, then slight yellow,
// bright yellow, orange and, full (9), a red glowing on and off.
func adjustPower() {
	if goal[SCR_POWER] >= 9 {
		eyeGlow = !eyeGlow
		slowAdjust |= 1 << SCR_POWER
	} else {
		eyeGlow = false
		slowAdjust &^= 1 << SCR_POWER
	}
	drawGargoyle()
}

// adjustPanel turns the right-hand panel over a frame at a time.
func adjustPanel() {
	if setting[SCR_PANEL] != goal[SCR_PANEL] {
		if !pbufFlag {
			pbufFlag = true
			initPanelflip(int(goal[SCR_PANEL]), 0xEC, 0xC0, 0x53, 0x72)
			RightPanel = PANEL_TURNING
		}
		if doPanelFrame() || doPanelFrame() { // two steps a frame (UltimaHacks' faster panels)
			setting[SCR_PANEL] = goal[SCR_PANEL]
			RightPanel = int(goal[SCR_PANEL])
			pbufFlag = false
			slowAdjust &^= 1 << SCR_PANEL
		}
	}
}

// loadWeapon asks for weapon kind id's pictures (0..3; -1 none) at the next redraw.
func loadWeapon(id int) {
	weapid = id
	if id >= 0 && id <= 3 || lastWeap != weapid {
		fastAdjust |= 1 << SCR_WEAPON
	}
}

// doWeapload loads the pictures of weapon kind weapid for the player's hand from
// WEAPONS.GR, and their places from DATA\weapons.dat.
func doWeapload() bool {
	if lastWeap == weapid {
		return true
	}
	lastWeap = weapid
	if weapid > 3 || weapid < 0 {
		return false
	}
	lefty := player.Lefty()
	start := (1-lefty)*0x70 + weapid*0x1C
	ok := gronkGr("weapons", start, 0x1C, func(img []byte, size, n int) bool {
		weapPics[n] = img
		return true
	})
	off := (1-lefty)*0xE0 + weapid*0x38
	b := readFile("DATA\\weapons.dat")
	if b == nil || len(b) < off+0x38 {
		return false
	}
	copy(weapX[:], b[off:])
	copy(weapY[:], b[off+0x1C:])
	return ok
}

// jiggleWeapon sways the weapon as the player moves: how 0 still, 1 a little, 2 more.
func jiggleWeapon(how int) {
	switch how {
	case 0:
		wxo, wfo = 0, 1
	case 1:
		if wxo > 0 {
			wxo = rand() % 5
		} else {
			wxo = -rand() % 5
		}
		wfo = 1
	case 2:
		if wxo > 0 {
			wxo = rand() % 10
		} else {
			wxo = -rand() % 10
		}
		wfo = 1
	}
}

var weapSwung bool

// adjustWeapon steps the first-person weapon.
func adjustWeapon() {
	editchng(2)
	if goal[SCR_WEAPON] > WEAP_SHEATHED {
		goal[SCR_WEAPON] = WEAP_SHEATHED
	}
	s := &setting[SCR_WEAPON]
	switch goal[SCR_WEAPON] {
	case WEAP_SHEATHED:
		if *s != WEAP_SHEATHED && *s != WEAP_SHEATHING {
			*s = WEAP_SHEATHING
			weapFrame = -1
		}
	case WEAP_READY:
		switch {
		case *s == WEAP_SHEATHED || *s == WEAP_SHEATHING:
			*s = WEAP_DRAWING
			weapFrame = 3
		case weapSwung:
			*s = WEAP_READY
			weapFrame = -1
		case *s >= 0 && *s <= 2:
			if weapFrame -= 2; weapFrame < -1 {
				*s = WEAP_READY
			}
		case *s == WEAP_READY && weapid == lastWeap:
			fastAdjust &^= 1 << SCR_WEAPON
			return
		}
	default:
		*s = goal[SCR_WEAPON]
	}
	switch *s {
	case WEAP_SHEATHING:
		if weapFrame++; weapFrame > 2 {
			*s = WEAP_SHEATHED
		}
	case WEAP_DRAWING:
		if weapFrame--; weapFrame < 0 {
			*s = WEAP_READY
		}
	case WEAP_SHEATHED:
		if weapid != lastWeap {
			doWeapload()
			goal[SCR_WEAPON] = int8(wlstate)
			wlstate = WEAP_SHEATHED
			break
		}
		weapFrame = -1
		fastAdjust &^= 1 << SCR_WEAPON
	case WEAP_READY:
		if weapid != lastWeap {
			goal[SCR_WEAPON] = WEAP_SHEATHED
			wlstate = WEAP_READY
			break
		}
		weapFrame = -1
		fastAdjust &^= 1 << SCR_WEAPON
	default:
		weapFrame++
		switch weapFrame {
		case 0:
			weapSwung = false
		case 3:
			fastAdjust &^= 1 << SCR_WEAPON
			weapSwung = true
		case 9:
			goal[SCR_WEAPON], *s = WEAP_READY, WEAP_READY
			weapFrame = -1
			fastAdjust &^= 1 << SCR_WEAPON
		}
	}
}

// loadWeapcm reads the weapon's colour map for the player's skin into palette 30.
func loadWeapcm() bool {
	b := readFile("DATA\\weapons.cm")
	off := 0
	if player.Body() == 1 {
		off = 0x10
	}
	if b == nil || len(b) < off+0x10 {
		return false
	}
	copy(Palettes[30][:], b[off:off+0x10])
	return true
}

// setRunes shows the three runes on the shelf (0x18 and up leaves a place empty).
func setRunes(runes []byte) {
	for i := 0; i < 3; i++ {
		icon := -1
		if runes[i] < 0x18 {
			icon = int(runes[i]) + 0xE8
		}
		drawSmallIcon(icon, runeX[i], runeY, 0x10)
	}
}

// activeSpells shows the icons of up to three active spells in the 3D view mode.
func activeSpells(spells []byte) {
	if inplist.mode != MODE_GAME {
		return
	}
	for i := 0; i < 3; i++ {
		icon := -1
		if spells[i] < 0x15 {
			icon = int(spells[i]) + ICON_SPELLS
		}
		drawSmallIcon(icon, spellX[i], spellY, 0x12)
	}
}

var panelflipInited bool

// initPanelflip prepares a panel turn: the new panel drawn with its contents, and the
// edge picture, into the turn's buffers.
func initPanelflip(panel, x, y, w, h int) {
	flipLeft, flipTop, pnlWide, flipHigh = x, y, w, h
	if !panelflipInited {
		for i := range flipBuf {
			if flipBuf[i] == nil {
				flipBuf[i] = make([]byte, 0x4000)
			}
		}
		flipFlags |= 1
		panelflipInited = true
	}
	if flipFlags&1 != 0 {
		pic, ok := readGrFar("panels", panel)
		edge, ok2 := readGrFar("panels", PANEL_EDGE)
		if !ok || !ok2 {
			pfatalCode(ERR_READ | 0x00E)
		}
		copy(flipBuf[1], pic)
		copy(flipBuf[0][0x2800:], edge)
		mouseHide()
		gr.GrSoftPageFlip()
		gr.Show(0xEC, 0xC0, flipBuf[1], 0x72, 0x53, 0, 0)
		old := RightPanel
		RightPanel = panel
		invRefresh = false
		panelDispatch[panel]()
		RightPanel = old
		invRefresh = true
		gr.Grab(flipBuf[1], 0xEC, 0xC0, 0x53, 0x72)
		gr.GrSoftPageFlip()
		mouseShow()
	}
	flipPage = panel
}

// prettyPanelagain redraws the right-hand panel in full.
func prettyPanelagain() {
	if RightPanel > PANEL_STATS {
		return
	}
	pic, ok := readGrFar("panels", RightPanel)
	if !ok {
		return
	}
	mouseHide()
	gr.GrSoftPageFlip()
	gr.Show(0xEC, 0xC0, pic, 0x72, 0x53, 0, 0)
	panelDispatch[RightPanel]()
	gr.GrSoftPageFlip()
	gr.SetTheColor(0x106)
	gr.Rectangle(0xEC, 0xC0, 0x13E, 0x4F)
	panelCorners()
	mouseShow()
}

func freePanelflip() {
	for i := range flipBuf {
		flipBuf[i] = nil
	}
}

// doPanelFrame draws one frame of the panel turn: the old panel squeezed narrower over
// frames 1 to 3, the edge at 4, the new panel widening over 5 to 7, in place at 8.
// It returns whether the turn is done.
func doPanelFrame() bool {
	pbufFrame++
	if flipFlags&1 == 0 {
		switch pbufFrame {
		case 3:
			src, _ := readGrFar("panels", PANEL_EDGE)
			mouseHide()
			gr.SetTheColor(0xF1)
			gr.Rectangle(0xEC, 0xC0, 0x13F, 0x4E)
			gr.Show(0x114, 0xC3, src, 0x78, 3, 0, 0)
			mouseShow()
		case 6:
			src, _ := readGrFar("panels", flipPage)
			mouseHide()
			gr.SetTheColor(0xF1)
			gr.Rectangle(0x114, 0xC3, 0x117, 0x4B)
			old := RightPanel
			RightPanel = flipPage
			gr.GrSoftPageFlip()
			gr.Show(0xEC, 0xC0, src, 0x72, 0x53, 0, 0)
			panelDispatch[flipPage]()
			gr.GrSoftPageFlip()
			RightPanel = old
			gr.SetTheColor(0x106)
			gr.Rectangle(0xEC, 0xC0, 0x13E, 0x4F)
			mouseShow()
		case 8:
			pbufFrame = 0
		}
		return pbufFrame == 0
	}
	mouseHide()
	dst := flipBuf[2]
	sides := func() {
		gr.SetTheColor(0xF1)
		gr.Rectangle(flipLeft, flipTop+(flipCh-flipHigh)/2,
			flipLeft+(pnlWide-flipWidthNow)/2, flipTop-(flipCh+flipHigh)/2)
		gr.Rectangle(flipLeft+(pnlWide+flipWidthNow)/2, flipTop+(flipCh-flipHigh)/2,
			flipLeft+pnlWide, flipTop-(flipCh+flipHigh)/2)
	}
	ends := func() {
		gr.SetTheColor(0xF1)
		gr.Rectangle(flipLeft, flipTop+(flipCh-flipHigh)/2, flipLeft+pnlWide, flipTop)
		gr.Rectangle(flipLeft, flipTop-flipHigh, flipLeft+pnlWide, flipTop-(flipCh+flipHigh)/2)
	}
	showDst := func() {
		gr.Show(flipLeft+(pnlWide-flipWidthNow)/2, flipTop+(flipCh-flipHigh)/2, dst, flipCh, flipWidthNow, 0, 0)
	}
	switch pbufFrame {
	case 1:
		gr.Grab(flipBuf[0], flipLeft, flipTop, pnlWide, flipHigh)
		flipScale(flipBuf[0], dst, pbufFrame)
		sides()
		showDst()
	case 2, 3:
		flipScale(flipBuf[0], dst, pbufFrame)
		sides()
		showDst()
	case 4:
		gr.SetTheColor(0xF1)
		gr.Rectangle(flipLeft+(pnlWide-flipWidthNow)/2, flipTop+(flipCh-flipHigh)/2,
			flipLeft+(pnlWide+flipWidthNow)/2, flipTop-(flipCh+flipHigh)/2)
		gr.Show(flipLeft+(pnlWide-3)/2, flipTop+(0x78-flipHigh)/2, flipBuf[0][0x2800:], 0x78, 3, 0, 0)
	case 5, 6, 7:
		ends()
		flipScale(flipBuf[1], dst, pbufFrame)
		showDst()
	case 8:
		ends()
		gr.Show(flipLeft, flipTop, flipBuf[1], flipHigh, pnlWide, 0, 0)
		pbufFrame = 0
	}
	panelTurnKey(pbufFrame == 0)
	mouseShow()
	return pbufFrame == 0
}

// flipScale draws the panel picture src squeezed into dst for turn step frame, skewed
// so that the nearer edge grows, one column at a time.
func flipScale(src, dst []byte, frame int) {
	flipWidthNow = pnlWide * flipWpct[frame] / 100
	tall := flipHigh * flipHpct[frame] / 100
	step := 100 / flipWpct[frame]
	var runs, per int
	if step == 1 {
		runs = pnlWide - flipWidthNow
		per = pnlWide/(runs+1) - 1
	} else {
		runs = flipWidthNow
		per = 1
	}
	s, d := 0, 0
	if frame < 4 {
		flipCh = tall
		d1 := (flipHigh - tall) << 1
		d2 := (flipHigh - tall + flipWidthNow) << 1
		err := (flipHigh-tall)<<1 + flipWidthNow
		pnlSkew = 0
		for k := 0; k < runs; k++ {
			for j := 0; j < per; j++ {
				if err > 0 {
					err += d1
				} else {
					pnlSkew++
					flipCh -= 2
					err += d2
				}
				flipColumn(src, s, dst, d)
				s++
				d++
			}
			s += step
		}
	} else {
		flipCh = flipHigh<<1 - tall
		pnlSkew = tall - flipHigh
		d1 := (tall - flipHigh) << 1
		d2 := (tall - flipHigh - flipWidthNow) << 1
		err := (tall-flipHigh)<<1 - flipWidthNow
		for k := 0; k < runs; k++ {
			for j := 0; j < per; j++ {
				if err < 0 {
					err += d1
				} else {
					pnlSkew--
					flipCh += 2
					err += d2
				}
				flipColumn(src, s, dst, d)
				s++
				d++
			}
			s += step
		}
	}
	for rest := flipWidthNow - runs*per; rest > 0; rest-- {
		flipColumn(src, s, dst, d)
		s++
		d++
	}
	flipCh = tall
}

// flipColumn copies one column of the panel picture into dst at height flipCh, pnlSkew
// blank pixels above and below, dropping or doubling rows evenly. Out-of-buffer
// accesses (which the original makes into EMS) are ignored.
func flipColumn(src []byte, s int, dst []byte, d int) {
	put := func(v byte) {
		if d >= 0 && d < len(dst) {
			dst[d] = v
		}
		d += flipWidthNow
	}
	get := func() byte {
		if s >= 0 && s < len(src) {
			return src[s]
		}
		return 0
	}
	diff := flipCh - flipHigh
	for i := 0; i < pnlSkew; i++ {
		put(0)
	}
	var rest int
	switch {
	case diff == 0:
		for i := 0; i < flipHigh; i++ {
			put(get())
			s += pnlWide
		}
	case diff > 0:
		n := flipHigh / (diff + 1)
		for i := 0; i < diff; i++ {
			for j := 0; j < n; j++ {
				put(get())
				s += pnlWide
			}
			put(get())
		}
		rest = flipHigh - diff*n
	default:
		n := flipHigh/(diff-1) + 1
		for i := 0; i > diff; i-- {
			for j := 0; j > n; j-- {
				put(get())
				s += pnlWide
			}
			s += pnlWide
		}
		rest = flipCh - diff*n - 1
	}
	for ; rest > 0; rest-- {
		put(get())
		s += pnlWide
	}
	for i := 0; i < pnlSkew; i++ {
		put(0)
	}
	if d >= 0 && d < len(dst) {
		dst[d] = 0
	}
}

// doFbufBms draws the first-person weapon into the frame buffer (unless sheathed or
// turned off), swaying with the player's speed. WEAPONS.DAT places it in the original's
// view, 172 wide; weapShift keeps it as far from the full-width view's middle.
func doFbufBms() {
	weapShift := (gr.FBW - 0xAC) / 2
	gr.Transparency = true
	if setting[SCR_WEAPON] != WEAP_SHEATHED && weapFrame < 0x1C && lastWeap > -1 && ShowStupidFirstPersonWeapon {
		jig := 0
		if PN.Speed != 0 {
			jig = int(PN.Speed)*2/0x31F + 1
		}
		jiggleWeapon(jig)
		var n int
		switch {
		case setting[SCR_WEAPON] == WEAP_DRAWING || setting[SCR_WEAPON] == WEAP_SHEATHING:
			n = weapFrame + 0x12
		case weapFrame < 0 || setting[SCR_WEAPON] == WEAP_READY:
			n = 0x1C - wfo
		default:
			n = int(setting[8])*9 + weapFrame
			wxo = 0
		}
		if n >= 0 && n < len(weapPics) {
			if bm := weapPics[n]; len(bm) >= 3 {
				gr.FBShow(grsUnpack(bm), int(weapX[n])+wxo+weapShift, int(weapY[n]), int(bm[1]), int(bm[2]))
			}
		}
	}
	gr.Transparency = false
}

// sendFB copies the 3D frame buffer to the screen, after drawing the weapon in demo mode.
func sendFB() {
	if demoMode {
		doFbufBms()
	}
	gr.FBToScreen()
}

// playerLookShaft records the level in cutscene value 0x100 (LOOK.C).
func playerLookShaft() { valueCuts(0x100, PlayerLevel) }

// playerLookGrave records gravestone grave in cutscene value 0x101.
func playerLookGrave(grave int) {
	if grave != 0 {
		valueCuts(0x101, grave)
	}
}
