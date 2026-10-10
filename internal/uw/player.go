package uw

// Port of PLAYER.C: setting up the player object, the key and mouse bindings for play,
// the mouse regions over the 3D view, the position and version reports, and the camera.

var (
	MoveCrits = true
	PMsHndle  int

	PHgt, PLeft, PBot, PWid int
	curvrad                 int
)

// ih adapts a handler taking an int argument.
func ih(f func(int)) Handler { return func(a any) { f(a.(int)) } }

// PuntPlayerInv empties the player's inventory.
func PuntPlayerInv() {
	FixBagArea()
	FreePlayerInv(&ThePlayer.OL)
	ClearInventory()
}

func initPlayerRec() {
	p := ThePlayer
	p.SetLinkF(0)
	p.Whoami = 0xFD
	p.SetIsQuant(0)
	p.SetDoorDir(1)
	p.SetInvis(0)
	p.SetHeading(0)
	p.SetFineHead(0)
	p.SetNext(0)
	p.SetQuality(0)
	p.SetLinkF(0)
	p.SetOwner(0)
	p.B11 = 0
	p.SetItem(ITEM_ADVENTURER)
}

// initPlayer sets up the player object and registers the game's bindings.
func initPlayer() {
	ThePlayer = &Level.Objs[1]
	UsPtr = ThePlayer
	GrSq = -1
	PlayerHeading = 0
	PlayerFacing = 0
	PlayerPitch = 0
	PlayerBank = 0
	PlayerLevel = 1
	PN.B24 = 8
	PN.Index = 1
	PT.special = playerSqhandler
	PT.mask = 0x1100
	PT.ignore = 0
	setLight(0)
	PMsHndle = 0
	initPlayerRec()
	playerdat = &Creatures[ThePlayer.InMajor()]
	ThePlayer.HP = playerdat.Avghit
	if playerNameHandle == 0 {
		name := player.Name[:]
		playerNameHandle = makeString(&name, STRBLK_PLAYER)
	}
	// the original's letter keys (w s a d z c x e q, W A S D X) are UltimaHacks' below
	// the turn and step buttons under the view are art only: the mouse never walks
	inputAddkey('2', 0, 0x11, ih(chgPlyp))
	inputAddkey('`', 0, 0x11, func(any) { mouseLookToggle() })
	inputAddkey(KEY_ALT|'`', 0, 0x11, func(any) { mouseLookOrient() })
	// UltimaHacks' keys (keys.go), its input-mode masks
	inputAddkey(' ', 0, 1, func(any) { keySpace() })
	inputAddkey('r', 1, 1, ih(keyPanel))
	inputAddkey('f', 2, 1, ih(keyPanel))
	inputAddkey('q', 0, 1, ih(keyInteract))
	inputAddkey('e', 1, 1, ih(keyInteract))
	inputAddkey('z', 0, 1, func(any) { newscr(2) })
	inputAddkey('v', 1, 1, ih(keyBag))
	inputAddkey('b', 2, 1, ih(keyBag))
	inputAddkey('g', 0, 1, func(any) { printInfo() })
	inputAddkey('h', 0, 1, func(any) { keyFlasks() })
	inputAddkey('1', 1, 0x11, ih(chgPlyp))
	inputAddkey(KEY_GUP, 1, 0x11, ih(chgPlyp))
	inputAddkey('3', -1, 0x11, ih(chgPlyp))
	inputAddkey(KEY_GDOWN, -1, 0x11, ih(chgPlyp))
	inputAddkey(KEY_SHIFT|keyLShift, 0, 0x11, func(any) { shiftTap() }) // run
	// its rune keys
	for c := 'a'; c <= 'y'; c++ {
		inputAddkey(KEY_CTRL|KEY_ALT|int(c), int(c), 0x11, ih(keyRune))
	}
	inputAddkey(KEY_CTRL|KEY_ALT|KEY_BACKSPACE, KEY_BACKSPACE, 0x11, ih(keyRune))
	inputAddkey(KEY_CTRL|KEY_ALT|' ', ' ', 0x11, ih(keyRune))
	inputAddkey('j', 7, 0x1B, ih(parsePlayin))
	inputAddkey('J', 6, 0x1B, ih(parsePlayin))
	inputAddkey(KEY_F7, 0, 0x1B, ih(pullChain))
	inputAddkey(KEY_F10, 0, 0x1B, ih(playerKeySleep))
	inputAddkey(KEY_F9, 2, 0x1B, ih(func(n int) { playerUseSkill(n) }))
	inputAddkey(KEY_F8, 1, 0x1B, ih(tryCast))
	for _, c := range "srmfq" {
		k := KEY_CTRL | int(c)
		inputAddkey(k, k, 1, ih(doOptionShortcut))
	}
	helpKeys() // F1 opens the help (help.go), Esc the options panel
	inputAddkey(KEY_F5, 1, 1, ih(dealWithIcons))
	for _, k := range []int{KEY_SHIFT | KEY_BACKTAB, '\t'} {
		inputAddkey(k, k, 7, ih(keyboardMouse))
	}
	inputAddkey(KEY_ESC, 4, 4, ih(doEscapeKey))
	for i := 1; i <= 4; i++ {
		inputAddkey('0'+i, i, 4, ih(convPlayMenu))
	}
	inputAddmouse(0x52, 0x98, 0x88, 0xBE, 4, 4, ih(npcBarter))
	inputAddmouse(0x8B, 0x98, 0xC1, 0xBE, 4, 4, ih(playBarter))
	inputAddmouse(0x0F, 1, 0x131, 0x1E, 0, 4, ih(convPlayMenu))
	inputAddkey(KEY_ALT|KEY_F7, 0, 0x1B, func(any) { showVersion() })
	inputAddkey(KEY_ALT|KEY_F8, 0, 0x1B, func(any) { reportLoc() })
}

