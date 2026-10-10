package uw

import (
	"strconv"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of INVPANEL.C: the inventory panel: the paperdoll, the slots and the open bag on
// screen, the mouse in the panel, and whether an object may go into a slot.

// InvRect is one inventory slot on screen: the rectangle the mouse hits (y up, so
// top >= bottom) and where its picture goes.
type InvRect struct {
	left, top, right, bottom int
	x, y                     int
	w, h                     byte
}

const PIC_BODY = ICON_INV

var (
	invArmorObj  [6]int
	invArmorQ    [6]int
	SaveHandles  [NUM_DISPLAY]*gr.Block
	panelMouse   int
	CursorObjPtr *Object

	ValidLightSlots = [4]int{INV_SHOULDER, INV_SHOULDER + 1, INV_HAND, INV_HAND + 1}
	OpenBagList     *Bag
	OpenBag         *Bag

	InvDisplay = [NUM_DISPLAY]InvRect{
		{0x0, 0x0, 0x0, 0x0, 0x104, 0xBC, 0x24, 0x45},
		{0x10D, 0x90, 0x11D, 0x80, 0x10C, 0xAF, 0x13, 0x32},
		{0x10D, 0xBF, 0x11E, 0xAF, 0x10B, 0xBD, 0x14, 0x14},
		{0x107, 0xAE, 0x123, 0x9D, 0x106, 0xB0, 0x21, 0x2C},
		{0x107, 0x9C, 0x123, 0x90, 0x105, 0x9D, 0x21, 0xC},
		{0x107, 0x80, 0x123, 0x77, 0x10A, 0x85, 0x15, 0xD},
		{0xF4, 0xBB, 0x105, 0xAA, 0xF5, 0xBA, 0x10, 0x10},
		{0x125, 0xBB, 0x136, 0xAA, 0x126, 0xBA, 0x10, 0x10},
		{0xF1, 0xA5, 0x102, 0x92, 0xF2, 0xA4, 0x10, 0x10},
		{0x127, 0xA5, 0x138, 0x92, 0x128, 0xA4, 0x10, 0x10},
		{0xF1, 0x93, 0x10C, 0x88, 0xFF, 0x94, 0x10, 0x10},
		{0x11E, 0x93, 0x138, 0x82, 0x11D, 0x94, 0x10, 0x10},
		{0xF0, 0x76, 0x101, 0x65, 0xF1, 0x75, 0x10, 0x10},
		{0x103, 0x76, 0x114, 0x65, 0x104, 0x75, 0x10, 0x10},
		{0x116, 0x76, 0x127, 0x65, 0x117, 0x75, 0x10, 0x10},
		{0x129, 0x76, 0x13A, 0x65, 0x12A, 0x75, 0x10, 0x10},
		{0xF0, 0x64, 0x101, 0x53, 0xF1, 0x63, 0x10, 0x10},
		{0x103, 0x64, 0x114, 0x53, 0x104, 0x63, 0x10, 0x10},
		{0x116, 0x64, 0x127, 0x53, 0x116, 0x63, 0x10, 0x10},
		{0x129, 0x64, 0x13A, 0x53, 0x12A, 0x63, 0x10, 0x10},
		{0xF0, 0x87, 0x101, 0x76, 0xF1, 0x87, 0x10, 0x10},
		{0x127, 0x81, 0x130, 0x78, 0x128, 0x81, 0x8, 0xA},
		{0x131, 0x81, 0x13A, 0x78, 0x132, 0x81, 0x8, 0xA},
	}

	// SlotToDisplay and DisplayToSlot share one array: OpenTheBag writes an open bag's
	// slots 20..27 to SlotToDisplay[40..47], which are DisplayToSlot[12..19].
	SlotToDisplay = [NUM_INV_SLOTS + 21]byte{
		2, 3, 4, 1, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		12, 13, 14, 15, 16, 17, 18, 19,
		1, 3, 0, 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
	}

	panelInput    int
	shownCapacity = -1
	InvUpArrow    bool
	InvDownArrow  bool
	invBegun      bool
	invRefresh    = true
)

func DisplayToSlot() []byte { return SlotToDisplay[NUM_INV_SLOTS:] }

// loadInventoryPix loads the paperdoll body picture and forgets the armour pictures.
func loadInventoryPix() {
	reloadGrVpic(PIC_BODY, "bodies", player.Female()*10/2+player.Body())
	for i := 1; i <= 5; i++ {
		invArmorObj[i] = 0
	}
}

// BeginInventory saves the background under each slot and registers the panel's mouse
// region and handler, once per game.
func BeginInventory() {
	if invBegun {
		return
	}
	invBegun = true
	gr.Transparency = true
	loadInventoryPix()
	for i := 6; i < NUM_DISPLAY; i++ {
		SaveHandles[i] = gr.NewBlock(int(InvDisplay[i].w), int(InvDisplay[i].h))
	}
	SaveHandles[0] = gr.NewBlock(0x10, 0x0A)
	SaveHandles[1] = gr.NewBlock(0x54, 0x29)
	for i := 6; i < NUM_DISPLAY; i++ {
		x, w := InvDisplay[i].x, int(InvDisplay[i].w)
		if i == 10 {
			x, w = x+5, w-5
		} else if i == 11 {
			w -= 5
		}
		SaveHandles[i].Save(x, InvDisplay[i].y, w, int(InvDisplay[i].h))
	}
	SaveHandles[1].Save(0xEC, 0x77, 0x54, 0x29)
	SaveHandles[0].Save(0x12B, 0x8D, 0x10, 0x0A)
	panelMouse = defineMouseRegion(0xF0, 0x52, 0x13B, 0xBD, ICON_CURSORS)
	panelInput = inputAddmouse(0xF0, 0x52, 0x13B, 0xBD, nil, 5, func(a any) { mousInPanel(a) })
}

func EndInventory() {}

// ClearInventory empties every slot and the cursor.
func ClearInventory() {
	for i := range Inventory {
		Inventory[i].SetIndex(0)
	}
	for i := 1; i <= 5; i++ {
		invArmorObj[i] = 0
	}
	OpenBag, OpenBagList = nil, nil
	InvUpArrow, InvDownArrow = false, false
	CursorObjPtr = nil
	shownCapacity = -1
}

// DoInventoryMouse handles a click in the inventory panel (how: the kind of click, -2 a
// look).
func DoInventoryMouse(how int) {
	pick := false
	var split *Object
	held := CursorObjPtr != nil
	hit := FindInventoryHit(inplist.x+0xF0, inplist.y+0x52)
	if hit > 0 && hit < DISP_UP {
		slot := int(DisplayToSlot()[hit])
		if CursorObjPtr == nil && Inventory[slot].Index() == 0 {
			if INV_WEAPON_HAND-player.Lefty() == slot {
				toggleFightmode()
			}
			mouseRelease(true)
			return
		}
		if CursorObjPtr == nil && slot != -1 && slot != INV_BAG {
			pick = true
		}
		if pick && mouseDragged(true) {
			obj := ObjPtrTMem(&Inventory[slot])
			if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
				if obj.LinkF() != 1 {
					if split = AskHowMany(obj); split == nil {
						return
					}
					if split != obj {
						ObjAdd(&obj.QN, split)
					}
				}
			} else if obj.Major() == MAJOR_MISC && obj.Minor() == MINOR_CONTAINER {
				if inplist.mode == 4 && obj.InClass() != 0xF {
					gameSprint(0xBA)
					return
				}
				for bag2 := OpenBagList; bag2 != nil; bag2 = bag2.next {
					if ObjPtrTMem(&bag2.obj) == obj {
						return
					}
				}
			}
			held = true
			SetCursorObj(slot, split != nil && split != obj)
			if slot > INV_BAG {
				for bag := OpenBag; bag != nil; bag = bag.prev {
					bag.weight -= ItemWeight(obj)
				}
				FixOpenBag()
				DisplayOpenBag()
			} else {
				DisplayInvObject(int(SlotToDisplay[slot]))
			}
			if CursorObjPtr == nil {
				return
			}
			if split != nil {
				GameInputMode = 1
				return
			}
		}
		mouseRelease(true)
		x, y := mouseGetxy()
		hit = FindInventoryHit(x, y)
	}
	mouseRelease(true)
	if CursorObjPtr != nil && GameInputMode != 2 {
		GameInputMode = 1
		if RightPanel != 0 && hit != DISP_WORLD {
			return
		}
		if hit > 0 {
			if hit < DISP_UP {
				RearrangeInventory(int(DisplayToSlot()[hit]))
			} else {
				DoSpecialActions(hit)
				held = false
			}
		}
	} else if hit > 0 {
		if how >= 0 {
			DoSpecialActions(hit)
			held = false
		} else if how == -2 {
			invLook()
		}
	}
	if held && CursorObjPtr == nil {
		unforceMouseCursor(3)
		GameInputMode = 0
	}
}

