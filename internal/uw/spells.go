package uw

// Port of SPELLS.C (seg038_3307): casting spells. doSpell dispatches every spell, whoever
// casts it, by class: 0..3 a timed active spell, 4 heals, 5 a missile, 6 an area ahead of
// the caster, 7 a critter in front of the player, 8 creates things, 9 a backfire, 10
// restores mana, 11 the extra spells, 13 two special effects, 14 a cutscene. The spell
// table itself (spells) is in runes.go.

var (
	// the map square of an object (not a critter) that casts
	inanmMapX, inanmMapY int
	// the mana a missile spell will cost once it is released
	mspellMused int
)

// areaFn is a per-square handler of an area spell.
type areaFn func(x, y int, target *Object, tile *Tile, src int) bool

// antiMagic: no magic works on a square whose tile has door bit 0 set.
func antiMagic(x, y int) bool { return MapGetAddr(x, y).Door()&1 != 0 }

// cast casts spell number spell, an index into spells.
func cast(spell int, who, target *Object) {
	if spell < NUM_SPELLS {
		doSpell(spells[spell].class(), int(spells[spell].sub), who, target)
	}
}

// doSpell casts a spell of class cls, minor and flags sub. False when it did not take.
func doSpell(cls, sub int, who, target *Object) bool {
	if ObjMemTPtr(who) >= NUM_MOBILE && cls <= SPELLC_XT {
		if antiMagic(inanmMapX, inanmMapY) {
			return false
		}
	} else if antiMagic(who.HomeX(), who.HomeY()) || PlayerLevel == 9 {
		return false
	}
	switch cls {
	case SPELLC_MOTION, SPELLC_LIGHT, SPELLC_ARMOUR, SPELLC_PROTECT:
		if cls == SPELLC_MOTION && (sub&^SPELL_FLAGS == 3 || sub&^SPELL_FLAGS == 5) {
			physBounceUp(who)
		}
		if who == ThePlayer && setCurmagic(cls, sub&SPELL_MINOR, sub&SPELL_FLAGS) {
			break
		}
		return false
	case SPELLC_HEAL:
		if target == nil {
			return false
		}
		healing(target, sub)
	case SPELLC_MISSILE:
		if who == ThePlayer {
			GameInputMode = 3
			ObjectActing = ThePlayer
			ObjectActorArg = sub
			forceMouseCursor(ICON_CURSORS + 9)
		} else {
			releaseMissile(who, sub)
		}
	case SPELLC_AREA:
		nailArea(who, sub)
	case SPELLC_1AREA:
		nail1Area(who, sub)
	case SPELLC_CREATE:
		creatSpell(who, sub)
	case SPELLC_BACKFIRE:
		backfire(who, sub)
	case SPELLC_MANA:
		restoreMana(who, int(int8(sub)))
	case SPELLC_XT:
		xtSpells(who, sub&SPELL_FLAGS, sub&SPELL_MINOR)
	case SPELLC_CUTSCENE:
		runcutscene(sub)
		updateAnimobj(4)
	case SPELLC_SPECIAL:
		switch sub {
		case 3:
			workBullfrogTiles(4, 0, 0)
		case 5:
			gameSprint(0xE4) // 'Your vision distorts and you feel light headed.'
			player.SetShrooms(3)
			FixPlayerEquips()
		}
	}
	return true
}

// restoreMana: amount > 0 gives max_mana * (amount + 0..3) / 16 + 1, a negative amount
// adds -amount.
func restoreMana(who *Object, amount int) {
	if who != ThePlayer {
		return
	}
	mana := int(player.PlayMana)
	if amount > 0 {
		mana += (int(player.MaxMana)*(amount+rand()&3))>>4 + 1
	} else {
		mana -= amount
	}
	if mana > int(player.MaxMana) {
		mana = int(player.MaxMana)
	}
	player.PlayMana = byte(mana)
	panelCheckHpmp()
}

