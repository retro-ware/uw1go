package uw

// Port of OBJPHYS.C: object physics: bouncing, missile hits, collisions between objects,
// copying an object to and from the physics record, moving objects between the static
// and mobile lists, and settling a placed object into its tile. The caller passes the
// object's tile in XP and YP.
//
// A mobile non-creature keeps its exact position in GoalWord and AttitudeWord (x and y in
// 1/256 tiles) and B0F (z in 1/8 units).

var (
	objhitUsed               bool
	objhitTilex, objhitTiley byte
	objhitMyx, objhitMyy     byte
	dealBounced, dealBlocked bool
)

func isMobile(obj *Object) bool { return ObjMemTPtr(obj) < NUM_MOBILE }

// bounceObj: CP (the moving object) pushes other. Returns 4, "bounce".
func bounceObj(obj, other *Object) int {
	if other != nil {
		var phys Phys
		pp := &phys
		XP, YP = int(objhitTilex), int(objhitTiley)
		getPhysData(other, pp)
		if pp.Mass != 0 {
			scale := int(CP.Mass) << 6 / int(pp.Mass)
			if scale > 0x80 {
				scale = 0x80
			}
			pp.Heading = CP.Heading
			pp.Speed = 0xEB
			pp.Vel[2] = int16(int(CP.Vel[2]) * scale / 0x40)
			XP, YP = int(objhitTilex), int(objhitTiley)
			setPhysData(other, pp)
		} else if pp.Speed != 0 {
			pp.Speed = 0
		}
	}
	return 4
}

// missileNewhit: proj strikes hit.
func missileNewhit(proj, hit *Object) {
	mi := &Missile[proj.InClass()]
	damage := int(mi.Damage)
	if proj.LastHit == 1 && mi.Ammo == -64 {
		scale := int(player.Skills[SKILL_MISSILE])<<3 + 0xC0
		switch skillCheck(int(player.Skills[SKILL_MISSILE]), 10) {
		case -1:
			scale -= 0x80
		case 2:
			scale += 0xC0
		}
		damage = int(uint16(damage*scale) >> 8)
	}
	x, y := int(objhitMyx), int(objhitMyy)
	if objhitUsed {
		x, y = int(objhitTilex), int(objhitTiley)
	}
	missileThwack(int(proj.LastHit), proj, hit, x, y, damage, -int(mi.Ammo))
}

// doObjhit: the object index (the mover) touches oCollisions[ci] (ci -1: the floor).
// Returns 2 to pass through, 4 to bounce, 0x10 if using destroyed the mover, or a
// trigger's result.
func doObjhit(ci, index int) int {
	if ci != -1 {
		c := &oCollisions[ci]
		if c.link.Low()&0x20 != 0 {
			return 2
		}
		c.link.SetLow(c.link.Low() | 0x20)
	}
	objhitMyx = byte(Ppd.x >> 3)
	objhitMyy = byte(Ppd.y >> 3)
	obj := ObjIntTMem(index)
	myItem := obj.Item()
	var other *Object
	item := -1
	touch := true
	if ci != -1 {
		c := &oCollisions[ci]
		other = ObjIntTMem(c.link.Index())
		item = int(c.offset) & MAP_MASK
		objhitTilex = byte((int(objhitMyx) + item) & MAP_MASK)
		item = int(objhitTilex) - int(objhitMyx)
		objhitTiley = byte((int(objhitMyy) + (int(c.offset)-item)/MAP_SIZE) & MAP_MASK)
		item = other.Item()
		touch = ComObjData[item].Touch() != 0
		if index < NUM_MOBILE && c.link.Index() < NUM_MOBILE {
			if myItem>>6 != MAJOR_CREATURE && obj.B157() != 0 {
				return 2
			}
			obj.SetB157(1)
		}
	}
	if item != -1 {
		if ComObjData[item].Usable() != 0 {
			MapObjX, MapObjY = int(objhitTilex), int(objhitTiley)
			objhitUsed = false
			UseObj(obj, other, 0)
		} else if item>>6 == MAJOR_TRAP {
			return UseTrigger(obj, nil, other, 0)
		}
	}
	if touch {
		if ComObjData[myItem].Usable() != 0 {
			MapObjX, MapObjY = int(objhitMyx), int(objhitMyy)
			objhitUsed = true
			UseObj(other, obj, 0)
			if MapObjX < 0 {
				return 0x10
			}
		}
		return bounceObj(obj, other)
	}
	return 2
}

