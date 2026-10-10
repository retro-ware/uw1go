package uw

// From WORLDEV.C: finding a free square near another, and teleporting.

// findGoodXAndY searches outward from (x, y), breadth first, at most 20 tiles a ring and
// within the 9x9 box around it, for a tile where obj fits. With clear it first throws out
// the tile's mobiles and objects with height.
func findGoodXAndY(obj *Object, x, y int, clear bool) (int, int, bool) {
	var curList, nextList [20][2]int8
	var visited [9]uint16
	xmin := min(max(1, x-4), 0x3A)
	ymin := min(max(1, y-4), 0x3A)
	xmax := min(xmin+9, 0x3E)
	ymax := min(ymin+9, 0x3E)
	ncur, nnext := 1, 0
	cur, next := &curList, &nextList
	cur[0] = [2]int8{int8(x), int8(y)}
	// the original's visited[] is 9 words but cleared by 9 bytes, and a ring reaches column
	// xmin+9: indexes past it are guarded
	visit := func(tx, ty int) {
		col := tx - xmin
		if col >= 0 && col < 9 && visited[col]&(1<<(ty-ymin)) != 0 {
			return
		}
		if nnext < 20 && tx >= xmin && tx <= xmax && ty >= ymin && ty <= ymax {
			next[nnext] = [2]int8{int8(tx), int8(ty)}
			nnext++
			if col < 9 {
				visited[col] |= 1 << (ty - ymin)
			}
		}
	}
	if c := x - xmin; c >= 0 && c < 9 {
		visited[c] |= 1 << (y - ymin)
	}
	for ncur > 0 {
		for i := 0; i < ncur; i++ {
			tx, ty := int(cur[i][0]), int(cur[i][1])
			tile := &Level.Tiles[(ty&63)<<6+tx&63]
			if clear {
				var o *Object
				for link := &tile.Objects; link.Index() != 0; link = &o.QN {
					o = ObjPtrTMem(link)
					if ComObjData[o.Item()].Height() != 0 || IsMobElem(o) {
						ObjPunt(&tile.Objects, o, false)
					}
				}
			}
			z := tile.Height() << 3
			if tileWalls[tile.Type()]&0x20 != 0 {
				z += 4
			}
			if canPlace(obj.Item(), ObjMemTPtr(obj), tx*8+3, ty*8+3, z, false, 8) {
				return tx, ty, true
			}
			switch tile.Type() {
			case TILE_SOLID:
			case TILE_DIAG_SE:
				visit(tx+1, ty)
				visit(tx, ty-1)
			case TILE_DIAG_SW:
				visit(tx-1, ty)
				visit(tx, ty-1)
			case TILE_DIAG_NE:
				visit(tx+1, ty)
				visit(tx, ty+1)
			case TILE_DIAG_NW:
				visit(tx-1, ty)
				visit(tx, ty+1)
			default:
				visit(tx-1, ty)
				visit(tx, ty-1)
				visit(tx+1, ty)
				visit(tx, ty+1)
			}
		}
		cur, next = next, cur
		ncur, nnext = nnext, 0
	}
	return 0, 0, false
}

// doTeleport sends who to square (x, y) of level (0 or this level: a free square near it).
// Only the player changes level, and its move is only recorded for the main loop. Returns
// 0x10 on success, else 2.
func doTeleport(who *Object, x, y, level int) int {
	dest := level
	if dest != PlayerLevel && who != ThePlayer {
		return 2
	}
	if (dest == 0 || dest == PlayerLevel) && x != 0x3F && y != 0x3F {
		dest = PlayerLevel
		nx, ny, ok := findGoodXAndY(who, x, y, false)
		if !ok {
			return 2
		}
		x, y = nx, ny
	}
	if who == ThePlayer {
		NewPlayerLevel = dest
		NewPlayerX = x
		NewPlayerY = y
		editchng(0x20)
	}
	return 0x10
}

