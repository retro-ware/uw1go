package uw

// Port of TREASURE.C (ovr150): spilling a container's contents onto the map, and the loot
// a critter is generated with the first time it needs one - treasure scaled by the dungeon
// level, food, its two weapons and two other items, each by chance, from its Creature
// record.

// lootCreature is the Creature record being looted.
var lootCreature *Creature

// dropLinkChain empties container cont onto the floor where it stands (MapObjX, MapObjY
// for a static container), giving the container to owner. True if anything was inside.
func dropLinkChain(cont *Object, owner int) bool {
	if cont.LinkF() == 0 {
		return false
	}
	obj := ObjPtrTMem(&cont.OL)
	cont.SetLinkF(0)
	var x, y int
	if IsMobElem(cont) {
		x, y = cont.HomeX(), cont.HomeY()
	} else {
		x, y = MapObjX, MapObjY
	}
	z := cont.Z()
	x = x<<3 + cont.FineX()
	y = y<<3 + cont.FineY()
	for obj != nil {
		next := ObjPtrTMem(&obj.QN)
		if owner != 0 && ComObjData[cont.Item()].CanOwn() != 0 {
			cont.SetOwner(owner)
		}
		putAt(x, y, z, obj, 6, false)
		obj = next
	}
	return true
}

// dropSomeObjects: a critter drops what it carries; the objects belong to its race.
func dropSomeObjects(critter *Object) {
	dropLinkChain(critter, int(Creatures[critter.InMajor()].Race))
}

func addLoot(npc, obj *Object) {
	if obj == nil {
		obj = &Level.Objs[0] // FARNULLTRAP
	}
	ObjAdd(&npc.OL, obj)
}

func lootQuality() int {
	if rand()%2 == 0 {
		q := PlayerLevel << 2
		if q > 0 {
			q += rand() % q
		}
		return q
	}
	return rand() % 64
}

// generateTreasure maybe adds treasure (items 0xA0 on) to npc.
func generateTreasure(npc *Object) {
	prob := lootCreature.TreasureProb()
	rate := lootCreature.TreasureRate()
	if rand()%16 >= rate {
		return
	}
	typ := int(int8(rand()%(40-PlayerLevel*3) - (33 - PlayerLevel*3)))
	if typ < 0 {
		typ = 0
	}
	// value is a char: values from 25 up overflow below
	value := int(int8(byte(ComObjData[typ+FIRST_TREASURE].Value())))
	if value == 0 {
		value = 1
	}
	switch {
	case value >= 12:
		value = int(int8(value*8 - 68))
	case value >= 8:
		value = int(int8(value*4 - 20))
	case value >= 4:
		value = int(int8(value*2 - 4))
	}
	var qty int
	if prob<<2 < value {
		if rand()%value >= prob<<2 {
			return
		}
		qty = 1
	} else {
		dice := int(int8((prob << 2) / value << 1))
		qty = int(int8(rollem(4, dice) >> 2))
	}
	if qty < 1 {
		return
	}
	obj := CreateObj(typ+FIRST_TREASURE, false)
	if obj != nil {
		obj.SetLinkF(qty)
	}
	addLoot(npc, obj)
}

// generateFood adds the critter's food item with chance food_prob in 16.
func generateFood(npc *Object) {
	if rand()%16 < lootCreature.FoodProb() {
		addLoot(npc, CreateObj(lootCreature.FoodItem()+FIRST_FOOD, false))
	}
}

// generateWeapons adds the critter's two weapons if present, at a random quality;
// ammunition comes in a stack of 4..11.
func generateWeapons(npc *Object) {
	for i := 0; i < 2; i++ {
		if lootCreature.ArmsPresent(i) == 0 {
			continue
		}
		item := lootCreature.ArmsItem(i)
		obj := CreateObj((item>>4&3)<<4+item&0xF, false)
		if obj == nil {
			addLoot(npc, nil)
			continue
		}
		obj.SetQuality(lootQuality())
		if obj.Minor() == MINOR_MISSILE && byte(Missile[obj.InClass()].Ammo) == 0xC0 {
			obj.SetLinkF(rand()%8 + 4)
		}
		addLoot(npc, obj)
	}
}

// generateEquipment adds each of the two other items with its chance in 16.
func generateEquipment(npc *Object) {
	for i := 0; i < 2; i++ {
		if rand()%16 < lootCreature.OtherProb(i) {
			obj := CreateObj(lootCreature.OtherItem(i), false)
			if obj != nil {
				obj.SetQuality(lootQuality())
			}
			addLoot(npc, obj)
		}
	}
}

// generateInventory gives npc its loot once.
func generateInventory(npc *Object) {
	if npc.HasInv() != 0 {
		return
	}
	lootCreature = &Creatures[npc.Minor()<<4+npc.InClass()]
	generateTreasure(npc)
	generateFood(npc)
	generateWeapons(npc)
	generateEquipment(npc)
	npc.SetHasInv(1)
}
