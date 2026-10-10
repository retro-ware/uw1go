package uw

// Port of DAMAGE.C: damaging objects. damageItem applies resistances, then sends critters
// to damageCritter (AI.C) and other objects to damageObject, removing them through
// removeObject once destroyed.

// removeLock removes the first lock in obj's contents, or every one with all. True only
// when all was set and a lock was found.
func removeLock(obj *Object, all bool) bool {
	found := false
	if obj.IsQuant() == 0 && obj.LinkF() > 0 {
		head := &obj.OL
		for {
			lock := ObjInList(&head, true, MAJOR_SPEC, 0, 0xF)
			if lock == nil {
				break
			}
			ObjRem(head, lock)
			ObjFree(lock)
			if !all {
				break
			}
			found = true
		}
	}
	return found
}

// tryRemove takes a dying object off the list head; a mobile has its hit points zeroed.
func tryRemove(head *Link, obj *Object) bool {
	if ObjMemTPtr(obj) >= NUM_MOBILE {
		return ObjPunt(head, obj, false) == nil
	}
	obj.HP = 0
	return false
}

// removeObject: what happens when obj is destroyed by damage of type typ at (x, y). True
// when the object has left the map.
func removeObject(obj, who *Object, typ, x, y int) bool {
	if x < 0 {
		return true
	}
	debris := -2
	item := obj.Item()
	switch {
	case item>>4 == CLASS_DOOR:
		if item&ID_INCLASS <= 7 {
			MapObjX, MapObjY = x, y
			OpenDoor(who, obj)
		}
		removeLock(obj, true)
		debris = -1
	case item == ITEM_CHEST || item == ITEM_BARREL:
		MapObjX, MapObjY = x, y
		removeLock(obj, false)
		useCont(nil, obj, 0)
	case item>>4 == CLASS_CONTAINER:
		if !ObjElemFate(10, obj) {
			debris = -1
		} else {
			dumpTheBag(obj, false)
		}
	default:
		if typ&DMG_FIRE != 0 {
			if obj.Item() == ITEM_PILE_OF_DEBRIS_D5 || obj.Item() == ITEM_PILE_OF_DEBRIS_D6 {
				if tryRemove(&MapGetAddr(x, y).Objects, obj) {
					return true
				}
				debris = -1
			} else if rand()&3 == 0 {
				putEffect(obj, 8, rollem(6, 10), 0, 0, x, y)
				debris = rand()*2/0x8000 + ITEM_PILE_OF_DEBRIS_D5
			}
		}
		if obj.IsQuant() == 0 && obj.LinkF() > 0 {
			ObjFreeChain(&obj.OL)
		}
	}
	if debris < -1 {
		debris = rand()*2/0x8000 + ITEM_PILE_OF_DEBRIS_D5
	}
	if debris >= 0 {
		obj.SetItem(debris)
		if ObjMemTPtr(obj) >= NUM_MOBILE && objDeal(obj, x, y, 1) == nil {
			return true
		}
	}
	return false
}

// checkRes applies obj's resistances to damage of type typ; returns the damage left.
func checkRes(obj *Object, damage, typ int) int {
	res := ComObjData[obj.Item()].Resist()
	typ &= 0xFF
	if typ&res != 0 {
		if typ&DMG_MAGIC != 0 {
			if rand()%3 < res&3 {
				return 0
			}
			typ &= 0xFC
		}
		if res&typ != 0 {
			return 0
		}
	}
	return damage & 0xFF
}

// damageItem damages any object. True when it is gone.
func damageItem(obj, who *Object, x, y, damage, typ int) bool {
	damage = checkRes(obj, damage, typ)
	if obj.Major() == MAJOR_CREATURE {
		return damageCritter(obj, damage, who)
	}
	if damageObject(obj, who, damage, x, y) {
		return removeObject(obj, who, typ, x, y)
	}
	return false
}

// damageObject takes damage off an object that is not a critter. True when destroyed.
func damageObject(obj, who *Object, damage, x, y int) bool {
	co := &ComObjData[obj.Item()]
	scale := co.QualClass()
	if obj.DoorDir() != 0 || scale == 3 {
		return false
	}
	if damage >>= uint(scale); damage <= 0 {
		return false
	}
	destroyed := false
	mobile := IsMobElem(obj)
	switch {
	case mobile:
		hp := int(obj.HP) - damage
		if hp <= 0 {
			hp = 0
			destroyed = true
		}
		obj.HP = byte(hp)
	case obj.Item() >= FIRST_RECT && obj.Item() <= ITEM_SECRET_DOOR_147 && obj.Owner()&1 != 0 && obj.Owner()>>1 > 0:
		hp := obj.Owner()>>1 - damage
		if hp <= 0 {
			hp = 0
		}
		obj.SetOwner(obj.Owner()&1 | hp<<1)
	default:
		hp := obj.Quality() - damage
		if hp <= 0 {
			destroyed = true
			hp = 0
		}
		obj.SetQuality(hp)
	}
	if destroyed && !mobile && x > -1 {
		checkTrap(who, obj, 4, x, y)
	}
	return destroyed
}
