package uw

// Port of USEITEMS.C (seg040_352B): using objects. UseObj dispatches on the object's
// class, then sets off what trap or trigger it holds (checkTrap) and casts any spell it
// carries (checkSpell). An object that needs a second object ("Use <name> on what?") goes
// through useThing, which puts it on the cursor and stores the second half in
// ObjectActor, with the first object as ObjectActing.

// nextSpellTime is when an object may next cast its spell for the player (once every
// 0x2FD clock ticks).
var nextSpellTime uint32

// UseObj uses obj for who (how set: the player's use command; clear: from the world).
// Returns obj, or nil when the silver tree was picked.
func UseObj(who, obj *Object, how int) *Object {
	minor := obj.Minor()
	trap := true
	if inplist.mode == 4 && (obj.Major() != MAJOR_MISC || obj.Minor() != MINOR_CONTAINER) {
		return obj
	}
	switch obj.Major() {
	case MAJOR_HACK:
		if minor == 1 && how == 0 && who != nil {
			missileNewhit(obj, who)
		}
	case MAJOR_MISC:
		switch minor {
		case 0:
			useCont(who, obj, how)
		case 1:
			if obj.InClass() < 8 {
				useLight(obj, how)
			} else {
				useWand(obj, how)
				trap = false
			}
		case 3:
			UseFood(who, obj, how)
			trap = false
		}
	case MAJOR_STUFF:
		switch minor {
		case 0, 1:
			useUtil(obj, how)
		case 2:
			if obj.Item() == ITEM_KEY_OF_INFINITY && how != 0 {
				useThing(obj, useKeyOfInfinityOn)
			}
		}
	case MAJOR_SPEC:
		switch minor {
		case 0:
			useKey(obj, how)
		case 1:
			useUnique(who, obj, how)
		case 2:
			useMagic(who, obj, how)
		case 3:
			useBook(obj, how)
			trap = false
		}
	case MAJOR_RECT:
		useRect(who, obj)
	case MAJOR_ANIMOBJ:
		switch obj.InClass() {
		case 0xF:
			if obj.Owner()&0xF < 8 {
				CloseDoor(obj)
			} else {
				OpenDoor(who, obj)
			}
		case 0xA:
			if usingPunt(obj, how != 0, true) {
				gameSprint(9)
				obj = placeNew(nil, ITEM_SILVER_SEED)
				player.SetTree(0)
				if obj != nil {
					obj.SetDoorDir(1)
				}
				return nil
			}
		case 9:
			if obj.Next() != 0 {
				if fount := ObjPtrTMem(&obj.QN); fount.Item() == ITEM_FOUNTAIN_12E {
					useMagic(who, fount, how)
				}
			}
		}
	}
	if trap {
		checkTrap(who, obj, 4, MapObjX, MapObjY)
		checkSpell(MapObjX, MapObjY, who, obj, how)
	}
	return obj
}

// usingPunt uses up obj, from the inventory (inv) or from its tile (MapObjX, MapObjY).
// True when the object is gone.
func usingPunt(obj *Object, inv, how bool) bool {
	if inv && obj == carriedUse {
		if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 && obj.LinkF() > 1 {
			obj.SetLinkF(obj.LinkF() - 1)
			return false
		}
		carriedUse = nil
		ObjPunt(nil, obj, true)
		return true
	}
	if inv {
		InvRemoveOneObject(obj)
		obj = ObjPunt(nil, obj, how)
	} else {
		tile := MapGetAddr(MapObjX, MapObjY)
		if ObjFind(&tile.Objects, true, ObjMemTPtr(obj)) != nil {
			obj = ObjPunt(ObjFindHead, obj, how)
			editchng(2)
		} else {
			var head Link
			head.SetIndex(ObjMemTPtr(obj))
			ObjFreeChain(&head)
			obj = nil
		}
	}
	return obj == nil
}

// placeNew puts obj, or a new object of item, on the cursor. nil when the cursor is full.
func placeNew(obj *Object, item int) *Object {
	if CursorObjPtr != nil {
		return nil
	}
	if obj != nil {
		item = obj.Item()
	} else {
		obj = CreateObj(item, false)
	}
	GameInputMode = 1
	CursorObjPtr = obj
	forceMouseCursor(item)
	return obj
}

// releaseCursor drops the cursor object of a two-object use.
func releaseCursor() {
	unforceMouseCursor(3)
	CursorObjPtr = nil
	GameInputMode = 0
}