// changeTerrain changes the tiles from (x, y) to (x+dx, y+dy): wall and floor textures
// (0x30 and 0xB or more unchanged), type (10 or more unchanged) and height (at most 13;
// adjust 1 and 3 raise or lower by one instead). Objects on a rising tile are lifted to
// the new floor; those resting on a sinking tile's old floor go down with it.
func changeTerrain(x, y, wall, floor, height, typ, dx, dy, adjust int) int {
	for cx := x; cx <= x+dx; cx++ {
		for cy := y; cy <= y+dy; cy++ {
			tile := MapGetAddr(cx, cy)
			old := tile.Height()
			if adjust == 1 || adjust == 3 {
				height = old + 2 - adjust
				if height >= 0 && height <= 0xD {
					tile.SetHeight(height)
				}
			} else if height <= 0xD {
				tile.SetHeight(height)
			}
			if tile.Height() > old {
				for obj := ObjPtrTMem(&tile.Objects); obj != nil; obj = ObjPtrTMem(&obj.QN) {
					if obj.Major() == MAJOR_TRAP {
						continue
					}
					if obj.Z() < height<<3 {
						obj.SetZ(height << 3)
						if IsMobElem(obj) && obj.Major() != MAJOR_CREATURE {
							obj.B0F = uint16(height << 6)
						} else if obj == ThePlayer {
							PN.Z = int16(height << 6)
						}
					}
				}
			} else if tile.Height() < old {
				for obj := ObjPtrTMem(&tile.Objects); obj != nil; obj = ObjPtrTMem(&obj.QN) {
					if obj.Major() == MAJOR_TRAP {
						continue
					}
					if obj.Z() == old<<3 {
						obj.SetZ(height << 3)
						if IsMobElem(obj) && obj.Major() != MAJOR_CREATURE {
							obj.B0F = uint16(height << 6)
						} else if obj == ThePlayer {
							parsePlayerTerr(0x10, true)
						}
					}
				}
			}
			if floor < 0xB {
				tile.SetFloor(floor)
			}
			if wall < 0x30 {
				tile.SetWall(wall)
			}
			if typ < 10 {
				tile.SetType(typ)
			}
		}
	}
	editchng(6)
	return 2
}

// whackThing is the damage trap: damage of type how to object index. Negative damage
// poisons the player at that strength (unless resisted), or is plain damage to others.
// 0x10 when the object was destroyed, else 2.
func whackThing(index, damage, how, extra int) int {
	target := ObjIntTMem(index)
	if target == nil {
		return 2
	}
	if damage < 0 {
		if target == ThePlayer {
			if -damage > player.Poison() && checkRes(ThePlayer, 1, 0x10) != 0 {
				player.SetPoison(-damage)
			}
		} else {
			damage = -damage
		}
	}
	if damage > 0 && damageItem(target, nil, target.HomeX(), target.HomeY(), int(byte(damage)), 4) {
		return 0x10
	}
	return 2
}

// inanimateSpell is a spell cast by a trap at (x, y): by class and minor (major >= 0)
// or by spell number.
func inanimateSpell(x, y int, trap, who *Object, major, effect int) int {
	inanmMapX, inanmMapY = x, y
	if major >= 0 {
		doSpell(int(byte(major)), int(byte(effect)), trap, who)
	} else {
		cast(int(byte(effect)), trap, who)
	}
	return 2
}

// goFish: the player must face water about a square ahead; a fish bites one time in
// five and is caught if he can carry it.
func goFish() bool {
	x, y := PN.X>>5, PN.Y>>5
	moveAlong(int(PlayerFacing>>8), 0xB, &x, &y)
	tile := MapGetAddr(int(x>>3), int(y>>3))
	switch {
	case tile.Type() == TILE_SOLID || int(TxmTerr[tile.Floor()]>>4) != TERRAIN_WATER ||
		ThePlayer.Z()>>3 <= int(uint16(tile.Height()-1)):
		gameSprint(0x65) // 'You cannot fish there. Perhaps somewhere else.'
	case rand()%5 != 0:
		gameSprint(0x64) // 'No luck this time.'
	case int(player.Weight)+ComObjData[ITEM_FISH].Mass() >= int(player.MaxWeight):
		gameSprint(0x66) // 'You feel a nibble, but the fish gets away.'
	default:
		gameSprint(0x63) // 'You catch a lovely fish.'
		return true
	}
	return false
}

// workBullfrogTiles is the bullfrog puzzle on level 4: game variables 24 and 25 select a
// square of the 8 by 8 area at (0x30, 0x30), 26 counts the moves left.
func workBullfrogTiles(mode, x, y int) {
	if PlayerLevel != 4 {
		gameSprint(0xBF)
		return
	}
	gv := &player.GameVars
	switch mode {
	case 0, 1:
		x = int(gv[0x18]) + 0x30
		y = int(gv[0x19]) + 0x30
		if gv[0x1A] > 1 {
			gv[0x1A]--
			tx, ty, dx, dy := x-1, y-1, 2, 2
			if gv[0x18] == 0 {
				tx = x
			}
			if gv[0x18] == 0 || gv[0x18] == 7 {
				dx = 1
			}
			if gv[0x19] == 0 {
				ty = y
			}
			if gv[0x19] == 0 || gv[0x19] == 7 {
				dy = 1
			}
			changeTerrain(tx, ty, 0x3F, 0xF, 0xF, 0xF, dx, dy, mode*2+1)
			changeTerrain(x, y, 0x3F, 0xF, 0xF, 0xF, 0, 0, mode*2+1)
		} else {
			gameSprint(0xC0) // 'There is an empty clicking sound.'
			gv[0x1A] = 1
		}
	case 2:
		gv[0x19] = (gv[0x19] + 1) & 7
	case 3:
		gv[0x18] = (gv[0x18] + 1) & 7
	case 4:
		gv[0x1A] = 0x3F
		changeTerrain(0x30, 0x30, 0x3F, 0xF, 4, 0xF, 7, 7, 0)
		gameSprint(0xC1) // 'A voice utters the words "Reset Activated."'
	}
}