// mousPlayer makes the 3D view the given rectangle and its click region. The DOS game also
// put eight cursor regions round its edges, arrows for walking with the mouse; here the keys
// walk, so the pointer stays a pointer.
func mousPlayer(left, bot, wid, hgt int) {
	inputDel(PMsHndle)
	PLeft, PBot, PWid, PHgt = left, bot, wid, hgt
	PMsHndle = inputAddmouse(left, bot, left+wid-1, bot+hgt-1, 0, 0x1B, ih(mousIn3d))
}

func demousPlayer() {
	inputDel(PMsHndle)
	PMsHndle = 0
}

func reportLoc() {
	x := ThePlayer.HomeX()
	y := ThePlayer.HomeY()
	buf := []byte{byte(PlayerLevel + '0'), byte(x>>3 + '0'), byte(x&7 + '0'), byte(y>>3 + '0'), byte(y&7 + '0'), '\n'}
	scrollPrint(string(buf))
}

func showVersion() {
	gameSprint(0x113)
	scrollPrint("F1.94S\n")
	scrollPrint("with UltimaHacks by John Glassmyer\n")
}

func homeCam(index int) {
	if index <= 1 {
		campos[0] = PN.X
		campos[1] = PN.Y
		camang[0] = int16(PlayerFacing)
		if index == 1 {
			campos[2] = PN.Z + 0xA4
		} else {
			campos[2] = 0x458
			camang[1] = -0x400
		}
	} else if index < NUM_MOBILE {
		obj := ObjIntTMem(index)
		campos[0] = int16(obj.HomeX()<<8 + obj.FineX()<<5)
		campos[1] = int16(obj.HomeY()<<8 + obj.FineY()<<5)
		campos[2] = int16(obj.Z() << 3)
		camang[0] = int16(obj.HeadingBits() << 13)
	}
	if usNull() {
		editchng(2)
	}
}

