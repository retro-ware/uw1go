package uw

import "strings"

// Port of INVDATA.C: the player's inventory as data: adding to and taking from slots,
// finding objects carried, weights, and damage to what is carried.

// Inventory holds the objects shown in each slot (inv.h has the numbering).
var Inventory [NUM_INV_SLOTS]Link

func RedisplayInvSlot(slot int) { DisplayInvObject(int(SlotToDisplay[slot])) }

func AskInventory(slot int) *Object { return ObjPtrTMem(&Inventory[slot]) }

// FindEmptySlot is the first empty slot from 5 to 18, or -1. Not called.
func FindEmptySlot() int {
	for slot := INV_SHOULDER; slot <= INV_PACK_LAST; slot++ {
		if Inventory[slot].Index() == 0 {
			return slot
		}
	}
	return -1
}

// AddToInventory adds obj to the inventory in slot (-1: to the contents without a slot).
func AddToInventory(obj *Object, slot int) bool {
	done := false
	owner := ThePlayer
	fits := 1
	if slot != -1 {
		fits = ItemFitsSlot(obj, slot)
	}
	if fits > 0 {
		mass := ItemWeight(obj)
		if slot >= 0 {
			if slot > INV_PACK_LAST {
				owner = ObjPtrTMem(&OpenBag.obj)
				for bag := OpenBag; bag != nil; bag = bag.prev {
					bag.weight += mass
				}
			}
			Inventory[slot].SetIndex(ObjMemTPtr(obj))
		}
		ObjAddEnd(&owner.OL, obj)
		player.Weight += uint16(mass)
		done = true
	}
	FixPlayerEquips()
	return done
}

// FindSlot is the slot holding obj, minus the slot of a container holding it, or -1.
func FindSlot(obj *Object) int {
	index := ObjMemTPtr(obj)
	for i := 0; i < DISP_BAG; i++ {
		slot := int(DisplayToSlot()[i])
		if Inventory[slot].Index() != 0 {
			if Inventory[slot].Index() == index {
				return slot
			}
			inslot := ObjPtrTMem(&Inventory[slot])
			openIdx := 0
			if OpenBag != nil {
				openIdx = OpenBag.obj.Index()
			}
			if inslot != nil && inslot.IsQuant() == 0 && Inventory[slot].Index() != openIdx &&
				ObjFind(&inslot.OL, true, index) != nil {
				return -slot
			}
		}
	}
	return -1
}

func objMatches(o *Object, major, minor, cls int) bool {
	return (major < 0 || o.Major() == major) && (minor < 0 || o.Minor() == minor) &&
		(cls < 0 || o.InClass() == cls)
}

// FindObj finds an object by major, minor and class (-1 any) in the inventory and sets
// *where to its slot. how 1: paperdoll and hands only; 2 and 3: the backpack too; else
// also inside the containers carried.
func FindObj(major, minor, cls, how int, where *int) *Object {
	var objs [INV_PACK_LAST + 1]*Object
	i := 0
	for ; i < INV_PACK; i++ {
		objs[i] = ObjPtrTMem(&Inventory[i])
		if objs[i] != nil && objMatches(objs[i], major, minor, cls) {
			*where = i
			return objs[i]
		}
	}
	if how == 1 {
		return nil
	}
	for ; i <= INV_PACK_LAST; i++ {
		objs[i] = ObjPtrTMem(&Inventory[i])
		if objs[i] != nil && objMatches(objs[i], major, minor, cls) {
			*where = i
			return objs[i]
		}
	}
	if how == 2 || how == 3 {
		return nil
	}
	for i = 0; i <= INV_PACK_LAST; i++ {
		if objs[i] != nil && objs[i].IsQuant() == 0 {
			contents := ObjPtrTMem(&objs[i].OL)
			objs[i] = findObj(major, minor, cls, &contents)
			if objs[i] != nil {
				*where = i
				return objs[i]
			}
		}
	}
	return nil
}