// emeraldTrap: when each corner of the 9 by 9 square around (x, y) holds an emerald, a
// new object is put at (x, y+1) and the emeralds are destroyed.
func emeraldTrap(trap *Object, x, y int) {
	var tiles [4]*Tile
	var found [4]*Object
	count := 0
	for dx := -4; dx < 5; dx += 8 {
		for dy := -4; dy < 5; dy += 8 {
			tiles[count] = MapGetAddr(x+dx, y+dy)
			head := &tiles[count].Objects
			if found[count] = ObjInList(&head, false, 2, 2, 7); found[count] != nil {
				count++
			}
		}
	}
	if count == 4 {
		obj := CreateObj(ITEM_VAS_STONE, false)
		tile := MapGetAddr(x, y+1)
		obj.SetZ(0x40)
		obj.SetFineX(3)
		obj.SetFineY(3)
		ObjAdd(&tile.Objects, obj)
		objDeal(obj, x, y+1, 1)
		for i := 0; i < 4; i++ {
			ObjPunt(&tiles[i].Objects, found[i], true)
		}
	}
}

// playerDidBad is the trespass trap: the owner race is told.
func playerDidBad(owner int) { playerGrabbed(ThePlayer, int(byte(owner))) }

// eightPosSwitch: with the switch's position times 8 added to the trap's height, hack 3
// sets the height of the tile, hack 4 that of the trap's linked object.
func eightPosSwitch(flags int, trap *Object, x, y int) {
	height := trap.Z() + flags<<3
	if trap.Quality() == 3 {
		if height < 0x68 {
			changeTerrain(x, y, 0xFF, 0xFF, height>>3, 0xFF, 0, 0, 0)
		}
	} else if obj := ObjIntTMem(trap.LinkF() & 0x1FF); obj != nil {
		obj.SetZ(height)
	}
}

// explodingBook: an exploding book on the player's square blows up in his face.
func explodingBook(trap *Object, x, y int) {
	head := &MapGetAddr(ThePlayer.HomeX(), ThePlayer.HomeY()).Objects
	if book := ObjInList(&head, true, 4, 1, 4); book != nil {
		scrollPrint("The book explodes in your face!\n")
		player.Quests |= 1 << QUEST_BRONUS_BOOK
		backfire(ThePlayer, 3)
		InvRemoveOneObject(book)
		ObjPunt(nil, book, true)
		DisplayInventory()
		FixPlayerEquips()
	}
}

// talkingDoorTrap: a conversation with no body, whoami 0x19.
func talkingDoorTrap(trap *Object, x, y int) {
	door := CreateObj(FIRST_CREATURE, true)
	if door == nil {
		return
	}
	door.Whoami = 0x19
	door.SetAttitude(ATT_FRIENDLY)
	door.SetGoal(GOAL_TALK)
	TalkTo(door)
	ObjFree(door)
}

// doArialTalking: four frames of animation, then cutscene 3.
func doArialTalking(trap *Object, x, y int) {
	updateAnimobj(4)
	runcutscene(3)
}

// removeWhoami removes an NPC and its contents from the map (a gronk callback).
func removeWhoami(obj *Object) bool {
	ObjFreeLinkChain(&MapGetAddr(obj.HomeX(), obj.HomeY()).Objects, obj)
	return true
}

var tyballAllies = [10]byte{0xDE, 0xD1, 0xDB, 0xD2, 0xDC, 0xD5, 0xD8, 0xD4, 0xD3, 0xDD}

// tyballDeath: cutscene 2, a dream bit, his allies removed and the move triggers on
// square (0x17, 0x38) destroyed.
func tyballDeath() {
	runcutscene(2)
	player.Dreams |= 4
	for i := 9; i > 0; i-- {
		gronkWhoami(int(tyballAllies[i]), 0, 0, func(o *Object, _ int) bool { return removeWhoami(o) })
	}
	head := &MapGetAddr(0x17, 0x38).Objects
	var next *Object
	for obj := ObjPtrTMem(head); obj != nil; obj = next {
		next = ObjPtrTMem(&obj.QN)
		if obj.Item() == ITEM_MOVE_TRIGGER {
			ObjRem(head, obj)
			ObjFree(obj)
		}
	}
}