// restoreHp heals the player like restoreMana, capped at the maximum.
func restoreHp(who *Object, amount int) {
	if who != ThePlayer {
		return
	}
	var hp int
	if amount > 0 {
		hp = (int(playerdat.Avghit)*(amount+rand()&3))>>4 + int(ThePlayer.HP) + 1
	} else {
		hp = int(ThePlayer.HP) - amount
	}
	if hp > int(playerdat.Avghit) {
		ThePlayer.HP = playerdat.Avghit
	} else {
		ThePlayer.HP = byte(hp)
	}
	panelCheckHpmp()
}

// getHpBack adds amount hit points to a critter, capped at its avghit.
func getHpBack(who *Object, amount int) {
	max := int(Creatures[who.InMajor()].Avghit)
	if hp := int(who.HP) + amount&0xFF; hp > max {
		who.HP = byte(max)
	} else {
		who.HP = byte(hp)
	}
	if who == ThePlayer {
		panelCheckHpmp()
	}
}

// healing: minor d8 hit points, minor 15 all of them.
func healing(who *Object, sub int) {
	if who.Major() != MAJOR_CREATURE {
		return
	}
	amount := 0xFF
	if int8(sub) != 15 {
		amount = rollem(int(int8(sub)), 8) & 0xFF
	}
	getHpBack(who, amount)
}

// backfire: (sub)d8 damage to the caster, never below 3 hit points.
func backfire(who *Object, sub int) {
	if who.Major() != MAJOR_CREATURE {
		return
	}
	damage := int(int8(rollem(int(int8(sub)), 8)))
	if who.HP <= 3 {
		return
	}
	if int(who.HP)-damage <= 3 {
		who.HP = 3
	} else {
		who.HP = byte(int(who.HP) - damage)
	}
	if who == ThePlayer {
		fillFB(0xA8)
	}
}

// releaseMissile fires missile spell sub (1..4). The player pays mspellMused if it left.
func releaseMissile(who *Object, sub int) {
	items := [4]int{7, 5, 4, 6}
	ok := spellFire(who, items[(sub-1)&3])
	if who == ThePlayer {
		if !ok {
			gameSprint(0xFF) // 'There is not enough room to release that spell.'
		} else if mspellMused != 0 {
			player.PlayMana -= byte(mspellMused)
		}
		mspellMused = 0
	}
}

// buildNewObj creates an effect object for a tile at a random height.
func buildNewObj(item int, tile *Tile) *Object {
	obj := CreateObj(item, false)
	z := tile.Height() * 8
	if z < 0x80 {
		z += rand() % (0x80 - z)
	}
	if obj != nil {
		obj.SetZ(z)
	}
	return obj
}

// spTrueSight (Reveal): sets off a hidden trigger the target holds.
func spTrueSight(_, _ int, target *Object, _ *Tile, _ int) bool {
	if target.IsQuant() != 0 || target.LinkF() == 0 || target.Major() == MAJOR_TRAP {
		return false
	}
	head := &target.OL
	if trig := ObjInList(&head, false, MAJOR_TRAP, 2, 3); trig != nil {
		search := player.Skills[SKILL_SEARCH]
		player.Skills[SKILL_SEARCH] = 0x2D
		UseTrigger(ThePlayer, target, trig, 5)
		player.Skills[SKILL_SEARCH] = search
		return true
	}
	return false
}

// addSpellAnim starts an effect object's animation, adding it to the tile.
func addSpellAnim(obj *Object, tile *Tile, frame, x, y int) bool {
	if obj == nil {
		return false
	}
	if addAnimobj(ObjMemTPtr(obj), 4, frame, x, y) == -1 {
		ObjFree(obj)
		return false
	}
	ObjAdd(&tile.Objects, obj)
	return true
}

// spSheetLight: Sheet Lightning on one square.
func spSheetLight(x, y int, _ *Object, tile *Tile, src int) bool {
	obj := buildNewObj(ITEM_SPLASH_1C5, tile)
	damageSquare(x, y, 2, src)
	addSpellAnim(obj, tile, rand()%4, x, y)
	return true
}

