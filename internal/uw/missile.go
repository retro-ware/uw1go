package uw

// Port of MISSILE.C (seg025): launching missiles. Each front end (playerFire,
// critterFire, spellFire, trapFire, ReturnObject's throw) fills in the launch globals and
// calls missileFire, which creates the projectile as a mobile, aims it, lifts it to the
// source's eye height, steps it clear of the source and adds it to the map. The motion
// code flies it from there.

var (
	missileClass           int
	missileX, missileY     int
	missileItem            int
	missileSrc             *Object
	missileArc             int
	missileTrx, missileTry int
)

// playerSettr aims from the mouse position in the 3D view. True when the pointer is low
// enough that a drop is a throw.
func playerSettr() bool {
	x, y := mouseGetxy()
	if x -= 0x34; x > 0xAC {
		x = 0xAC
	} else if x < 0 {
		x = 0
	}
	if y -= 0x43; y > 0x71 {
		y = 0x71
	} else if y < 0 {
		y = 0
	}
	missileTrx = (x-0x56)*5/0xD + 1
	missileTry = int(PlayerPitch)/0x300 + (y-0x38)/6
	return y >= 0x25
}

// copyThrown gives a launched projectile the properties of the object it was made from.
func copyThrown(proj, src *Object) {
	proj.SetIsQuant(src.IsQuant())
	proj.SetLinkF(src.LinkF())
	proj.SetFlags(src.Flags())
	proj.HP = byte(src.Quality())
	proj.SetOwner(src.Owner())
	proj.SetDoorDir(src.DoorDir())
	if src.Major() != MAJOR_RECT && ComObjData[src.Item()].Render() != 2 {
		proj.Whoami = byte(src.HeadingBits())
	}
}

// playerFire fires the player's missile weapon (8 sling, 9 bow, 10 crossbow).
func playerFire(weapon int) {
	slot := checkAmmo(weapon)
	if slot < 0 {
		return
	}
	ammo := int(Missile[weapon].Ammo)
	missileClass = int(Missile[ammo].Type)
	missileItem = ammo + FIRST_MISSILE
	missileX, missileY = ThePlayer.HomeX(), ThePlayer.HomeY()
	missileArc = 1
	missileSrc = ThePlayer
	playerSettr()
	if proj := missileFire(); proj != nil {
		ammoObj := RemoveOneFromSlot(0, 1, ammo, slot)
		copyThrown(proj, ammoObj)
		ObjFree(ammoObj)
	} else {
		gameSprint(0xFE) // 'You need more space to fire that weapon.'
	}
	if weapon == ITEM_BOW-FIRST_MISSILE || weapon == ITEM_CROSSBOW-FIRST_MISSILE {
		playEffectHere(9, 0x40, 0)
	}
}

// critterFire: a critter shoots straight along its heading.
func critterFire(who *Object, item, typ int) {
	missileItem = item + FIRST_MISSILE
	missileClass = typ
	missileX, missileY = who.HomeX(), who.HomeY()
	missileArc = 1
	missileSrc = who
	missileTrx = 0
	missileFire()
}

// spellFire launches a missile spell. An object caster fires flat from inanmMapX,
// inanmMapY.
func spellFire(who *Object, spell int) bool {
	missileItem = spell + FIRST_MISSILE
	missileClass = int(Missile[spell].Type)
	missileX, missileY = who.HomeX(), who.HomeY()
	missileArc = 1
	missileSrc = who
	if who == ThePlayer {
		playerSettr()
	} else {
		if ObjMemTPtr(who) >= NUM_MOBILE {
			missileX, missileY = inanmMapX, inanmMapY
			missileTry = 0
			missileArc = 0
		}
		missileTrx = 0
	}
	return missileFire() != nil
}

// ReturnObject throws or drops the cursor object. True when it left the cursor.
func ReturnObject(obj *Object, message bool) bool {
	missileX, missileY = ThePlayer.HomeX(), ThePlayer.HomeY()
	if inplist.mode == 1 && playerSettr() {
		missileArc = 1
		missileSrc = ThePlayer
		missileItem = obj.Item()
		missileClass = 0xF
		if thrown := missileFire(); thrown != nil {
			copyThrown(thrown, obj)
			ObjFree(obj)
			obj = nil
		}
	}
	if obj == nil {
		return true
	}
	x := int16(missileX<<3 + ThePlayer.FineX())
	y := int16(missileY<<3 + ThePlayer.FineY())
	obj.SetZ(ThePlayer.Z())
	dist := int(byte(ComObjData[ThePlayer.Item()].Radius() + ComObjData[obj.Item()].Radius() + 1))
	heading := ThePlayer.HeadingBits()<<5 + ThePlayer.FineHead()
	moveAlong(heading, dist, &x, &y)
	cannot := !canPlace(obj.Item(), 0, int(x), int(y), ThePlayer.Z(), true, dist)
	if !cannot {
		moveAlong(heading, 3, &x, &y)
		cannot = !canPlace(obj.Item(), 0, int(x), int(y), ThePlayer.Z(), true, dist)
	}
	tx, ty := int(x)>>3, int(y)>>3
	tile := MapGetAddr(tx, ty)
	if cannot {
		if message {
			gameSprint(0xFD) // 'There is no space to drop that.'
		}
		playEffectHere(0xF, 0x40, -10)
		return false
	}
	obj.SetFineX(int(x) & 7)
	obj.SetFineY(int(y) & 7)
	ObjAddEnd(&tile.Objects, obj)
	// lit lights 4..6 go out; a lit taper (7) stays lit
	if obj.Class() == CLASS_LIGHT && obj.InClass() >= 4 && obj.InClass() <= 6 {
		obj.SetInClass(obj.InClass() - 4)
	}
	objDeal(obj, tx, ty, 1)
	return true
}

