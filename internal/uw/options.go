package uw

import "github.com/memmaker/uw1go/internal/gr"

// OPTIONS.C: the options panel: the left-hand panel's pages of buttons (save, restore,
// music, sound effects, quit) from the "optbtns" art, the loop that moves the highlight and
// presses buttons, and each page's handler. A page is a number, optionsPage: 0 save, 1
// restore, 2 music, 3 sound effects, 4 the detail page (gone: the detail is always very
// high), 5 quit, 6 the top page (7 while closed). pagePics gives each of a page's seven rows (row 0 at the
// bottom) its button's picture, 0 for none; the lit picture is the next one.

var (
	LeftPanel    = 2 // what the left panel shows: 1 the options
	optionsPage  = -1
	optionsQuick bool // opened by doOptionShortcut: close after one choice
	pageDraw     [7]func()
	pagePress    [7]func(int)
	pagePics     = [7][7]int{
		{0, 0x18, 0x24, 0x22, 0x20, 0x1E, 0},
		{0, 0x18, 0x24, 0x22, 0x20, 0x1E, 0},
		{0, 0, 0x1A, 0x16, 0x14, 0, 0},
		{0, 0, 0x1A, 0x16, 0x14, 0, 0},
		{},
		{0, 0, 0, 0x3B, 0x39, 0, 0},
		{0, 0x12, 0x10, 0x0C, 0x0A, 0x08, 0x06}, // the top page, quit and return a row up
	}
	gameoptsDone bool
	hilitePic    int
	hiliteRow    int
)

func init() {
	pageDraw = [7]func(){drawSavePage, drawSavePage, drawMusicAndSoundPage, drawMusicAndSoundPage,
		nil, drawQuitYNPage, drawTopPage}
	pagePress = [7]func(int){chooseSave, chooseSave, chooseMusic, chooseSounds, nil,
		chooseQuit, chooseTop}
}

func optionsFrame() {
	gr.SetTheColor(0x106)
	gr.Rectangle(4, 0xBD, 0x26, 0x52)
}

// runOptionsPanel runs the panel until a choice closes it, drawing the top page first if
// show is nonzero. Mouse look is off meanwhile, the pointer on the panel to click its
// buttons, and comes back as it was if the view is still there.
func runOptionsPanel(show int) {
	if was := lookOn; was {
		lookX, lookY = 0x15, 0x88 // the panel's middle, where mouse look going off leaves it
		mouseLookSet(false)
		renderFB() // the view without the crosshair, which no frame clears while the panel is up
		sendFB()
		defer func() {
			if inplist.mode&0x11 != 0 {
				mouseLookSet(true)
			} else {
				lookWas = true // quit to the menu: on again with the next game
			}
		}()
	}
	LeftPanel = 1
	if show != 0 {
		mouseHide()
		gr.SetLayer(4, 0xBD, 0x23, 0x6C, gr.LayerPage) // over the view
		gr.GrSoftPageFlip()
		setOptionsPage(6)
		gr.GrSoftPageFlip()
		optionsFrame()
		mouseShow()
		mouseRelease(false)
	}
	gameoptsDone = false
	for !gameoptsDone {
		input := mouseGetInput()
		for input < 0 {
			loopMusicMaybe()
			input = mouseGetInput()
		}
		switch input {
		case 1, 2, 3:
			x, y := mouseGetxy()
			x -= 4
			y -= 0x52
			if x >= 0 && x <= 0x23 && y >= 0 && y <= 0x6C {
				optionsMouseClick(x, y)
			}
		case ' ', KEY_GDOWN:
			optionsHandleKey(0)
		case '\r':
			optionsHandleKey(1)
		case KEY_GUP:
			optionsHandleKey(2)
		case KEY_ESC:
			closeOptionPanel()
		}
	}
}

func drawOptionsBack(pic int) {
	reloadGrVpic(ICON_OPTB, "optbtns", pic)
	picToScreen(ICON_OPTB, 4, 0xBD)
}

func drawOptionsBtn(row, pic int) {
	reloadGrVpic(ICON_OPTB+1, "optbtns", pic)
	picToScreen(ICON_OPTB+1, 5, 0xBD-(6-row)*15-2)
}

// hilite puts out the lit button and lights button pic at row.
func hilite(row, pic int) {
	if hilitePic >= 0 {
		drawOptionsBtn(hiliteRow, hilitePic-1)
	}
	drawOptionsBtn(row, pic+1)
	hilitePic = pic + 1
	hiliteRow = row
}

// closeOptionPanel gives the panel's place back to the view.
func closeOptionPanel() {
	mouseHide()
	LeftPanel = 0
	optionsPage = 7
	gr.SetLayer(4, 0xBD, 0x23, 0x6C, gr.LayerFrame) // the view again
	gr.RefreshRect(4, 0xBD, 0x23, 0x6C)
	editchng(2)
	gameoptsDone = true
	mouseShow()
}

// drawTopPage draws the top page without its detail button: the art with return and quit
// a row up and the bottom row the quit page's empty board.
func drawTopPage() {
	drawOptionsBack(1)
	reloadGrVpic(ICON_OPTB, "optbtns", 3)
	slotVram(grsWhich1(ICON_OPTB)).Show(4, 0xBD-92, 0x23, 0x6C, 0, 92)
	drawOptionsBtn(2, 0x10)
	drawOptionsBtn(1, 0x12)
	hilitePic = -1
	hilite(6, 6)
}

