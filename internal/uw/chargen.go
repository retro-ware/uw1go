package uw

import (
	"strconv"

	"github.com/felix/uw1go/internal/gr"
)

// Port of CHARGEN.C: character creation. A new player's starting record (initChar), the
// chargen screens (sex, hand, class, skills, portrait, difficulty, name, confirmation)
// drawn from CHRBTNS.GR on DATA\CHARGEN.BYT, with DATA\SKILLS.DAT and DATA\CHRGEN.DAT.

// ChrOpt is one chargen question from DATA\chrgen.dat (0x12 bytes in the file).
type ChrOpt struct {
	question int
	name     *[]byte  // the name typed, for the name question
	strings  []uint16 // the answers' strings, 0-terminated (a window on chrStrings)
	count    int
	pic      int
	rows     int
	cols     int
	spacing  int
}

var (
	sknow    int
	chrPics  [][]byte // the button pictures with their headers (chrbuf + chroff[n] - 5)
	chrBg    []byte   // DATA\CHARGEN.BYT, cuts block 1
	classTab []byte   // stdat: DATA\SKILLS.DAT
)

// chrpic is chrbuf + chroff[n]: picture n's pixels; chrw and chrh are pic[-4], pic[-3].
func chrpic(n int) []byte {
	if n < 0 || n >= len(chrPics) || len(chrPics[n]) < 5 {
		return nil
	}
	return chrPics[n][5:]
}
func chrw(n int) int { return int(chrPics[n][1]) }
func chrh(n int) int { return int(chrPics[n][2]) }

// initChar sets up a new character's record; blank leaves skills and attributes at 0.
func initChar(blank bool) {
	p := player
	p.SetLefty(1)
	p.Exp = 0
	p.SkillPoints = 1
	p.SkillPointsEarned = 0
	p.Level = 1
	p.GameClock = 0x10B3000
	p.SetMoonstone(2)
	p.SetTree(0)
	p.SetPoison(0)
	p.SetActiveSpells(0)
	p.SetNRunes(0)
	p.SetArmageddon(0)
	p.SetOrb(0)
	p.SetCup(0)
	p.SetKey(0)
	p.SetTalismanOK(0)
	p.SetGaramon(0)
	p.SetMaze(0)
	p.SetIncense(0)
	p.SetDrunk(0)
	p.SetShrooms(0)
	p.SetDetail(3)
	setGraphicsLevel()
	p.Talismans = 8
	p.Quests = 0
	p.Dreams = 0
	p.SetFPS(0)
	p.MotionState = 0
	p.SwimCount = 0
	p.Fatigue = 0x40
	p.FoodHeal = 0x40
	p.B3C = 0
	p.QuestBytes = [4]byte{}
	p.Shelf = [3]byte{0x18, 0x18, 0x18}
	p.Runebag = [3]byte{}
	p.GameVars = [0x40]byte{}
	p.Lore = [8]byte{}
	p.GameVars[0x1A] = 0x35
	p.Hunger = 0xC0
	p.SetBody(rand() % 5)
	p.SetFemale(rand() & 1)
	for i := 0; i < NUM_SKILLS; i++ {
		if blank {
			p.Skills[i] = 0
		} else {
			p.Skills[i] = byte(rollem(3, 4))
		}
	}
	for i := 0; i < 3; i++ {
		if blank {
			playerdat.Attr[i] = 0
		} else {
			playerdat.Attr[i] = byte(rollem(2, 10) + 10)
		}
	}
	playerCompute(true)
	p.Weight = 0
	ThePlayer.HP = byte(int(playerdat.Avghit) - 6 - rand()%6)
	PlayerLevel = 1
	FixPlayerEquips()
}