// trapFire: a missile trap fires item quality*32+owner from square (x, y).
func trapFire(trap *Object, x, y int) {
	missileItem = trap.Quality()<<5 | trap.Owner()
	missileClass = 0x14
	missileTrx, missileTry = 2, 2
	missileX, missileY = x, y
	missileSrc = trap
	missileArc = 0
	missileFire()
}

// missileFire creates and launches the missile the launch globals describe.
func missileFire() *Object {
	proj := ObjAlloc(true)
	if proj == nil {
		return nil
	}
	proj.SetNext(0)
	proj.SetIsQuant(1)
	proj.SetLinkF(1)
	proj.SetItem(missileItem)
	if missileArc != 0 {
		missileArc = missileSrc.FineHead()
	}
	missileArc += missileSrc.HeadingBits() << 5
	missileArc += missileTrx
	missileArc = (missileArc + 0x100) & 0xFF
	mobInit(proj, missileX, missileY)
	proj.SetHeading(missileArc >> 5)
	proj.SetFineHead(missileArc)
	proj.Heading = byte(missileArc)
	proj.SetDoorDir(0)
	proj.SetZ(missileSrc.Z())
	proj.SetFineX(missileSrc.FineX())
	proj.SetFineY(missileSrc.FineY() & 7)
	if h := ComObjData[missileSrc.Item()].Height(); h != 0 {
		z := proj.Z()
		proj.SetZ(z + h*5/6 + missileTry*2)
		if missileSrc == ThePlayer && player.SwimCount > 0x50 {
			proj.SetZ(z + missileTry*2 + (h - int(player.SwimCount>>3)))
		}
		if !pushMissile(proj, missileSrc) {
			ObjFree(proj)
			return nil
		}
	}
	if proj.Major() != MAJOR_CREATURE {
		launcher := 0
		proj.GoalWord = uint16(proj.HomeX()<<8 + proj.FineX()<<5 + 0xF)
		proj.AttitudeWord = uint16(proj.HomeY()<<8 + proj.FineY()<<5 + 0xF)
		proj.B0F = uint16(proj.Z() << 3)
		if missileSrc.Major() == MAJOR_CREATURE {
			if launcher = ObjMemTPtr(missileSrc); launcher >= NUM_MOBILE {
				launcher = 0
			}
		}
		proj.LastHit = byte(launcher)
		proj.SetB157(0)
	}
	proj.SetPitch(int(byte(missileTry)) + 0x10)
	proj.SetRate(1)
	proj.SetSpeed(int(byte(missileClass)))
	if ComObjData[missileItem].CanOwn() != 0 {
		proj.SetOwner(0)
	}
	ObjAdd(&MapGetAddr(proj.HomeX(), proj.HomeY()).Objects, proj)
	playEffectOnMobile(0xA, proj, 0)
	return proj
}

// pushMissile moves proj just clear of src, if nothing is in the way.
func pushMissile(proj, src *Object) bool {
	var calc MotionCalc
	item := proj.Item()
	curP = &calc
	curP.index = int16(ObjMemTPtr(proj))
	curP.radius = byte(ComObjData[item].Radius())
	curP.height = byte(ComObjData[item].Height())
	curP.x = int16(proj.HomeX()<<3 + proj.FineX())
	curP.y = int16(proj.HomeY()<<3 + proj.FineY())
	moveAlong(proj.HeadingBits()<<5+proj.FineHead(),
		ComObjData[src.Item()].Radius()+ComObjData[item].Radius()+4, &curP.x, &curP.y)
	curP.z = int16(proj.Z())
	ObjectCheck(false, true)
	TerrainCheck(0)
	if (calc.hits0|calc.hits1)&0x300 != 0 {
		return false
	}
	if curP.found != 0 {
		processObjlist()
		if curP.count != 0 {
			return false
		}
	}
	proj.SetHomeX(int(curP.x) >> 3)
	proj.SetHomeY(int(curP.y) >> 3)
	proj.SetFineX(int(curP.x) & 7)
	proj.SetFineY(int(curP.y) & 7)
	return true
}
