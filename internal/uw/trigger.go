package uw

// Port of TRIGGER.C (ovr153): triggers and traps. Triggers and traps are MAJOR_TRAP
// objects (minor 0 traps, 2 triggers). A trigger's quality and owner name a target
// square and its link a trap there; UseTrigger runs the trap chain when the action kind
// matches the trigger's mode (Triggers[], from OBJECTS.DAT). Each trap passes on to the
// object its link names. A trigger without ID_FLAG10 is used up.

var (
	removeTrapIndex            int
	removeTrapFlags            int
	objRunCodeAround           *Object
	characterThatTriggeredTrap *Object
	triggeringButton           *Object
	createdObjectFound         bool
)

// UseTrigger fires trigger trig for an action of kind typ by who on object start (typ
// -1: reached along a trap chain, no checks). 2 when nothing ran.
func UseTrigger(who, start, trig *Object, typ int) int {
	if trig == nil || trig.Minor() != 2 {
		return 2
	}
	sub := trig.InClass()
	if typ >= 0 {
		if start != nil && start.Class() == CLASS_SWITCH && start.InClass() > 7 && trig.Next() != 0 {
			return UseTrigger(who, start, ObjPtrTMem(&trig.QN), typ)
		}
		if int(Triggers[sub]) != typ {
			return 2
		}
		if who != nil && who.Item() == ITEM_ADVENTURER {
			if trig.ID&ID_FLAG11 == 0 {
				return 2
			}
			if typ == 5 && trig.Z() > 0 && skillCheck(int(player.Skills[SKILL_SEARCH]), trig.Z()) <= 0 {
				return 2
			}
		} else {
			// FARNULLTRAP: a null who reads object 0's fields
			w := who
			if w == nil {
				w = &Level.Objs[0]
			}
			if w.Major() == MAJOR_CREATURE && (trig.ID&ID_ENCHANT == 0 || trig.Major() == MAJOR_RECT) {
				return 2
			}
			if w.Major() != MAJOR_CREATURE && trig.ID&ID_ENCHANT != 0 &&
				trig.Major() != MAJOR_RECT && trig.ID&ID_FLAG11 == 0 {
				return 2
			}
		}
	}
	trap := ObjPtrTMem(&trig.OL)
	quality, owner := trig.Quality(), trig.Owner()
	if trap == nil {
		return 2
	}
	result := SetOffTrap(who, start, trap, quality, owner)
	if trig.ID&ID_FLAG10 == 0 && trig.LinkF() > 0 {
		deleteTrap(&MapGetAddr(quality, owner).Objects, trap)
		result |= 0x20
	}
	return result
}

// SetOffTrap runs trap at square (x, y), remembering who set the chain off and with
// which object.
func SetOffTrap(who, context, trap *Object, x, y int) int {
	if characterThatTriggeredTrap == nil {
		characterThatTriggeredTrap = who
		triggeringButton = context
	}
	result := UseTrap(trap, x, y)
	characterThatTriggeredTrap = nil
	return result
}

