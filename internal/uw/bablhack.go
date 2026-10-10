package uw

// BABLHACK.C: conversation built-ins that reach into the game: critters' attitudes,
// skills, game variables and quest flags, objects taken and placed, doors, and an
// object's fields. They act on talkingTo and the player.

// setAttitude is set_attitude(arg2 whoami, arg1 attitude): the first active critter with
// that whoami takes it.
func setAttitude(top int) {
	att := bargv(top, 1)
	who := bargv(top, 2)
	gronkWhoami(who, 0, att, func(npc *Object, a int) bool {
		npc.SetAttitude(a)
		return false
	})
}

// setRaceAttitude is set_race_attitude(arg3 race, arg2 attitude, arg1 range): every
// critter of the talker's item and that race, not a loner, near the talker's home.
func setRaceAttitude(top int) {
	rng := bargv(top, 1)
	att := bargv(top, 2)
	race := bargv(top, 3)
	item := talkingTo.Item()
	x0 := max(talkingTo.HomeX()-rng, 1)
	y0 := max(talkingTo.HomeY()-rng, 1)
	x1 := talkingTo.HomeX() + rng
	if x1 >= MAP_SIZE {
		x1 = 0x3F
	}
	y1 := talkingTo.HomeY() + rng
	if y1 >= MAP_SIZE {
		y1 = 0x3F
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			for obj := ObjPtrTMem(&MapGetAddr(x, y).Objects); obj != nil; obj = ObjPtrTMem(&obj.QN) {
				if obj.Item() == item && obj.Loner() == 0 && int(Creatures[obj.InMajor()].Race) == race {
					obj.SetAttitude(att)
				}
			}
		}
	}
}

// xSkills is x_skills(arg2 skill, arg1 value): 10000 calls getSkill, 0..30 sets it.
func xSkills(top int) int {
	skill := bargv(top, 2) & 0xFF
	val := bargv(top, 1)
	if skill >= len(player.Skills) {
		return 0
	}
	if val == 10000 {
		getSkill(skill)
	} else if val >= 0 && val <= 30 {
		player.Skills[skill] = byte(val)
	}
	return int(player.Skills[skill])
}

// xTraps is x_traps(arg2 variable, arg1 value): one of the 64 game variables.
func xTraps(top int) int {
	v := bargv(top, 2) & 0x3F
	val := bargv(top, 1)
	if val >= 0 && val <= 0x3F {
		player.GameVars[v] = byte(val)
	}
	return int(player.GameVars[v])
}

// placeObject is place_object(arg3 object, arg2 x, arg1 y): take an object from the
// talker and put it at tile (x, y), or at the player's feet for a negative x.
func placeObject(top int) int {
	index := bargv(top, 3)
	obj := ObjIntTMem(index)
	x := bargv(top, 2)
	y := bargv(top, 1)
	if obj == nil {
		return 0
	}
	link := &talkingTo.OL
	for link.Index() != 0 && link.Index() != index {
		link = &ObjPtrTMem(link).QN
	}
	if link.Index() != 0 {
		ObjRem(&talkingTo.OL, obj)
	}
	if x < 0 {
		stayCentered = true
		putAt(ThePlayer.HomeX()<<3, ThePlayer.HomeY()<<3, ThePlayer.Z(), obj, 6, true)
		stayCentered = false
		return 1
	}
	if x < 1 || x >= MAP_SIZE || y < 1 || y >= MAP_SIZE {
		return 0
	}
	tile := MapGetAddr(x, y)
	obj.SetZ(tile.Height() << 3)
	if canPlace(obj.Item(), index, x<<3, y<<3, tile.Height()<<3, true, ComObjData[obj.Item()].Radius()+4) {
		ObjAddEnd(&tile.Objects, obj)
		objDeal(obj, x, y, 1)
		return 1
	}
	return 0
}

// takeFromNpcInv is take_from_npc_inv(arg1 n): the n-th object of the talker's inventory.
func takeFromNpcInv(top int) int {
	n := bargv(top, 1)
	link := &talkingTo.OL
	for i := 0; i < n && link.Index() != 0; i++ {
		link = &ObjPtrTMem(link).QN
	}
	return link.Index()
}

// addToNpcInv is add_to_npc_inv(arg1 object).
func addToNpcInv(top int) {
	if obj := ObjIntTMem(bargv(top, 1)); obj != nil {
		ObjAddEnd(&talkingTo.OL, obj)
	}
}

// removeTalker takes the talker off the map.
func removeTalker() {
	tile := MapGetAddr(talkingTo.HomeX(), talkingTo.HomeY())
	ObjPunt(&tile.Objects, talkingTo, true)
}

// setQuest is set_quest(arg2 quest, arg1 value): 0..31 bits of one long (the bit an int
// widened with its sign, so 15 also sets 16..31 and higher bits do nothing), 32..35 bytes.
func setQuest(top int) {
	quest := bargv(top, 2)
	val := bargv(top, 1)
	if quest < 0 {
		return
	}
	if quest < FIRST_QUEST_BYTE {
		bit := int32(int16(1 << (quest & 15)))
		if quest >= 16 {
			bit = 0
		}
		if val != 0 {
			player.Quests |= bit
		} else {
			player.Quests &^= bit
		}
	} else if quest < FIRST_QUEST_BYTE+4 {
		player.QuestBytes[quest-FIRST_QUEST_BYTE] = byte(val)
	}
}

