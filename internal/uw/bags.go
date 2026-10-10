package uw

// Port of BAGS.C: open bags in the inventory panel. Bags opened inside bags form a chain
// (OpenBag is the innermost).

// Bag is an open bag.
type Bag struct {
	next, prev *Bag
	obj        Link
	weight     int
}

// BagSaveHandles are the backpack slots' screen saves while a bag is open.
var BagSaveHandles [8]int

// DoSpecialActions handles a click on display position slot that is not a pick-up or
// put-down: the bag's arrows and picture, the 3D view, the barter area, the weapon hand,
// and otherwise using the object.
func DoSpecialActions(slot int) {
	held := CursorObjPtr != nil
	switch slot {
	case DISP_UP:
		ScrollItemsUp()
	case DISP_DOWN:
		ScrollItemsDown()
	case DISP_BAG:
		CloseTheBag()
	case DISP_WORLD:
		if CursorObjPtr != nil && ReturnObject(CursorObjPtr, true) {
			if HasOrIsObj(CursorObjPtr, ITEM_MOONSTONE) {
				player.SetMoonstone(PlayerLevel)
			}
			CursorObjPtr = nil
			FixPlayerEquips()
		}
	case DISP_BARTER:
		convInvSpecial()
	default:
		if slot == 8 || slot == 9 {
			if 9-player.Lefty() == slot {
				obj := ObjPtrTMem(&Inventory[DisplayToSlot()[slot]])
				id, cls := 0x19, 1 // an empty hand reads 19h, a bow
				if obj != nil {
					id, cls = obj.Item(), obj.Class()
				}
				if cls == CLASS_WEAPON || id == ITEM_SLING || id == ITEM_BOW || id == ITEM_CROSSBOW || id == ITEM_JEWELED_BOW {
					toggleFightmode()
					break
				}
			}
		}
		if obj := ObjPtrTMem(&Inventory[DisplayToSlot()[slot]]); obj != nil {
			UseObj(ThePlayer, obj, 1)
		}
	}
	if held && CursorObjPtr == nil {
		unforceMouseCursor(3)
		GameInputMode = 0
	}
}

// makeBagClose turns an open container back into its closed item.
func makeBagClose(bag *Bag) {
	obj := ObjIntTMem(bag.obj.Index())
	if cls := obj.InClass(); cls < 12 && cls&1 != 0 {
		obj.SetInClass(cls - 1)
	}
}

// CloseAllBags closes every open bag, innermost first.
func CloseAllBags() {
	if OpenBag == nil {
		return
	}
	for OpenBag.prev != nil {
		makeBagClose(OpenBag)
		OpenBag = OpenBag.prev
	}
	OpenBagList = nil
	makeBagClose(OpenBag)
	OpenBag = nil
}

// FixBagArea closes every bag and gives the panel its backpack rows back.
func FixBagArea() {
	if OpenBag == nil {
		return
	}
	CloseAllBags()
	Inventory[INV_BAG].SetIndex(0)
	d := DisplayToSlot()
	for i := INV_PACK; i <= INV_PACK_LAST; i++ {
		d[i+1] = byte(i)
	}
	for i := DISP_PACK; i <= DISP_PACK_LAST; i++ {
		BagSaveHandles[i-DISP_PACK], SaveHandles[i] = SaveHandles[i], BagSaveHandles[i-DISP_PACK]
	}
	mouseHide()
	if (scrmode == 1 || scrmode == 4) && RightPanel == 0 {
		restoreRect(SaveHandles[1])
		DisplayInventory()
	}
	mouseShow()
	InvUpArrow, InvDownArrow = false, false
	DisplayInvObject(DISP_UP)
	DisplayInvObject(DISP_DOWN)
}

// CloseTheBag closes the innermost open bag.
func CloseTheBag() {
	if OpenBagList == nil {
		return
	}
	if OpenBag.prev != nil {
		makeBagClose(OpenBag)
		OpenBag = OpenBag.prev
		OpenBag.next = nil
		Inventory[INV_BAG] = OpenBag.obj
		Inventory[INV_BAG_ITEMS].SetIndex(ObjPtrTMem(&OpenBag.obj).LinkF())
		FixOpenBag()
		DisplayOpenBag()
		displayInventoryArray(DISP_BAG, DISP_BAG)
	} else {
		FixBagArea()
	}
}

// DisplayOpenBag draws the open bag's eight slots and its scroll arrows.
func DisplayOpenBag() {
	mouseHide()
	displayInventoryArray(DISP_PACK, DISP_PACK_LAST)
	obj := ObjPtrTMem(&ObjPtrTMem(&Inventory[INV_BAG]).OL)
	for obj != nil && obj.Invis() != 0 {
		obj = ObjPtrTMem(&obj.QN)
	}
	InvDownArrow = ObjMemTPtr(obj) != Inventory[INV_BAG_ITEMS].Index()
	InvUpArrow = Inventory[INV_BAG_LAST].Index() != 0
	DisplayInvObject(DISP_UP)
	DisplayInvObject(DISP_DOWN)
	mouseShow()
}