// setSklmnu walks the class's five skill entries from *idx; true when it built a menu.
func setSklmnu(idx *int, skills []byte, opt *ChrOpt, dat []byte) bool {
	cls := player.PClass()
	pos := 0
	for n := 0; cls*5+*idx > n; n++ {
		pos += int(dat[pos]) + 1
	}
	for ; *idx < 5; *idx++ {
		if dat[pos] == 0 {
			skills[*idx] = NUM_SKILLS
		} else if dat[pos] == 1 {
			skills[*idx] = dat[pos+1]
			pos += int(dat[pos]) + 1
		} else {
			opt.count = int(dat[pos])
			for i := 0; int(dat[pos]) > i; i++ {
				opt.strings[i] = uint16(dat[pos+i+1]) + 0x1F
			}
			*idx++
			return true
		}
	}
	return false
}

func showAtts() {
	gr.SetTheColor(0x106)
	gr.Rectangle(0x5D, 0x96, 0x8C, 0x4E)
	row := func(label string, v, y int) {
		buf := strconv.Itoa(v)
		gr.StringToScreen(label, 0x5D, y)
		gr.StringToScreen(buf, 0x8C-gr.StringWidth(buf), y)
	}
	row("Str:", int(playerdat.Attr[ATTR_STR]), 0x96)
	row("Dex:", int(playerdat.Attr[ATTR_DEX]), 0x84)
	row("Int:", int(playerdat.Attr[ATTR_INT]), 0x72)
	row("Vit:", int(playerdat.Avghit), 0x60)
}

func showSkills() {
	gr.SetTheColor(0x106)
	gr.Rectangle(0x1E, 0x43, 0x7D, 0x0C)
	row := 0
	for i := 0; i < 20 && row <= 5; i++ {
		if player.Skills[i] != 0 {
			name := getString((i + STRN_SKILLS) | STR_CHARGEN)
			buf := strconv.Itoa(int(player.Skills[i]))
			gr.StringToScreen(name, 0x1E, 0x43-row*11)
			gr.StringToScreen(buf, 0x7D-gr.StringWidth(buf), 0x43-row*11)
			row++
		}
	}
}

func setSkills(start int, skills []byte) int {
	for i := start; i < 6; i++ {
		if skills[i] < NUM_SKILLS {
			addToSkill(int(skills[i]))
			start++
		}
	}
	return start
}

// rollStats gives the class's attributes and spreads its bonus points.
func rollStats() {
	cls := player.PClass()
	for i := 0; i < 3; i++ {
		playerdat.Attr[i] = classTab[cls*4+i]
	}
	for i := 0; i < NUM_SKILLS; i++ {
		player.Skills[i] = 0
	}
	i := int(classTab[cls*4+3])
	for i > 0 {
		pts := rand()&3 + 1
		if pts > i {
			pts = i
		}
		a := rand() % 3
		if int(playerdat.Attr[a])+pts > 30 {
			pts = 30 - int(playerdat.Attr[a])
		}
		playerdat.Attr[a] += byte(pts)
		i -= pts
	}
	playerCompute(true)
	ThePlayer.HP = playerdat.Avghit
}

// drawopt draws a question and its answer buttons.
func drawopt(opt *ChrOpt) {
	var h, w, x, y int
	if opt.strings != nil {
		hasq := b2i(opt.question != 0)
		w = chrw(opt.pic)
		h = chrh(opt.pic)
		opt.cols = (opt.count*(h+4)-4)/(0xC4-hasq*20) + 1
		opt.rows = (opt.count + opt.cols - 1) / opt.cols
		opt.spacing = (0xA0 - opt.cols*w) / (opt.cols + 1)
		y = 0xC5 - (0xC4-(opt.rows+hasq)*(h+4)+4)/2
	} else {
		y = 0x6A
	}
	if opt.question != 0 {
		s := getString(opt.question | STR_CHARGEN)
		x = 0xA4
		if opt.name != nil {
			gr.Show(x, y, chrpic(6), 0x10, 0x91, 0, 0)
			x += 4
			gr.StringToScreen(s, x, y-3)
		} else {
			gr.StringToScreen(s, x+(0x91-gr.StringWidth(s))/2, y-3)
		}
	} else {
		y += h + 4
	}
	if opt.strings != nil {
		gr.Transparency = false
		for i := 0; opt.count > i; i++ {
			if i%opt.cols == 0 {
				x = 0xA0 - w
				y -= h + 4
			}
			x += opt.spacing + w
			gr.Show(x, y, chrpic(opt.pic), h, w, 0, 0)
			switch opt.pic {
			case 0:
				s := getString(int(opt.strings[i]) | STR_CHARGEN)
				gr.StringToScreen(s, x+(w-gr.StringWidth(s))/2, y-3)
			case 3:
				k := int(opt.strings[0]) + i
				gr.Transparency = true
				gr.Show(x, y, chrpic(k), h, w, 0, 0)
				gr.Transparency = false
			}
		}
	}
}