// findObj searches an object list and its containers depth first.
func findObj(major, minor, cls int, list **Object) *Object {
	for *list != nil {
		if objMatches(*list, major, minor, cls) {
			found := *list
			*list = nil
			return found
		}
		if (*list).IsQuant() == 0 {
			if next := ObjPtrTMem(&(*list).OL); next != nil {
				if found := findObj(major, minor, cls, &next); found != nil {
					if next != nil {
						*list = next
					}
					return found
				}
			}
		}
		*list = ObjPtrTMem(&(*list).QN)
	}
	return nil
}

// pickInv is the object under the pointer in the inventory panel: with how 2 just looked
// at, else taken out of its slot.
func pickInv(how int) *Object {
	x := inplist.x + 0xF0
	y := inplist.y + 0x52
	hit := FindInventoryHit(x, y)
	if hit < 0 || hit >= DISP_BAG {
		return nil
	}
	if how == 2 {
		return WhatsInSlot(int(DisplayToSlot()[hit]))
	}
	return RemoveAllFromSlot(-1, -1, -1, int(DisplayToSlot()[hit]))
}

func WhatsInSlot(slot int) *Object { return ObjPtrTMem(&Inventory[slot]) }

func InvRemoveObject(obj *Object) bool    { return invRemoveObject(obj, -1) }
func InvRemoveOneObject(obj *Object) bool { return invRemoveObject(obj, 1) }

// splitStack cuts qty off the stack obj when it holds more, the rest going after it.
func splitStack(obj *Object, qty int) *Object {
	if qty > 0 && obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
		if have := obj.LinkF(); have > 1 && qty < have {
			cp := ObjAlloc(false)
			setStatic(cp, obj)
			cp.SetLinkF(have - qty)
			obj.SetLinkF(qty)
			ObjAdd(&obj.QN, cp)
			return cp
		}
	}
	return nil
}

// invRemoveObject removes obj, or qty of a stack (-1 all), from the inventory.
func invRemoveObject(obj *Object, qty int) bool {
	mass := ItemWeight(obj)
	index := ObjMemTPtr(obj)
	i := 0
	for ; i < NUM_INV_SLOTS; i++ {
		if Inventory[i].Index() == index {
			break
		}
	}
	if i < NUM_INV_SLOTS {
		removeFromSlot(-1, -1, -1, i, qty)
		if i >= INV_BAG {
			FixOpenBag()
			DisplayOpenBag()
			for bag := OpenBag; bag != nil; bag = bag.prev {
				bag.weight -= mass
			}
		} else {
			DisplayInvObject(int(SlotToDisplay[i]))
		}
	} else {
		obj = ObjFind(&ThePlayer.OL, true, index)
		if obj == nil {
			return false
		}
		splitStack(obj, qty)
		ObjRem(ObjFindHead, obj)
		player.Weight -= uint16(mass)
		DisplayInvObject(0x13)
		FixPlayerEquips()
	}
	return true
}

func redrawAfterRemove(slot int) {
	if inslot := ObjPtrTMem(&Inventory[slot]); inslot != nil && inslot.Major() == MAJOR_MISC &&
		inslot.Minor() == MINOR_CONTAINER && OpenBag != nil {
		FixOpenBag()
		DisplayOpenBag()
	} else {
		DisplayInvObject(int(SlotToDisplay[slot]))
	}
}

// RemoveAllFromSlot takes the whole object in slot (or the first match inside it) out of
// the inventory and redraws.
func RemoveAllFromSlot(major, minor, cls, slot int) *Object {
	taken := removeFromSlot(major, minor, cls, slot, 0)
	redrawAfterRemove(slot)
	return taken
}

func RemoveOneFromSlot(major, minor, cls, slot int) *Object {
	taken := removeFromSlot(major, minor, cls, slot, 1)
	redrawAfterRemove(slot)
	return taken
}

func removeFromSlot(major, minor, cls, slot, qty int) *Object {
	obj := takeFromSlot(major, minor, cls, slot, qty)
	FixPlayerEquips()
	return obj
}