// UseTrap performs one trap at square (x, y), then passes on to the trap or trigger its
// link names unless the trap ends the chain.
func UseTrap(trap *Object, x, y int) int {
	result := 2
	continueChain := true
	who := characterThatTriggeredTrap
	switch trap.InMajor() {
	case TRAP_ARROW:
		trapFire(trap, x, y)
	case TRAP_TELEPORT:
		result = doTeleport(who, trap.Quality(), trap.Owner(), trap.Z())
	case TRAP_CHANGE_TERRAIN:
		i := trap.HeadingBits()<<1 + trap.Quality()&1
		if i == 15 {
			i = 10
		}
		result = changeTerrain(x, y, trap.Owner(), trap.Quality()>>1, trap.Z()>>3, i,
			trap.FineX(), trap.FineY(), 0)
	case TRAP_SET_VARIABLE:
		questChanged()
		i := trap.Z()
		d := (trap.Quality()<<5|trap.Owner())<<3 | trap.FineY()
		c := trap.HeadingBits()
		if i == 0 {
			bit := int32(1) << uint(d&31)
			switch c {
			case 5:
				player.Quests ^= bit
			case 1:
				player.Quests &^= bit
			default:
				player.Quests |= bit
			}
		} else {
			v := &player.GameVars[i&0x3F]
			switch c {
			case 0:
				*v += byte(d)
			case 1:
				*v -= byte(d)
			case 2:
				*v = byte(d)
			case 3:
				*v &= byte(d)
			case 4:
				*v |= byte(d)
			case 5:
				*v ^= byte(d)
			case 6:
				*v <<= uint(d)
			}
			*v &= 0x3F
		}
	case TRAP_CHECK_VARIABLE:
		i := trap.Z()
		c := i + trap.HeadingBits()
		d := (trap.Quality()<<5|trap.Owner())<<3 | trap.FineY()
		v := 0
		for ; i <= c; i++ {
			gv := int(player.GameVars[i&0x3F])
			if trap.FineX() != 0 {
				v += gv
			} else {
				v = v<<3 | gv&7
			}
		}
		if v != d && trap.LinkF() > 0 {
			obj := ObjPtrTMem(&trap.OL)
			if obj.Next() > 0 {
				return UseTrigger(who, triggeringButton, ObjPtrTMem(&obj.QN), -1)
			}
			return 2
		}
	case TRAP_DAMAGE:
		h := 0
		if rand()*10/0x8000 < 7 {
			h = 2
		}
		dmg := trap.Quality()
		if trap.Owner() != 0 {
			dmg = -dmg
		}
		result = whackThing(ObjMemTPtr(who), dmg, 4, h)
	case TRAP_DO:
		result = doTrapHack(trap, x, y)
	case TRAP_SPELL:
		result = inanimateSpell(x, y, trap, who, trap.Quality(), trap.Owner())
	case TRAP_CREATE_OBJECT:
		if rand()*0x3F/0x8000 < trap.Quality() {
			return 2
		}
		continueChain = false
		if trap.IsQuant() != 0 {
			break
		}
		linked := ObjPtrTMem(&trap.OL)
		if linked == nil {
			break
		}
		if linked.Major() == MAJOR_CREATURE && dontCreateWanderingMonsterHere(linked) {
			break
		}
		mob := IsMobElem(linked)
		if obj := ObjAlloc(mob); obj != nil {
			if mob {
				*obj = *linked
			} else {
				setStatic(obj, linked)
			}
			stayCentered = true
			placed := putAt(x<<3+obj.FineX(), y<<3+obj.FineY(), obj.Z(), obj, 4, false)
			stayCentered = false
			if placed && obj.IsQuant() == 0 && obj.LinkF() > 0 {
				if cp := ObjAlloc(false); cp != nil {
					setStatic(cp, ObjIntTMem(obj.LinkF()))
					obj.SetLinkF(ObjMemTPtr(cp))
					cp.SetNext(0)
					if cp.IsQuant() == 0 && cp.LinkF() > 0 {
						cp.SetLinkF(0)
					}
				}
			}
			if placed && obj.Major() == MAJOR_ANIMOBJ {
				addAnimobj(ObjMemTPtr(obj), -1, 0, x, y)
			}
		}
		result = 2
	case TRAP_DOOR:
		continueChain = false
		head := &MapGetAddr(x, y).Objects
		linked := ObjInList(&head, false, MAJOR_RECT, 0, -1)
		MapObjX, MapObjY = x, y
		if linked != nil {
			head = &linked.OL
			if lock := ObjInList(&head, false, MAJOR_SPEC, 0, 0xF); lock != nil {
				ObjRem(head, lock)
				ObjFree(lock)
			}
			if trap.IsQuant() == 0 && trap.LinkF() != 0 {
				lock := ObjPtrTMem(&trap.OL)
				if obj := ObjAlloc(false); obj != nil {
					setStatic(obj, lock)
					ObjAdd(head, obj)
				}
			}
			switch trap.Quality() {
			case 1:
				OpenDoor(who, linked)
			case 2:
				CloseDoor(linked)
			case 3:
				ToggleDoor(who, linked)
			}
		} else if linked = ObjInList(&head, false, MAJOR_ANIMOBJ, -1, 0xF); linked != nil {
			if linked.Owner()&0xF < 8 {
				if q := trap.Quality(); q == 2 || q == 3 {
					CloseDoor(linked)
				}
			} else if q := trap.Quality(); q == 1 || q == 3 {
				OpenDoor(who, linked)
			}
		}
	case TRAP_DELETE_OBJECT:
		head := &MapGetAddr(trap.Quality(), trap.Owner()).Objects
		if obj := ObjPtrTMem(&trap.OL); obj != nil {
			ObjFreeLinkChain(head, obj)
		}
		editchng(2)
		continueChain = false
	case TRAP_INVENTORY:
		i := trap.Quality()<<5 | trap.Owner()
		var slot int
		obj := FindObj(i>>6, (i&ID_MINOR)>>4, i&0xF, 4, &slot)
		if obj == nil {
			return 2
		}
		if trap.Z() > 0 && obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 && trap.Z() > obj.LinkF() {
			return 2
		}
	case TRAP_WARD, TRAP_TELL:
		if who != nil && (trap.Quality() == 0x3F || trap.Quality() == who.InClass()) {
			damage := rand()*int(player.Skills[SKILL_CASTING])/0x8000 + 3
			printPathTo(getString(0xF5|STR_GAME), ThePlayer.HomeX(), ThePlayer.HomeY(), 0,
				who.HomeX(), who.HomeY(), 0, 0)
			result = whackThing(ObjMemTPtr(who), damage, 4, 0)
		}
		continueChain = false
	case TRAP_TEXT_STRING:
		dprintf("Look, it's a text trap\n")
		if id := trap.Quality()<<5 | trap.Owner() | STR_TRAPTEXT; stringExists(id) {
			scrollPrint(getString(id))
		}
	}
	if continueChain && trap.LinkF() > 0 {
		next := ObjPtrTMem(&trap.OL)
		if next.Major() == MAJOR_TRAP {
			if next.Minor() < 2 {
				result |= UseTrap(next, x, y)
			} else {
				result |= UseTrigger(characterThatTriggeredTrap, triggeringButton, next, -1)
			}
		}
	}
	return result
}