// DoInventoryDrag makes an object dragged in from the 3D view the cursor object and puts
// it down where the button is released.
func DoInventoryDrag(obj *Object) {
	CursorObjPtr = obj
	forceMouseCursor(CursorObjPtr.Item())
	if mouseGetbut() == 0 {
		return
	}
	mouseRelease(true)
	if CursorObjPtr == nil {
		return
	}
	x, y := mouseGetxy()
	hit := FindInventoryHit(x, y)
	if hit > 0 {
		GameInputMode = 1
		if RightPanel != 0 && hit != DISP_WORLD {
			return
		}
		if hit < DISP_UP {
			RearrangeInventory(int(DisplayToSlot()[hit]))
			if CursorObjPtr == nil {
				GameInputMode = 0
				unforceMouseCursor(3)
			}
		} else {
			DoSpecialActions(hit)
		}
	}
}

// loadInvPic loads paperdoll picture img for display slot n from ARMOR_F or ARMOR_M.
func loadInvPic(n, img int) bool {
	name := "armor_m"
	if player.Female() == 1 {
		name = "armor_f"
	}
	reloadGrVpic(n+PIC_BODY, name, img)
	return true
}

// DisplayInvSpecial draws the paperdoll: the body, the armour, the rings and the weight
// left.
func DisplayInvSpecial() {
	if RightPanel != 0 {
		return
	}
	mouseHide()
	if invRefresh {
		gr.GrSoftPageFlip()
		gr.SetTheColor(0x106)
		gr.Rectangle(0xF0, 0xBD, 0x13B, 0x52)
	}
	picToScreen(PIC_BODY, InvDisplay[DISP_BODY].x, InvDisplay[DISP_BODY].y)
	gr.Transparency = true
	for slot := 1; slot <= 5; slot++ {
		if Inventory[DisplayToSlot()[slot]].Index() != 0 {
			obj := ObjPtrTMem(&Inventory[DisplayToSlot()[slot]])
			typ := obj.InMajor() & 0x1F
			q := obj.Quality() >> 4
			if typ > 14 {
				q = 3
			}
			if typ+1 != invArmorObj[slot] || q+1 != invArmorQ[slot] {
				invArmorObj[slot] = typ + 1
				invArmorQ[slot] = q + 1
				loadInvPic(slot, typ+q*15)
			}
			picToScreen(slot+PIC_BODY, InvDisplay[slot].x, InvDisplay[slot].y)
		}
	}
	gr.Transparency = false
	SaveHandles[11].Save(InvDisplay[11].x, InvDisplay[11].y, int(InvDisplay[11].w)-5, int(InvDisplay[11].h))
	SaveHandles[10].Save(InvDisplay[10].x+5, InvDisplay[10].y, int(InvDisplay[10].w)-5, int(InvDisplay[10].h))
	if Inventory[INV_RING].Index() != 0 || Inventory[INV_RING+1].Index() != 0 {
		displayInventoryArray(10, 11)
	}
	if invRefresh {
		gr.GrSoftPageFlip()
		gr.SetTheColor(0x106)
		gr.Rectangle(0xF0, 0xBD, 0x13B, 0x52)
	}
	if displayEnc(true) {
		grfxLoadFont("font5x6p.sys")
	}
	mouseShow()
}