// spMeteor: Flame Wind, a cross of five squares.
func spMeteor(x, y int, _ *Object, tile *Tile, src int) bool {
	obj := buildNewObj(ITEM_EXPLOSION_1C2, tile)
	damageSquare(x, y, 1, src)
	damageSquare(x+1, y, 1, src)
	damageSquare(x-1, y, 1, src)
	damageSquare(x, y+1, 1, src)
	damageSquare(x, y-1, 1, src)
	if addSpellAnim(obj, tile, 0, x, y) {
		fireballEffect(obj, x, y)
	}
	return true
}

// spWardUndead: Smite Undead.
func spWardUndead(x, y int, target *Object, _ *Tile, src int) bool {
	if checkRes(target, 1, RES_UNDEAD) == 0 {
		damageItem(target, ObjIntTMem(src), x, y, 0xFF, DMG_MAGIC)
		return true
	}
	return false
}

// spPoison: 5d4 of magic and poison.
func spPoison(x, y int, target *Object, _ *Tile, src int) bool {
	putEffect(target, 7, 4, 0, 7, x, y)
	damageItem(target, ObjIntTMem(src), x, y, rollem(5, 4), DMG_MAGIC|DMG_POISON)
	return true
}

// hitCritterGoal: a mind spell lands unless resisted.
func hitCritterGoal(goal, attitude int, npc *Object, x, y int) bool {
	if checkRes(npc, 1, DMG_MAGIC) != 0 {
		putEffect(npc, 7, 4, 0, 7, x, y)
		changeCritterGoal(npc, goal, 1)
		if attitude != -1 {
			npc.SetAttitude(attitude)
		}
	}
	return true
}

// spCharm (Ally): the critter becomes the player's friendly ally.
func spCharm(x, y int, target *Object, _ *Tile, _ int) bool {
	if checkRes(target, 1, DMG_MAGIC) != 0 {
		putEffect(target, 7, 4, 0, 7, x, y)
		if target.Ally() == 0 {
			changeCritterGoal(target, GOAL_WANDER, 0)
		}
		target.SetAlly(1)
		target.SetAttitude(ATT_FRIENDLY)
	}
	return true
}

func spConfusion(x, y int, target *Object, _ *Tile, _ int) bool {
	return hitCritterGoal(GOAL_WANDER, ATT_UPSET, target, x, y)
}

func spFear(x, y int, target *Object, _ *Tile, _ int) bool {
	return hitCritterGoal(GOAL_FLEE, -1, target, x, y)
}

func spHold(x, y int, target *Object, _ *Tile, _ int) bool {
	return hitCritterGoal(GOAL_STAND_7, ATT_UPSET, target, x, y)
}

// processArea calls fn on the squares of a w+1 by h+1 rectangle, until count calls
// returned true. typ: 0 critters other than src, AREA_RANDOM random open squares, 0x80
// and 0xC0 every object.
func processArea(count, src int, fn areaFn, typ, x0, y0, w, h int) {
	count = int(int8(count))
	x0, y0, w, h = int(int8(x0)), int(int8(y0)), int(int8(w)), int(int8(h))
	if x0 >= MAP_SIZE || x0+w < 0 || y0 >= MAP_SIZE || y0+h < 0 {
		return
	}
	if x0 < 0 {
		w -= -x0
		x0 = 0
	} else if x0+w >= MAP_SIZE {
		w -= x0 + w - MAP_SIZE
	}
	if y0 < 0 {
		h -= -y0
		y0 = 0
	} else if y0+h > MAP_SIZE {
		h -= y0 + h - MAP_SIZE
	}
	if w <= 0 || h <= 0 {
		return
	}
	for tries := 0; ; tries++ {
		for x := x0; x <= x0+w; x++ {
			for y := y0; y <= y0+h; y++ {
				if x < 0 || x >= MAP_SIZE || y < 0 || y >= MAP_SIZE {
					continue
				}
				tile := MapGetAddr(x, y)
				if typ == AREA_RANDOM {
					if tile.Type() > TILE_SOLID && rand()%(w*h+3) < count {
						if fn(x, y, nil, tile, src) {
							if count--; count == 0 {
								return
							}
						}
					}
					continue
				}
				link := &tile.Objects
				for obj := ObjPtrTMem(link); obj != nil; obj = ObjPtrTMem(link) {
					next := link.Index()
					if typ == AREA_OBJECTS ||
						typ == AREA_CRITTERS && obj.Major() == MAJOR_CREATURE && ObjMemTPtr(obj) != src ||
						typ == AREA_ALL {
						if fn(x, y, obj, tile, src) {
							if count--; count <= 0 {
								return
							}
						}
					}
					if link.Index() == next {
						link = &obj.QN
					}
				}
			}
		}
		if !(typ == AREA_RANDOM && count > 0 && tries < 4) {
			break
		}
	}
}