// fillBagSlots fills Inventory[20..27] from obj on, skipping invisible objects; it
// returns the object after the last one shown.
func fillBagSlots(from int, obj *Object) *Object {
	for i := from; i <= INV_BAG_LAST; i++ {
		Inventory[i].SetIndex(ObjMemTPtr(obj))
		if obj != nil {
			if obj.Invis() != 0 {
				i--
			}
			obj = ObjPtrTMem(&obj.QN)
		}
	}
	return obj
}

// FixOpenBag refills Inventory[20..27] from the open bag's contents.
func FixOpenBag() {
	i := INV_BAG_ITEMS
	for ; i <= INV_BAG_LAST; i++ {
		if Inventory[i].Index() != 0 {
			break
		}
	}
	obj := ObjPtrTMem(&ObjPtrTMem(&Inventory[INV_BAG]).OL)
	if i > INV_BAG_LAST {
		obj = fillBagSlots(INV_BAG_ITEMS, obj)
		for obj != nil {
			for i = INV_BAG_ITEMS; i < INV_BAG_ITEMS+4; i++ {
				Inventory[i].SetIndex(Inventory[i+4].Index())
			}
			obj = fillBagSlots(i, obj)
		}
	} else {
		for ObjPtrTMem(&Inventory[i]) != obj {
			obj = ObjPtrTMem(&obj.QN)
			if obj == nil {
				return
			}
		}
		fillBagSlots(INV_BAG_ITEMS, obj)
	}
}

// OpenTheBag opens the container in slot in the panel.
func OpenTheBag(slot int) {
	obj := ObjPtrTMem(&Inventory[slot])
	if obj.Major() != MAJOR_MISC || obj.Minor() != MINOR_CONTAINER {
		return
	}
	if obj.InClass() == 0xF {
		setScreenFrame(6, 1)
		return
	}
	if OpenBagList != nil {
		for bag := OpenBagList; bag != nil; bag = bag.next {
			if bag.obj.Index() == Inventory[slot].Index() {
				FixBagArea()
				return
			}
		}
		if slot < INV_PACK {
			CloseAllBags()
		}
	} else {
		mouseHide()
		if (scrmode == 1 || scrmode == 4) && RightPanel == 0 {
			picToScreen(ICON_INV+6, 0xEC, 0x77)
		}
		if BagSaveHandles[0] == 0 {
			for j := DISP_PACK; j <= DISP_PACK_LAST; j++ {
				r := &InvDisplay[j]
				BagSaveHandles[j-DISP_PACK] = valloc(int(r.w), int(r.h))
				saveRect(BagSaveHandles[j-DISP_PACK], r.x, r.y, int(r.w), int(r.h))
			}
		}
		mouseShow()
		for i := INV_BAG_ITEMS; i <= INV_BAG_LAST; i++ {
			SlotToDisplay[i+20] = byte(i)
		}
		for i := DISP_PACK; i <= DISP_PACK_LAST; i++ {
			BagSaveHandles[i-DISP_PACK], SaveHandles[i] = SaveHandles[i], BagSaveHandles[i-DISP_PACK]
		}
	}
	bag := &Bag{}
	if OpenBagList == nil {
		OpenBag, OpenBagList = bag, bag
	} else {
		OpenBag.next = bag
		bag.prev = OpenBag
		OpenBag = bag
	}
	OpenBag.obj.SetIndex(Inventory[slot].Index())
	Inventory[INV_BAG].SetIndex(Inventory[slot].Index())
	cont := ObjPtrTMem(&Inventory[INV_BAG])
	var w int16
	BagWeight(&cont.OL, &w)
	OpenBag.weight = int(w)
	fillBagSlots(INV_BAG_ITEMS, ObjPtrTMem(&cont.OL))
	if cls := cont.InClass(); cls < 12 && cls&1 == 0 {
		cont.SetInClass(cls + 1)
	}
	DisplayOpenBag()
	displayInventoryArray(DISP_BAG, DISP_BAG)
	if SlotToDisplay[slot] < 11 {
		DisplayInvObject(int(SlotToDisplay[slot]))
	}
}

// ScrollItemsUp shows the next row of four of the open bag.
func ScrollItemsUp() {
	if OpenBagList == nil || !InvUpArrow {
		return
	}
	Inventory[INV_BAG_ITEMS] = Inventory[INV_BAG_ITEMS+4]
	FixOpenBag()
	DisplayOpenBag()
}