// DisplayInvObject redraws display position slot.
func DisplayInvObject(slot int) {
	if RightPanel != 0 {
		return
	}
	if slot < 6 {
		DisplayInvSpecial()
	} else if slot < DISP_UP {
		displayInventoryArray(slot, slot)
	} else {
		SaveHandles[slot].Restore()
		pic := -1
		if slot == DISP_UP {
			if InvUpArrow {
				pic = ICON_BUTTONS + 0x1B
			}
		} else if InvDownArrow {
			pic = ICON_BUTTONS + 0x1C
		}
		if pic >= 0 {
			gr.Transparency = true
			picToScreen(pic, InvDisplay[slot].x, InvDisplay[slot].y)
			gr.Transparency = false
		}
	}
}

// SetCursorObj takes the object in slot onto the mouse cursor; keep: the slot holds a
// split stack and the remainder stays in the slot.
func SetCursorObj(slot int, keep bool) {
	had := CursorObjPtr != nil
	link := 0
	if keep {
		link = ObjMemTPtr(ObjPtrTMem(&AskInventory(slot).QN))
	}
	CursorObjPtr = takeFromSlot(-1, -1, -1, slot, 0)
	if CursorObjPtr != nil {
		if keep {
			Inventory[slot].SetIndex(link)
			FixPlayerEquips()
		}
		mouseHide()
		if had {
			unforceMouseCursor(0)
		}
		forceMouseCursor(CursorObjPtr.Item())
		mouseShow()
		FixPlayerEquips()
	}
}

