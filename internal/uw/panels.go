package uw

import "github.com/felix/uw1go/internal/gr"

// Port of PANELS.C: the screen furniture around the 3D view: the vitality and mana
// flasks, the compass, the power gem, the dragons, the eyes, the first-person weapon, the
// rune shelf and active spell icons, the turning right-hand panel and the frame buffer
// send. setScreenFrame(which, value) sets an element's goal; updateScreen, each frame,
// steps each element towards its goal through its adjust function: the 'now' ones at
// once, the fast ones every 32 ticks and the slow ones every 64.

var (
	flipCh, flipLeft, flipHigh, flipTop, flipPage int
	weapX, weapY                                  [0x1C]byte
	setting, goal                                 [9]int8
	flipWidthNow, pnlWide, pnlSkew                int

	// RightPanel is the panel showing on the right (PANEL_*, PANEL_TURNING while turning).
	RightPanel                         int
	slowAdjust, fastAdjust, nowAdjust  int
	dstate, dspr                       [2]int
	weapid, lastWeap                   = -1, -1
	wlstate                            = WEAP_SHEATHED
	weapFrame                          = -1
	wxo, wfo                           = 0, 1
	flX                                = [2]int{0xF8, 0x11C}
	liquidY                            = [14]int{0x2C, 0x30, 0x32, 0x34, 0x36, 0x38, 0x3A, 0x3B, 0x3C, 0x3D, 0x3F, 0x41, 0x43, 0x45}
	liquidH                            = [14]int{0, 4, 5, 6, 7, 7, 7, 7, 6, 5, 4, 4, 4, 4}
	bubbling, swishing                 [2]byte
	nedlX                              = [16]int{0x88, 0x80, 0x78, 0x74, 0x70, 0x70, 0x74, 0x7C, 0x88, 0x90, 0x9C, 0xA0, 0xA0, 0x9C, 0x98, 0x90}
	nedlY                              = [16]int{0x44, 0x42, 0x41, 0x3E, 0x3A, 0x36, 0x34, 0x31, 0x2F, 0x31, 0x34, 0x36, 0x3A, 0x3E, 0x41, 0x42}
	dpart1X                            = [2]int{0x24, 0xE4}
	dpart2X                            = [2]int{0x24, 0xCC}
	danimX                             = [2][3]int{{0x28, 0x30, 0x24}, {0xCC, 0xCC, 0xC8}}
	danimY                             = [2][3]int{{0x2C, 0x36, 0x36}, {0x2C, 0x36, 0x36}}
	danimW                             = [2][3]int{{0x21, 0x18, 0x25}, {0x22, 0x18, 0x26}}
	danimH                             = [2][3]int{{0x0E, 0x10, 0x17}, {0x0E, 0x10, 0x17}}
	dtailX                             = [2]int{0x28, 0xE0}
	dtailSeq                           = [2][7]int{
		{ICON_DRAGONS + 0xE, ICON_DRAGONS + 0xF, ICON_DRAGONS + 0x10, ICON_DRAGONS + 0x11, ICON_DRAGONS + 0x10, ICON_DRAGONS + 0xF, ICON_DRAGONS + 0xE},
		{ICON_DRAGONS + 0x20, ICON_DRAGONS + 0x21, ICON_DRAGONS + 0x22, ICON_DRAGONS + 0x23, ICON_DRAGONS + 0x22, ICON_DRAGONS + 0x21, ICON_DRAGONS + 0x20},
	}
	dpart1Pic = [2]int{ICON_DRAGONS, ICON_DRAGONS + 0x12}
	dpart2Pic = [2]int{ICON_DRAGONS + 1, ICON_DRAGONS + 0x13}
	flipHpct  = [8]int{0x64, 0x67, 0x69, 0x6A, 0x6A, 0x6A, 0x69, 0x67}
	flipWpct  = [8]int{0x64, 0x5C, 0x47, 0x26, 0x00, 0x26, 0x47, 0x5C}
	// The turning panel's pictures (EMS handles in the original): 0 the old panel and,
	// at 0x2800, the edge; 1 the new panel; 2 the work buffer.
	flipBuf                         [3][]byte
	pbufFrame                       int
	pbufFlag                        bool
	flipFlags                       = 1
	eyesSeq                         = [5]int{ICON_EYES + 1, ICON_EYES + 2, ICON_EYES + 3, ICON_EYES + 2, ICON_EYES + 1}
	eyes, eyesFrame                 int
	runeX                           = [3]int{0xB0, 0xBF, 0xCE}
	spellX                          = [3]int{0x56, 0x45, 0x34}
	panelDispatch                   = [3]func(){RedispInv, RedispRune, RedispStat}
	level                           [4]int
	dpart1Spr, dpart2Spr, dtailSpr  [2]int
	weapPics                        [0x1C][]byte // the weapon's pictures (EMS in the original)
	ShowStupidFirstPersonWeapon     = true
	demoMode                        bool
)