// getPhysData fills pp from obj and its ComObjData entry.
func getPhysData(obj *Object, pp *Phys) {
	jitter := true
	co := &ComObjData[obj.Item()]
	pp.Index = int16(ObjMemTPtr(obj))
	pp.Mass = int16(co.Mass())
	pp.Light = byte(co.Light())
	pp.Bounce = byte(co.Bounce())
	pp.Resist = byte(co.Resist())
	pp.B1D = 0
	pp.Heading = int16(obj.HeadingBits() << 13)
	pp.B24 = 0
	pp.Radius = byte(co.Radius())
	pp.Height = byte(co.Height())
	pp.X = int16(obj.FineX())
	pp.Y = int16(obj.FineY())
	pp.Z = int16(obj.Z())
	if isMobile(obj) {
		pp.X += int16(obj.HomeX() << 3)
		pp.Y += int16(obj.HomeY() << 3)
		pp.Heading = int16(uint16(obj.Heading) << 8)
		pp.Terrain = 1 << obj.Terrain()
		pp.Vel[2] = int16((obj.Pitch() - 0x10) << 6)
		pp.Acc[2] = int16(int(obj.B13&0x80) >> 7 * -4)
		pp.HP = obj.HP
		if obj.Major() != MAJOR_CREATURE {
			jitter = false
			pp.X = int16(obj.GoalWord)
			pp.Y = int16(obj.AttitudeWord)
			pp.Z = int16(obj.B0F)
		}
		pp.Speed = int16(obj.Speed())
		if obj.Major() != MAJOR_CREATURE && pp.Acc[2]|pp.Vel[2] == 0 && co.NoHit() == 0 {
			light := int(int8(pp.Light))
			if light*2+2 >= int(pp.Speed) {
				pp.Speed = 0
			} else {
				pp.Speed = int16(obj.Speed() * (light<<2 + 0x29))
			}
		} else {
			pp.Speed *= 0x2F
			if obj.Major() == MAJOR_CREATURE {
				pp.B24 = 8
			}
		}
	} else {
		pp.Speed, pp.Acc[2], pp.Vel[2] = 0, 0, 0
		pp.HP = byte(obj.Quality())
		pp.X += int16(XP << 3)
		pp.Y += int16(YP << 3)
	}
	if jitter {
		pp.X = pp.X<<5 + int16(rand()&0x1F)
		pp.Y = pp.Y<<5 + int16(rand()&0x1F)
		pp.Z = pp.Z<<3 + int16(rand()&7)
	}
	pp.Impact = 0
}

// setPhysData writes pp back to obj, whose tile is XP, YP on entry. Returns whether obj
// is still mobile.
func setPhysData(obj *Object, pp *Phys) bool {
	if int(pp.X>>8) != XP || int(pp.Y>>8) != YP {
		if t := MapGetAddr(XP, YP); t != nil {
			ObjRem(&t.Objects, obj)
		}
		XP, YP = int(pp.X>>8), int(pp.Y>>8)
		if t := MapGetAddr(XP, YP); t != nil {
			ObjAdd(&t.Objects, obj)
		}
	}
	obj.SetZ(int(pp.Z >> 3))
	obj.SetFineX(int(pp.X>>5) & 7)
	obj.SetFineY(int(pp.Y>>5) & 7)
	storeHP := func() {
		if isMobile(obj) {
			obj.HP = pp.HP
		} else {
			obj.QN.SetLow(int(pp.HP))
		}
	}
	storeHP()
	if pp.Impact > 0x100 {
		damage := int(pp.Impact >> 8)
		mass := ComObjData[obj.Item()].Mass()
		if obj.Major() != MAJOR_CREATURE {
			playEffectOnMobile(0xF, obj, (mass-600)/50)
		}
		damageItem(obj, nil, XP, YP, damage, 0)
	}
	storeHP()
	if pp.Terrain&FOOT_LAVA != 0 && rand()%5 == 0 {
		damageItem(obj, nil, XP, YP, 1, DMG_FIRE)
	}
	if obj.Major() != MAJOR_CREATURE {
		moving := pp.Speed|pp.Vel[2]|pp.Acc[2] != 0
		if ObjMemTPtr(obj) > NUM_MOBILE {
			if moving {
				obj = staticToMob(obj)
			}
		} else if !moving {
			obj.SetTerrain(int(resToTerr[pp.Terrain&0x1F]))
			if obj = mobToStatic(obj); obj == nil {
				return false
			}
			if obj = objDeal(obj, XP, YP, 0); obj == nil {
				return false
			}
			if isMobile(obj) {
				updateHackVecs(pp)
			}
		}
	}
	if obj != nil && isMobile(obj) {
		obj.Heading = byte(uint16(pp.Heading) >> 8)
		obj.SetHomeX(XP)
		obj.SetHomeY(YP)
		obj.SetGravity(b2i(pp.Acc[2] == -4))
		v := int(pp.Vel[2])/0x40 + 0x10
		if v < 0 {
			v = 0
		} else if v > 0x1F {
			v = 0x1F
		}
		obj.SetPitch(v)
		obj.SetSpeed(int(pp.Speed) / 0x2F)
		obj.SetTerrain(int(resToTerr[pp.Terrain&0x1F]))
		if obj.Major() != MAJOR_CREATURE {
			obj.GoalWord = uint16(pp.X)
			obj.AttitudeWord = uint16(pp.Y)
			obj.B0F = uint16(pp.Z)
		}
		return true
	}
	if obj != nil && obj.Major() == MAJOR_RECT {
		obj.SetHeading(int(uint16(pp.Heading) >> 13))
	}
	return false
}

