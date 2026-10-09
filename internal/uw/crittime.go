package uw

// Port of CRITTIME.C: critters while time passes away from them (a level left, the player
// asleep), wandering monsters, the record of the last critter hit, and owners' reactions
// to theft.

var wanderFound bool

// changeCritterGoal is critter_set_goal (AI.C) for any critter.
func changeCritterGoal(npc *Object, goal, gtarg int) {
	save := meptr
	meptr = npc
	critterSetGoal(goal, gtarg)
	meptr = save
}

// activeMobs is the active mobile list (ActiveMob .. LastActiveMob).
func activeMobs() []byte { return Level.Active[:nActive] }

// yearlyCheckup: for every active mobile the fed bit becomes random and three times in
// four the ally bit is cleared.
func yearlyCheckup() {
	for _, i := range activeMobs() {
		npc := &Level.Objs[i]
		npc.SetFed(rand() % 2)
		if rand()%4 != 1 {
			npc.SetAlly(0)
		}
	}
}

// setGridxAndYBasedOnTileType is where in a tile of this type to place a critter; false
// for a solid tile. TILE_DIAG_NW gets (6, 1) like TILE_DIAG_SE, as in the original.
func setGridxAndYBasedOnTileType(typ int) (x, y int, ok bool) {
	switch typ {
	case TILE_SOLID:
		return 0, 0, false
	case TILE_DIAG_SE, TILE_DIAG_NW:
		return 6, 1, true
	case TILE_DIAG_SW:
		return 1, 1, true
	case TILE_DIAG_NE:
		return 6, 6, true
	}
	return 4, 4, true
}

// upCrit lets time pass for one critter.
func upCrit(npc *Object, counts *[64]int8) {
	cst := &Creatures[npc.InMajor()]
	xhome, yhome := npc.HomeX(), npc.HomeY()
	hometile := MapGetAddr(xhome, yhome)
	if npc.Temp() != 0 {
		ObjFreeLinkChain(&hometile.Objects, npc)
		return
	}
	npc.SetFed(0)
	npc.SetAlly(0)
	npc.SetB194(0)
	npc.SetB195(0)
	npc.SetB190(0)
	npc.SetB191(0)
	npc.SetHeading(rand() % 8)
	if npc.HP < cst.Avghit && npc.NoHeal() == 0 {
		npc.HP = byte((int(npc.HP) + int(cst.Avghit)) / 2)
	}
	if npc.Loner() == 0 {
		switch npc.Attitude() {
		case ATT_HOSTILE:
			counts[cst.Race&0x3F]--
		case ATT_FRIENDLY:
			counts[cst.Race&0x3F]++
		}
	}
	x, y := npc.Quality(), npc.Owner()
	tile := MapGetAddr(x, y)
	if xhome == x && yhome == y {
		return
	}
	fx, fy, ok := setGridxAndYBasedOnTileType(tile.Type())
	if !ok {
		return
	}
	z := tile.Height() << 3
	if cst.Flier() != 0 {
		z = byte8((tile.Height()<<3 + 0x80) >> 1)
	}
	if !canPlace(npc.Item(), ObjMemTPtr(npc), x<<3+fx, y<<3+fy, z, cst.Flier() != 0, 8) {
		return
	}
	ObjRem(&hometile.Objects, npc)
	ObjAdd(&tile.Objects, npc)
	npc.SetHomeX(x)
	npc.SetHomeY(y)
	npc.SetFineX(fx)
	npc.SetFineY(fy)
	npc.SetZ(z)
}

func byte8(v int) int { return v & 0xFF }

// upMob lets time pass for a mobile object that is not a critter: it is deleted if
// ObjPunt allows, or else made static and dropped. Always returns true.
func upMob(obj *Object) bool {
	XP = obj.HomeX()
	x := XP<<3 + obj.FineX()
	YP = obj.HomeY()
	y := YP<<3 + obj.FineY()
	tile := MapGetAddr(XP, YP)
	head := &tile.Objects
	if ObjPunt(head, obj, false) == nil {
		return true
	}
	stopped := mobToStatic(obj)
	if stopped == nil {
		return true
	}
	ObjRem(head, stopped)
	stayCentered = true
	if !dropAroundPlace(stopped, x, y, tile.Height()<<3, 6) {
		stopped.SetZ(tile.Height() << 3)
		ObjAdd(head, stopped)
	}
	return true
}

// updateAllCrittersWhilstPlayerSnoozes lets time pass for the level, then shifts each
// non-loner's attitude by its race's net mood.
func updateAllCrittersWhilstPlayerSnoozes() {
	var counts [64]int8
	for p := 0; p < nActive; p++ {
		npc := &Level.Objs[Level.Active[p]]
		if npc.Major() == MAJOR_CREATURE {
			upCrit(npc, &counts)
		} else if upMob(npc) {
			p--
		}
	}
	for _, i := range activeMobs() {
		npc := &Level.Objs[i]
		if npc.Loner() != 0 {
			continue
		}
		cst := &Creatures[npc.InMajor()]
		c := int(counts[cst.Race&0x3F])
		if c != 0 {
			att := int(int8(npc.Attitude() + c))
			if c < 0 {
				att = max(att, 0)
			} else if att > ATT_FRIENDLY {
				att = ATT_FRIENDLY
			}
			npc.SetAttitude(att)
		}
	}
}

var hostileFound bool

func checkForHostileCreature(x, y int, target *Object, tile *Tile, src int) bool {
	if ObjMemTPtr(target) == 1 {
		return false
	}
	if g := target.Goal(); g == GOAL_ATTACK || g == GOAL_GUARD || g == GOAL_CORNERED {
		if target.B190() != 0 {
			hostileFound = true
		}
	}
	return false
}