func adjustElement(i int) {
	switch i {
	case 0, 1:
		adjustFlasks(i)
	case 2:
		adjustCompass()
	case 3:
		adjustPower()
	case 4, 5:
		adjustDragons(i)
	case 6:
		adjustPanel()
	case 7:
		adjustEyes()
	case 8:
		adjustWeapon()
	}
}

// setFlask draws flask which (0 vitality, 1 mana) at its goal level at once.
func setFlask(which int) {
	var frame int
	switch which {
	case 0:
		frame = ICON_FLASKS
		if player.Poison() != 0 {
			frame = ICON_FLASKS + 0x32
		}
	case 1:
		frame = ICON_FLASKS + 0x19
	default:
		return
	}
	picToScreen(ICON_FLASKS+0x4B, flX[which], 0x4A)
	for i := 0; i < int(goal[which]); i++ {
		moveSprite(level[which], flX[which], liquidY[i+1])
		drawSprite(level[which], frame+i)
		updateSprites()
	}
	setting[which] = goal[which]
}

// setCompass draws the compass at its goal heading at once.
func setCompass() {
	dir := int(goal[SCR_COMPASS])
	drawSprite(level[2], dir&3+ICON_COMPASS)
	moveSprite(level[3], nedlX[dir], nedlY[dir])
	drawSprite(level[3], dir+ICON_COMPASS+4)
	updateSprites()
}

// resetScrgr resets every display element.
func resetScrgr() {
	for i := range setting {
		setting[i], goal[i] = 0, 0
	}
	eraseSprite(dspr[0])
	eraseSprite(dspr[1])
	dstate = [2]int{}
	RightPanel = PANEL_INV
	eyesFrame = 0
	slowAdjust &^= 1 << SCR_EYES
	goal[SCR_EYES] = 4
	setting[SCR_WEAPON], goal[SCR_WEAPON] = WEAP_SHEATHED, WEAP_SHEATHED
	wlstate = WEAP_SHEATHED
	weapFrame = 0
	updateSprites()
}

var scrgrInited bool