func moveCam(how int) {
	turn := inplist.x * 3 / PWid
	step := inplist.y * 3 / PHgt
	camang[0] += int16((turn - 1) << 10)
	if step != 1 {
		dx, dy := cSinCos(int(camang[0]))
		campos[0] += (dx >> 8) * int16(step-1)
		campos[1] += (dy >> 8) * int16(step-1)
	}
	campos[0] = max(0x180, min(campos[0], 0x3D80))
	campos[1] = max(0x180, min(campos[1], 0x3D80))
	if usNull() {
		editchng(2)
	}
}

// usIndex is UsPtr's index relative to critdata (UsPtr - critdata); -1 and -2 are the
// camera's pseudo-objects of attach_eye's modes 2 and 3, kept in usPseudo.
var usPseudo int

// curelem is the editor's current object (data in the EXE that nothing in the game sets).
var curelem *Object

// attachEye chooses what the view is attached to.
func attachEye(mode int) {
	switch mode {
	case 3:
		if UsPtr != nil || usPseudo == -1 {
			UsPtr = nil
			usPseudo = -2
			editchng(2)
		}
		fallthrough
	case 2:
		if UsPtr != nil {
			UsPtr = nil
			usPseudo = -1
			editchng(2)
		}
	case 1:
		if UsPtr != ThePlayer {
			UsPtr = ThePlayer
			usPseudo = 0
			editchng(2)
		}
	case 0:
		if curelem == nil {
			break
		}
		i := ObjMemTPtr(curelem)
		if i == 0 || i >= NUM_MOBILE || i <= 1 {
			break
		}
		UsPtr = &Level.Objs[i]
		usPseudo = 0
		editchng(2)
	case -1:
		UsPtr = nil
		usPseudo = 0
	}
}

func releaseCamera(index int) {
	homeCam(index)
	attachEye(-1)
}

func crystalBall(obj *Object, x, y int) {
	campos[0] = int16(x<<8 + obj.FineX()<<5)
	campos[1] = int16(y<<8 + obj.FineY()<<5)
	campos[2] = int16(obj.Z() << 3)
	camang[0] = int16(obj.HeadingBits() << 13)
	camang[1], camang[2] = 0, 0
	setLight(6)
	automap := automapEnabled
	if automap {
		automapEnabled = false
	}
	camerasFade()
	if automap {
		automapEnabled = true
	}
	FixPlayerEquips()
}

// vortex spins the view into the moongate at (32, 32).
func vortex(unused int) {
	vortX, vortY = 0x20, 0x20
	attachEye(3)
	xd := int32(vortX)<<8 - int32(PN.X)
	yd := int32(vortY)<<8 - int32(PN.Y)
	vortRad = int16(cSqRt(uint32(xd*xd+yd*yd)) >> 6)
	x := int((xd << 15) / (int32(uint16(vortRad)) << 6))
	y := int((yd << 15) / (int32(uint16(vortRad)) << 6))
	vortTheta = int16(cAtan2(y, x))
	for vortTimer = 0; uint16(vortTimer) < 0x40; vortTimer++ {
		establishView()
		vortTheta += 0xCCB - 0x10000*0
	}
	attachEye(1)
}

func chgPlys(val *int16, dir, limit int) {
	if dir == 0 {
		editchng(2)
		*val = 0
	} else if limit == 0 {
		*val += int16(dir << 10)
		editchng(2)
	} else {
		l := limit
		if dir == -1 {
			l = -limit
		}
		if mvcheck(val, l, 0x400, dir) {
			editchng(2)
		}
	}
}

func rollView(step int) {
	if MoveCamera {
		chgPlys(&camang[2], step, 0)
	} else {
		chgPlys(&PlayerBank, step, 0)
	}
}

func chgPlyp(step int) {
	if MoveCamera {
		chgPlys(&camang[1], step, pitchBound())
	} else {
		chgPlys(&PlayerPitch, step, pitchBound())
	}
}

// pitchBound is the pitch's bound either way: three times the original's 0x1000 (UltimaHacks'
// pitchBound). Mouse-look keeps to it too.
func pitchBound() int { return 0x3000 }

// usNull is UsPtr == 0: the view on the roaming camera.
func usNull() bool { return UsPtr == nil && usPseudo == 0 }