// selopt moves the highlight from answer old to answer new.
func selopt(opt *ChrOpt, newSel, old int) {
	hasq := 0
	if opt.question != 0 {
		hasq = -1
	}
	if newSel&0xFF == old&0xFF {
		return
	}
	sel := [2]int{old & 0xFF, newSel & 0xFF}
	w := chrw(opt.pic)
	h := chrh(opt.pic)
	xbase := opt.spacing + 0xA0
	ybase := 0xC5 - (0xC4-(opt.rows+hasq)*(h+4)+4)/2
	for j := 0; j < 2; j++ {
		if opt.count <= sel[j] {
			continue
		}
		y := ybase - sel[j]/opt.cols*(h+4)
		x := xbase + sel[j]%opt.cols*(w+opt.spacing)
		pic := chrpic(opt.pic + j + 1)
		mouseHide()
		gr.Transparency = true
		gr.Show(x, y, pic, h, w, 0, 0)
		mouseShow()
	}
	gr.Transparency = false
}

// mousopt follows the mouse while a button is held; the answer released on, or -1 - cur.
func mousopt(opt *ChrOpt, cur int) int {
	sel := cur
	q := opt.question
	w := chrw(opt.pic)
	h := chrh(opt.pic)
	wstep := w + opt.spacing
	hstep := h + 4
	xleft := opt.spacing + 0xA0
	ytop := 0xC5 - (0xC4-(opt.rows+b2i(q != 0))*(h+4)+4)/2
	if q != 0 {
		ytop -= hstep
	}
	for mouseGetInput() > 0 {
		if sel != cur && sel != -1 {
			mouseHide()
			selopt(opt, sel, cur)
			mouseShow()
			cur = sel
		}
		mx, my := mouseGetxy()
		sel = (ytop-my)/hstep*opt.cols + (mx-xleft)/wstep
		if sel < 0 || opt.count <= sel || my > ytop || mx < xleft {
			sel = -1
		} else {
			mx -= xleft + wstep*(sel%opt.cols)
			my = ytop - my - hstep*(sel/opt.cols)
			if mx >= w || my >= h {
				sel = -1
			}
		}
	}
	if sel == -1 {
		sel = -1 - cur
	}
	return sel
}