// killTriggers removes from the list, and from containers, every trigger whose link is
// removeTrapIndex.
func killTriggers(head *Link) {
	obj := ObjPtrTMem(head)
	for obj != nil {
		if obj.Class() == CLASS_TRIGGER && obj.LinkF() == removeTrapIndex {
			ObjRem(head, obj)
			ObjFree(obj)
			obj.OL &= 0x3F
			removeTrapFlags--
		}
		if obj.IsQuant() == 0 && obj.OL.Index() != 0 {
			killTriggers(&obj.OL)
		}
		obj = ObjPtrTMem(&obj.QN)
	}
}

// deleteTrap deletes trap and its chain, and the triggers that point at it.
func deleteTrap(head *Link, trap *Object) {
	removeTrapFlags = trap.Flags()
	if removeTrapFlags != 0 {
		removeTrapIndex = ObjMemTPtr(trap)
		for i := 0; removeTrapFlags > 0 && i < MAP_TILES; i++ {
			if h := &Level.Tiles[i].Objects; h.Index() > 0 {
				killTriggers(h)
			}
		}
	}
	if trap = ObjFind(head, true, ObjMemTPtr(trap)); trap != nil {
		ObjFreeLinkChain(ObjFindHead, trap)
	}
}

// gronkifyTalkToPlayer: a waiting critter talks to the player and goes home afterwards.
func gronkifyTalkToPlayer(npc *Object, _ int) bool {
	if npc.Goal() == GOAL_STAND_7 {
		npc.SetGoal(GOAL_GO_HOME)
	}
	mouseFreereign()
	TalkTo(npc)
	return false
}

// doTrapHack runs the hack traps (trap type 3), chosen by the trap's quality.
func doTrapHack(trap *Object, x, y int) int {
	switch trap.Quality() {
	case 2:
		crystalBall(trap, x, y)
	case 3, 4:
		b := triggeringButton
		if b == nil {
			b = &Level.Objs[0]
		}
		eightPosSwitch(b.Flags(), trap, x, y)
	case 5:
		playerDidBad(trap.Owner())
	case 0x18:
		workBullfrogTiles(trap.Owner(), x, y)
	case 0x28:
		emeraldTrap(trap, x, y)
	case 0x29:
		explodingBook(trap, x, y)
	case 0x2A:
		talkingDoorTrap(trap, x, y)
	case 0x32:
		gronkWhoami(0xD8, 0, 0, gronkifyTalkToPlayer)
	case 0x39:
		doArialTalking(trap, x, y)
	case 0x3C, 0x3D, 0x3E:
		if characterThatTriggeredTrap == ThePlayer {
			QuakeTrap(trap.Quality()-0x3B, trap.Owner())
		}
	case 0x3F:
		EndGameMode = true
		editchng(0x400)
	}
	return 2
}

