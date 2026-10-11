package uw

import (
	"os"
	"strings"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of MAINMENU.C: the start menu on DATA\OPSCR.BYT with the OPBTN.GR buttons
// (introduction, create character, acknowledgements, journey onward), and the list of
// saved games.

// Button is a menu button: its normal and selected pictures, top left and size.
type Button struct {
	img  [2][]byte
	x, y int
	w, h int
}

var (
	cycleTime uint16
	buttons   []Button
)

func moveOpbtn(img []byte, size, n int) bool {
	if len(img) < 5 {
		return size != 0
	}
	buttons[n>>1].img[n&1] = img[5:]
	if n&1 == 0 {
		buttons[n>>1].w = int(img[1])
		buttons[n>>1].h = int(img[2])
	}
	return size != 0
}

// menuCycle rotates palette colours 40h..7Fh every 14 ticks while the menu waits.
func menuCycle() {
	if uint16(GameTime())-cycleTime >= 0xE {
		rotateBank(0x40, 0x40, true)
		gr.LocalDoPalette(0x40, 0x40)
		cycleTime = uint16(GameTime())
	}
}

// drawStartButtons draws the buttons, or the strings when text is given, sel highlighted.
func drawStartButtons(b []Button, text []string, sel int) {
	mouseHide()
	if text == nil {
		for i := range b {
			img := b[i].img[b2i(i == sel)]
			gr.Show(b[i].x, b[i].y, img, b[i].h, b[i].w, 0, 0)
		}
	} else {
		for i := range text {
			if i == sel {
				gr.ForegroundColor, gr.BackgroundColor = 0xA2, 0xA2
			} else {
				gr.ForegroundColor, gr.BackgroundColor = 0xAA, 0xAA
			}
			w := gr.StringWidth(text[i])
			for w > 0x13E {
				text[i] = text[i][:len(text[i])-1]
				w = gr.StringWidth(text[i])
			}
			gr.StringToScreen(text[i], 0xA0-w/2, 100-i*0x16)
		}
	}
	mouseShow()
}

// realStart runs the main menu until a game is started or restored.
func realStart(intro bool) {
	wideOff()
	var pal [0x300]byte
	buttons = []Button{
		{x: 0x62, y: 0x76, w: 1, h: 1},
		{x: 0x51, y: 0x5F, w: 1, h: 1},
		{x: 0x48, y: 0x47, w: 1, h: 1},
		{x: 0x55, y: 0x2E, w: 1, h: 1},
	}
	_, found := getSaveDescs()
	n, sel := 4, 3
	if found == 0 {
		n, sel = 3, 1
	}
	// no introduction before the menu (UltimaHacks' skip): the menu still offers it
	forceMouseCursor(ICON_CURSORS)
	mouseShow()
	done := false
	choice := 0
	for !done {
		loopMusicMaybe()
		if choice < 4 && choice > -1 {
			opscr := make([]byte, 0xFA00)
			bltfromdrive("DATA\\opscr.byt", opscr)
			mouseHide()
			gr.Show(0, 0xC7, opscr, 0xC8, 0x140, 0, 0)
			mouseShow()
			if !gronkGr("opbtn", 0, -1, moveOpbtn) {
				pfatalCode(ERR_READ | 0x00D)
			}
			if choice != 3 {
				drawStartButtons(buttons[:n], nil, sel)
				readQuikpal(PAL_OPENING, pal[:])
				fadein(pal[:], 2)
			}
		}
		choice = parseStartInput(buttons[:n], nil, sel)
		shot("menu")
		switch choice {
		case 2:
			showCredits()
		case 3:
			r := doJourney()
			done = r == 1
			if r == -1 {
				msg := getString(0x2A9)
				loadBitMap(-1, "DATA\\opscr.byt")
				grfxLoadFont("fontbig.sys")
				gr.ForegroundColor, gr.BackgroundColor = 0xA2, 0xA2
				gr.StringToScreen(msg, 0xA0-gr.StringWidth(msg)/2, 0x5A)
				mouseShow()
				for mouseGetInput() < 0 {
					menuCycle()
				}
				grfxLoadFont("font5x6p.sys")
			} else if done {
				puntFightmode()
			}
		case 1:
			fadeout(pal[:], 2)
			if createPlayer() {
				clearDir("SAVE0\\")
				copyFile("DATA\\lev.ark", "SAVE0\\lev.ark")
				if err := initBabl(); err != 0 {
					pfatalCode(err)
				}
				done = GetLevel(1) > 0
				if done {
					playerSetup(0x20, 2)
					doLevelHacks(1, 0)
				}
			}
		case 0:
			runcutscene(0)
		case -1:
			freeWorld(false)
			os.Exit(1)
		}
	}
	unforceMouseCursor(3)
	mouseShow()
	changeScreen(MODE_GAME)
	editchng(0x7FFE)
	LeftPanel = 0
}

// parseStartMouse follows the mouse over the buttons (or strings) until the button comes
// up; it returns the one under it, plus n if the release was on none of them.
func parseStartMouse(b []Button, text []string) int {
	none := true
	last := -1
	n := len(b)
	if text != nil {
		n = len(text)
		grfxLoadFont("fontbig.sys")
	}
	for mouseGetInput() > 0 {
		menuCycle()
		x, y := mouseGetxy()
		i := 0
		for ; i < n; i++ {
			var hit bool
			if text == nil {
				p := &b[i]
				hit = p.x <= x && p.x+p.w-1 >= x && p.y-p.h+1 <= y && p.y >= y
			} else {
				w := gr.StringWidth(text[i])
				left := (0x140 - w) / 2
				bottom := 100 - i*0x16
				hit = x >= left && left+w > x && y <= bottom && bottom-fontHeight() < y
			}
			if hit {
				none = true
				if i != last {
					drawStartButtons(b, text, i)
					last = i
				}
				break
			}
		}
		if i == n {
			none = last == -1
		}
	}
	if text != nil {
		grfxLoadFont("font5x6p.sys")
	}
	if none {
		return last
	}
	return last + n
}

// parseStartInput runs the menu until a choice is made; -1 for Escape where allowed.
func parseStartInput(b []Button, text []string, sel int) int {
	n := len(b)
	if text != nil {
		n = len(text)
	}
	result := -2
	cur := sel
	for result < -1 {
		grfxLoadFont("fontbig.sys")
		drawStartButtons(b, text, cur)
		grfxLoadFont("font5x6p.sys")
		var key int
		for {
			key = mouseGetInput()
			if key >= 0 {
				break
			}
			loopMusicMaybe()
			menuCycle()
		}
		switch key {
		case 1, 2, 3:
			hit := parseStartMouse(b, text)
			if hit >= 0 && hit < n && hit != result {
				result, cur = hit, hit
			} else if hit >= n {
				cur = hit - n
			}
		case KEY_GRIGHT, KEY_GDOWN, KEY_CTRL | 'f', KEY_CTRL | 'n':
			cur++
		case KEY_GUP, KEY_GLEFT, KEY_CTRL | 'b', KEY_CTRL | 'p':
			cur--
		case KEY_ENTER:
			result = cur
		case KEY_GHOME, KEY_GPGUP, KEY_ALT | '<':
			cur = 0
		case KEY_GEND, KEY_GPGDN, KEY_ALT | '>':
			cur = n - 1
		case KEY_ESC:
			if text != nil {
				result = -1
			}
		case KEY_ALT | 'x':
			result = -1
		}
		switch {
		case n > 0 && cur < 0: // the cursor wraps round
			cur = n - 1
		case n > 0 && cur >= n:
			cur = 0
		}
	}
	return result
}

// doJourney lists the saved games and restores the one chosen: 1 restored, -1 failed, 0
// none chosen.
func doJourney() int {
	mouseHide()
	loadBitMap(-1, "DATA\\opscr.byt")
	mouseShow()
	names, found := getSaveDescs()
	var descs []string
	for i := 0; i < 4; i++ {
		if found&(1<<i) != 0 {
			descs = append(descs, strings.TrimRight(names[i], " "))
		}
	}
	i := parseStartInput(nil, descs, 0)
	if i < 0 {
		return 0
	}
	count := -1
	for j := -1; j != i; count++ {
		if found&(1<<(count+1)) != 0 {
			j++
		}
	}
	mouseHide()
	loadBitMap(-1, "DATA\\opscr.byt")
	msg := getString(0x301)
	grfxLoadFont("fontbig.sys")
	gr.ForegroundColor, gr.BackgroundColor = 0xA2, 0xA2
	gr.StringToScreen(msg, (0x140-gr.StringWidth(msg))/2+10, 0x5A)
	grfxLoadFont("font5x6p.sys")
	if RestoreGame(count + 1) {
		loadWeapcm()
		return 1
	}
	return -1
}