// pickopt gets one answer; -1 for Escape.
func pickopt(opt *ChrOpt) int {
	sel, old := 0, 0
	done := false
	empty := true
	if opt.question != 0 && opt.name != nil {
		name := *opt.name
		ln := 0
		s := getString(opt.question | STR_CHARGEN)
		x := gr.StringWidth(s) + 0xA8
		y := 0x6B
		for {
			c := mouseGetInput()
			if (c == '\r' && !empty) || c == 0x1B {
				break
			}
			if c >= ' ' && c <= '~' && x < 0x12E && ln < 0x1D {
				buf := string(rune(c))
				mouseHide()
				gr.StringToScreen(buf, x, y-3)
				mouseShow()
				x += gr.StringWidth(buf)
				name[ln] = byte(c)
				ln++
				empty = false
			} else if c == 8 || c == 0x91 {
				if ln > 0 {
					ln--
					x -= gr.StringWidth(string(name[ln : ln+1]))
					mouseHide()
					gr.Show(x, y, chrpic(6), 0x10, 0x91, x-0xA4, 0)
					mouseShow()
				} else {
					empty = true
				}
			}
		}
		name[ln] = 0
		return 0
	}
	for !done {
		var key int
		for {
			loopMusicMaybe()
			if key = mouseGetInput(); key >= 0 {
				break
			}
		}
		switch key {
		case 1, 2, 3:
			sel = mousopt(opt, sel)
			done = sel > -1
			if sel < 0 {
				sel = -1 - sel
			}
			old = sel
		case 0x93, 0xAB, 0x16E:
			sel += opt.cols
		case 0x8D, 0xA6, 0x170:
			sel -= opt.cols
		case 0x91, 0xA9, 0x166:
			sel++
		case 0x8F, 0xA8, 0x162:
			sel--
		case 0x8C, 0x8E, 0xA5, 0xA7, 0x23C:
			sel = 0
		case 0x92, 0x94, 0xAA, 0xAC, 0x23E:
			sel = opt.count - 1
		case '\r':
			done = true
		case 0x1B, 0x278:
			return -1
		}
		if sel < 0 {
			sel = 0
		} else if opt.count <= sel {
			sel = opt.count - 1
		} else {
			selopt(opt, sel, old)
		}
		old = sel
	}
	return sel
}

// genChar runs the eight questions; false if Escape is pressed at the first.
func genChar(dat []byte, opts []ChrOpt) bool {
	stage, idx := 0, 0
	var skills [6]byte
	dat32 := dat[0x20:]
	resetSkills := func() {
		for i := range skills {
			skills[i] = 20
		}
	}
	resetSkills()
	nameBuf := make([]byte, 30)
	opts[6].name = &nameBuf
	mouseHide()
	gr.CopyHiddenToVisible()
	mouseShow()
	restart := func() {
		mouseShow()
		stage, sknow, idx = 0, 0, 0
		resetSkills()
	}
	for stage < 8 {
		strs := opts[stage].strings
		mouseHide()
		gr.GrSoftPageFlip()
		gr.SetTheColor(0x106)
		gr.Rectangle(0x11, 0, 0x8E, 0xC7)
		drawopt(&opts[stage])
		selopt(&opts[stage], 0, 0xFF)
		gr.GrPageFlip()
		gr.Show(0, 0xC7, chrBg, 0xC8, 0x140, 0, 0)
		gr.GrSoftPageFlip()
		mouseShow()
		choice := pickopt(&opts[stage])
		if choice < 0 {
			if stage == 0 {
				return false
			}
			mouseHide()
			gr.CopyVisibleToHidden()
			restart()
			continue
		}
		switch stage {
		case 0:
			s := getString(int(strs[choice]) | STR_CHARGEN)
			if choice != 0 {
				opts[4].strings[0] = 12
			} else {
				opts[4].strings[0] = 7
			}
			player.SetFemale(choice)
			mouseHide()
			gr.StringToScreen(s, 0x11, 0xB2)
			mouseShow()
			stage++
		case 1:
			player.SetLefty(choice)
			stage++
		case 2:
			s := getString(int(strs[choice]) | STR_CHARGEN)
			player.SetPClass(choice)
			rollStats()
			if !setSklmnu(&idx, skills[:], &opts[3], dat32) {
				stage++
			}
			sknow = setSkills(0, skills[:])
			mouseHide()
			gr.StringToScreen(s, 0x8F-gr.StringWidth(s), 0xB2)
			showAtts()
			showSkills()
			mouseShow()
			stage++
		case 3:
			skills[idx-1] = byte(int(opts[3].strings[choice]) + 0xE1)
			sknow = setSkills(sknow, skills[:])
			mouseHide()
			showSkills()
			mouseShow()
			if !setSklmnu(&idx, skills[:], &opts[3], dat32) {
				stage++
			}
		case 4:
			gr.Transparency = true
			k := player.Female()*5 + choice + 0x11
			w := chrw(k)
			h := chrh(k)
			x := (0x38-w)/2 + 0x10
			y := 0x9C - (0x4C-h)/2
			mouseHide()
			gr.Show(x, y, chrpic(k), h, w, 0, 0)
			mouseShow()
			gr.Transparency = false
			player.SetBody(choice)
			stage++
		case 5:
			player.Easy = byte(choice)
			stage++
		case 6:
			s := cstr(nameBuf)
			mouseHide()
			gr.StringToScreen(s, (0x7E-gr.StringWidth(s))/2+0x11, 0xBD)
			mouseShow()
			if s != "" {
				copy(player.Name[:29], nameBuf[:29])
			}
			player.Name[29] = 0
			stage++
		case 7:
			if choice == 0 {
				stage++
				playerCompute(true)
			} else {
				mouseHide()
				gr.SetTheColor(0x106)
				gr.Rectangle(0x11, 0xC7, 0x8F, 0)
				restart()
			}
		}
	}
	done := getString(0x300)
	name := cstr(nameBuf)
	mouseHide()
	gr.GrSoftPageFlip()
	gr.Show(0, 0xC7, chrBg, 0xC8, 0x140, 0, 0)
	gr.StringToScreen(name, (0xA0-gr.StringWidth(name))/2+0xA0, gr.CurFont.Height+0x62)
	gr.StringToScreen(done, (0xA0-gr.StringWidth(done))/2+0xA0, 0x62)
	gr.GrSoftPageFlip()
	gr.SetTheColor(0x106)
	gr.Rectangle(0xA0, 0, 0x13F, 0xC7)
	return true
}