// useLockpickOn picks obj's lock with the Pick Lock skill.
func useLockpickOn(obj *Object, how, _ int) {
	if how == 0 {
		return
	}
	releaseCursor()
	switch checkLock(ThePlayer, obj, -int(player.Skills[SKILL_PICKLOCK])) {
	case 1:
		gameSprint(3)
	case 0:
		gameSprint(0x78)
	case 4:
		gameSprint(0x7A)
	default:
		playEffectHere(0x13, 0x40, 0)
		gameSprint(0x79)
	}
}

// useKeyOn tries the key (ObjectActing) in obj's lock.
func useKeyOn(obj *Object, how, _ int) {
	if how == 0 {
		return
	}
	releaseCursor()
	gameSprint(checkLock(ThePlayer, obj, ObjectActing.Owner()) + 2)
}

// useKey: the lockpick and the keys ask for a target.
func useKey(obj *Object, how int) {
	if how == 0 {
		return
	}
	if obj.Item() == ITEM_LOCKPICK {
		useThing(obj, useLockpickOn)
	} else if obj.Item() < ITEM_LOCK {
		useThing(obj, useKeyOn)
	}
}

// useThing starts a two-object use: obj goes on the cursor and fn waits for the target.
// An inventory object is instead picked up as a left click would (mousInInv sees
// takeAfterUse) and used on the target with a right click (useCarriedOn).
func useThing(obj *Object, fn ActorFn) {
	n, ok := getName(obj, false, false)
	if !ok {
		n = "UNNAMED"
	}
	if inInventory(obj) {
		if obj.Item() == ITEM_POLE {
			UsingPole = false
			FixPlayerEquips()
		}
		scrollPrint("Right-click what to use the " + n + " on.\n")
		takeAfterUse = true
		return
	}
	scrollPrint("Use " + n + " on what?\n")
	forceMouseCursor(obj.Item())
	CursorObjPtr = obj
	GameInputMode = 2
	ObjectActing = obj
	ObjectActor = fn
}

// spikeDoorOn spikes a closed door with the spike (ObjectActing).
func spikeDoorOn(door *Object, _, _ int) {
	if door.Item() < ITEM_DOOR_140 || door.Item() > ITEM_SECRET_DOOR_147 {
		gameSprint(0x80)
	} else {
		gameSprint(0x81)
		door.SetOwner(door.Owner()&0xFFFE | 1)
		door.SetOwner(door.Owner()&1 | 0x3E)
		usingPunt(ObjectActing, true, true)
	}
	releaseCursor()
}

// spikeDoor: using the spike asks for the door.
func spikeDoor(obj *Object, how int) {
	if how == 0 {
		return
	}
	useThing(obj, spikeDoorOn)
}

// useBonesOn: Garamon's bones buried in his grave start his last conversation.
func useBonesOn(obj *Object, how, _ int) {
	releaseCursor()
	used := false
	if obj.Item() == ITEM_GRAVESTONE {
		if ObjectActing.Owner() == 0x3E {
			if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL != 0 && obj.LinkF()&0x1FF == 0x21 {
				questChanged()
				player.SetTalismanOK(1)
				player.SetGaramon(1)
				obj.SetLinkF(0x222)
				var tmp Object
				tmp.SetItem(0x7E)
				tmp.SetGoal(GOAL_STAND_7)
				tmp.SetAttitude(ATT_FRIENDLY)
				tmp.Whoami = 0x1B
				TalkTo(&tmp)
				if o := ObjPtrTMem(&MapGetAddr(0x36, 0x34).Objects); o != nil {
					UseTrigger(ThePlayer, nil, o, 0)
				}
				used = true
			} else {
				gameSprint(0x103)
				return
			}
		} else {
			used = true
			gameSprint(0x86)
		}
	}
	if !used {
		gameSprint(0x84)
	} else {
		usingPunt(ObjectActing, how != 0, true)
	}
}

// usePoleOn uses a switch from afar.
func usePoleOn(obj *Object, _, _ int) {
	UsingPole = false
	FixPlayerEquips()
	if obj.Class() == CLASS_SWITCH {
		gameSprint(0x9D)
		UseObj(ThePlayer, obj, 0)
	} else {
		gameSprint(0x9E)
	}
}

// useAnvilOn repairs obj.
func useAnvilOn(obj *Object, how, other int) {
	if how == 0 || other == 0 {
		return
	}
	releaseCursor()
	repairItem(obj, int(player.Skills[SKILL_REPAIR]), true)
}