// getQuest is get_quest(arg1 quest); above 35 the talisman count.
func getQuest(top int) int {
	quest := bargv(top, 1)
	switch {
	case quest < 0:
		return 0
	case quest < FIRST_QUEST_BYTE:
		bit := int32(int16(1 << (quest & 15)))
		if quest >= 16 {
			bit = 0
		}
		return b2i(player.Quests&bit != 0)
	case quest < FIRST_QUEST_BYTE+4:
		return int(player.QuestBytes[quest-FIRST_QUEST_BYTE])
	}
	return int(player.Talismans)
}

// sexFn is sex: arg2 for a male player, arg1 for a female one.
func sexFn(top int) int { return getmem(barg(top, 2-player.Female())) }

// gronkDoor is gronk_door(arg3 x, arg2 y, arg1 how): open, close or toggle the door at a
// tile; 0 when there is none.
func gronkDoor(top int) int {
	x, y := bargv(top, 3), bargv(top, 2)
	head := &MapGetAddr(x, y).Objects
	door := ObjInList(&head, false, MAJOR_RECT, 0, -1)
	if door == nil {
		door = ObjInList(&head, false, MAJOR_ANIMOBJ, 0, 0xF)
		if door == nil {
			return 0
		}
	}
	ox, oy := MapObjX, MapObjY
	MapObjX, MapObjY = x, y
	switch bargv(top, 1) {
	case 0:
		OpenDoor(nil, door)
	case 1:
		CloseDoor(door)
	case 2:
		ToggleDoor(nil, door)
	}
	MapObjX, MapObjY = ox, oy
	return 1
}

// xObjStuff is x_obj_stuff(arg9 object, arg8 set, arg7 heading, arg6 owner, arg5 flags,
// arg4 link, arg3 flag10, arg2 flag9, arg1 quality): write or read an object's fields; a
// variable holding -1 is left alone.
func xObjStuff(top int) {
	heading := bmem(barg(top, 7))
	owner := bmem(barg(top, 6))
	flags := bmem(barg(top, 5))
	link := bmem(barg(top, 4))
	flag10 := bmem(barg(top, 3))
	flag9 := bmem(barg(top, 2))
	quality := bmem(barg(top, 1))
	obj := ObjIntTMem(bargv(top, 9))
	if obj == nil {
		obj = &Level.Objs[0] // FARNULLTRAP
	}
	hasHeading := obj.Major() != MAJOR_RECT && ComObjData[obj.Item()].Render() != 2
	if bargv(top, 8) != 0 {
		if *heading != -1 && hasHeading {
			obj.SetHeading(int(*heading))
		}
		if *owner != -1 {
			obj.SetOwner(int(*owner))
		}
		if *flags != -1 {
			obj.SetFlags(int(*flags))
		}
		if *link != -1 {
			obj.SetLinkF(LINK_SPECIAL | int(*link))
		}
		if *flag10 != -1 {
			obj.SetFlag10(int(*flag10))
		}
		if *flag9 != -1 {
			obj.SetFlag9(int(*flag9))
		}
		if *quality != -1 {
			obj.SetQuality(int(*quality))
		}
		return
	}
	if *heading != -1 && hasHeading {
		*heading = int16(obj.HeadingBits())
	}
	if *owner != -1 {
		*owner = int16(obj.Owner())
	}
	if *flags != -1 {
		*flags = int16(obj.Flags())
	}
	if *link != -1 {
		*link = int16(obj.LinkF() & 0x1FF)
	}
	if *flag10 != -1 {
		*flag10 = int16(obj.ID & ID_FLAG10)
	}
	if *flag9 != -1 {
		*flag9 = int16(obj.ID & ID_FLAG9)
	}
	if *quality != -1 {
		*quality = int16(obj.Quality())
	}
}

// xObjPos is x_obj_pos(arg5 object, arg4 mode, arg3 x, arg2 y, arg1 z): mode non-zero
// sets an object's fine position and height (z above 0x7F: tile (x, y)'s floor), else
// reads them.
func xObjPos(top int) {
	x := bmem(barg(top, 3))
	y := bmem(barg(top, 2))
	z := bmem(barg(top, 1))
	obj := ObjIntTMem(bargv(top, 5))
	if obj == nil {
		obj = &Level.Objs[0] // FARNULLTRAP
	}
	if bargv(top, 4) != 0 {
		if *x != -1 {
			obj.SetFineX(int(*x))
		}
		if *y != -1 {
			obj.SetFineY(int(*y))
		}
		if *z != -1 {
			if *z > 0x7F {
				obj.SetZ(MapGetAddr(int(*x)&0x3F, int(*y)&0x3F).Height() << 3)
			} else {
				obj.SetZ(int(*z))
			}
		}
		return
	}
	if *x != -1 {
		*x = int16(obj.FineX())
	}
	if *y != -1 {
		*y = int16(obj.FineY())
	}
	if *z != -1 {
		*z = int16(obj.Z())
	}
}