func drawSavePage() {
	drawOptionsBack(2)
	hilitePic = -1
	hilite(5, 0x1E)
	if optionsPage == 1 {
		drawOptionsBtn(6, 0x2E)
	}
	gr.GrSoftPageFlip()
	optionsFrame()
	ShowSaveRest()
	gr.GrSoftPageFlip()
}

func drawQuitYNPage() {
	drawOptionsBack(3)
	hilitePic = -1
	hilite(3, 0x3B)
}

func drawMusicAndSoundPage() {
	drawOptionsBack(4)
	hilitePic = -1
	var title, state, row int
	if optionsPage == 2 {
		title = 0x33
		state = b2i(!musicIsOn()) + 0x2F
		row = b2i(musicIsOn()) + 3
	} else {
		title = 0x34
		state = b2i(!fxIsOn()) + 0x31
		row = b2i(fxIsOn()) + 3
	}
	drawOptionsBtn(6, state)
	drawOptionsBtn(5, title)
	if row == 3 {
		hilite(row, 0x16)
	} else {
		hilite(row, 0x14)
	}
}

// leaveOptionsPage goes back to the top page, or closes a quick panel.
func leaveOptionsPage() {
	if optionsQuick {
		closeOptionPanel()
	} else {
		setOptionsPage(6)
	}
}

// chooseMusic and chooseSounds: row 4 on, row 3 off, row 2 done.
func chooseMusic(row int) {
	switch row {
	case 4:
		turnMusic(true)
		drawMusicAndSoundPage()
	case 3:
		turnMusic(false)
		drawMusicAndSoundPage()
	}
	if optionsQuick || row == 2 {
		leaveOptionsPage()
	}
}

func chooseSounds(row int) {
	switch row {
	case 4:
		turnFx(true)
		drawMusicAndSoundPage()
	case 3:
		turnFx(false)
		drawMusicAndSoundPage()
	}
	if optionsQuick || row == 2 {
		leaveOptionsPage()
	}
}

// chooseTop: from the top, save, restore, music, sound, return, quit; the bottom row is empty.
func chooseTop(row int) {
	switch row {
	case 6:
		if checkSave() {
			setOptionsPage(0)
		}
	case 5:
		if checkRest() {
			setOptionsPage(1)
		}
	case 4:
		setOptionsPage(2)
	case 3:
		setOptionsPage(3)
	case 2:
		closeOptionPanel()
	case 1:
		setOptionsPage(5)
	}
}

// chooseSave: on the save or restore page rows 5 to 2 are slots 1 to 4, row 1 cancels.
func chooseSave(row int) {
	if row < 1 || row > 5 {
		return
	}
	hilite(row, (5-row)*2+0x1E)
	gr.GrSoftPageFlip()
	optionsFrame()
	scrollClear(false)
	if row >= 2 {
		slot := 6 - row
		DoSaveRest(optionsPage == 1, slot)
		if optionsPage == 1 {
			RightButtonThing = IMODE_DEFAULT
			puntFightmode()
		}
	}
	gr.GrSoftPageFlip()
	closeOptionPanel()
}

// chooseQuit: row 4 quits, row 3 goes back.
func chooseQuit(row int) {
	switch row {
	case 4:
		hilite(4, 0x39)
		gr.GrSoftPageFlip()
		optionsFrame()
		mouseShow()
		editexit()
		mouseHide()
		gr.GrSoftPageFlip()
		fallthrough
	case 3:
		closeOptionPanel()
	}
}

func setOptionsPage(page int) {
	optionsPage = page
	pageDraw[optionsPage]()
}

func pressOptionsBtn(row int) {
	if optionsPage < 0 || optionsPage > 6 {
		return
	}
	mouseHide()
	gr.GrSoftPageFlip()
	pagePress[optionsPage](row)
	gr.GrSoftPageFlip()
	optionsFrame()
	mouseShow()
	mouseRelease(false)
}

// optionsMouseClick presses the row under (x, y).
func optionsMouseClick(x, y int) { pressOptionsBtn(y / 15) }

// optionsHandleKey: 0 moves the highlight down a row, 2 up, 1 presses the lit button;
// the Ctrl shortcut codes press a top page button.
func optionsHandleKey(key int) {
	switch key {
	case 0, 2:
		row := hiliteRow + 1
		if key == 0 {
			row = hiliteRow - 1
		}
		if row >= 0 && row < 7 && optionsPage >= 0 && optionsPage < 7 && pagePics[optionsPage][row] != 0 {
			mouseHide()
			gr.GrSoftPageFlip()
			hilite(row, pagePics[optionsPage][row])
			gr.GrSoftPageFlip()
			optionsFrame()
			mouseShow()
		}
	case 1:
		pressOptionsBtn(hiliteRow)
	case 0x173:
		pressOptionsBtn(6)
	case 0x172:
		pressOptionsBtn(5)
	case 0x16D:
		pressOptionsBtn(4)
	case 0x166:
		pressOptionsBtn(3)
	case 0x171:
		pressOptionsBtn(1)
	}
}

// doOptionShortcut is a Ctrl shortcut: press keycode's top page button and run the panel.
// Refused partway through an action.
func doOptionShortcut(keycode int) {
	if GameInputMode != 0 {
		gameSprint(0xA0) // "You cannot select options partway through an action."
		return
	}
	optionsQuick = true
	optionsPage = 6
	optionsHandleKey(keycode)
	runOptionsPanel(b2i(optionsPage == 6))
	optionsQuick = false
}