// useUtil: bones, the anvil and the pole ask for a target; a few plants are eaten.
func useUtil(obj *Object, how int) {
	switch item := obj.Item(); {
	case item >= ITEM_SKULL_C2 && item <= ITEM_PILE_OF_BONES_C6:
		if how != 0 {
			useThing(obj, useBonesOn)
		}
	case item == ITEM_ANVIL:
		useThing(obj, useAnvilOn)
	case item == ITEM_POLE:
		UsingPole = true
		FixPlayerEquips()
		useThing(obj, usePoleOn)
	case how != 0 && (item == ITEM_DEAD_ROTWORM || item == ITEM_PLANT_CE || item == ITEM_PLANT_CF):
		UseFood(ThePlayer, obj, how)
	}
}

// weakenWithOrb divides npc's hit points by div (plus one) and stops its healing.
func weakenWithOrb(npc *Object, div int) bool {
	npc.HP = byte(int(npc.HP)/div + 1)
	npc.SetNoHeal(1)
	return false
}

// tybalsOrbOn: the orb rock shatters Tybal's orb.
func tybalsOrbOn(obj *Object, how, _ int) {
	if obj.Item() != ITEM_ORB {
		if how != 0 {
			gameSprint(0x84)
		}
	} else {
		gameSprint(0x85)
		if how != 0 {
			usingPunt(ObjectActing, true, true)
		}
		putEffect(obj, 4, 5, 0, 0, MapObjX, MapObjY)
		ObjPunt(&MapGetAddr(MapObjX, MapObjY).Objects, obj, true)
		MapObjX = -1
		questChanged()
		player.SetOrb(1)
		player.MaxMana = player.SavedMana
		player.PlayMana = player.SavedMana
		gronkWhoami(0xE7, 0, 2, weakenWithOrb)
	}
	if CursorObjPtr != nil {
		releaseCursor()
	}
}

// useKeyOfInfinityOn: the key used on a door texture sets off its traps.
func useKeyOfInfinityOn(obj *Object, how, _ int) {
	releaseCursor()
	if obj.Item() == ITEM_TMAP_C && byte(w64Types[obj.Owner()]&0xFF) == TERR_TEXTURED_DOOR {
		usingPunt(ObjectActing, how != 0, true)
		checkTrap(ThePlayer, obj, 7, MapObjX, MapObjY)
	} else {
		gameSprint(0x84)
	}
}

// useUnique: incense, the orb rock, the exploding book and rotworm stew.
func useUnique(who, obj *Object, how int) {
	switch obj.Item() {
	case ITEM_BLOCK_OF_BURNING_INCENSE:
		n := rand() % 3
		if player.Incense() < 3 {
			player.SetIncense(player.Incense() + 1)
			n = 3 - player.Incense()
		}
		runcutscene(n + 0xB)
		obj.SetItem(ITEM_PILE_OF_DEBRIS_D5)
		if how != 0 {
			DisplayInventory()
		} else {
			editchng(2)
		}
	case ITEM_ORB_ROCK:
		if how != 0 {
			useThing(obj, tybalsOrbOn)
		} else {
			ObjectActing = obj
			tybalsOrbOn(who, 0, 0)
		}
	case ITEM_BOOK_114:
		if how != 0 {
			explodingBook(nil, 0, 0)
		}
	case ITEM_ROTWORM_STEW:
		UseFood(who, obj, how)
	}
}

// useLight lights or puts out a light, moving it to a free shoulder or hand first.
func useLight(obj *Object, how int) {
	qty := 1
	if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
		qty = obj.LinkF()
	}
	if how == 0 {
		gameSprint(0x7B)
		return
	}
	if obj.Quality() == 0 {
		gameSprint(0x7C)
		return
	}
	slot := FindSlot(obj)
	j := 0
	for ; j < 4; j++ {
		if ValidLightSlots[j] == slot && qty == 1 {
			break
		}
	}
	if j == 4 {
		newslot := 0
		if id := obj.Item(); id == ITEM_TORCH || id == ITEM_CANDLE || id == ITEM_LANTERN || id == ITEM_TAPER {
			for i := 5; i <= 8; i++ {
				inslot := AskInventory(i)
				if inslot == nil && newslot == 0 {
					newslot = i
				} else if inslot == obj && qty == 1 {
					newslot = i
					break
				}
			}
			if newslot == 0 {
				gameSprint(0xF6)
				return
			}
			InvRemoveOneObject(obj)
			AddToInventory(obj, newslot)
			DisplayInventory()
			slot = newslot
		}
	}
	if obj.InClass() >= 4 {
		obj.SetInClass(obj.InClass() - 4)
	} else {
		obj.SetInClass(obj.InClass() + 4)
	}
	FixPlayerEquips()
	RedisplayInvSlot(slot)
}