// hostileCreaturesNear: is a hostile, alert critter within 2 tiles of the player?
func hostileCreaturesNear() bool {
	hostileFound = false
	gronkArea(ThePlayer, 0x7F, checkForHostileCreature, 0, 0, 2)
	return hostileFound
}

// wanderThatMonster may bring a hostile critter near the sleeping player.
func wanderThatMonster(x, y int, target *Object, where *Tile, src int) bool {
	if ObjMemTPtr(target) == 1 {
		return false
	}
	obj := target
	if obj.Attitude() > ATT_HOSTILE {
		return false
	}
	if rand()%2 == 0 {
		return false
	}
	setCritterVars(obj)
	px, py := ThePlayer.HomeX(), ThePlayer.HomeY()
	dist := (myxpos-px)*(myxpos-px) + (myypos-py)*(myypos-py)
	if mycst.Range()*mycst.Range()*3 < dist {
		return false
	}
	if floodPath(myxpos, myypos, meptr.Z()>>3, px, py, ThePlayer.Z()>>3, 0) && pathlen >= 2 {
		for i := 0; i < pathlen; i++ {
			tile := MapGetAddr(int(pathsq[i].x), int(pathsq[i].y))
			for link := &tile.Objects; link.Index() != 0; {
				next := ObjPtrTMem(link)
				if next.Major() == MAJOR_TRAP && next.Minor() == MINOR_TRIGGER && next.LinkF() > 0 {
					trap := ObjPtrTMem(&next.OL)
					if trap.Major() == MAJOR_TRAP && trap.Minor() == MINOR_TRAP && trap.InClass() == TRAP_WARD {
						UseTrap(trap, int(pathsq[i].x), int(pathsq[i].y))
					}
				}
				link = &next.QN
			}
		}
		tx, ty := int(pathsq[pathlen-2].x), int(pathsq[pathlen-2].y)
		tile := MapGetAddr(tx, ty)
		fx, fy, ok := setGridxAndYBasedOnTileType(tile.Type())
		if !ok {
			return false
		}
		z := tile.Height() << 3
		if canPlace(obj.Item(), ObjMemTPtr(obj), tx<<3+fx, ty<<3+fy, z, mycst.Flier() != 0, 8) {
			ObjRem(&MapGetAddr(myxpos, myypos).Objects, obj)
			ObjAdd(&tile.Objects, obj)
			obj.SetHomeX(tx)
			obj.SetHomeY(ty)
			obj.SetFineX(fx)
			obj.SetFineY(fy)
			obj.SetZ(z)
			obj.SetB190(1)
			setLoc(px, py, ThePlayer.Z()>>3)
			wanderFound = true
			return true
		}
	}
	return false
}

// wanderingMonsterCheck: one hostile critter within 8 tiles may come to the sleeping
// player.
func wanderingMonsterCheck() bool {
	wanderFound = false
	gronkArea(ThePlayer, 1, wanderThatMonster, 0, 0, 8)
	return wanderFound
}

// setCreaturesFromSavedGame restores AI.C's record of the last critter hit.
func setCreaturesFromSavedGame() {
	crithit = int(player.Crithit)
	typehit = int(player.Typehit)
	crithittime = player.Crithittime
	hitx = int(player.Hitx)
	hity = int(player.Hity)
}

func setCreaturesToSavedGame() {
	player.Crithit = byte(crithit)
	player.Typehit = byte(typehit)
	player.Crithittime = crithittime
	player.Hitx = byte(hitx)
	player.Hity = byte(hity)
}

// initLevelCreatureStuff forgets the last critter hit and frees the paths.
func initLevelCreatureStuff() {
	crithit = 0
	typehit = 0xFF
	clearPaths()
}

var (
	stolen    *Object
	grabOwner int
)

// critterGetTold: a critter of the owning race who sees the stolen object grows less
// friendly.
func critterGetTold(x, y int, target *Object, tile *Tile, src int) bool {
	npc := target
	cst := &Creatures[npc.InMajor()]
	if int(cst.Race) != grabOwner&0x1F || npc.Loner() != 0 && grabOwner&0x20 == 0 ||
		grabOwner == 0x20 && npc.Loner() == 0 {
		return false
	}
	if grabOwner == 0x0D && player.QuestBytes[QB_CRUX] >= 3 {
		return false
	}
	nx := x<<3 + npc.FineX()
	ny := y<<3 + npc.FineY()
	nz := npc.Z() + ComObjData[npc.Item()].Height()
	ox := MapObjX<<3 + stolen.FineX()
	oy := MapObjY<<3 + stolen.FineY()
	oz := stolen.Z() + ComObjData[stolen.Item()].Height() + 12
	dx, dy := (nx-ox)/8, (ny-oy)/8
	if dx*dx+dy*dy > cst.Sight()*cst.Sight() {
		return false
	}
	if lineOfSight(nx, ny, nz, ox, oy, oz) {
		att := max(npc.Attitude()-1, 0)
		npc.SetAttitude(att)
		name, _ := getName(npc, true, false)
		scrollPrint(name + getString((att+0xE1)|STR_GAME))
		return true
	}
	return false
}

// playerGrabbed: the player picked up obj, an owned object (or owner names the owner).
func playerGrabbed(obj *Object, owner int) {
	grabOwner = 0
	if owner > 0 {
		grabOwner = owner
	} else if ComObjData[obj.Item()].CanOwn() != 0 {
		grabOwner = obj.Owner()
	}
	if grabOwner != 0 {
		stolen = obj
		processArea(0x14, 0, critterGetTold, 0, MapObjX-7, MapObjY-7, 0xF, 0xF)
		if obj.Owner()&0x1F <= 0x1B {
			obj.SetOwner(0)
		}
	}
}