// gronkArea runs an area spell dist squares ahead of who, radius squares around.
func gronkArea(who *Object, count int, fn areaFn, typ, dist, radius int) {
	var x, y int16
	var src, heading int
	if index := ObjMemTPtr(who); index < NUM_MOBILE {
		src = index
		heading = who.HeadingBits()<<5 + who.FineHead()
		x, y = int16(who.HomeX()), int16(who.HomeY())
	} else {
		heading = who.HeadingBits() << 5
		x, y = int16(inanmMapX), int16(inanmMapY)
	}
	moveAlong(heading, dist, &x, &y)
	processArea(count, src, fn, typ, int(int8(x))-radius, int(int8(y))-radius, radius*2+1, radius*2+1)
}

// gronkWhoami calls fn(npc, arg) for the first (with all, every) active critter of the
// given whoami. fn returns true when it took the critter off the active list.
func gronkWhoami(whoami, all, arg int, fn func(*Object, int) bool) {
	for p := 0; p < nActive; p++ {
		npc := ObjIntTMem(int(Level.Active[p]))
		if int(npc.Whoami) == whoami {
			if fn(npc, arg) {
				p--
			}
			if all == 0 {
				return
			}
		}
	}
}

// the per-square handlers of class 6 and class 7 spells (set in init: they reach back here)
var areaSpells, area1Spells []areaFn

func init() {
	areaSpells = []areaFn{spTrueSight, spSheetLight, spConfusion, spMeteor}
	area1Spells = []areaFn{spFear, spWardUndead, spCharm, spPoison, spHold}
}

// nailArea: an area spell acts 3d4 times within 2 of the square four ahead.
func nailArea(who *Object, sub int) {
	count := rollem(3, 4)
	gronkArea(who, count, areaSpells[(sub&^SPELL_FLAGS-1)&3], sub&SPELL_FLAGS, 4, 2)
}

// nail1Area: a spell on one critter in front of the player.
func nail1Area(who *Object, sub int) {
	if who == ThePlayer {
		if i := sub&^SPELL_FLAGS - 1; i >= 0 && i < len(area1Spells) {
			gronkArea(who, 1, area1Spells[i], sub&SPELL_FLAGS, 4, 2)
		}
	}
}