// useWand casts the wand's spell; a wand left without one breaks.
func useWand(wand *Object, how int) {
	if how == 0 {
		return
	}
	if wand.InClass() < 0xC || wand.InClass() > 0xF {
		checkTrap(ThePlayer, wand, 4, MapObjX, MapObjY)
		checkSpell(MapObjX, MapObjY, ThePlayer, wand, how)
		if wand.IsQuant() == 0 {
			link := &wand.OL
			if ObjInList(&link, false, MAJOR_SPEC, 2, 0) == nil {
				wand.SetInClass(wand.InClass() + 4)
				gameSprint(0x7D)
				RedisplayInvSlot(FindSlot(wand))
			}
		}
	}
}

// UseFood eats or drinks food: 1 eaten, 0 refused, -1 not food, -2 cannot be eaten
// from here.
func UseFood(who, food *Object, how int) int {
	taste, nutrition := 0, 0xFF
	qty := 1
	if food.IsQuant() != 0 && food.LinkF()&LINK_SPECIAL == 0 {
		qty = food.LinkF()
	}
	if food == CursorObjPtr {
		if qty > 1 {
			gameSprint(0x77)
			return -2
		}
	} else if how == 0 {
		return -2
	}
	if food.Class() == CLASS_FOOD {
		nutrition = int(int8(Food[food.InClass()]))
	}
	switch food.Item() {
	case ITEM_BOTTLE_OF_WINE:
		gameSprint(0x7F)
		return 0
	case ITEM_MUSHROOM, ITEM_TOADSTOOL, ITEM_CANDLE, ITEM_LEECHES:
		switch food.Item() {
		case ITEM_MUSHROOM:
			if skillCheck(int(playerdat.Attr[ATTR_INT]), 20) != 0 {
				restoreMana(ThePlayer, -(rand() * 3 / 0x8000))
			}
			if player.Shrooms() < 3 {
				player.SetShrooms(player.Shrooms() + 1)
			}
			FixPlayerEquips()
			taste = 3
		case ITEM_TOADSTOOL:
			taste = 2
		case ITEM_CANDLE:
			taste = 1
		}
		taste += 0xE5
	case ITEM_PLANT_CE:
		taste, nutrition = 0xEC, 4
	case ITEM_PLANT_CF:
		taste, nutrition = 0xE9, 0x17
	case ITEM_DEAD_ROTWORM:
		taste, nutrition = 0xEA, 4
	case ITEM_ROTWORM_STEW:
		taste, nutrition = 0xEB, 0x40
	case ITEM_RED_POTION, ITEM_GREEN_POTION:
		taste = 3 + 0xED
	case ITEM_BOTTLE_OF_ALE:
		taste = 2 + 0xED
	case ITEM_FLASK_OF_PORT:
		taste = 1 + 0xED
	case ITEM_BOTTLE_OF_WATER:
		taste = 0xED
	default:
		if nutrition == 0xFF {
			return -1
		}
	}
	switch {
	case nutrition > 0:
		if nutrition != 0xFF && !playerEat(nutrition) {
			gameSprint(0x7E)
			return 0
		}
		if taste == 0 {
			n, ok := getName(food, false, false)
			if !ok {
				n = "UNNAMED"
			}
			taste = (rand()*20/0x8000 + food.Quality()) >> 4
			if taste > 4 {
				taste = 4
			}
			scrollPrint("That " + n)
			taste += 0xAC
		}
		gameSprint(taste)
		if food.Item() == ITEM_TOADSTOOL {
			if player.Poison() < 4 {
				player.SetPoison(4)
			} else if player.Poison() < 0xD {
				player.SetPoison(player.Poison() + 2)
			}
		}
	case nutrition < 0:
		gameSprint(taste)
		if nutrition < -1 && nutrition > -0x7F {
			if player.Drunk()-nutrition > 0x3F {
				player.SetDrunk(0x3F)
			} else {
				player.SetDrunk(player.Drunk() - nutrition)
			}
			switch skillCheck(int(playerdat.Attr[ATTR_STR]), player.Drunk()) {
			case -1:
				gameSprint(0xF1)
				playerSleep(-2)
				if ThePlayer.HP != 0 {
					gameSprint(0xF3)
					setEffect(0x40, player.Drunk()/6+10)
				}
			case 0:
				setEffect(0x40, player.Drunk()/6)
			case 2:
				gameSprint(0xF2)
				restoreHp(ThePlayer, -2)
			}
		}
	default:
		gameSprint(taste)
	}
	checkSpell(MapObjX, MapObjY, who, food, 1)
	checkTrap(who, food, 4, MapObjX, MapObjY)
	if usingPunt(food, how != 0, true) && GameInputMode == 1 {
		CursorObjPtr = nil
	}
	return 1
}