// AskHowMany asks how many of the stack obj to move; nil for none.
func AskHowMany(obj *Object) *Object {
	orig := obj.LinkF()
	var split *Object
	n := 1
	initial := "1"
	key, result := wdialog("Move how many? ", &initial, false, 3)
	if key != 27 && key != 3 {
		if key != 0 && key < 4 {
			if key == 2 {
				n = orig
			}
			wdReplace(n)
		} else {
			n, _ = strconv.Atoi(result)
			if n > orig {
				n = orig
			}
		}
		scrollPrint("\n")
		if n != 0 {
			if obj.LinkF() != n {
				split = ObjAlloc(false)
				setStatic(split, obj)
				split.SetLinkF(split.LinkF() - n)
				obj.SetLinkF(n)
			} else {
				split = obj
			}
		}
	}
	mouseDragged(true)
	return split
}

// ItemFitsSlot is whether obj may go into slot: 1 yes, 0 no, -1 it was eaten.
func ItemFitsSlot(obj *Object, slot int) int {
	var cont *Object
	id := obj.Item()
	ActiveObj = obj
	com := &ComObjData[id]
	major, minor, sub := obj.Major(), obj.Minor(), obj.InClass()
	if slot == INV_BAG {
		if OpenBag == nil {
			return 0
		}
		if OpenBag.prev == nil {
			j := INV_PACK
			for ; j <= INV_PACK_LAST; j++ {
				if Inventory[j].Index() == 0 {
					break
				}
			}
			if j > INV_PACK_LAST {
				gameSprint(0x102)
			}
			return b2i(j <= INV_PACK_LAST)
		}
		cont = ObjPtrTMem(&OpenBag.prev.obj)
	} else if slot > INV_BAG {
		if o := ObjPtrTMem(&Inventory[slot]); o != nil && o.Class() == CLASS_CONTAINER {
			cont = o
		} else {
			cont = ObjPtrTMem(&Inventory[INV_BAG])
		}
	} else {
		cont = ObjPtrTMem(&Inventory[slot])
	}
	if slot < INV_SHOULDER {
		if major != MAJOR_HACK {
			if slot == INV_HEAD && UseFood(ThePlayer, obj, 0) > 0 {
				return -1
			}
			return 0
		}
		if obj.Minor() < 2 {
			return 0
		}
		cat := int(getClassData()[3])
		switch slot {
		case INV_TORSO:
			return b2i(cat == ARMOUR_BODY)
		case INV_LEGS:
			return b2i(cat == ARMOUR_LEGGINGS)
		case INV_GLOVES:
			return b2i(cat == ARMOUR_GLOVES)
		case INV_HEAD:
			return b2i(cat == ARMOUR_HAT)
		case INV_BOOTS:
			return b2i(cat == ARMOUR_BOOTS)
		}
		return 0
	}
	if slot == INV_RING || slot == INV_RING+1 {
		if major != MAJOR_HACK || obj.Minor() < MINOR_ARMOR {
			return 0
		}
		return b2i(int(getClassData()[3]) == ARMOUR_RING)
	}
	if INV_WEAPON_HAND-player.Lefty() == slot && major == MAJOR_HACK && minor == 0 {
		if cont != nil && cont.Item() == id ||
			obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 && obj.LinkF() > 1 {
			return 0
		}
	} else if major == MAJOR_MISC && minor == 1 && sub >= 4 && sub < 8 {
		for i := 0; i < 4; i++ {
			if ValidLightSlots[i] == slot {
				return 1
			}
		}
		obj.SetInClass(sub - 4)
		if ItemFitsSlot(obj, slot) == 0 {
			obj.SetInClass(sub)
			return 0
		}
		return 1
	}
	if cont != nil && cont.Class() == CLASS_CONTAINER {
		ok := true
		weight := int16(ItemWeight(obj))
		if slot > INV_BAG {
			for bag := OpenBag; bag != nil; bag = bag.prev {
				capa := int(Containers[ObjPtrTMem(&bag.obj).InClass()].Capacity)
				ok = ok && (capa == 0 || bag.weight+int(weight) <= capa)
			}
		}
		BagWeight(&cont.OL, &weight)
		capa := int(Containers[cont.InClass()].Capacity)
		ok = ok && (capa == 0 || int(weight) <= capa)
		if !ok {
			name, found := getName(cont, false, false)
			if !found {
				name = "UNNAMED"
			}
			scrollPrint("The ")
			scrollPrint(name)
			scrollPrint(" is too full.\n")
			return 0
		}
		mask := int(Containers[cont.InClass()].Mask)
		if mask >= 0 {
			if mask < CONT_RUNES {
				if id != mask {
					gameSprint(0xF8)
				}
				return b2i(id == mask)
			}
			var res bool
			switch mask {
			case CONT_RUNES:
				if !(major == MAJOR_STUFF && (minor == 3 || minor == 2 && sub > 7)) {
					gameSprint(0xF7)
					return 0
				}
				res = true
			case CONT_MISSILES:
				res = major == MAJOR_HACK && minor == 1 && sub < 3
			case CONT_SCROLLS:
				res = major == MAJOR_SPEC && minor == 3 && sub >= 8
			case CONT_FOOD:
				res = major == MAJOR_MISC && minor == 3 || id == ITEM_PLANT_CE || id == ITEM_PLANT_CF ||
					id == ITEM_CANDLE || id == ITEM_LEECHES || id == ITEM_ROTWORM_STEW || id == ITEM_DEAD_ROTWORM
			}
			if !res {
				gameSprint(0xF8)
			}
			return b2i(res)
		}
	}
	return com.Pickup()
}