// staticToMob replaces the static object obj, in tile XP, YP, with a new mobile copy.
// It returns the mobile object, or nil if none is free (obj then stays).
func staticToMob(obj *Object) *Object {
	tile := MapGetAddr(XP, YP)
	mob := ObjAlloc(true)
	if mob != nil && tile != nil {
		setStatic(mob, obj)
		mobInit(mob, XP, YP)
		mob.HP = byte(obj.Quality())
		if obj.Major() != MAJOR_RECT && ComObjData[obj.Item()].Render() != 2 {
			mob.Whoami = byte(obj.HeadingBits())
		}
		if mob.Major() == MAJOR_ANIMOBJ {
			changeAnimPtr(mob, obj)
		}
		ObjRem(&tile.Objects, obj)
		ObjFree(obj)
		ObjAdd(&tile.Objects, mob)
	}
	return mob
}

// mobInit sets a new mobile object's motion fields.
func mobInit(obj *Object, x, y int) {
	obj.Heading = byte(obj.HeadingBits() << 5)
	obj.SetFineHead(0)
	obj.SetPitch(0x10)
	obj.SetGravity(1 - ComObjData[obj.Item()].NoHit())
	obj.SetHomeX(x)
	obj.SetHomeY(y)
	obj.SetBin(curBin + 1)
	obj.SetRate(2)
	obj.SetSpeed(0)
	obj.SetInvis(0)
	obj.HP = 0x3F
	obj.SetTerrain(0)
	if obj.Major() != MAJOR_CREATURE {
		obj.GoalWord = uint16(x<<8 + obj.FineX()<<5 + 0xF)
		obj.AttitudeWord = uint16(y<<8 + obj.FineY()<<5 + 0xF)
		obj.B0F = uint16(obj.Z() << 3)
		obj.LastHit = 0
	}
}

// mobToStatic replaces the mobile object obj, in tile XP, YP, with a new static copy and
// releases the mobile record. Its type's fate may destroy it instead.
func mobToStatic(obj *Object) *Object {
	keep := true
	fate := ComObjData[obj.Item()].Fate()
	var who int
	if obj.Terrain() == 1 {
		fate = 8
		putEffect(obj, 6, 3, 0, 0, XP, YP)
	} else if obj.Terrain() == 2 && fate == 10 && PlayerLevel == 8 &&
		abs(XP-32)+abs(YP-32) < 6 && ThePlayer.HP != 0 {
		if player.TalismanOK() != 0 {
			fate = 8
			keep = false
			questChanged()
			player.Talismans--
			if player.Talismans == 0 {
				gameSprint(0x116)
				editchng(0x400)
			} else {
				obj.SetItem(ITEM_EXPLOSION_1C2)
				for i := 8; int(player.Talismans) <= i; i-- {
					obj.SetZ(obj.Z() + rand()&7 + 4)
					fireballEffect(obj, XP+rand()%3-1, YP+rand()%3-1)
				}
			}
		} else {
			player.Dreams |= 8
		}
	}
	if fate > 0 && fate <= 8 && rand()&7 < fate && ObjElemFate(10, obj) {
		keep = false
	}
	if PlayerLevel == LEVEL_VOID {
		keep = false
	}
	tile := MapGetAddr(XP, YP)
	if tile == nil {
		return nil
	}
	head := &tile.Objects
	var st *Object
	if keep {
		if st = ObjAlloc(false); st != nil {
			setStatic(st, obj)
			obj.OL = 0
			if st.Major() == MAJOR_ANIMOBJ {
				changeAnimPtr(st, obj)
			} else if st.Class() == CLASS_LIGHT && st.InClass() >= 4 && st.InClass() <= 6 {
				st.SetInClass(st.InClass() - 4)
			}
			st.QN.SetLow(int(obj.HP))
			if st.Major() != MAJOR_RECT && st.Major() != MAJOR_TRAP && ComObjData[st.Item()].Render() != 2 {
				st.SetHeading(int(obj.Whoami))
			}
		}
	}
	if fate == 9 {
		if obj.Major() != MAJOR_CREATURE {
			who = int(obj.LastHit)
		}
	}
	ObjPunt(head, obj, true)
	if st != nil {
		ObjAdd(head, st)
	}
	if fate == 9 && !mtsDoanim(st, XP, YP, who) {
		return ObjPunt(head, st, false)
	}
	return st
}