// takeFromSlot takes the object in slot (or a match inside it) out of the inventory; qty
// splits a stack (0 takes it all).
func takeFromSlot(major, minor, cls, slot, qty int) *Object {
	obj := ObjPtrTMem(&Inventory[slot])
	if obj == nil {
		return nil
	}
	var owner *Object
	if slot <= INV_PACK_LAST {
		owner = ThePlayer
	} else {
		owner = ObjPtrTMem(&OpenBag.obj)
	}
	if major >= 0 || minor >= 0 || cls >= 0 {
		if !objMatches(obj, major, minor, cls) {
			o := owner
			if obj = findObj(major, minor, cls, &o); obj == nil {
				return nil
			}
		}
	}
	var cp *Object
	if qty != 0 {
		cp = splitStack(obj, qty)
	}
	if owner == ThePlayer || slot >= INV_BAG_ITEMS {
		Inventory[slot].SetIndex(ObjMemTPtr(cp))
	}
	ObjRem(&owner.OL, obj)
	mass := ItemWeight(obj)
	player.Weight -= uint16(mass)
	if OpenBag != nil && ObjMemTPtr(owner) == OpenBag.obj.Index() {
		index := ObjMemTPtr(obj)
		for i := INV_BAG_ITEMS; i <= INV_BAG_LAST; i++ {
			if Inventory[i].Index() == index {
				Inventory[i].SetIndex(ObjMemTPtr(cp))
				for bag := OpenBag; bag != nil; bag = bag.prev {
					bag.weight -= mass
				}
				break
			}
		}
	}
	return obj
}

// ObjWorn is whether an object of type id is worn or wielded in slot.
func ObjWorn(id, slot int) bool {
	if slot >= 0 && slot <= INV_BOOTS {
		return true
	}
	if slot == INV_RING+1 || slot == INV_RING {
		return true
	}
	if player.Lefty()+INV_HAND != slot {
		return false
	}
	return id>>6 == MAJOR_HACK && (id&ID_MINOR)>>4 >= 2 && id&ID_INCLASS >= 0xB && id&ID_INCLASS <= 0xF
}

// DamageInventory damages the object in slot: -2 none or immune, -1 unharmed, 1 destroyed,
// 0 damaged.
func DamageInventory(slot, damage, typ, how int, debris bool) int {
	obj := AskInventory(slot)
	if obj == nil {
		return -2
	}
	if how != 2 {
		if how == 0 {
			if obj.Class() != CLASS_WEAPON {
				return -2
			}
		} else if !ObjWorn(obj.Item(), slot) {
			return -2
		}
	}
	quality := obj.Quality()
	var msg string
	var result int
	if damageItem(obj, nil, -1, -1, damage, typ) {
		if debris {
			junk := CreateObj(rand()*2/0x8000+ITEM_PILE_OF_DEBRIS_D5, false)
			nearMobPutAt(ThePlayer, junk, 6, false)
		}
		InvRemoveOneObject(obj)
		ObjPunt(nil, obj, true)
		FixPlayerEquips()
		msg = " destroyed.\n"
		result = 1
	} else if obj.Quality() != quality {
		msg = " damaged.\n"
		result = 0
	} else {
		return -1
	}
	if obj == ThePlayer {
		obj.SetItem(ITEM_FIST)
	}
	name, _ := getName(obj, false, false)
	text := "Your " + name
	if strings.HasSuffix(text, "s") {
		text += " were"
	} else {
		text += " was"
	}
	scrollPrint(text + msg)
	RedisplayInvSlot(slot)
	return result
}

// ItemWeight is an object's weight in tenths of a stone.
func ItemWeight(obj *Object) int {
	com := &ComObjData[obj.Item()]
	var mass int16
	if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
		mass = int16(obj.LinkF() * com.Mass())
	} else {
		mass = int16(com.Mass())
		if obj.IsQuant() == 0 && obj.LinkF() > 0 {
			BagWeight(&obj.OL, &mass)
		}
	}
	return int(mass)
}

// EncumCheck is whether the player can carry obj as well.
func EncumCheck(obj *Object) bool {
	return ItemWeight(obj)+int(int16(player.Weight)) <= int(int16(player.MaxWeight))
}