// deathCheck handles the deaths that matter to the plot (mode: the death happened; else
// the killing blow is about to land). False when the creature is spared.
func deathCheck(obj *Object, mode bool) bool {
	switch obj.Whoami {
	case 0x1B:
		if mode {
			player.SetTalismanOK(0)
		}
	case 0xE7:
		if mode {
			tyballDeath()
		}
	case 0x16:
		if !mode {
			if obj.Terrain() == 0 {
				obj.SetTerrain(1)
				playerGetExp(0x1F4)
			}
			obj.LastHit = 0
			clearFightState()
			obj.SetSeq(SEQ_STAND)
			obj.SetFrame(0)
			TalkTo(obj)
			return false
		}
	case 0x0B:
		if !mode {
			TalkTo(obj)
			obj.HP = 0x3C
			return false
		}
	case 0x6E:
		if mode {
			player.Quests |= 1 << QUEST_GAZER
		}
	case 0x8E:
		if mode {
			player.Quests |= 1 << QUEST_RODRICK
		}
	case 0x18:
		if mode {
			player.Quests |= 1 << QUEST_LIZARDMEN
		}
	}
	return true
}

// getRepDiff is an item's durability for repair; -1 if it cannot be repaired.
func getRepDiff(obj *Object) int {
	if obj.Major() == MAJOR_HACK {
		switch obj.Minor() {
		case 0:
			return int(int8(Weapons[obj.InClass()].Durability))
		case 2, 3:
			return int(int8(Armor[(obj.InMajor()-FIRST_ARMOR)&31].Durability))
		}
	}
	return -1
}

// doRepair is one repair attempt: 3 fully repaired, 2 partly, 1 no effect, -1 damaged,
// -2 destroyed, 0 not repairable. *time gets the minutes it takes.
func doRepair(obj *Object, skill int, time *int) int {
	durability := getRepDiff(obj)
	if durability == -1 {
		return 0
	}
	quality := obj.Quality()
	*time = max(durability*3-skill-quality/2, 15)
	var newQuality int
	switch skillCheck(skill, durability) {
	case 2:
		newQuality = 64
	case 1:
		newQuality = 3 + skill/5
	case 0:
		return 1
	case -1:
		if rand()&0x3F > obj.Quality()+skill {
			return -2
		}
		newQuality = -(4 + rand()&7)
	}
	if quality+newQuality > 63 {
		obj.SetQuality(63)
		return 3
	}
	if quality+newQuality > 0 {
		obj.SetQuality(quality + newQuality)
		if newQuality > 0 {
			return 2
		}
		return -1
	}
	obj.SetQuality(0)
	return -2
}

// repairItem repairs obj with skill, by the player (who) or by an NPC in a conversation.
func repairItem(obj *Object, skill int, who bool) {
	answer := true
	name, _ := getName(obj, false, false)
	if who {
		durability := getRepDiff(obj)
		if durability < 0 {
			gameSprint(0x8E) // 'You cannot repair that.'
			return
		}
		estimate := durability - skill + 15
		switch {
		case estimate < 0:
			estimate = 0
		case estimate > 30:
			estimate = 4
		default:
			estimate = estimate/10 + 1
		}
		gameSprint(0xD8)
		gameSprint(estimate + 0xDB)
		gameSprint(0xD9)
		scrollPrint(name)
		if reply := wyorn("", 0xDA, &answer); reply != 0 && reply < 4 {
			answer = reply == 2
			wdBool(answer)
		}
		scrollPrint("\n")
		if !answer {
			return
		}
	}
	runcutscene(0x104)
	var time int
	result := doRepair(obj, skill, &time)
	if who {
		playerdat.SetNoise(0xF)
		player.GameClock += uint32(int32(time<<8) * 60)
		if result == -2 {
			if ObjElemFate(10, obj) {
				InvRemoveOneObject(obj)
				ObjPunt(nil, obj, true)
			} else {
				result = 0
			}
		}
		gameSprint(result + 0x8E)
		if result != 0 {
			scrollPrint(name)
			gameSprint(0x53)
		}
		FixPlayerEquips()
		editchng(0x200)
	} else if result == -2 {
		ObjPunt(&MapGetAddr(MapObjX, MapObjY).Objects, obj, false)
	}
}

// clearLoretry clears an object's "already examined" bit, so a better Lore skill can
// identify it again.
func clearLoretry(obj *Object) bool {
	if IsMobElem(obj) || obj.Major() == MAJOR_RECT || obj.Major() == MAJOR_TRAP ||
		ComObjData[obj.Item()].Render() == 2 {
		return false
	}
	obj.SetHeading(obj.HeadingBits() & 3)
	return false
}

// clearAllLoretries clears the examined bit on every object on the map.
func clearAllLoretries() {
	for i := range Level.Tiles {
		head := &Level.Tiles[i].Objects
		if head.Index() > 0 {
			ObjCheck(ObjPtrTMem(head), clearLoretry)
		}
	}
}