// updateHackVecs: after objDeal set a settled object moving again, it slides off an
// object it landed on the edge of, or tumbles.
func updateHackVecs(pp *Phys) {
	if dealBounced {
		pp.Heading += int16(rand()&0x3FFF) - 0x2000
		pp.Speed = 0xBC
	} else {
		pp.Speed = int16((rand() + 1) & 3 * 0x2F)
		pp.Acc[2] = -4
	}
}

// objDeal settles obj in tile x, y: destroyed in a wall, a high floor, another object or
// water; in lava it may burn; over a drop it becomes mobile and falls. Returns the object,
// or nil if destroyed.
func objDeal(obj *Object, x, y, how int) *Object {
	destroy := false
	retried := false
	low := false
	dealBlocked, dealBounced = false, false
	var calc MotionCalc
	for {
		curP = &calc
		curP.index = int16(ObjMemTPtr(obj))
		co := &ComObjData[obj.Item()]
		if co.NoHit() != 0 {
			return obj
		}
		curP.radius = byte(co.Radius())
		curP.height = byte(co.Height())
		curP.z = int16(obj.Z())
		curP.x = int16(x<<3 + obj.FineX())
		curP.y = int16(y<<3 + obj.FineY())
		TerrainCheck(curP.radius)
		low = int(curP.floor)+int(curP.radius) >= int(curP.z) && !retried
		ObjectCheck(low, true)
		processObjlist()
		MP.hit = -1
		if curP.count == 0 && curP.first > 0 && int(curP.first) <= int(curP.found) {
			for curP.first--; curP.first >= 0; curP.first-- {
				if int(oCollisions[curP.first].top) != int(curP.z) {
					break
				}
				MP.hit = curP.first
				MP.item = int16(ObjPtrTMem(&oCollisions[MP.hit].link).Item())
				if ComObjData[MP.item].Solid() == 1 {
					if oCollisions[MP.hit].link.Low()&0x10 != 0 {
						dealBlocked = true
						break
					}
					dealBounced = true
				}
			}
		}
		if (curP.hits0|curP.hits1)&0x300 != 0 || curP.count != 0 {
			destroy = true
		} else if curP.hits0&7 == 5 {
			destroy = true
		} else if curP.hits0&7 == 6 {
			if ComObjData[obj.Item()].QualClass() == 3 {
				destroy = false
			} else {
				destroy = checkRes(obj, 1, DMG_FIRE) != 0
			}
		} else if curP.hits0&8 == 0 && !dealBlocked {
			if low {
				retried = true
				continue
			}
			oldx, oldy := XP, YP
			XP, YP = x, y
			if m := staticToMob(obj); m != nil {
				obj = m
			}
			XP, YP = oldx, oldy
			if dealBounced {
				obj.SetSpeed(3)
				obj.Heading += byte(rand()%9<<4) + 0xC0
			}
			if how != 0 {
				obj.SetSpeed(rand()&3 + 1)
				obj.SetPitch(rand()&3 + 0xE)
			}
		}
		break
	}
	if destroy {
		if t := MapGetAddr(x, y); t != nil {
			return ObjPunt(&t.Objects, obj, false)
		}
	}
	return obj
}