// creatSpell creates food (1), a rune of warding (3) or a monster (4) in front of caster.
func creatSpell(caster *Object, which int) {
	msg := 0x115 // 'There is no room to create that.'
	heading := (caster.HeadingBits()<<5 + caster.FineHead() + (rand()%0x1B - 0xD)) % 0xFF
	x := int16(caster.HomeX()<<3 + caster.FineX())
	y := int16(caster.HomeY()<<3 + caster.FineY())
	dist := 9
	if which == 4 {
		dist = 0xC
	}
	moveAlong(heading, dist, &x, &y)
	homex, homey := int(x)>>3, int(y)>>3
	if which == 3 {
		if castTrapSpell(homex, homey, 9) != 0 {
			msg--
		}
		if caster == ThePlayer {
			gameSprint(msg)
		}
		return
	}
	tile := MapGetAddr(homex, homey)
	z := tile.Height() << 3
	item := 0
	switch which {
	case 1:
		item = rand()%7 + FIRST_FOOD
	case 4:
		lvl := PlayerLevel << 2
		if caster == ThePlayer {
			lvl = int(player.Skills[SKILL_CASTING])
		}
		lvl &= 0xFF
		if lvl < 2 {
			lvl = 2
		}
		for {
			item = lvl + rand()%lvl + FIRST_CREATURE
			c := &Creatures[item&^0x1C0]
			if !(c.Avghit == 0 || c.B0A&2 != 0 || item == 0x7B || item == 0x7C || c.Swims() != 0) {
				break
			}
		}
	}
	if !canPlace(item, 0, int(x), int(y), z, true, 8) {
		if caster == ThePlayer {
			gameSprint(msg)
		}
		return
	}
	obj := CreateObj(item, which == 4)
	if obj == nil {
		return
	}
	obj.SetFineX(int(x) & 7)
	obj.SetFineY(int(y))
	if which == 4 {
		save := ActiveObj
		ActiveObj = obj
		creatureObjInit()
		ActiveObj = save
		obj.SetHomeX(homex)
		obj.SetHomeY(homey)
		if Creatures[item&^0x1C0].Flier() != 0 {
			z = (z + 0x80) / 2
		}
		if caster == ThePlayer {
			obj.SetAlly(1)
		} else {
			obj.SetAttitude(ATT_HOSTILE)
			obj.SetB190(1)
			obj.SetDestX(ThePlayer.HomeX())
			obj.SetDestY(ThePlayer.HomeY())
		}
	} else {
		obj.SetQuality(0x3F)
	}
	obj.SetZ(z)
	ObjAdd(&tile.Objects, obj)
	if which != 4 {
		objDeal(obj, homex, homey, 1)
	}
}

// tremorArea drops a boulder from high up on one square.
func tremorArea(x, y int, _ *Object, _ *Tile, _ int) bool {
	boulder := CreateObj(rand()%3+ITEM_LARGE_BOULDER_154, false)
	if boulder == nil {
		return true
	}
	boulder.SetZ(0x6E)
	if putAt(x*8+3, y*8+3, 0x6E, boulder, 0, false) && IsMobElem(boulder) {
		boulder.SetSpeed(rand()&3 + 2)
		boulder.Heading = byte(rand())
		boulder.SetBin(curBin + rand()&3)
		boulder.B14 = boulder.B14&0xF8 | byte((rand()%3+1)&7)
	}
	return true
}

// printMonster prints 'You detect ...' with the direction dir.
func printMonster(dir, n int) {
	id := 0x3B + b2int(n > 1) + b2int(n > 4)
	printPathTo(getString(id|STR_GAME), 0, 0, 0, 0, 0, 0, -(dir + 1))
}

// mdetect: Detect Monster, active critters within dist squares counted by direction.
func mdetect(dist, skill int) {
	var counts [8]int
	px, py := ThePlayer.HomeX(), ThePlayer.HomeY()
	for _, i := range activeMobs() {
		obj := ObjIntTMem(int(i))
		if obj.Major() != MAJOR_CREATURE {
			continue
		}
		xv, yv := int8(obj.HomeX()-px), int8(obj.HomeY()-py)
		if iabs(int(xv)) < dist && iabs(int(yv)) < dist &&
			skillCheck(skill, 0xF-Creatures[obj.InMajor()].Noise()) > 0 {
			counts[mpos(xv, yv)]++
		}
	}
	max := 0
	for i := 0; i < 8; i++ {
		if counts[i] > max {
			max = counts[i]
		}
	}
	if max == 0 {
		gameSprint(0x3E) // 'You detect no monster activity.'
		return
	}
	best := max
	if max >= 4 {
		best = 3
	}
	for i := 0; i < 8; i++ {
		if counts[i] == max {
			printMonster(i, counts[i])
			max = best
			best = i
			break
		}
	}
	i := rand() & 7
	for tries := 0; tries < 8; tries, i = tries+1, i+1 {
		if i&7 != best && counts[i&7] > max {
			printMonster(i&7, counts[i&7])
			return
		}
	}
}