// castTrapSpell builds a move trigger and a trap of type sub at square (x, y), the
// trigger aimed at its own square. Returns the trigger's index.
func castTrapSpell(x, y, sub int) int {
	trigger := ObjAlloc(false)
	if trigger == nil {
		return 0
	}
	trap := ObjAlloc(false)
	if trap == nil {
		ObjFree(trigger)
		return 0
	}
	tile := MapGetAddr(x, y)
	trigger.SetInvis(1)
	trigger.SetDoorDir(1)
	trigger.SetMajor(MAJOR_TRAP)
	trigger.SetMinor(MINOR_TRIGGER)
	trigger.SetInClass(0)
	trigger.SetZ(tile.Height() << 3)
	trigger.SetIsQuant(1)
	trigger.SetFineX(3)
	trigger.SetFineY(3)
	trigger.SetHeading(0)
	trigger.SetFlag11(0)
	trigger.SetEnchanted(1)
	trigger.SetFlag10(0)
	trigger.QN = 0
	trigger.SetLinkF(ObjMemTPtr(trap))
	trigger.SetQuality(x)
	trigger.SetOwner(y)
	ObjAdd(&tile.Objects, trigger)
	trap.SetMajor(MAJOR_TRAP)
	trap.SetMinor(MINOR_TRAP)
	trap.SetInClass(sub)
	trap.SetInvis(1)
	trap.SetDoorDir(1)
	trap.SetZ(tile.Height() << 3)
	trap.SetIsQuant(1)
	trap.SetFineX(3)
	trap.SetFineY(3)
	trap.SetNext(0)
	trap.SetLinkF(0)
	trap.SetQuality(0x3F)
	trap.SetFlags(1)
	ObjAdd(&tile.Objects, trap)
	return ObjMemTPtr(trigger)
}

// triggerObjDel deletes a trigger: the last trigger of a trap deletes the trap too.
func triggerObjDel(head *Link, obj *Object) {
	linked := ObjPtrTMem(&obj.OL)
	if linked == nil {
		ObjRem(head, obj)
		ObjFree(obj)
		return
	}
	if flags := linked.Flags(); flags == 1 {
		deleteTrap(&MapGetAddr(obj.Quality(), obj.Owner()).Objects, linked)
	} else {
		linked.SetFlags(flags - 1)
		ObjRem(head, obj)
		ObjFree(obj)
	}
}

// trapObjDel deletes a trap or trigger object.
func trapObjDel(head *Link, obj *Object) {
	if obj.Minor() > 1 {
		triggerObjDel(head, obj)
	} else {
		deleteTrap(head, obj)
	}
}

// isThisWandering notes a temporary object near objRunCodeAround, other than the player.
func isThisWandering(x, y int, obj *Object, _ *Tile, _ int) bool {
	if obj.Temp() != 0 && obj != objRunCodeAround && obj != ThePlayer {
		createdObjectFound = true
	}
	return createdObjectFound
}

// dontCreateWanderingMonsterHere: a created object is already within four squares.
func dontCreateWanderingMonsterHere(obj *Object) bool {
	createdObjectFound = false
	objRunCodeAround = obj
	gronkArea(obj, 1, isThisWandering, 0, 0, 4)
	return createdObjectFound
}

// checkAlert: false when the player is within 8 squares of (x, y), so a change would be
// seen; true otherwise or when isPlayer is clear.
func checkAlert(isPlayer bool, x, y int) bool {
	if !isPlayer {
		return true
	}
	return !(iabs(ThePlayer.HomeX()-x) < 8 && iabs(ThePlayer.HomeY()-y) < 8)
}

// DoWanderingMonsters runs every create-object trap whose template is a mobile, out of
// the player's sight.
func DoWanderingMonsters(isPlayer int) {
	x, y := 0, 0
	for {
		obj := ObjFindInMap(MAJOR_TRAP, MINOR_TRAP, TRAP_CREATE_OBJECT, &x, &y)
		if obj == nil {
			break
		}
		if obj.Flags() == 0 {
			if n := ObjPtrTMem(&obj.OL); n != nil && IsMobElem(n) {
				n.SetTemp(1)
				if checkAlert(isPlayer != 0, x, y) {
					UseTrap(obj, x, y)
				}
			}
		}
		x++
	}
}

// DoClosingDoors: each open door closes with chance 3 in 10 where the player cannot see
// it; with isPlayer clear the moving doors are run to the end.
func DoClosingDoors(isPlayer int) {
	x, y := 0, 0
	for {
		door := ObjFindInMap(MAJOR_RECT, MINOR_DOOR, -1, &x, &y)
		if door == nil {
			break
		}
		if MapGetAddr(x, y).Door()&2 == 0 && door.InClass() >= 8 && rand()*10/0x8000 < 3 {
			MapObjX, MapObjY = x, y
			if checkAlert(isPlayer != 0, x, y) {
				CloseDoor(door)
			}
		}
		x++
	}
	if isPlayer == 0 && DoAnimO {
		for i := 0; i < 8; i++ {
			updateAnimobj(1)
		}
	}
}
