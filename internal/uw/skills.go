package uw

import (
	"strconv"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of SKILLS.C (skills, levelling, mantras, end-game statistics, sleep and dreams,
// eating, the silver tree, death and traps), SKILLCHK.C (skill checks, experience) and
// PLAYTIME.C (the player's timed updates).

var (
	skillRng    = [3]int{25, 40, 10}
	EndGameMode bool
	levelTable  = [15]byte{1, 2, 3, 4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192}
	plyregen    [2]byte
)

func useSkill(who *Object, skill, value int) bool {
	if who != ThePlayer {
		return false
	}
	switch skill {
	case SKILL_TRACK:
		mdetect(8, value)
	case SKILL_TRAPS, SKILL_SEARCH:
	default:
		scrollPrint(getString((skill + STRN_SKILLS) | STR_CHARGEN))
		scrollPrint("\n")
	}
	return true
}

func playerUseSkill(skill int) bool {
	return useSkill(ThePlayer, skill+SKILL_TRAPS, int(player.Skills[skill+SKILL_TRAPS]))
}

// playerCompute derives maximum HP, mana and carrying weight; restore fills the mana.
func playerCompute(restore bool) {
	playerdat.Avghit = byte(30 + int(player.Level)*int(playerdat.Attr[ATTR_STR])/5)
	mana := (int(player.Skills[SKILL_MANA]) + 1) * int(playerdat.Attr[ATTR_INT]) >> 3
	if PlayerLevel == LEVEL_TYBAL {
		player.SavedMana = byte(mana)
	} else {
		player.MaxMana = byte(mana)
	}
	player.MaxWeight = uint16(playerdat.Attr[ATTR_STR]) * 2 * 10
	if restore {
		player.PlayMana = player.MaxMana
	}
}

func advance(levels int) {
	s := []byte(" 0\n")
	player.Level += byte(levels)
	if player.Level >= 10 {
		s[0] = player.Level/10 + '0'
	} else {
		s[0] = ' '
	}
	s[1] = player.Level%10 + '0'
	gameSprint(0x93)
	scrollPrint(string(s))
	player.SkillPoints += byte(levels)
	playerCompute(false)
	panelCheck()
}

func prime(skill int) int {
	if skill < SKILL_MANA {
		return 0
	}
	if skill < SKILL_TRAPS {
		return 2
	}
	return 1
}

func addToSkill(skill int) {
	var rolls, base, divisor int
	if player.Skills[skill] != 0 {
		base, divisor, rolls = 1, 13, 2
	} else {
		base, divisor, rolls = 3, 9, 3
	}
	stat := int(playerdat.Attr[prime(skill)])
	player.Skills[skill] += byte(base)
	player.Skills[skill] += byte(stat / divisor)
	player.Skills[skill] += byte(rand() * rolls / 0x8000)
	for ; rolls > 0; rolls-- {
		player.Skills[skill] += byte(skillCheck(stat, 20))
	}
	if player.Skills[skill] > 30 {
		player.Skills[skill] = 30
	}
}

func getSkill(skill int) bool {
	result := true
	stat := prime(skill)
	statRng := skillRng[stat]
	attribute := int(playerdat.Attr[stat])
	if attribute*2 < int(player.Skills[skill]) || player.Skills[skill] >= 30 {
		result = false
	} else {
		player.Skills[skill]++
		if stat != 0 && attribute/2 > int(player.Skills[skill]) {
			player.Skills[skill]++
		}
		if int(player.Skills[skill]) < attribute && rand()%statRng < attribute-int(player.Skills[skill]) {
			player.Skills[skill]++
		}
		if player.Skills[skill] > 30 {
			player.Skills[skill] = 30
		}
	}
	if skill == SKILL_LORE {
		clearAllLoretries()
		if PlayerLevel <= 8 {
			player.Lore[PlayerLevel] = player.Skills[SKILL_LORE]
		}
	}
	return result
}

func greatAdvance(skill int, ok bool) {
	if ok {
		gameSprint(0x1C)
		scrollPrint(getString((skill + STRN_SKILLS) | STR_CHARGEN))
		scrollPrint(".\n")
	} else {
		gameSprint(0x1B)
	}
}

func reportAdvance(list []byte) {
	if list[0] == 0xFF {
		gameSprint(0x1E)
		return
	}
	gameSprint(0x1D)
	for i := 0; i < 4 && list[i] != 0xFF; i++ {
		if i == 3 || (i != 0 && (i+1 >= len(list) || list[i+1] == 0xFF)) {
			scrollPrint(" and ")
		} else if i != 0 {
			scrollPrint(", ")
		}
		scrollPrint(getString((int(list[i]) + STRN_SKILLS) | STR_CHARGEN))
	}
	scrollPrint(".\n")
}

// mantraAdvance is chanting at a shrine.
func mantraAdvance() {
	_, text := wdialog("Chant the mantra: ", nil, true, 0xA)
	scrollPrint("\n")
	mantra := STRN_MANTRAS
	up := strUpper(text)
	for ; mantra < STRN_MANTRAS+NUM_MANTRAS; mantra++ {
		if getString(mantra|STR_CHARGEN) == up {
			break
		}
	}
	if mantra == STRN_MANTRAS+NUM_MANTRAS {
		gameSprint(0x19)
	} else {
		mantra -= STRN_MANTRAS
		if mantra < NUM_SKILLS {
			if player.SkillPoints == 0 {
				gameSprint(0x18)
			} else {
				r1 := getSkill(mantra)
				r2 := getSkill(mantra)
				if r1 || r2 {
					gameSprint(0x1A)
					player.SkillPoints--
				}
				greatAdvance(mantra, r1 || r2)
			}
		} else {
			var base, size, tries int
			switch mantra - NUM_SKILLS {
			case 0:
				if player.Cup() == 0 {
					printPathTo(getString(0x223), ThePlayer.HomeX(), ThePlayer.HomeY(), PlayerLevel, 0x18, 0x2D, 3, 4)
				}
				waitTicks(0x20)
				return
			case 1:
				if player.Key() == 0 && placeNew(nil, ITEM_KEY_OF_TRUTH) != nil {
					gameSprint(0x1E)
					questChanged()
					player.SetKey(1)
				}
				waitTicks(0x20)
				return
			case 2:
				gameSprint(0x1F)
				waitTicks(0x20)
				return
			case 3:
				base, size, tries = SKILL_ATTACK, 7, 3
			case 4:
				base, size, tries = SKILL_MANA, 3, 2
			case 5:
				base, size, tries = SKILL_TRAPS, 10, 4
			default:
				waitTicks(0x20)
				return
			}
			if player.SkillPoints == 0 {
				gameSprint(0x18)
			} else {
				found := 0
				list := []byte{0xFF, 0xFF, 0xFF, 0xFF}
				count := size
				for tries != 0 {
					c := count
					count--
					if c == 0 {
						break
					}
					var skill int
					if base == SKILL_MANA && player.Skills[SKILL_MANA] < 8 && rand()&2 != 0 {
						skill = SKILL_MANA
					} else {
						skill = base + rand()*size/0x8000
					}
					if getSkill(skill) {
						list[found] = byte(skill)
						found++
					}
					tries--
				}
				reportAdvance(list)
				player.SkillPoints--
			}
		}
		playerCompute(false)
		panelCheck()
	}
	FixPlayerEquips()
	waitTicks(0x20)
	mouseClearQ()
	flushKeys()
}

// gameStats draws the end-of-game statistics.
func gameStats() {
	grfxLoadFont("fontchar.sys")
	gr.ForegroundColor, gr.BackgroundColor = 0x5C, 0x5C
	fh := gr.CurFont.Height
	str := getString(playerNameHandle)
	y := 0xB4
	gr.StringToScreen(str, 0xA0-gr.StringWidth(str)/2, y)
	text := getString(0x2BB) + strconv.Itoa(int(player.Level)) + " " +
		getString((player.PClass()+STRN_CLASSES)|STR_CHARGEN)
	y -= fh
	gr.StringToScreen(text, 0xA0-gr.StringWidth(text)/2, y)
	str = getString(0x2BC)
	y -= fh
	gr.StringToScreen(str, 0xA0-gr.StringWidth(str)/2, y)
	hours := int(player.GameClock / 0x1C2000)
	days := hours / 12
	text = getString(0x2BD) + strconv.Itoa(days) + getString(0x2BE)
	y -= fh
	gr.StringToScreen(text, 0xA0-gr.StringWidth(text)/2, y)
	y -= fh
	for i := 0; i < 6; i++ {
		x := 0x50
		if i/3 != 0 {
			x = 0xBE
		}
		value := i % 3 * fh
		str = getString((i + STRN_STATS) | STR_CHARGEN)
		switch i {
		case 0, 1, 2:
			text = strconv.Itoa(int(playerdat.Attr[i]))
		case 3:
			text = strconv.Itoa(int(playerdat.Avghit))
		case 4:
			text = strconv.Itoa(int(player.MaxMana))
		case 5:
			text = strconv.Itoa(int(player.Exp / 10))
		}
		gr.StringToScreen(str, x, y-value)
		gr.StringToScreen(text, x+0x2D, y-value)
	}
	y -= fh * 2
	for i := 0; i < NUM_SKILLS; i++ {
		value := int(player.Skills[i])
		str = getString((i + STRN_SKILLS) | STR_CHARGEN)
		text = strconv.Itoa(value)
		x := i%3*0x4A + 0x32
		if i%3 == 0 {
			y -= fh
		}
		gr.StringToScreen(str, x, y)
		gr.StringToScreen(text, x+0x46-gr.StringWidth(text), y)
	}
}

// dream shows one of Garamon's dreams after a night's sleep; true if a cutscene played.
func dream(sleepfactor int) bool {
	found := -1
	dreams := int(player.Dreams)
	if dreams&1 != 1 {
		found = 0
	} else if PlayerLevel > 1 && dreams&2 != 2 {
		found = 1
	} else if dreams&4 == 4 {
		found = 2
	} else if dreams&8 == 8 {
		found = 3
	}
	if found < 0 {
		if rand()%(4+sleepfactor<<2) == 0 {
			found = 4 + rand()%6
			if 1<<found&dreams != 0 {
				found = -1
			}
		}
	}
	if found >= 0 && player.Garamon() == 0 {
		runcutscene(found + 0x18)
		player.Dreams ^= 1 << found
		return true
	}
	timer := GameTime()
	for timer+0x180 > GameTime() {
	}
	return false
}

func dropDrunkPlayer() {
	if player.MotionState&(MS_SWIM|MS_LAVA) != 0 {
		damageItem(ThePlayer, nil, 0, 0, 0xFF, 0)
	}
	FixPlayerEquips()
	finishPlayer()
	if player.MotionState&MS_FLOAT != 0 && motionbits&(MB_SLOW_FALL|MB_LEVITATE|MB_FLY) == 0 {
		damage := rand()%6*10 + 12
		damageItem(ThePlayer, nil, 0, 0, damage, DMG_POISON)
	}
	if player.MotionState&(MS_SWIM|MS_LAVA) != 0 {
		damageItem(ThePlayer, nil, 0, 0, 0xFF, 0)
	}
}

// playerSleep sleeps: how 0 camping, 1 a bedroll, negative passing out.
func playerSleep(how int) {
	fade := true
	if how >= 0 {
		if player.MotionState&0x1B != 0 || PN.Acc[2] != 0 || PlayerLevel == LEVEL_VOID {
			gameSprint(0x14)
			return
		}
		if hostileCreaturesNear() {
			gameSprint(0xE)
			return
		}
		gameSprint(0xF)
	}
	renderFB()
	setNewMusic(MUSIC_MAPS)
	changeMusicMaybe()
	fadeout3d(5)
	if how >= 0 {
		gameSprint(0x10)
	}
	DoClosingDoors(0)
	ObjGarbageCollect(1, 0x14)
	hours := rand()%5 + 2
	player.GameClock += uint32(hours * 0xE1000)
	player.SetActiveSpells(0)
	player.SetShrooms(0)
	DegradeLights(hours*180, 0)
	if player.Poison() != 0 {
		comfort := (player.Poison() + 1) * player.Poison() >> 1
		damageItem(ThePlayer, nil, 0, 0, comfort, DMG_POISON)
		player.SetPoison(0)
	}
	if how < 0 {
		dropDrunkPlayer()
	}
	if ThePlayer.HP == 0 {
		setRandomWalkingMusic()
		return
	}
	if wanderingMonsterCheck() {
		if player.Fatigue > 0x20 {
			player.Fatigue -= 0x20
		} else {
			player.Fatigue = 0
		}
		gameSprint(0x15)
		playerEat(-12 - rand()&0xF)
		if player.Drunk() < 0x10 {
			player.SetDrunk(0)
		} else {
			player.SetDrunk(player.Drunk() - 0x10)
		}
	} else {
		updateAllCrittersWhilstPlayerSnoozes()
		DoWanderingMonsters(0)
		hours = rand()%4 + 7 - hours
		if ThePlayer.HP < 10 {
			hours += rand()%2 + 1
		}
		player.GameClock += uint32(hours * 0xE1000)
		DegradeLights(hours*180, 0)
		comfort := b2i(player.Hunger > 0x40 && how > 0)
		regen := int(player.Fatigue)/2 + 2
		if regen > 5 {
			regen = 5
		}
		player.Fatigue = 0
		if player.Hunger != 0 {
			restoreHp(ThePlayer, regen+regen*comfort-1)
			restoreMana(ThePlayer, -6)
			restoreMana(ThePlayer, regen+(regen+1)*comfort-1)
		} else {
			gameSprint(0x11)
			damageItem(ThePlayer, nil, 0, 0, 2, 0)
		}
		playerEat(-24 - rand()&0x1F)
		if player.Drunk() < 0x20 {
			player.SetDrunk(0)
		} else {
			player.SetDrunk(player.Drunk() - 0x20)
		}
		if how >= 0 {
			fade = !dream(comfort)
		}
		gameSprint(0x13 - comfort)
	}
	FixPlayerEquips()
	NightCleanCritPages()
	PN.Speed = 0
	PN.Acc = [3]int16{}
	PN.Vel = [3]int16{}
	panelCheck()
	renderFB()
	setRandomWalkingMusic()
	if fade {
		fadein3d(5)
	} else {
		sendFB()
	}
}

func playerKeySleep(bedroll int) {
	var where int
	if FindObj(4, 2, 1, 4, &where) == nil {
		bedroll = 0
	} else {
		bedroll = 1
	}
	playerSleep(bedroll)
}

// playerEat changes hunger by nutrition; false, eating nothing, when too full.
func playerEat(nutrition int) bool {
	value := int(player.Hunger) + nutrition
	if value > 0xFF {
		return false
	}
	if value < 0 {
		player.Hunger = 0
	} else {
		player.Hunger = byte(value)
	}
	if nutrition > 0 {
		value = int(player.FoodHeal) / 8
		if value > 8 {
			value = 8
		}
		getHpBack(ThePlayer, value)
		player.FoodHeal = 0
	}
	return true
}

// checkVictory runs every frame: the ending once won; the moongate once the last
// talisman is destroyed.
func checkVictory() {
	if EndGameMode {
		inplist.mode = 0
		LeftPanel = 2
		runcutscene(1)
		mouseHide()
		grfxClear()
		loadBitMap(7, "DATA\\win1.byt")
		for mouseGetInput() < 0 {
		}
		gr.GrSoftPageFlip()
		loadBitMap(-1, "DATA\\win2.byt")
		gameStats()
		gr.GrSoftPageFlip()
		gr.CopyVisibleToHidden()
		for mouseGetInput() < 0 {
		}
		realDeath(0)
		EndGameMode = false
	} else if player.Talismans == 0 {
		var head *Link
		gate := CreateObj(ITEM_MOONGATE, false)
		if gate != nil {
			gate.SetIsQuant(1)
			gate.SetLinkF(0x2C0)
			head = &MapGetAddr(0x20, 0x20).Objects
			ObjAddEnd(head, gate)
		}
		gameSprint(0x117)
		vortex(-1)
		fadeout3d(5)
		if gate != nil {
			ObjRem(head, gate)
			ObjFree(gate)
		}
		doTeleport(ThePlayer, 0x1B, 0x17, 9)
		questChanged()
		player.Talismans = 0xFF
		gameSprint(0x118)
		NewPlyFade &= 0xFE
		newPlayerPos()
		NewPlyFade |= 1
	}
}

// plantSeed plants the silver seed ahead of the player: 1 rooted, 0 not, -1 not here.
func plantSeed() int {
	if PlayerLevel == LEVEL_VOID {
		return -1
	}
	x := PN.X >> 5
	y := PN.Y >> 5
	moveAlong(int(PlayerFacing>>8), 0xB, &x, &y)
	tile := MapGetAddr(int(x>>3), int(y>>3))
	if tile == nil || tile.Type() != TILE_OPEN {
		return 0
	}
	terrain := int(floorIDs[tile.Floor()])
	if (terrain < 5 || terrain > 11) && (terrain < 18 || terrain > 22) &&
		(terrain < 27 || terrain > 31) && (terrain < 35 || terrain > 40) {
		return 0
	}
	z := tile.Height() << 3
	if canPlace(ITEM_SILVER_TREE, 0, int(x), int(y), z, false, 0) {
		tree := CreateObj(ITEM_SILVER_TREE, false)
		tree.SetZ(z)
		tree.SetFineX(int(x) & 7)
		tree.SetFineY(int(y))
		tree.SetDoorDir(1)
		if addAnimobj(ObjMemTPtr(tree), -1, 0, int(x>>3), int(y>>3)) != 0 {
			player.SetTree(PlayerLevel)
			ObjAdd(&tile.Objects, tree)
			return 1
		}
		ObjFree(tree)
	}
	return 0
}

func moveto(level, item int) bool {
	x, y := 0, 0
	nppFunc = nil
	if level == PlayerLevel {
		ObjFindInMap(item>>6, (item&ID_MINOR)>>4, item&ID_INCLASS, &x, &y)
		dprintf("At %d %d\n", x, y)
		NewPlayerX = x
		NewPlayerY = y
		return true
	}
	return false
}

func doResurrect() {
	if moveto(player.Tree(), ITEM_SILVER_TREE) {
		if playerdat.Avghit > 8 {
			ThePlayer.HP = byte(int(playerdat.Avghit) - 2 - rand()*3/0x8000)
		} else {
			ThePlayer.HP = playerdat.Avghit
		}
		player.PlayMana = player.MaxMana
		if player.MaxMana > 8 {
			player.PlayMana -= player.MaxMana/8 + 2
		}
		ThePlayer.SetSeq(0x2C)
		player.SetPoison(0)
		player.SetActiveSpells(0)
		FixPlayerEquips()
		setNewMusic(4)
	}
}

func doMstone() { moveto(player.Moonstone(), ITEM_MOONSTONE) }

// playerIsDead: HP has reached 0.
func playerIsDead() {
	if player.Talismans == 0 {
		ThePlayer.HP = 4
		return
	}
	killAllEffects()
	loadNewMusic(MUSIC_DEATH, true)
	playerGetExp(-int(player.Exp >> 3))
	renderFB()
	fadeout3d(5)
	clearFightState()
	if CursorObjPtr != nil {
		if GameInputMode == 1 || GameInputMode == 0 {
			nearMobPutAt(ThePlayer, CursorObjPtr, 6, false)
			CursorObjPtr = nil
			GameInputMode = 0
			unforceMouseCursor(3)
		} else if GameInputMode == 2 {
			CursorObjPtr = nil
			GameInputMode = 0
			unforceMouseCursor(3)
		}
	}
	bones := CreateObj(rand()%5+ITEM_SKULL_C2, false)
	if putAt(int(PN.X>>5), int(PN.Y>>5), int(PN.Z>>3), bones, 0, true) {
		bones.SetZ(ThePlayer.Z())
		bones.SetOwner(0x3F)
		bones.SetFineX(ThePlayer.FineX())
		bones.SetFineY(ThePlayer.FineY())
		objDeal(bones, int(PN.X>>8), int(PN.Y>>8), 1)
	}
	if player.Tree() != 0 && PlayerLevel != LEVEL_VOID {
		doTeleport(ThePlayer, 0x3F, 0x3F, player.Tree())
		nppFunc = doResurrect
		NewPlyFade = 0
		ok := newPlayerPos()
		NewPlyFade = 3
		if ok {
			runcutscene(0x102)
			cFillFB(0xF1)
			scrollClear(true)
		} else {
			realDeath(1)
		}
	} else {
		realDeath(1)
	}
}

func DetectedTrap(obj *Object, skill int) int {
	if obj.IsQuant() != 0 || obj.LinkF() == 0 {
		return 0
	}
	head := &obj.OL
	trap := ObjInList(&head, false, MAJOR_TRAP, -1, -1)
	if trap != nil {
		if trap.Minor() >= MINOR_TRIGGER {
			trap = ObjPtrTMem(&trap.OL)
		}
		if trap.InMajor() <= 2 {
			return skillCheck(skill, 8)
		}
	}
	return 0
}

func RemoveTrap(obj *Object, skill int) int {
	result := 0
	if obj.IsQuant() != 0 || obj.LinkF() == 0 {
		return 0
	}
	head := &obj.OL
	trig := ObjInList(&head, false, MAJOR_TRAP, -1, -1)
	if trig != nil {
		var trap *Object
		if trig.Minor() >= MINOR_TRIGGER {
			trap = ObjPtrTMem(&trig.OL)
		} else {
			trap, trig = trig, nil
		}
		if trap.InMajor() <= 2 {
			nameOf := func(o *Object) string {
				n, ok := getName(o, false, false)
				if !ok {
					return "UNNAMED"
				}
				return n
			}
			result = skillCheck(skill, 8)
			if result > 0 {
				scrollPrint("The ")
				scrollPrint(nameOf(trap))
				scrollPrint(" on the ")
				scrollPrint(nameOf(obj))
				scrollPrint(" was successfully dearmed.\n")
				ObjFreeChain(head)
			} else if result < 0 {
				scrollPrint("Your bumbling attempts have set off the ")
				scrollPrint(nameOf(trap))
				scrollPrint(".\n")
				if trig != nil {
					UseTrigger(ThePlayer, obj, trig, -1)
				} else {
					SetOffTrap(ThePlayer, obj, trap, MapObjX, MapObjY)
					deleteTrap(head, trap)
				}
			} else {
				scrollPrint("Unable to defuse trap.\n")
			}
		}
	}
	return result
}

// SKILLCHK.C

func skillCheck(value, target int) int {
	score := value - target + rand()%31
	if score > 28 {
		return 2
	}
	if score > 15 {
		return 1
	}
	if score > 2 {
		return 0
	}
	return -1
}

func playerGetExp(n int) {
	if n < 0 {
		if uint32(-n) > player.Exp {
			player.Exp = 0
		} else {
			player.Exp += uint32(n)
		}
		return
	}
	if player.Exp > 0x17700 {
		return
	}
	if PlayerLevel*2+2 < int(player.Level) {
		n = n/2 + 1
	}
	if points := (int(player.Exp) + n) / 3000; points > int(player.SkillPointsEarned) {
		player.SkillPoints += byte(points - int(player.SkillPointsEarned))
		player.SkillPointsEarned = byte(points)
	}
	redraw := player.Exp >> 4
	player.Exp += uint32(n)
	doRedraw := player.Exp>>4 > redraw
	levels := 0
	points := int(player.Exp / 500)
	for int(levelTable[int(player.Level)+levels-1]) <= points && int(player.Level)+levels < 16 {
		levels++
	}
	if levels != 0 {
		advance(levels)
	}
	if doRedraw {
		panelCheckHpmp()
	}
}

func panelCheckHpmp() {
	if RightPanel == 2 {
		gr.ForegroundColor = 0xF1
		gr.BackgroundColor = 0xF1
		mouseHide()
		grfxLoadFont("font5x6i.sys")
		if spsave[1] != nil {
			spsave[1].Restore()
		}
		spHp()
		spMp()
		spXp()
		grfxLoadFont("font5x6p.sys")
		mouseShow()
	}
}

// PLAYTIME.C

func dispelSpell(i *int) bool {
	s := player.Spells[*i]
	cls := int(s & 0xF)
	sub := int(s&0xF0) >> 4
	if cls == SPELLC_MOTION && (sub == 3 || sub == 5) {
		player.Spells[*i] = player.Spells[*i]>>8<<8 + 0x21
		player.Spells[*i] = player.Spells[*i]&0xFF + 0x100
	} else {
		if cls == SPELLC_XT && sub == 1 {
			attachEye(1)
			GameInputMode -= 8
		}
		if cls == SPELLC_MOTION {
			fizUpdate = true
		}
		player.SetActiveSpells(player.ActiveSpells() - 1)
		j := *i
		*i--
		if j < player.ActiveSpells() {
			player.Spells[*i+1] = player.Spells[player.ActiveSpells()]
		}
	}
	return true
}

func durationCheck() {
	changed := false
	plyregen[1]++
	for i := 0; i < player.ActiveSpells(); i++ {
		stab := int(player.Spells[i] >> 8)
		if stab == 1 {
			changed = dispelSpell(&i)
		} else {
			player.Spells[i] = player.Spells[i]&0xFF + uint16(stab-1)<<8
		}
	}
	if DegradeLights(1, int(plyregen[1])) {
		changed = true
	}
	if player.Shrooms() != 0 {
		player.SetShrooms(player.Shrooms() - 1)
		if player.Shrooms() == 0 {
			changed = true
		}
	}
	if changed {
		FixPlayerEquips()
		editchng(2)
	}
	if plyregen[0] != 0 {
		if plyregen[0]&1 != 0 {
			restoreHp(ThePlayer, -1)
		}
		if plyregen[0]&2 != 0 {
			restoreMana(ThePlayer, -1)
		}
	}
	if player.SwimCount > 0x50 {
		sinkSinkSink()
	}
	if plyregen[1]%3 == 0 {
		if player.Poison() != 0 {
			dmg := player.Poison()
			player.SetPoison(dmg - 1)
			damageItem(ThePlayer, nil, 0, 0, dmg, DMG_POISON)
		}
		if dmg := skillCheck(int(player.Skills[SKILL_MANA]), 10); dmg > 0 {
			restoreMana(ThePlayer, -dmg)
		}
	}
	if plyregen[1]%24 == 0 {
		playerEat(-3 - rand()&3)
		if player.Drunk() != 0 {
			player.SetDrunk(player.Drunk() - 1)
		}
		if rand()&3 == 0 {
			DoWanderingMonsters(1)
		}
		yearlyCheckup()
		for _, f := range []*byte{&player.Fatigue, &player.FoodHeal, &player.B3C} {
			if *f < 0xFF {
				*f++
			}
		}
		if skillCheck(int(playerdat.Attr[ATTR_STR]), 15) > 0 {
			restoreHp(ThePlayer, -1)
		}
		plyregen[1] = 0
	}
}

// DegradeLights burns the lit lights in the light slots; true if one went out.
func DegradeLights(amount, counter int) bool {
	changed := false
	counter &= 0xFF
	for i := 0; i < 4; i++ {
		obj := AskInventory(ValidLightSlots[i])
		if obj == nil {
			continue
		}
		light := obj.InClass()
		if obj.Class() != CLASS_LIGHT || light < 4 || light >= 8 {
			continue
		}
		if light = int(Lights[light].Duration); light == 0 {
			continue
		}
		burn := 0
		if counter%light == 0 {
			burn = 1
		}
		if amount > 1 {
			burn += amount / light
		}
		if burn == 0 {
			continue
		}
		if obj.Quality() > burn {
			obj.SetQuality(obj.Quality() - burn)
		} else {
			obj.SetQuality(0)
			obj.SetInClass(obj.InClass() - 4)
			RedisplayInvSlot(ValidLightSlots[i])
			changed = true
		}
	}
	return changed
}

func sinkSinkSink() {
	load := 0
	if player.MaxWeight != 0 {
		load = int(player.Weight<<5) / int(player.MaxWeight) & 0xFF
	}
	skill := skillCheck(int(player.Skills[SKILL_SWIMMING]), load)
	if skill < 1 && player.SwimCount < 0x8C {
		player.SwimCount += byte(rollem(3-skill, 4))
	}
	if player.SwimCount > 0x78 {
		hurt := 2 - skillCheck(int(player.Skills[SKILL_SWIMMING]), load)
		if hurt != 0 {
			fillFB(0xC6)
			editchng(2)
			damageItem(ThePlayer, nil, 0, 0, rollem(2, hurt+2), 0)
		}
	}
}

// setCurmagic starts an active spell; false when all three slots are taken.
func setCurmagic(cls, sub, stability int) bool {
	n := player.ActiveSpells()
	if n == 3 {
		return false
	}
	player.Spells[n] = player.Spells[n]>>8<<8 + uint16(cls+sub<<4)
	var stab int
	switch stability {
	case 1:
		stab = 1
	case 0:
		stab = rollem(2, 3)
	case 0x40:
		stab = rollem(2, 8) + 6
	case 0x80:
		stab = rollem(3, 20) + 24
	}
	player.Spells[n] = player.Spells[n]&0xFF + uint16(stab&0xFF)<<8
	player.SetActiveSpells(n + 1)
	FixPlayerEquips()
	return true
}