// initScrgr creates the sprites once, then draws the whole frame.
func initScrgr() {
	if !scrgrInited {
		for i := 0; i < 2; i++ {
			level[i] = createSprite(0)
			changeSprite(level[i], 0, 0, 0x18, 4)
			dpart1Spr[i] = createSprite(2, 0xD, 0xA)
			changeSprite(dpart1Spr[i], dpart1X[i], 0x41, 0xD, 0xA)
			gr.Transparency = true
			dpart2Spr[i] = createSprite(2, 0x25, 0x17)
			gr.Transparency = false
			changeSprite(dpart2Spr[i], dpart2X[i], 0x36, 0x25, 0x17)
			dtailSpr[i] = createSprite(0)
			changeSprite(dtailSpr[i], dtailX[i], 0x86, 0xC, 0x1C)
			goal[i+4], setting[i+4] = 0, 0
		}
		level[2] = createSprite(0)
		changeSprite(level[2], 0x70, 0x44, 0x38, 0x20)
		level[3] = createSprite(0)
		changeSprite(level[3], nedlX[0], nedlY[0], 3, 4)
		eyes = createSprite(0)
		changeSprite(eyes, 0x80, 0xC3, 1, 1)
		goal[SCR_WEAPON], setting[SCR_WEAPON] = WEAP_SHEATHED, WEAP_SHEATHED
		scrgrInited = true
	}
	gr.GrSoftPageFlip()
	gr.CopyVisibleToHidden()
	for i := 0; i < 2; i++ {
		setFlask(i)
		drawSprite(dpart1Spr[i], dpart1Pic[i])
		drawSprite(dpart2Spr[i], dpart2Pic[i])
		drawSprite(dtailSpr[i], 0x12*i+ICON_DRAGONS+0xE)
	}
	setCompass()
	drawSprite(eyes, ICON_EYES)
	setRunes(player.Shelf[:])
	if pic, ok := readGrFar("panels", RightPanel); ok {
		gr.Show(0xEC, 0xC0, pic, 0x72, 0x53, 0, 0)
	}
	panelDispatch[RightPanel]()
	updateSprites()
	gr.GrPageFlip()
	gr.GrSoftPageFlip()
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
	case SCR_DRAGON:
		v := int8(val)
		if goal[4] == v || goal[5] == v || setting[4] == v || setting[5] == v {
			break
		}
		if setting[4] == 0 && setting[5] == 0 {
			which += rand() & 1
		} else if setting[4] != 0 {
			if setting[5] == 0 {
				which++
			} else if goal[4] == 0 && goal[5] == 0 {
				which += rand() & 1
			} else if goal[4] != 0 && goal[5] == 0 {
				which++
			}
		}
		goal[which] = v
		slowAdjust |= 1 << which
	case SCR_WEAPON:
		if weapid != lastWeap && goal[SCR_WEAPON] == WEAP_SHEATHED {
			break
		}
		fastAdjust |= 1 << which
		goal[SCR_WEAPON] = int8(val)
	case SCR_PANEL:
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
		if r := rand(); r < 0x666 && swishing[r&1] == 0 {
			swishing[r&1] = 1
			slowAdjust |= 1 << ((r + 4) & 1)
		}
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

var (
	bubFrame                  = [2]int{ICON_FLASKS + 0xD, ICON_FLASKS + 0x26}
	bubSpr, drainSpr, maskSpr [2]int
)

// adjustFlasks moves flask f one step towards its goal and animates its bubbles.
func adjustFlasks(f int) {
	var base, bubFirst, bubLast int
	switch f {
	case 0:
		if player.Poison() != 0 {
			base, bubFirst, bubLast = ICON_FLASKS+0x32, ICON_FLASKS+0x3F, ICON_FLASKS+0x4A
			if bubFrame[0] >= ICON_FLASKS+0xD && bubFrame[0] <= ICON_FLASKS+0x18 {
				setFlask(0)
				bubFrame[0] += 0x32
			}
		} else {
			base, bubFirst, bubLast = ICON_FLASKS, ICON_FLASKS+0xD, ICON_FLASKS+0x18
			if bubFrame[0] >= ICON_FLASKS+0x3F && bubFrame[0] <= ICON_FLASKS+0x4A {
				setFlask(0)
				bubFrame[0] -= 0x32
			}
		}
	case 1:
		base, bubFirst, bubLast = ICON_FLASKS+0x19, ICON_FLASKS+0x26, ICON_FLASKS+0x31
	default:
		return
	}
	if drainSpr[f] == 0 {
		drainSpr[f] = createSprite(0)
		changeSprite(drainSpr[f], flX[f], 0x4A, 0x18, 0x21)
		bubSpr[f] = createSprite(0)
		changeSprite(bubSpr[f], 0, 0, 0x18, 4)
		maskSpr[f] = createSprite(0)
		changeSprite(maskSpr[f], flX[f], 0x4A, 0x18, 4)
	}
	var n int
	diff := goal[f] - setting[f]
	switch {
	case diff > 0:
		setting[f]++
		n = int(setting[f])
		moveSprite(level[f], flX[f], liquidY[n])
		drawSprite(level[f], base+n-1)
	case diff < 0:
		setting[f]--
		n = int(setting[f])
		y := liquidY[n+1]
		changeSprite(drainSpr[f], flX[f], y, 0x18, liquidH[n+1])
		setYoff(drainSpr[f], 0x4A-y)
		drawSprite(drainSpr[f], ICON_FLASKS+0x4B)
		if n > 0 {
			moveSprite(level[f], flX[f], liquidY[n])
			drawSprite(level[f], base+n-1)
		}
	default:
		if bubbling[f] == 0 {
			slowAdjust &^= 1 << f
		}
		n = int(setting[f])
	}
	if bubbling[f] == 1 && n < 8 && n > 0 {
		if bubFrame[f] == bubLast {
			bubFrame[f] = bubFirst
			drawSprite(level[f], base+n-1)
			bubbling[f] = 0
		} else {
			moveSprite(bubSpr[f], flX[f], liquidY[n])
			bubFrame[f]++
			drawSprite(bubSpr[f], bubFrame[f])
			setYoff(maskSpr[f], 0x4A-liquidY[n])
			moveSprite(maskSpr[f], flX[f], liquidY[n])
			drawMask(maskSpr[f], ICON_FLASKS+0x4C)
		}
	}
}

var (
	dragFrame  [2]int
	dragTail   int
	dragCount  [2]int
	dragFirst  = [2][3]int{{ICON_DRAGONS + 2, ICON_DRAGONS + 6, ICON_DRAGONS + 0xA}, {ICON_DRAGONS + 0x14, ICON_DRAGONS + 0x18, ICON_DRAGONS + 0x1C}}
	dragLast   = [2][3]int{{ICON_DRAGONS + 5, ICON_DRAGONS + 9, ICON_DRAGONS + 0xD}, {ICON_DRAGONS + 0x17, ICON_DRAGONS + 0x1B, ICON_DRAGONS + 0x1F}}
)

// adjustDragons animates dragon which-4: its tail when swishing, and the animation its
// setting names (1 to 3), in states 1 (start) to 5 (end).
func adjustDragons(which int) {
	if which != 4 && which != 5 {
		return
	}
	d := which - 4
	if dspr[d] == 0 {
		gr.Transparency = true
		dspr[d] = createSprite(3, 0x28, 0x18)
		gr.Transparency = false
	}
	if swishing[d] == 1 {
		drawSprite(dtailSpr[d], dtailSeq[d][dragTail])
		if dragTail++; dragTail >= 7 {
			dragTail = 0
			swishing[d] = 0
		}
	}
	if dstate[d] == 0 && goal[d+4] != 0 {
		setting[d+4] = goal[d+4]
		dstate[d] = 1
	}
	a := int(setting[d+4]) - 1
	start := func(count, state int) {
		goal[d+4] = 0
		changeSprite(dspr[d], danimX[d][a], danimY[d][a], danimW[d][a], danimH[d][a])
		dragFrame[d] = dragFirst[d][a]
		dragCount[d] = count
		dstate[d] = state
	}
	end := func() {
		eraseSprite(dspr[d])
		setting[d+4] = 0
		dstate[d] = 0
	}
	switch setting[d+4] {
	case 0:
		if swishing[d] == 0 {
			slowAdjust &^= 1 << which
		}
	case 1:
		switch dstate[d] {
		case 1:
			start(1, 3)
			fallthrough
		case 3:
			drawSprite(dspr[d], dragFrame[d])
			dragFrame[d]++
			if dragLast[d][a] < dragFrame[d] {
				dragFrame[d] = dragFirst[d][a]
				dragCount[d]--
			}
			if goal[d+4] != 0 || dragCount[d] == 0 {
				dstate[d] = 4
			}
		case 4:
			drawSprite(dspr[d], dragFrame[d])
			dragFrame[d]++
			if dragLast[d][a] < dragFrame[d] {
				dstate[d] = 5
			}
		case 5:
			end()
		}
	case 2:
		switch dstate[d] {
		case 1:
			start(3, 2)
			fallthrough
		case 2:
			drawSprite(dspr[d], dragFrame[d])
			dragFrame[d]++
			if dragFirst[d][a]+1 < dragFrame[d] {
				dstate[d] = 3
			}
		case 3:
			drawSprite(dspr[d], dragFrame[d])
			dragFrame[d]++
			if dragLast[d][a] < dragFrame[d] {
				dragFrame[d] = dragFirst[d][a] + 2
				dragCount[d]--
			}
			if goal[d+4] != 0 || dragCount[d] == 0 {
				dstate[d] = 4
			}
		case 4:
			drawSprite(dspr[d], dragFrame[d])
			dragFrame[d]--
			if dragFirst[d][a] > dragFrame[d] {
				dstate[d] = 5
			}
		case 5:
			end()
		}
	case 3:
		switch dstate[d] {
		case 1:
			eraseSprite(dpart2Spr[d])
			start(6, 2)
			fallthrough
		case 2:
			drawSprite(dspr[d], dragFrame[d])
			dragFrame[d]++
			if dragLast[d][a] < dragFrame[d] {
				dragFrame[d]--
				dstate[d] = 3
			}
		case 3:
			if goal[d+4] != 0 {
				dstate[d] = 4
			} else if dragCount[d]--; dragCount[d] == 0 {
				dstate[d] = 4
			}
		case 4:
			dragFrame[d]--
			drawSprite(dspr[d], dragFrame[d])
			if dragFirst[d][a] == dragFrame[d] {
				dstate[d] = 5
			}
		case 5:
			eraseSprite(dspr[d])
			drawSprite(dpart2Spr[d], dpart2Pic[d])
			setting[d+4] = 0
			dstate[d] = 0
		}
	}
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
	drawSprite(level[2], cur&3+ICON_COMPASS)
	moveSprite(level[3], nedlX[cur], nedlY[cur])
	drawSprite(level[3], cur+ICON_COMPASS+4)
	setting[SCR_COMPASS] = int8(cur)
}

var powSpr, powLast, powFrame = 0, 0, 9

// adjustPower shows the power gem: frame p, or 9 pulsing through frames 9 to 13.
func adjustPower() {
	p := int(goal[SCR_POWER])
	if p < 0 || p > 13 {
		return
	}
	if powSpr == 0 {
		powSpr = createSprite(0)
		changeSprite(powSpr, 4, 0x3C, 1, 1)
	}
	if p == 9 {
		drawSprite(powSpr, powFrame+ICON_POWER)
		if powFrame++; powFrame > 13 {
			powFrame = 9
		}
		slowAdjust |= 1 << SCR_POWER
	} else {
		if powLast == 9 {
			powFrame = 9
		}
		drawSprite(powSpr, p+ICON_POWER)
		slowAdjust &^= 1 << SCR_POWER
	}
	powLast = p
}

// adjustPanel turns the right-hand panel over a frame at a time.
func adjustPanel() {
	if setting[SCR_PANEL] != goal[SCR_PANEL] {
		if !pbufFlag {
			pbufFlag = true
			initPanelflip(int(goal[SCR_PANEL]), 0xEC, 0xC0, 0x53, 0x72)
			RightPanel = PANEL_TURNING
		}
		if doPanelFrame() {
			setting[SCR_PANEL] = goal[SCR_PANEL]
			RightPanel = int(goal[SCR_PANEL])
			pbufFlag = false
			slowAdjust &^= 1 << SCR_PANEL
		}
	}
}

var eyesWait byte

// adjustEyes animates the gargoyle's eyes: a five-frame blink, the set by the goal.
func adjustEyes() {
	if setting[SCR_EYES] == goal[SCR_EYES] {
		goal[SCR_EYES] += 4
		eyesFrame = 2
		eyesWait = 0
	} else if setting[SCR_EYES] != goal[SCR_EYES]-4 {
		if eyesWait != 0 {
			eyesFrame = 2
			eyesWait = 0
		}
		setting[SCR_EYES] = goal[SCR_EYES]
		goal[SCR_EYES] += 4
	}
	if eyesFrame == 3 && eyesWait < 0x10 {
		eyesWait++
	} else {
		drawSprite(eyes, eyesSeq[eyesFrame]+(int(setting[SCR_EYES])-1)*3)
		eyesFrame++
	}
	if eyesFrame > 5 {
		setting[SCR_EYES] = 0
		eyesFrame, eyesWait = 0, 0
		drawSprite(eyes, ICON_EYES)
		slowAdjust &^= 1 << SCR_EYES
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

var runeSpr, spellSpr [3]int

// setRunes shows the three runes on the shelf (0x18 and up leaves a place empty).
func setRunes(runes []byte) {
	if runeSpr[0] == 0 {
		gr.Transparency = true
		for i := 0; i < 3; i++ {
			runeSpr[i] = createSprite(1, 0x10, 0x10)
			changeSprite(runeSpr[i], runeX[i], 0x3D, 0x10, 0x10)
		}
		gr.Transparency = false
	}
	for i := 0; i < 3; i++ {
		if runes[i] < 0x18 {
			drawSprite(runeSpr[i], int(runes[i])+0xE8)
		} else {
			eraseSprite(runeSpr[i])
		}
	}
	updateSprites()
}

// activeSpells shows the icons of up to three active spells in the 3D view mode.
func activeSpells(spells []byte) {
	if inplist.mode != MODE_GAME {
		return
	}
	if spellSpr[0] == 0 {
		gr.Transparency = true
		for i := 0; i < 3; i++ {
			spellSpr[i] = createSprite(1, 0x10, 0x12)
			changeSprite(spellSpr[i], spellX[i], 0x3F, 0x10, 0x12)
		}
		gr.Transparency = false
	}
	for i := 0; i < 3; i++ {
		if spells[i] < 0x15 {
			drawSprite(spellSpr[i], int(spells[i])+ICON_SPELLS)
		} else {
			eraseSprite(spellSpr[i])
		}
	}
	updateSprites()
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
			picToScreen(ICON_CHAINS+0xC, 0x110, 0xC4)
			picToScreen(ICON_CHAINS+4, 0x110, 0x4E)
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
			picToScreen(ICON_CHAINS+8, 0x110, 0xC4)
			picToScreen(ICON_CHAINS, 0x110, 0x4E)
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
	picToScreen(pbufFrame+ICON_CHAINS+8, 0x110, 0xC4)
	picToScreen(pbufFrame+ICON_CHAINS, 0x110, 0x4E)
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
// turned off), swaying with the player's speed, and the view's three frame pictures.
func doFbufBms() {
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
				gr.FBShow(grsUnpack(bm), int(weapX[n])+wxo, int(weapY[n]), int(bm[1]), int(bm[2]))
			}
		}
	}
	picToFbuf(ICON_3DWIN, 0x3E, 3)
	picToFbuf(ICON_3DWIN+1, 0, 0xD)
	picToFbuf(ICON_3DWIN+2, 0xAB, 0xD)
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
