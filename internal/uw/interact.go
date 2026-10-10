package uw

import (
	"strconv"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of INTERACT.C: the game screen's click areas and the player's interaction with
// the world: picking, looking, using, getting and talking in the 3D view, the inventory
// and right-hand panels' mouse, the interaction icons and fight mode.

// ActorFn acts with the cursor object or spell on obj (ObjectActor).
type ActorFn func(obj *Object, a, b int)

var (
	iconX = [6]int{8, 8, 6, 6, 7, 8}
	iconY = [6]int{0x64, 0x77, 0x86, 0x98, 0xAC, 0xBD}

	TimeStop, Hasted, WizEye, Blessed bool
	PickDist                          = 0x90
	defMode                           bool
	lastDurCheck                      int32
	DurCount                          int
	releaseable                       bool

	pTxtId           int
	ObjectActor      ActorFn
	ObjectActing     *Object
	RightButtonThing int
	inforMshandle    int
	ObjectActorArg   int
	actspMshandle    int
	iconsMshandle    int
	PickMap          *Tile
	newPlObj         *Object
	flaskMshandle    int
	spellMshandle    int
	releasePtr       *Link
	GameInputMode    int
)

// pullChain flips the right-hand panel between the inventory and the statistics.
func pullChain(int) {
	if RightPanel == PANEL_INV {
		setScreenFrame(SCR_PANEL, PANEL_STATS)
	} else if RightPanel != PANEL_TURNING {
		setScreenFrame(SCR_PANEL, PANEL_INV)
	}
}

// printInfo: a click on the compass prints hunger, fatigue, level, day and time of day.
func printInfo() {
	scrollPrint("\n")
	gameStrings3(0x40, int(player.Hunger)/0x1E+0x68, 0x67)
	n := min(int(player.Fatigue)/0x17, 5)
	gameSprint(0x76 - n)
	scrollPrint(".\n")
	gameStrings3(0x41, PlayerLevel+0x19A, 0x42)
	n = int(player.GameClock / 0x1C2000)
	day := n / 12
	n %= 12
	if day > 100 {
		gameSprint(0x45)
	} else {
		gameStrings3(0x43, day+0x19B, 0x44)
	}
	gameStrings3(0x46, n+0x47, 0x53)
	mouseRelease(true)
}

// flaskInfo: a click on the flasks pulls the chain or prints vitality or mana.
func flaskInfo() {
	if inplist.x > 0x19 && inplist.x < 0x28 {
		if inplist.y > 0xD {
			pullChain(0)
		}
		return
	}
	if inplist.y > 0x1E {
		return
	}
	msg := getString((b2i(inplist.x > 0x1E) + 0x59) | STR_GAME)
	var cur, max string
	if inplist.x < 0x1E {
		cur = strconv.Itoa(int(ThePlayer.HP))
		max = strconv.Itoa(int(playerdat.Avghit))
		if p := player.Poison(); p != 0 {
			gameStrings3(0x5B, (p-1)/3+0x54, 0x5C)
		}
	} else {
		cur = strconv.Itoa(int(player.PlayMana))
		max = strconv.Itoa(int(player.MaxMana))
	}
	scrollPrint(msg + cur + " out of " + max + "\n")
	mouseRelease(true)
}

// startGameinp registers the game screen's five click areas.
func startGameinp() {
	LeftPanel = 0
	iconsMshandle = inputAddmouse(8, 0x54, 0x20, 0xCE, -1, MODE_GAME, ih(dealWithIcons))
	spellMshandle = inputAddmouse(0xB0, 0x2D, 0xDE, 0x3D, 0, MODE_GAME, ih(tryCast))
	actspMshandle = inputAddmouse(0x34, 0x2F, 0x66, 0x3F, 0, MODE_GAME, ih(tryClear))
	inforMshandle = inputAddmouse(0x7A, 0x31, 0x98, 0x40, 0, MODE_GAME, func(any) { printInfo() })
	flaskMshandle = inputAddmouse(0xF4, 0x2C, 0x135, 0x50, 0, MODE_GAME, func(any) { flaskInfo() })
}

func clearGameinp() {
	inputDel(iconsMshandle)
	inputDel(spellMshandle)
	inputDel(actspMshandle)
	inputDel(flaskMshandle)
	inputDel(inforMshandle)
}

// initGamedisp sets up the game screen.
func initGamedisp() {
	BeginInventory()
	initScroll()
	playMusic()
	startGameinp()
	if LeftPanel == 0 {
		if RightButtonThing != IMODE_DEFAULT {
			newIconSelect(RightButtonThing)
		}
	} else {
		runOptionsPanel(1)
	}
	displayScr()
	initScrgr()
}

func clearGamedisp() {
	clearGameinp()
	EndInventory()
	stopMusic()
}

// displayScr is called every frame: the flasks, the compass, the damage flash, the
// palette cycle, death, the music and the duration checks.
func displayScr() {
	playerAttack(0)
	v := int(ThePlayer.HP)
	setScreenFrame(SCR_VITALITY, v)
	dam := ThePlayer.Damage()
	if dam<<2 > int(Creatures[63].Avghit) || v < 0x10 && dam > 0 {
		setScreenFrame(SCR_DRAGON, 3)
	}
	ThePlayer.B11 = 0
	setScreenFrame(SCR_MANA, int(player.PlayMana))
	if PlayerLevel != 9 {
		v = ThePlayer.HeadingBits()<<5 + ThePlayer.FineHead()
		v = ((v + 8) & 0xFF) >> 4
		setScreenFrame(SCR_COMPASS, v)
	}
	cycleColors(byte(GameTime()))
	if ThePlayer.HP == 0 {
		playerIsDead()
	}
	d := int32(GameTime()>>8) - lastDurCheck
	if d != 0 {
		lastDurCheck = int32(GameTime() >> 8)
		DurCount += int(d)
		if realDScheck {
			seg027_2861_EF9()
			realDScheck = false
		}
		changeMusicMaybe()
		if DurCount > 0x14 {
			DurCount = 0
			durationCheck()
		}
	}
	if PlayerLevel == 9 && rand()&0x1F == 0 {
		etherealVoidSpecialEffects()
	}
}

// RedispInv redraws the inventory panel when it is showing, or in a conversation.
func RedispInv() {
	if RightPanel == PANEL_INV || inplist.mode == MODE_CONV {
		loadInventoryPix()
		DisplayInvSpecial()
		DisplayInventory()
	}
}

// tileXY is a tile's map position.
func tileXY(tile *Tile) (int, int) {
	i := tileIndexOf(tile)
	return i & MAP_MASK, i >> 6
}

// InPickRange is whether obj on tile is within reach; it sets MapObjX, MapObjY.
func InPickRange(dist int, obj *Object, tile *Tile) bool {
	MapObjX, MapObjY = tileXY(tile)
	if dist != 0 {
		x := MapObjX<<3 + obj.FineX()
		y := MapObjY<<3 + obj.FineY()
		x = abs(x - (ThePlayer.HomeX()<<3 + ThePlayer.FineX()))
		y = abs(y - (ThePlayer.HomeY()<<3 + ThePlayer.FineY()))
		if x*x+y*y > dist {
			return false
		}
		dz := ThePlayer.Z() - obj.Z()
		pole := b2i(UsingPole)
		if (pole+1)*12 < dz || (pole+1)*-24 > dz {
			return false
		}
	}
	return true
}

// BridgeHeight is the height of the highest bridge on tile, or -1.
func BridgeHeight(tile *Tile) int {
	h := -1
	for obj := ObjPtrTMem(&tile.Objects); obj != nil; obj = ObjPtrTMem(&obj.QN) {
		if obj.Item() == ITEM_BRIDGE && obj.Z() > h {
			h = obj.Z()
		}
	}
	return h
}

func terrainOf(t *Tile) int { return int(TxmTerr[t.Floor()]) }

// BlockingTerrain is whether the terrain stops the player reaching obj.
func BlockingTerrain(dist int, obj *Object) bool {
	if dist == 0 {
		return false
	}
	px, py := ThePlayer.HomeX(), ThePlayer.HomeY()
	terr := terrainOf(MapGetAddr(px, py))
	if MapObjX == px && MapObjY == py {
		return false
	}
	sign := func(a, b int) int {
		if a < b {
			return -1
		} else if a > b {
			return 1
		}
		return 0
	}
	xdir, ydir := sign(MapObjX, px), sign(MapObjY, py)
	pole := b2i(UsingPole)
	reach, high := (pole+1)<<3, (pole+1)<<5
	pz := ThePlayer.Z()
	if pz > reach {
		pz -= reach
	} else {
		pz = 0
	}
	oz := obj.Z()
	if oz < pz || pz+high < oz {
		return true
	}
	x, y := px, py
	for {
		x += xdir
		if xdir < 0 && x < MapObjX || xdir > 0 && x > MapObjX {
			x = MapObjX
		}
		y += ydir
		if ydir < 0 && y < MapObjY || ydir > 0 && y > MapObjY {
			y = MapObjY
		}
		tile := MapGetAddr(x, y)
		bridge := BridgeHeight(tile)
		found := bridge >= 0 && bridge <= oz && bridge >= pz
		if !found && pz>>3 > tile.Height() && dist == 0x90 {
			return true
		}
		if x == MapObjX && y == MapObjY {
			break
		}
		if dist <= 0x90 && !found && terrainOf(tile) != 0 && terrainOf(tile) != terr {
			return true
		}
	}
	return false
}

// pick3d is the object drawn at the pointer in the 3D view, or nil.
func pick3d(how int) *Object {
	do3dGrab()
	c := pickPixel(inplist.x, inplist.y)
	idx := 0
	pTxtId = 0
	if c >= 1 && c < int(PickUp) {
		idx = int(colorToObj[c-1])
		PickMap = &Level.Tiles[int(colorToMap[c-1])&(MAP_TILES-1)]
	} else if c >= PICK_WALL && c <= PICK_CEILING {
		pTxtId = c - 0xBF
	}
	if idx == 0 {
		return nil
	}
	obj := ObjIntTMem(idx)
	co := &ComObjData[obj.Item()]
	releasePtr = &PickMap.Objects
	releaseable = co.Pickable() != 0 && !IsMobElem(obj)
	return obj
}

// lookNothing describes the texture clicked, or prints string 0x98 + how.
func lookNothing(how, txt int) {
	if txt > 0 && how == 2 {
		txt--
		var t int
		if txt < 0x30 {
			t = int(TxmID[txt])
		} else if txt < 0x3A {
			t = 0x1FE - int(floorIDs[txt-0x30])
		} else {
			t = 0x1FF
		}
		scrollPrint("You see ")
		scrollPrint(getString(t | STR_TEXTURES))
		scrollPrint(".\n")
	} else {
		gameSprint(how + 0x98)
	}
}

// release3d takes a picked-up object off its tile, firing its pick-up triggers.
func release3d(obj *Object) {
	if releaseable {
		checkTrap(ThePlayer, obj, 2, MapObjX, MapObjY)
		ObjRem(releasePtr, obj)
		editchng(2)
		releaseable = false
	}
}

// player3dget picks up newPlObj into the cursor if it can be taken.
func player3dget() {
	var split *Object
	inrange := InPickRange(PickDist, newPlObj, PickMap)
	clear := !BlockingTerrain(PickDist, newPlObj)
	switch {
	case releaseable && inrange && clear:
		if newPlObj.IsQuant() != 0 && newPlObj.LinkF()&LINK_SPECIAL == 0 && newPlObj.LinkF() != 1 {
			if split = AskHowMany(newPlObj); split == nil {
				return
			}
			if split != newPlObj {
				ObjAdd(&newPlObj.QN, split)
			}
		}
		if !EncumCheck(newPlObj) {
			if split != nil && split != newPlObj {
				newPlObj.SetLinkF(newPlObj.LinkF() + split.LinkF())
				ObjRem(&newPlObj.QN, split)
			}
			gameSprint(0x5F)
		} else {
			if HasOrIsObj(newPlObj, ITEM_MOONSTONE) {
				player.SetMoonstone(0)
			}
			playerGrabbed(newPlObj, 0)
			release3d(newPlObj)
			GameInputMode = GIM_CARRY
			DoInventoryDrag(newPlObj)
		}
	case releaseable:
		gameSprint(b2i(inrange) + 0x5D)
		mouseRelease(true)
	case defMode:
		if IsMobElem(newPlObj) && newPlObj.Major() == MAJOR_CREATURE {
			player3dtalk()
		} else {
			player3duse()
		}
	default:
		if newPlObj.Item() == ITEM_SILVER_TREE {
			if inrange && clear {
				UseObj(ThePlayer, newPlObj, 0)
			}
		} else {
			gameSprint(0x60)
		}
		mouseRelease(true)
	}
}

func player3dtalk() {
	mouseRelease(true)
	TalkTo(newPlObj)
}

// player3dlook looks at newPlObj, maybe disarming a trap in look mode.
func player3dlook() {
	if InPickRange(0x48, newPlObj, PickMap) && !releaseable {
		LookAt(newPlObj, 1)
	} else {
		LookAt(newPlObj, 0)
	}
	if RightButtonThing == IMODE_LOOK {
		if DetectedTrap(newPlObj, int(player.Skills[SKILL_SEARCH])) > 0 {
			yes := true
			r := wyorn("", 0xF4, &yes)
			if r != 0 && r < 4 {
				yes = r == 2
				wdBool(yes)
			}
			scrollPrint("\n")
			if yes {
				RemoveTrap(newPlObj, int(player.Skills[SKILL_TRAPS]))
			}
		}
	} else {
		defMode = true
	}
	checkTrap(ThePlayer, newPlObj, 5, MapObjX, MapObjY)
	if RightButtonThing != IMODE_LOOK {
		if mouseDragged(true) {
			player3dget()
		}
	} else {
		mouseRelease(true)
	}
	defMode = false
}

// player3duse uses newPlObj if it is in reach.
func player3duse() {
	mouseRelease(true)
	if InPickRange(PickDist, newPlObj, PickMap) && !BlockingTerrain(PickDist, newPlObj) {
		UseObj(ThePlayer, newPlObj, 0)
	} else if int(newPlObj.ID&0x1FE) != ITEM_TMAP_C {
		gameSprint(0xB9)
	}
}

// player3dattack: the swing is the ninth of the view clicked.
func player3dattack() {
	n := inplist.x*3/(PWid+2) + inplist.y*3/(PHgt+2)*3
	playerAttack(n + 1)
}

var playerDisp = [5]func(){player3duse, player3dattack, player3dlook, player3dget, player3dtalk}

// mousIn3d is the 3D view's mouse handler.
func mousIn3d(int) {
	if inplist.cmd&1 != 0 {
		playerMousMove()
	}
	if inplist.mode == MODE_GAME {
		newPlObj = nil
		if inplist.cmd&2 == 0 {
			return
		}
		switch GameInputMode {
		case GIM_NONE:
			how := 2
			if RightButtonThing != IMODE_DEFAULT {
				how = RightButtonThing - 1
			}
			if how != 1 {
				if inplist.cmd&1 != 0 {
					break
				}
				newPlObj = pick3d(2)
				if newPlObj == nil {
					lookNothing(how, pTxtId)
					mouseRelease(true)
					break
				}
			}
			playerDisp[how]()
		case GIM_CARRY:
			DoSpecialActions(0x17)
			mouseRelease(true)
		case GIM_TARGET:
			if newPlObj = pick3d(2); newPlObj != nil {
				if InPickRange(PickDist, newPlObj, PickMap) && !BlockingTerrain(PickDist, newPlObj) {
					ObjectActor(newPlObj, 1, 0)
				} else {
					gameSprint(0x5E)
				}
			}
			if CursorObjPtr != nil {
				unforceMouseCursor(3)
				CursorObjPtr = nil
				GameInputMode = GIM_NONE
			}
			mouseRelease(true)
		case GIM_AIM:
			BlastFunction()
		}
	} else if inplist.mode == 0x10 && inplist.cmd == 2 {
		if newPlObj = pick3d(2); newPlObj != nil {
			player3duse()
		}
	}
}

// invLook looks at an object in the inventory, identifying it with the Lore skill.
func invLook() {
	checkTrap(ThePlayer, newPlObj, 5, MapObjX, MapObjY)
	if newPlObj == nil {
		newPlObj = pickInv(2)
	}
	lore := 1
	if o := newPlObj; o != nil && o.Major() != MAJOR_RECT && o.Major() != MAJOR_TRAP &&
		ComObjData[o.Item()].Render() != 2 {
		head := o.HeadingBits()
		if head&4 != 0 {
			lore = head & 3
		} else {
			lore = skillCheck(int(player.Skills[SKILL_LORE]), 0xA) + 1
			if lore == 0 {
				lore = 1
			}
			if head&3 > lore {
				lore = head & 3
			}
			o.SetHeading(lore | 4)
		}
	}
	LookAt(newPlObj, lore)
	DoInventoryMouse(-1)
}

// mousInInv handles inventory panel clicks by GameInputMode.
func mousInInv() {
	newPlObj = nil
	switch GameInputMode {
	case GIM_NONE:
		if CursorObjPtr != nil {
			DoInventoryMouse(0)
		} else {
			switch inplist.cmd {
			case 1, 3:
				DoInventoryMouse(0)
			case 2:
				if RightButtonThing == IMODE_USE && inplist.mode != MODE_CONV {
					DoInventoryMouse(0)
				} else {
					DoInventoryMouse(-2)
				}
			}
		}
	case GIM_CARRY:
		DoInventoryMouse(4)
	case GIM_TARGET:
		hit := FindInventoryHit(inplist.x+0xF0, inplist.y+0x52)
		if hit == 0x15 || hit == 0x16 {
			DoInventoryMouse(1)
		} else if newPlObj = pickInv(2); newPlObj != nil {
			ObjectActor(newPlObj, 1, 1)
			mouseRelease(true)
		} else {
			unforceMouseCursor(3)
			CursorObjPtr = nil
			GameInputMode = GIM_NONE
		}
	}
}

// mousInPanel is the right panel's mouse handler.
func mousInPanel(any) {
	switch RightPanel {
	case PANEL_INV:
		mousInInv()
	case PANEL_RUNES:
		mousInRune()
	case PANEL_STATS:
		mousInStat()
	}
}

// newIconSelect draws interaction icon mode (1 to 5) selected.
func newIconSelect(mode int) {
	mode--
	mouseHide()
	picToScreen(ICON_LFTI+0xB-mode*2, iconX[mode], iconY[mode])
	mouseShow()
}

func newIconUnselect(mode int) {
	mode--
	mouseHide()
	picToScreen(ICON_LFTI+0xA-mode*2, iconX[mode], iconY[mode])
	mouseShow()
}

func iconCursor(m int) bool { return m == IMODE_USE || m == IMODE_LOOK || m == IMODE_GET }

// dealWithIcons handles a click on an interaction icon (-1: find which from the pointer).
func dealWithIcons(mode int) {
	if GameInputMode != GIM_NONE {
		return
	}
	if mode == -1 {
		if LeftPanel != 0 {
			optionsMouseClick(inplist.x, inplist.y)
		} else if mode = (inplist.y + 2) / 0x12; mode > 5 {
			return
		}
	}
	if mode == -1 {
		return
	}
	if mode == 5 {
		runOptionsPanel(1)
		return
	}
	setScreenFrame(SCR_WEAPON, WEAP_SHEATHED)
	player.SetDrawn(0)
	if iconCursor(RightButtonThing) {
		unforceMouseCursor(3)
	}
	if mode++; mode == RightButtonThing {
		newIconUnselect(RightButtonThing)
		RightButtonThing = IMODE_DEFAULT
	} else {
		if RightButtonThing != IMODE_DEFAULT {
			newIconUnselect(RightButtonThing)
		}
		RightButtonThing = mode
		if RightButtonThing == IMODE_FIGHT {
			if player.MotionState&1 == 0 {
				player.SetDrawn(1)
				setScreenFrame(SCR_WEAPON, WEAP_READY)
				newIconSelect(RightButtonThing)
				if m := getCurrentMusic(); m < MUSIC_FOE_HURT || m > MUSIC_DANGER {
					setNewMusic(MUSIC_ARMED)
				}
			} else {
				RightButtonThing = IMODE_DEFAULT
			}
		} else {
			newIconSelect(RightButtonThing)
		}
	}
	if player.Drawn() == 0 && getCurrentMusic() == MUSIC_ARMED {
		seg014_1DC5_15C5()
	}
	mouseRelease(true)
	if iconCursor(RightButtonThing) {
		forceMouseCursor(ICON_CURSORS + 0xB)
	}
}

// pickFightmode draws the weapon.
func pickFightmode() {
	if player.Drawn() == 1 || player.MotionState&1 != 0 {
		return
	}
	if iconCursor(RightButtonThing) {
		unforceMouseCursor(3)
	}
	if RightButtonThing != IMODE_DEFAULT {
		newIconUnselect(RightButtonThing)
	}
	RightButtonThing = IMODE_FIGHT
	player.SetDrawn(1)
	setScreenFrame(SCR_WEAPON, WEAP_READY)
	newIconSelect(RightButtonThing)
	if m := getCurrentMusic(); m < MUSIC_FOE_HURT || m > MUSIC_DANGER {
		setNewMusic(MUSIC_ARMED)
	}
}

// puntFightmode sheathes the weapon and leaves fight mode.
func puntFightmode() {
	if player.Drawn() != 0 {
		setScreenFrame(SCR_WEAPON, WEAP_SHEATHED)
		player.SetDrawn(0)
		if LeftPanel == 0 {
			newIconUnselect(IMODE_FIGHT)
		}
		RightButtonThing = IMODE_DEFAULT
		clearFightState()
		seg014_1DC5_15C5()
	}
}

func toggleFightmode() {
	if player.Drawn() != 0 {
		puntFightmode()
	} else {
		pickFightmode()
	}
}

// checkSave refuses saving while an action is in progress and on level 9.
func checkSave() bool {
	id := 0
	if GameInputMode != 0 {
		id = 0xA0
	}
	if PlayerLevel == 9 {
		id = 0x9F
	}
	if id != 0 {
		gr.GrSoftPageFlip()
		gameSprint(id)
		gr.GrSoftPageFlip()
		return false
	}
	return true
}

// checkRest: restoring is always allowed; an action in progress is abandoned.
func checkRest() bool {
	if GameInputMode != 0 {
		GameInputMode = GIM_NONE
		unforceMouseCursor(0)
	}
	return true
}

// MapObjX and MapObjY are the tile of the object being handled (set by InPickRange and
// the like).
var MapObjX, MapObjY int