// ScrollItemsDown shows the previous row of four of the open bag.
func ScrollItemsDown() {
	if OpenBagList == nil || !InvDownArrow {
		return
	}
	obj := ObjPtrTMem(&ObjPtrTMem(&OpenBag.obj).OL)
	first := obj
	target := ObjPtrTMem(&Inventory[INV_BAG_ITEMS])
	for obj != target {
		first = obj
		for i := 0; i < 4; i++ {
			obj = ObjPtrTMem(&obj.QN)
			if obj == nil {
				return
			}
			if obj == target {
				break
			}
		}
	}
	Inventory[INV_BAG_ITEMS].SetIndex(ObjMemTPtr(first))
	FixOpenBag()
	DisplayOpenBag()
}

// PutObjectInBag puts obj into the container at slot (19: the container the open bag sits
// in, or the backpack when it is the outermost).
func PutObjectInBag(obj *Object, slot int) bool {
	found := false
	if ItemFitsSlot(obj, slot) == 0 {
		return false
	}
	var cont *Object
	var bag *Bag
	i := 0
	if slot == INV_BAG && OpenBag.prev == nil {
		for i = INV_PACK; i <= INV_PACK_LAST; i++ {
			if Inventory[i].Index() == 0 {
				found = true
				break
			}
		}
		if i > INV_PACK_LAST {
			return false
		}
		cont = ThePlayer
	} else if slot == INV_BAG {
		bag = OpenBag.prev
		cont = ObjPtrTMem(&bag.obj)
	} else {
		cont = ObjPtrTMem(&Inventory[slot])
		if slot > INV_BAG {
			bag = OpenBag
		}
	}
	if cont.Item() == ITEM_RUNE_BAG {
		if addRune(obj) {
			return true
		}
		gameSprint(0xF7)
		return false
	}
	weight := ItemWeight(obj)
	player.Weight += uint16(weight)
	for ; bag != nil; bag = bag.prev {
		bag.weight += weight
	}
	next := ObjPtrTMem(&cont.OL)
	for next != nil {
		if AddTogether(obj, next) {
			if next.IsQuant() == 0 {
				next.SetIsQuant(1)
				next.SetLinkF(1)
			}
			qty := 1
			if obj.IsQuant() != 0 {
				qty = obj.LinkF()
			}
			next.SetLinkF(next.LinkF() + qty)
			next.SetQuality((next.Quality() + obj.Quality()) >> 1)
			ObjFree(obj)
			break
		}
		next = ObjPtrTMem(&next.QN)
	}
	if next == nil {
		ObjAddEnd(&cont.OL, obj)
		if found {
			Inventory[i].SetIndex(ObjMemTPtr(obj))
		}
	}
	if OpenBag != nil && ObjMemTPtr(cont) == OpenBag.obj.Index() {
		FixOpenBag()
		displayInventoryArray(DISP_PACK, DISP_PACK_LAST)
	} else if displayEnc(true) {
		grfxLoadFont("font5x6p.sys")
	}
	if obj.Item() >= FIRST_LIT_LIGHT && obj.Item() < FIRST_WAND {
		obj.SetInClass(obj.InClass() - 4)
	}
	return true
}

// SwapItemsInBag puts obj in the open bag's slot in place of the object there, which goes
// onto the cursor.
func SwapItemsInBag(obj *Object, slot int) bool {
	result := true
	head := &ObjPtrTMem(&OpenBag.obj).OL
	target := ObjPtrTMem(&Inventory[slot])
	for {
		cur := ObjPtrTMem(head)
		if cur == target {
			break
		}
		if cur == nil {
			return false
		}
		head = &cur.QN
	}
	SetCursorObj(slot, false)
	if ItemFitsSlot(obj, slot) == 0 {
		CursorObjPtr = obj
		unforceMouseCursor(3)
		forceMouseCursor(CursorObjPtr.Item())
		result = false
		obj = target
	}
	ObjAdd(head, obj)
	Inventory[slot].SetIndex(ObjMemTPtr(obj))
	diff := ItemWeight(obj) - ItemWeight(target)
	for bag := OpenBag; bag != nil; bag = bag.prev {
		bag.weight += diff
	}
	player.Weight += uint16(ItemWeight(obj))
	FixPlayerEquips()
	FixOpenBag()
	displayInventoryArray(int(SlotToDisplay[slot]), int(SlotToDisplay[slot]))
	return result
}

// BagWeight adds the weight of every object in the list head, and their contents, to total.
func BagWeight(head *Link, total *int16) {
	obj := ObjPtrTMem(head)
	if obj == nil {
		return
	}
	qty := 1
	if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
		qty = obj.LinkF()
	}
	*total += int16(ComObjData[obj.Item()].Mass() * qty)
	BagWeight(&obj.QN, total)
	if obj.IsQuant() == 0 {
		BagWeight(&obj.OL, total)
	}
}