// RearrangeInventory puts the cursor object down in slot.
func RearrangeInventory(slot int) {
	if Inventory[slot].Index() == 0 {
		if AddToEmptySlot(CursorObjPtr, slot) {
			CursorObjPtr = nil
		}
	} else if AddToOccupiedSlot(CursorObjPtr, slot) {
		CursorObjPtr = nil
	}
}

// AddToEmptySlot puts obj in the empty slot and redraws it.
func AddToEmptySlot(obj *Object, slot int) bool {
	if slot == INV_BAG {
		return false
	}
	if !AddToInventory(obj, slot) {
		return false
	}
	if slot > INV_PACK_LAST {
		FixOpenBag()
		DisplayOpenBag()
	} else {
		DisplayInvObject(int(SlotToDisplay[slot]))
	}
	return true
}

// AddTogether is whether obj can be merged into the stack onto.
func AddTogether(obj, onto *Object) bool {
	if obj.Item() != onto.Item() {
		return false
	}
	if obj.IsQuant() == 0 && obj.LinkF() > 0 || onto.IsQuant() == 0 && onto.LinkF() > 0 ||
		obj.LinkF()&LINK_SPECIAL != 0 || onto.LinkF()&LINK_SPECIAL != 0 ||
		ComObjData[obj.Item()].Stack() == 1 || ComObjData[obj.Item()].Stack() == 3 {
		return false
	}
	if obj.Class() == CLASS_KEY && obj.Owner() != onto.Owner() {
		return false
	}
	if obj.LinkF()+onto.LinkF() < 999 {
		if obj.Item() >= ITEM_SLING_STONE && obj.Item() <= ITEM_ARROW_12 {
			return true
		}
		q1, q2 := obj.Quality(), onto.Quality()
		return q1>>4 == q2>>4 && (q1 != 0 && q2 != 0 || q1 == q2)
	}
	return false
}