// useRockHammerOn breaks a boulder into smaller ones or sling stones.
func useRockHammerOn(obj *Object, how, other int) {
	releaseCursor()
	if how == 0 || other != 0 {
		return
	}
	if ObjFind(&ThePlayer.OL, true, ObjMemTPtr(obj)) != nil {
		return
	}
	id := obj.Item()
	if id < ITEM_LARGE_BOULDER_153 || id > ITEM_SMALL_BOULDER {
		gameSprint(0x84)
		return
	}
	gameSprint(0x87)
	tile := MapGetAddr(MapObjX, MapObjY)
	for count := rand()*2/0x8000 + (ITEM_BOULDER - id) + 1; count > 0; count-- {
		rock := CreateObj(1, false)
		if rock == nil {
			break
		}
		setStatic(rock, obj)
		n := id + rand()*2/0x8000 + 1
		if n > ITEM_SMALL_BOULDER {
			n = ITEM_SLING_STONE
		}
		rock.SetItem(n)
		if n == ITEM_SLING_STONE {
			rock.SetIsQuant(1)
			rock.SetLinkF(rand()%6 + 3)
		}
		putAt(MapObjX<<3+obj.FineX(), MapObjY<<3+obj.FineY(), obj.Z(), rock, 6, false)
	}
	ObjPunt(&tile.Objects, obj, true)
	editchng(2)
}

// useOilOn makes a torch of wood or fills a lantern or torch.
func useOilOn(obj *Object, how, other int) {
	id := obj.Item()
	off := 4
	if id == ITEM_LANTERN || id == ITEM_LIT_LANTERN {
		off = 0
	}
	releaseCursor()
	if how == 0 || other == 0 {
		return
	}
	switch {
	case id >= ITEM_PIECE_OF_WOOD_CC && id <= ITEM_PIECE_OF_WOOD_CD:
		gameSprint(0xB5)
		usingPunt(ObjectActing, true, true)
		obj.SetItem(ITEM_TORCH)
		RedisplayInvSlot(FindSlot(obj))
	case id == ITEM_LANTERN || id == ITEM_TORCH:
		if obj.Quality() == 0x3F {
			gameSprint(off + 0xB4)
		} else {
			if obj.Quality() < 0x20 {
				obj.SetQuality(obj.Quality() + 0x20)
			} else {
				obj.SetQuality(0x3F)
			}
			gameSprint(off + 0xB3)
			usingPunt(ObjectActing, true, true)
		}
	case id == ITEM_LIT_LANTERN || id == ITEM_LIT_TORCH:
		gameSprint(off + 0xB2)
	default:
		gameSprint(0xB1)
	}
}

// useMagic: the class 0x12 tools, and the fountain, glowing rocks and cauldron from the
// world.
func useMagic(who, obj *Object, how int) {
	if how != 0 {
		switch obj.Item() {
		case ITEM_BEDROLL:
			if inplist.mode == 1 {
				playerSleep(1)
			}
		case ITEM_MANDOLIN, ITEM_FLUTE:
			playInstrument(obj.Item() - ITEM_MANDOLIN)
		case ITEM_LEECHES:
			player.SetPoison(0)
			backfire(ThePlayer, 2)
			usingPunt(obj, true, true)
			gameSprint(0xE0)
		case ITEM_SPIKE:
			spikeDoor(obj, how)
		case ITEM_SILVER_SEED:
			msg := 9
			switch plantSeed() {
			case 1:
				msg += 3
				usingPunt(obj, true, true)
			case -1:
				msg += 2
				usingPunt(obj, true, true)
			case 0:
				msg++
			}
			gameSprint(msg)
		case ITEM_ROCK_HAMMER:
			useThing(obj, useRockHammerOn)
		case ITEM_FISHING_POLE:
			if goFish() {
				if o := placeNew(nil, ITEM_FISH); o != nil {
					o.SetQuality(0x3F)
				}
			}
			mouseRelease(true)
		case ITEM_OIL_FLASK:
			useThing(obj, useOilOn)
		}
		return
	}
	switch obj.Item() {
	case ITEM_FOUNTAIN_12E:
		var spell, power int
		var flag bool
		if decodeObjSpell(obj, &spell, &power, &flag) {
			inanimateSpell(MapObjX, MapObjY, obj, who, spell, power)
			if spell == 4 {
				gameSprint(0xF9)
			} else {
				gameSprint(0xED)
			}
		} else {
			gameSprint(0xED)
		}
	case ITEM_GLOWING_ROCK:
		if who == ThePlayer {
			var o *Object
			var where, found int
			if CursorObjPtr != nil && CursorObjPtr.Item() == ITEM_GLOWING_ROCK {
				o = CursorObjPtr
			} else if o = FindObj(MAJOR_SPEC, 2, 9, 2, &where); o != nil {
				found = 1
			} else if o = FindObj(MAJOR_SPEC, 2, 9, 4, &where); o != nil {
				found = 2
			}
			if o != nil {
				o.SetLinkF(o.LinkF() + obj.LinkF())
				switch found {
				case 1:
					RedisplayInvSlot(where)
				case 2:
					DisplayOpenBag()
				}
				usingPunt(obj, how != 0, true)
			}
		}
	case ITEM_CAULDRON:
		gameSprint(0x112)
	}
}

