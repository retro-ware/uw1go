package uw

import (
	"os"
	"strconv"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of UWEDIT.C: main, start-up and shut-down, the title, the screen dispatcher, game
// reset, death, and moving the player to a new position or level.

var (
	scrmode        int
	lastscrmode    int
	dungeonf       string
	NewPlayerX     int
	NewPlayerY     int
	NewPlayerLevel int
	NewPlyFade     = 3
	inGame         bool
	nppFunc        func()
)

func init() {
	editorDispatch = [3][16]func(){
		{strtDemscr, do3dView, nil, func() { newPlayerPos() }, nil, nil, nil, nil, nil,
			RedispInv, checkVictory, checkPhysics, displayScr, updateScreen, nil, freeDemscr},
		{nil, AutoMap, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, automapScr, nil, nil, ExitAutoMap},
		{strtConverse, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, freeConverse},
	}
}

// uwMain is UWEDIT.C's main.
func uwMain() {
	initWorld()
	if c := os.Getenv("UW1GO_CUTS"); c != "" {
		n, _ := strconv.ParseInt(c, 0, 32)
		grfxQuikpal(PAL_GAME)
		runcutscene(int(n))
		return
	}
	dprintf("before titlescr\n")
	titlescr()
	dprintf("after titlescr\n")
	realStart(true)
	inGame = true
	mainloop()
	freeWorld(true)
}

func initWorld() {
	checkDirs()
	initStrings()
	MapInit()
	initInput()
	initDebug()
	initKeyboard()
	ReadCfg()
	initSounds()
	initTimers()
	initCutscene()
	if !grfxInit() {
		pfatalCode(ERR_READ | 3)
	}
	loadBitMap(5, "DATA\\pres1.byt")
	shot("pres1")
	loadNewMusic(MUSIC_THEME, true)
	if err := loadAllGr(); err != 0 {
		pfatalCode(err)
	}
	loadBitMap(6, "DATA\\pres2.byt")
	shot("pres2")
	if initMouse() < 0 {
		pfatalCode(2)
	}
	if err := initObjects(); err != 0 {
		pfatalCode(err)
	}
	initTxtlib()
	init3d()
	initPlayer()
	initAi()
	initChar(false)
	initEdit()
	initLighting()
	initCombinables()
	if !initSave() {
		pfatal("Not enough disk space for save game.$")
	}
	copyFile("DATA\\lev.ark", "SAVE0\\lev.ark")
	if err := initBabl(); err != 0 {
		pfatalCode(err)
	}
	grfxClear()
	grfxQuikpal(PAL_GAME)
}

// freeWorld shuts everything down and empties SAVE0.
func freeWorld(flag bool) {
	grfxClose()
	freeInput()
	freeScrgr()
	freeSounds()
	clearDir("SAVE0\\")
}

func titlescr() {
	for mouseGetInput() > 3 {
	}
	// no title sequence (UltimaHacks' skip)
}

func initEdit() {
	dungeonf = "LEVEL1"
	inputAddkey(KEY_ALT|'x', nil, 0xBD, func(any) { editexit() })
	inputAddkey(KEY_ALT|'q', nil, 0xFF, func(any) { saveScreenshot() })
	notdone = true
	changed = 0x7FFF
	scrmode = 0
	scrnum = -1
	inplist.mode = 0
}

func editexit() { notdone = false }

func clearobj() {
	MapObjFix()
	editchng(2)
	GrSq = -1
}

func changeScreen(mode int) {
	mouseLookScreen(true)
	defer mouseLookScreen(false)
	scrmode = mode
	inplist.mode = mode
	switch mode {
	case 2:
		scrnum = 1
	case 4:
		scrnum = 2
	default:
		scrnum = 0
	}
}

// newscr switches screens; a negative mode returns to the one before.
func newscr(mode int) {
	if scrnum != -1 && editorDispatch[scrnum][15] != nil {
		editorDispatch[scrnum][15]()
	}
	if mode >= 0 {
		lastscrmode = scrmode
	} else {
		mode = lastscrmode
	}
	changeScreen(mode)
	if f := editorDispatch[scrnum][0]; f != nil {
		f()
	}
	if mode != 1 {
		editchng(0x7FFE)
	}
}

func strtDemscr() {
	var pal [0x300]byte
	mouseHide()
	demousPlayer()
	place3dView(0x34, 0xB4, 0xAC, 0x71)
	copy(pal[:], gr.Palette[:])
	fadeout(pal[:], 2)
	if !loadBitMap(-1, "DATA\\main.byt") {
		pfatalCode(ERR_READ | 0xB)
	}
	initGamedisp()
	editchng(0x7DFE)
	FixPlayerEquips()
	renderFB()
	sendFB()
	mouseShow()
	readQuikpal(PAL_GAME, pal[:])
	fadein(pal[:], 2)
}

func freeDemscr() {
	demousPlayer()
	clearGamedisp()
	holdScrgr()
}

// resetGame clears the player's transient state before a new or restored game.
func resetGame() {
	PuntPlayerInv()
	FixPlayerEquips()
	ThePlayer.SetHeading(0)
	ThePlayer.SetFineHead(0)
	PlayerHeading, PlayerFacing = 0, 0
	automapEnabled = true
	resetScrgr()
	lastscrmode = 0
	puntFightmode()
	LeftPanel = 2
	if RightButtonThing == 1 || RightButtonThing == 3 || RightButtonThing == 4 {
		unforceMouseCursor(3)
	}
	RightButtonThing = 0
	lstime, nextSpellTime = 0, 0
	if GameInputMode != 0 {
		if GameInputMode < 4 {
			unforceMouseCursor(3)
			CursorObjPtr = nil
			GameInputMode = 0
		} else {
			GameInputMode = 0
			attachEye(1)
		}
	}
}

// realDeath ends the game in progress and goes back to the start menu (how 1: death).
func realDeath(how int) {
	var pal [0x300]byte
	pmouseHandled = 0
	mouseFreereign()
	for mouseGetInput() > -1 {
	}
	if how == 1 {
		setScreenFrame(2, 0)
		setCompass()
		setFlask(0)
		runcutscene(0x103)
		scrollClear(true)
	}
	editorDispatch[scrnum][15]()
	scrmode, inplist.mode = 0, 0
	scr := scrnum
	scrnum = -1
	inGame = false
	if how == 1 {
		mouseHide()
	}
	resetGame()
	copy(pal[:], gr.Palette[:])
	fadeout(pal[:], 2)
	realStart(false)
	inGame = true
	scrnum = scr
	scrmode = 1 << scrnum
	editorDispatch[scrnum][0]()
}

func do3dView() { establishView() }

// newPlayerPos carries out a pending teleport.
func newPlayerPos() bool {
	if NewPlayerX > 0 {
		if NewPlyFade&1 != 0 {
			renderFB()
			fadeout3d(curvrad)
		}
		if PlayerLevel != NewPlayerLevel {
			if !ChangeLevel(PlayerLevel, NewPlayerLevel) {
				pfatalCode(ERR_READ | 0xC)
			}
			PlayerLevel = NewPlayerLevel
		}
		if nppFunc != nil {
			nppFunc()
		}
		x, y, ok := findGoodXAndY(ThePlayer, NewPlayerX, NewPlayerY, false)
		if !ok {
			x, y, ok = findGoodXAndY(ThePlayer, NewPlayerX, NewPlayerY, true)
		}
		if !ok {
			NewPlayerX = 0
			ThePlayer.HP = 0
			return false
		}
		NewPlayerX, NewPlayerY = x, y
		playerSetup(NewPlayerX, NewPlayerY)
		if NewPlyFade&2 != 0 {
			renderFB()
			fadein3d(curvrad)
		}
		NewPlayerX = 0
		if NewPlyFade&2 != 0 {
			editchng(0x7FFE)
		}
	}
	return true
}

// ReadCfg reads DATA\UW.CFG.
func ReadCfg() {
	b := readFile("DATA\\uw.cfg")
	if b != nil {
		readCfgData(b)
	}
}