// AddToOccupiedSlot puts obj down on the occupied slot: into a bag, merged, combined, or
// swapped. Returns whether obj was used up.
func AddToOccupiedSlot(obj *Object, slot int) bool {
	ok := false
	target := ObjPtrTMem(&Inventory[slot])
	if target.Major() == MAJOR_MISC && target.Minor() == MINOR_CONTAINER {
		r := PutObjectInBag(obj, slot)
		FixPlayerEquips()
		return r
	}
	if AddTogether(obj, target) {
		if ItemFitsSlot(obj, slot) != 1 {
			return false
		}
		qty := 1
		if obj.IsQuant() != 0 {
			qty = obj.LinkF()
		}
		if target.IsQuant() == 0 {
			target.SetIsQuant(1)
			target.SetLinkF(1)
		}
		weight := ComObjData[obj.Item()].Mass() * qty
		if slot > INV_BAG {
			for bag := OpenBag; bag != nil; bag = bag.prev {
				bag.weight += weight
			}
		}
		player.Weight += uint16(weight)
		FixPlayerEquips()
		target.SetLinkF(target.LinkF() + qty)
		target.SetQuality((target.Quality() + obj.Quality()) >> 1)
		ObjFree(obj)
		ok = true
	} else if combo := ObjsBeCombinable(obj, target); combo >= 0 {
		made := CombineObjs(combo)
		if made == nil {
			return false
		}
		used := RemoveAfterCombine(obj, combo)
		if used {
			ObjPunt(nil, obj, true)
			CursorObjPtr = made
			GameInputMode = 1
			unforceMouseCursor(3)
			forceMouseCursor(CursorObjPtr.Item())
		}
		if RemoveAfterCombine(target, combo) {
			if !used {
				AddToEmptySlot(made, slot)
			}
			InvRemoveObject(target)
			ObjPunt(nil, target, true)
		}
	} else if slot <= INV_PACK_LAST {
		old := removeFromSlot(-1, -1, -1, slot, 0)
		if !AddToEmptySlot(obj, slot) {
			AddToEmptySlot(old, slot)
		} else {
			CursorObjPtr = old
		}
		if CursorObjPtr != nil {
			unforceMouseCursor(0)
			forceMouseCursor(CursorObjPtr.Item())
		}
	} else {
		SwapItemsInBag(obj, slot)
	}
	DisplayInvObject(int(SlotToDisplay[slot]))
	return ok
}

// DisplayInventory redraws the backpack area, or the open bag's background, and every
// slot from position 6 on.
func DisplayInventory() {
	if RightPanel != 0 {
		return
	}
	shownCapacity = -1
	if OpenBag != nil {
		picToScreen(ICON_INV+6, 0xEC, 0x77)
	} else {
		SaveHandles[1].Restore() // the backpack background (FM Towns; DOS passes handle 1)
	}
	displayInventoryArray(6, DISP_DOWN)
}

// displayInventoryArray redraws display positions from..to.
func displayInventoryArray(from, to int) {
	anyQty := false
	var qty [NUM_DISPLAY]int
	mouseHide()
	gr.Transparency = true
	for i := from; i <= to; i++ {
		SaveHandles[i].Restore()
		qty[i] = 1
		if i <= DISP_BAG {
			slot := int(DisplayToSlot()[i])
			if Inventory[slot].Index() != 0 {
				obj := ObjPtrTMem(&Inventory[slot])
				picToScreen(obj.Item(), InvDisplay[i].x, InvDisplay[i].y)
				if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
					if n := obj.LinkF(); n > 1 {
						qty[i] = n
						anyQty = true
					}
				}
			}
		} else {
			DisplayInvObject(i)
		}
	}
	gr.Transparency = false
	if anyQty {
		grfxLoadFont("font4x5p.sys")
		gr.ForegroundColor = 0x60
		for i := from; i <= to; i++ {
			if n := qty[i]; n > 1 {
				gr.StringToScreen(strconv.Itoa(n), InvDisplay[i].x+3, InvDisplay[i].y-1)
			}
		}
		grfxLoadFont("font5x6p.sys")
	}
	displayEnc(false)
	mouseShow()
}

// displayEnc draws the weight the player can still carry if it changed.
func displayEnc(show bool) bool {
	drawn := false
	left := int(int16(player.MaxWeight)) - int(int16(player.Weight))
	if shownCapacity != left {
		SaveHandles[0].Restore()
		shownCapacity = left
		drawn = show
		gr.ForegroundColor = 0xE0
		buf := strconv.Itoa(left / 10)
		gr.StringToScreen(buf, 8-gr.StringWidth(buf)/2+0x129, 0x8C)
	}
	return drawn
}

// FindInventoryHit is the display position under screen point x, y, or -1.
func FindInventoryHit(x, y int) int {
	if inplist.mode != 4 {
		if x > PLeft && PLeft+PWid > x && y > PBot && PBot+PHgt > y {
			return DISP_WORLD
		}
	} else if x > 0x8B && x < 0xC1 && y > 0x98 && y < 0xBE {
		return DISP_BARTER
	}
	for i := range InvDisplay {
		r := &InvDisplay[i]
		if r.left <= x && r.right >= x && r.bottom <= y && r.top >= y {
			return i
		}
	}
	return -1
}