// useBook reads a book or scroll; the map opens the map screen, an enchanted scroll casts.
func useBook(obj *Object, how int) {
	if how == 0 {
		return
	}
	if obj.Item() == ITEM_MAP {
		if inplist.mode == 1 {
			newscr(2)
		}
	} else if obj.ID&ID_ENCHANT == 0 || obj.Major() == MAJOR_RECT {
		if obj.ID&ID_FLAG10 != 0 {
			runcutscene(obj.LinkF()&0x1FF + 0x100)
		} else if obj.LinkF()&0x1FF < 0x100 {
			n, ok := getName(obj, false, false)
			if !ok {
				n = "UNNAMED"
			}
			scrollPrint("You read the " + n + "...\n")
			scrollPrint(getString(obj.LinkF()&0x1FF | STR_BOOKS))
			scrollPrint("\n")
		} else {
			makeStew()
		}
	} else {
		checkTrap(ThePlayer, obj, 4, MapObjX, MapObjY)
		checkSpell(MapObjX, MapObjY, ThePlayer, obj, how)
		usingPunt(obj, true, false)
	}
}

// useRect: doors, furniture, levers and switches.
func useRect(who, obj *Object) {
	switch obj.Minor() {
	case 0:
		if obj.InClass() < 8 {
			if checkLock(who, obj, 0) == 0 {
				if who.Item() == ITEM_ADVENTURER {
					n, ok := getName(obj, false, false)
					if !ok {
						n = "UNNAMED"
					}
					scrollPrint("The " + n + " is locked.\n")
				}
			} else if who == ThePlayer || obj.Owner()&1 == 0 {
				OpenDoor(who, obj)
			}
		} else {
			CloseDoor(obj)
		}
	case 2:
		if obj.InClass() == 1 || obj.InClass() == 2 {
			obj.SetFlags((obj.Flags() + 1) & 7)
			editchng(2)
		} else {
			rectLook(obj, -1)
		}
	case 1:
		if obj.InClass() == 7 {
			mantraAdvance()
		} else if (obj.InClass() == 0xB || obj.InClass() == 0xD) && obj.IsQuant() == 0 {
			useCont(who, obj, 0)
		}
	case 3:
		playEffect(0x13, MapObjX<<3+3, MapObjY<<3+3, 0)
		obj.SetInClass((obj.InClass() + 8) & 0xF)
		editchng(2)
	}
}

// checkLock tries key in door's lock (negative: picking with that skill). 0 locked or
// failed, 1 no lock, 2 the key locked it, 3 unlocked, 4 not locked.
func checkLock(who, door *Object, key int) int {
	if door.IsQuant() != 0 || door.LinkF() <= 0 {
		return 1
	}
	link := &door.OL
	lock := ObjInList(&link, false, MAJOR_SPEC, 0, 0xF)
	if lock == nil {
		return 1
	}
	if lock.ID&ID_FLAG9 == 0 {
		if key > 0 {
			if (door.Class() == CLASS_DOOR && door.InClass() >= 8) ||
				(door.Class() == CLASS_CONTAINER && door.InClass() < 0xC && door.ID&1 != 0) {
				return 4
			}
			if lock.LinkF()&0x1FF == key {
				lock.SetFlag9(1)
				return 2
			}
			return 0
		}
		return 4
	} else if key < 0 {
		if (lock.Z() == 0xE && -key < 0x1F) || lock.Z() == 0xF || skillCheck(-key, lock.Z()*3) <= 0 {
			return 0
		}
	} else if key > 0 {
		if lock.LinkF()&0x1FF == 0 || lock.LinkF()&0x1FF != key {
			return 0
		}
	} else {
		return 0
	}
	checkTrap(who, door, 6, MapObjX, MapObjY)
	if lock.ID&ID_FLAG10 == 0 {
		ObjRem(link, lock)
		ObjFree(lock)
		return 3
	}
	lock.SetFlag9(0)
	return 3
}