// strtChargen loads the chargen screen and its data and runs genChar.
func strtChargen() bool {
	fail := func() bool {
		if inGame {
			loadTxtmaps()
		}
		initChar(false)
		grfxClear()
		pfatalCode(5)
		return true
	}
	chrPics = chrPics[:0]
	if !gronkGr("chrbtns", 0, -1, func(img []byte, size, n int) bool {
		chrPics = append(chrPics, img)
		return size != 0
	}) {
		return fail()
	}
	sk := readFile("DATA\\skills.dat")
	if len(sk) > 0x348 {
		sk = sk[:0x348]
	}
	if sk == nil || len(sk) < 0x28 {
		return fail()
	}
	classTab = sk
	cg := readFile("DATA\\chrgen.dat")
	if cg == nil {
		return fail()
	}
	opts := make([]ChrOpt, 8)
	all := make([]uint16, (len(cg)-8*0x12)/2)
	for i := range all {
		all[i] = le16(cg, 8*0x12+i*2)
	}
	off := 0
	for i := range opts {
		r := cg[i*0x12:]
		opts[i] = ChrOpt{question: int(int16(le16(r, 0))), count: int(int16(le16(r, 8))),
			pic: int(int16(le16(r, 0xA))), rows: int(int16(le16(r, 0xC))),
			cols: int(int16(le16(r, 0xE))), spacing: int(int16(le16(r, 0x10)))}
		opts[i].strings = all[off:]
		for off < len(all) && all[off] != 0 {
			off++
		}
		off++
	}
	grfxLoadFont("fontchar.sys")
	gr.ForegroundColor, gr.BackgroundColor = 0x49, 0x49
	chrBg = make([]byte, 0xFA00)
	var pal [0x300]byte
	ok := bltfromdrive("DATA\\chargen.byt", chrBg)
	ok = readQuikpal(PAL_CHARGEN, pal[:]) && ok
	if !ok {
		return fail()
	}
	mouseHide()
	gr.Show(0, 0xC7, chrBg, 0xC8, 0x140, 0, 0)
	drawopt(&opts[0])
	selopt(&opts[0], 0, 0xFF)
	mouseShow()
	fadein(pal[:], 2)
	result := genChar(classTab, opts)
	grfxLoadFont("font5x6p.sys")
	if inGame {
		loadTxtmaps()
	}
	if !result {
		fadeout(pal[:], 2)
	}
	return result
}

// createPlayer is the start menu's "create character".
func createPlayer() bool {
	initChar(true)
	r := strtChargen()
	loadWeapcm()
	return r
}