// objSpells: the clicked object of an object spell (Remove Trap, Name Enchantment, Open).
func objSpells(target *Object, _, _ int) {
	switch ObjectActorArg {
	case 3:
		if DetectedTrap(target, 0x2D) != 0 {
			RemoveTrap(target, 0x2D)
		}
	case 4:
		LookAt(target, 3)
		if target.Major() != MAJOR_RECT && target.Major() != MAJOR_CREATURE &&
			ComObjData[target.Item()].Render() != 2 {
			target.SetHeading(7)
		}
	case 5:
		if checkLock(ThePlayer, target, -45) == 3 {
			gameSprint(0x10E) // 'The spell unlocks the lock.'
		} else {
			gameSprint(0x10F) // 'The spell has no discernable effect.'
		}
	}
	unforceMouseCursor(3)
	GameInputMode = 0
	mouseRelease(true)
}

// xtSpells: the extra spells (class 11), the player's only.
func xtSpells(caster *Object, stab, sub int) {
	if caster != ThePlayer {
		return
	}
	switch sub {
	case 0: // speed
		setCurmagic(SPELLC_XT, 2, stab)
	case 1: // detect monster
		mdetect(10, 0x2D)
	case 2, 3, 4, 5: // the object spells
		GameInputMode = 2
		ObjectActorArg = sub
		ObjectActing = ThePlayer
		ObjectActor = objSpells
		forceMouseCursor(ICON_CURSORS + 10)
	case 6: // cure poison
		player.SetPoison(0)
	case 7: // roaming sight
		setCurmagic(SPELLC_XT, 1, stab)
		homeCam(0)
		attachEye(-1)
		GameInputMode += 8
	case 8: // telekinesis
		setCurmagic(SPELLC_XT, 3, stab)
	case 9: // tremor
		gronkArea(caster, rollem(8, 3), tremorArea, AREA_RANDOM, 5, 3)
		setEffect(0x40, 0x28)
		playEffectOnMobile(0x12, caster, 0)
	case 10: // gate travel
		if player.Moonstone() != 0 {
			nppFunc = doMstone
			doTeleport(ThePlayer, 0x3F, 0x3F, player.Moonstone())
			playerSetup(0, 0)
			editchng(0x7FFE)
		} else {
			gameSprint(0x111) // 'The moonstone is not available.'
		}
	case 11: // freeze time
		setCurmagic(SPELLC_XT, 0, stab)
	case 12: // armageddon
		FreePlayerInv(&ThePlayer.OL)
		clearobj()
		clearRunes()
		clearShelf()
		player.SetArmageddon(1)
		player.SetTree(0)
		PlayerDat.Weight = 0
		FixPlayerEquips()
		prettyPanelagain()
	}
}

var (
	sqDice  = [2]int{10, 6}
	sqSides = [2]int{6, 5}
	sqType  = [2]int{DMG_MAGIC | DMG_FIRE, DMG_MAGIC}
)

// damageSquare damages every object on a square: kind 1 10d6 magic fire, 2 6d5 magic.
func damageSquare(x, y, kind, src int) {
	if kind == 0 {
		return
	}
	kind--
	obj := ObjPtrTMem(&MapGetAddr(x, y).Objects)
	for obj != nil {
		next := ObjPtrTMem(&obj.QN)
		damageItem(obj, ObjIntTMem(src), x, y, rollem(sqDice[kind], sqSides[kind]), sqType[kind])
		obj = next
	}
}