// checkSpell casts the charged spell obj carries, at most once every 0x2FD ticks for a
// use. True when a spell was cast.
func checkSpell(x, y int, who, obj *Object, how int) bool {
	var major, effect int
	var flag bool
	if !decodeObjSpell(obj, &major, &effect, &flag) || !flag {
		return false
	}
	src := obj
	if how != 0 {
		if player.GameClock < nextSpellTime {
			playEffectHere(0x15, 0x40, 0)
			return false
		}
		nextSpellTime = player.GameClock + 0x2FD
		src = who
	} else if who == ThePlayer && obj.Item() >= ITEM_WAND_98 && obj.Item() <= ITEM_WAND_9B {
		return false
	}
	inanimateSpell(x, y, src, who, major, effect)
	useNSpellCharges(obj)
	return true
}

// checkTrap sets off the trigger or trap obj holds for an action of kind how.
func checkTrap(who, obj *Object, how, x, y int) {
	// FARNULLTRAP: DOS reads the interrupt vectors at 0000:0000 as the object
	o := obj
	if o == nil {
		o = &Level.Objs[0]
	}
	if o.IsQuant() != 0 || o.LinkF() == 0 {
		return
	}
	link := &o.OL
	trap := ObjInList(&link, false, MAJOR_TRAP, -1, -1)
	if trap == nil {
		return
	}
	if trap.Minor() >= 2 {
		UseTrigger(who, obj, trap, how)
	} else if trap.Flags() == 0 && how == 4 {
		SetOffTrap(who, obj, trap, x, y)
		deleteTrap(link, trap)
	}
}

// moveDoor turns a door into a moving door (its item kept in the owner) and animates it.
func moveDoor(door *Object) {
	length := 5
	if door.InClass()&7 == 6 {
		length = 4
	}
	door.SetOwner(door.InMajor())
	door.SetItem(ITEM_MOVING_DOOR)
	addAnimobj(ObjMemTPtr(door), length, 0, MapObjX, MapObjY)
}

// changeDoor reverses a moving door.
func changeDoor(door *Object) {
	length := 5
	if door.Major() == MAJOR_RECT && door.InClass()&7 == 6 ||
		door.Major() == MAJOR_ANIMOBJ && door.Owner()&7 == 6 {
		length = 4
	}
	if door.Flags()&8 != 0 {
		door.SetFlags(door.Flags() & 7)
	} else {
		door.SetFlags(door.Flags()&7 + 8)
	}
	if cur := getAnimlen(door); cur >= 0 {
		setAnimlen(door, length-cur)
	}
}

// doorSound plays the door or portcullis sound at the door.
func doorSound(door *Object) {
	typ := 0xB
	if door.InClass()&7 == 6 {
		typ = 0x14
	}
	playEffect(typ, MapObjX<<3+door.FineX(), MapObjY<<3+door.FineY(), 0)
}

// OpenDoor opens a closed door or turns a closing one round.
func OpenDoor(who, door *Object) {
	if door.Item() == ITEM_MOVING_DOOR {
		state := door.Owner() & 0xF
		if state < 8 {
			return
		}
		door.SetOwner(state - 8)
		changeDoor(door)
	} else if door.InClass() < 8 {
		door.SetOwner(door.Owner() & 0xFFFE)
		if door.InClass() != 6 {
			door.SetZ(door.Z() + 0x18)
		}
		checkTrap(who, door, 7, MapObjX, MapObjY)
		moveDoor(door)
	} else {
		return
	}
	doorSound(door)
}

// CloseDoor closes an open door or turns an opening one round.
func CloseDoor(door *Object) {
	if door.Item() == ITEM_MOVING_DOOR {
		state := door.Owner() & 0xF
		if state >= 8 {
			return
		}
		door.SetOwner(state + 8)
		changeDoor(door)
	} else {
		if door.InClass() < 8 {
			return
		}
		moveDoor(door)
	}
	doorSound(door)
}

// ToggleDoor opens a closed door or closes an open one.
func ToggleDoor(who, door *Object) {
	if door.InClass() < 8 {
		OpenDoor(who, door)
	} else {
		CloseDoor(door)
	}
}

// dumpTheBag empties a container onto the floor.
func dumpTheBag(bag *Object, toPlayer bool) {
	owner := 0
	if ComObjData[bag.Item()].CanOwn() != 0 {
		owner = bag.Owner()
	}
	if !dropLinkChain(bag, owner) && toPlayer {
		n, _ := getName(bag, false, false)
		scrollPrint("The " + n + " is empty.\n")
	}
	editchng(2)
}

// useCont opens a container in the inventory, or empties it from the world.
func useCont(who, obj *Object, how int) {
	if checkLock(who, obj, 0) == 0 {
		n, ok := getName(obj, false, false)
		if !ok {
			n = "UNNAMED"
		}
		scrollPrint("The " + n + " is locked.\n")
	} else if how != 0 {
		OpenTheBag(FindSlot(obj))
	} else {
		dumpTheBag(obj, who == ThePlayer)
	}
}

// BlastFunction releases an aimed missile spell.
func BlastFunction() {
	releaseMissile(ObjectActing, ObjectActorArg)
	GameInputMode = 0
	unforceMouseCursor(3)
	mouseRelease(true)
}

// removeSpell clears the enchantment of an enchanted is_quant object.
func removeSpell(obj *Object) {
	if obj.IsQuant() != 0 && obj.ID&ID_ENCHANT != 0 && obj.Major() != MAJOR_RECT {
		obj.SetEnchanted(0)
	}
}

// useNSpellCharges spends a charge of obj's spell; with none left it may vanish.
func useNSpellCharges(obj *Object) {
	if obj.IsQuant() != 0 || obj.LinkF() == 0 {
		return
	}
	link := &obj.OL
	spell := ObjInList(&link, false, MAJOR_SPEC, 2, 0)
	if spell != nil && spell.ID&ID_FLAG11 != 0 {
		if spell.Quality() > 0 {
			spell.SetQuality(spell.Quality() - 1)
		} else if rand()*10/0x8000 < 4 {
			ObjRem(link, spell)
			ObjFree(spell)
		}
	}
}

// takeAfterUse: the inventory object just used needs a target, so it is picked up.
var takeAfterUse bool

// carriedUse is the carried object while useCarriedOn uses it; nil once used up.
var carriedUse *Object

// inInventory reports whether obj sits in an inventory slot or the player's object list.
func inInventory(obj *Object) bool {
	index := ObjMemTPtr(obj)
	for i := 0; i < NUM_INV_SLOTS; i++ {
		if Inventory[i].Index() == index {
			return true
		}
	}
	return ObjFind(&ThePlayer.OL, true, index) != nil
}

// carriedActor is the second half of obj's two-object use, or nil when obj has none.
func carriedActor(obj *Object) ActorFn {
	switch item := obj.Item(); {
	case item == ITEM_KEY_OF_INFINITY:
		return useKeyOfInfinityOn
	case item == ITEM_LOCKPICK:
		return useLockpickOn
	case obj.Major() == MAJOR_SPEC && obj.Minor() == 0 && item < ITEM_LOCK:
		return useKeyOn
	case item >= ITEM_SKULL_C2 && item <= ITEM_PILE_OF_BONES_C6:
		return useBonesOn
	case item == ITEM_ANVIL:
		return useAnvilOn
	case item == ITEM_POLE:
		return usePoleOn
	case item == ITEM_SPIKE:
		return spikeDoorOn
	case item == ITEM_ORB_ROCK:
		return tybalsOrbOn
	case item == ITEM_ROCK_HAMMER:
		return useRockHammerOn
	case item == ITEM_OIL_FLASK:
		return useOilOn
	}
	return nil
}

// useCarriedOn uses the carried object on target (a right click while carrying; other: 1
// for an inventory target). The object stays carried unless the use took it up.
func useCarriedOn(target *Object, other int) {
	obj := CursorObjPtr
	fn := carriedActor(obj)
	if fn == nil {
		scrollPrint("You cannot use that on anything.\n")
		return
	}
	if obj.Item() == ITEM_POLE {
		UsingPole = true
	}
	if other == 0 && (!InPickRange(PickDist, target, PickMap) || BlockingTerrain(PickDist, target)) {
		gameSprint(0x5E)
	} else {
		carriedUse, ObjectActing, ObjectActor = obj, obj, fn
		fn(target, 1, other)
	}
	UsingPole = false
	FixPlayerEquips()
	obj, carriedUse = carriedUse, nil
	if obj == nil {
		releaseCursor()
		return
	}
	CursorObjPtr = obj
	GameInputMode = GIM_CARRY
	forceMouseCursor(obj.Item())
}
