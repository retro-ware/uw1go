package uw

// Port of INVSAVE.C: saving and restoring the player's inventory in PLAYER.DAT. The player
// object, the object on the cursor and every object reachable from them are copied into
// the workspace, their links renumbered to the copy's own indexes, and back again.
//
// The workspace holds the player object at 0, the cursor object at 0x1B, a copy of the
// inventory slots at 0x23, and from 0x5B the saved objects (8-byte static records),
// numbered from 1 so that 0 stays the empty link.

const (
	wsCursor = 0x1B
	wsSlots  = 0x23
	wsObjs   = 0x5B
)

var (
	saveNum int
	saveBuf []byte // the workspace, nil when released
)

func wsGetSlot(i int) Link    { return Link(le16(saveBuf, wsSlots+i*2)) }
func wsSetSlot(i int, l Link) { put16(saveBuf, wsSlots+i*2, uint16(l)) }

// wsLink is a link word in the workspace, at byte offset off.
type wsLink int

func (w wsLink) get() Link  { return Link(le16(saveBuf, int(w))) }
func (w wsLink) set(l Link) { put16(saveBuf, int(w), uint16(l)) }
func (w wsLink) index() int { return w.get().Index() }
func (w wsLink) setIndex(i int) {
	l := w.get()
	l.SetIndex(i)
	w.set(l)
}

func wsObjOff(n int) int { return wsObjs + n*8 }

func wsStatic(off int) *Object {
	var o Object
	o.decode(saveBuf[off:], false)
	return &o
}

func setStatic(dst, src *Object) {
	dst.ID, dst.Pos, dst.QN, dst.OL = src.ID, src.Pos, src.QN, src.OL
}

// makePlayerInvCopy fills the workspace with the player object, the inventory and, in input
// mode 1, the cursor object (which is freed from memory: it is in the copy now).
func makePlayerInvCopy() {
	FixBagArea()
	p := *ThePlayer
	p.SetNext(0)
	p.encode(saveBuf, true)
	saveNum = 0
	for i := 0; i <= INV_PACK_LAST; i++ {
		wsSetSlot(i, 0)
	}
	invSaveNexts(&ThePlayer.OL, wsLink(6))
	if GameInputMode == 1 && CursorObjPtr != nil {
		CursorObjPtr.encode(saveBuf[wsCursor:], false)
		if CursorObjPtr.IsQuant() == 0 {
			invSaveNexts(&CursorObjPtr.OL, wsLink(wsCursor+6))
		}
		var chain Link
		chain.SetIndex(ObjMemTPtr(CursorObjPtr))
		ObjFreeChain(&chain)
	}
}

// SavePlayerInv copies the inventory to the workspace and, given a save directory name,
// writes name + "player.dat" and releases the workspace.
func SavePlayerInv(name string) bool {
	ok := true
	saveBuf = make([]byte, 0xF000)
	makePlayerInvCopy()
	saveNum++
	if name != "" {
		fd := dosOpen(name+"player.dat", O_RDWR|O_CREAT|O_TRUNC)
		if fd >= 0 {
			savePlayerData(fd)
			var n [2]byte
			put16(n[:], 0, uint16(saveNum))
			dosWrite(fd, n[:])
			dosWrite(fd, saveBuf[:saveNum*8+wsObjs])
			dosClose(fd)
		} else {
			ok = false
		}
		saveBuf = nil
		editchng(0x200)
	}
	return ok
}

// invSaveNexts copies the list at src, and every list inside it, to the save area,
// pointing dst at it.
func invSaveNexts(src *Link, dst wsLink) {
	for {
		obj := ObjPtrTMem(src)
		if obj == nil {
			return
		}
		saveNum++
		off := wsObjOff(saveNum)
		obj.encode(saveBuf[off:], false)
		dst.setIndex(saveNum)
		replaceInInv(src.Index(), saveNum)
		src = &obj.QN
		dst = wsLink(off + 4)
		if obj.IsQuant() == 0 && obj.OL.Index() != 0 {
			invSaveNexts(&obj.OL, wsLink(off+6))
		}
	}
}

// replaceInInv points a slot holding the object at old at its copy instead.
func replaceInInv(old, new int) {
	for i := 0; i <= INV_PACK_LAST; i++ {
		if Inventory[i].Index() == old {
			l := wsGetSlot(i)
			l.SetIndex(new)
			wsSetSlot(i, l)
		}
	}
}

// putInInv points a slot holding the saved object at saved at its restored copy.
func putInInv(mem, saved int) {
	for i := 0; i <= INV_PACK_LAST; i++ {
		if wsGetSlot(i).Index() == saved {
			Inventory[i].SetIndex(mem)
		}
	}
}

// invRestoreNexts restores the saved list at src, and every list inside it, pointing dst
// at it.
func invRestoreNexts(dst *Link, src wsLink) {
	for {
		n := src.index()
		if n == 0 {
			return
		}
		off := wsObjOff(n)
		saved := wsStatic(off)
		obj := ObjAlloc(false)
		setStatic(obj, saved)
		dst.SetIndex(ObjMemTPtr(obj))
		putInInv(dst.Index(), n)
		dst = &obj.QN
		src = wsLink(off + 4)
		if saved.IsQuant() == 0 && saved.OL.Index() != 0 {
			invRestoreNexts(&obj.OL, wsLink(off+6))
		}
	}
}

// FreePlayerInv frees every object in the list head and in their contents.
func FreePlayerInv(head *Link) {
	obj := ObjPtrTMem(head)
	if obj == nil {
		return
	}
	if obj.IsQuant() == 0 && obj.OL.Index() != 0 {
		FreePlayerInv(&obj.OL)
	}
	if obj.QN.Index() != 0 {
		FreePlayerInv(&obj.QN)
	}
	ObjRem(head, obj)
	ObjFree(obj)
}

// getPlayerInvCopy rebuilds the player object, the inventory and the cursor object.
func getPlayerInvCopy() {
	var p Object
	p.decode(saveBuf, true)
	*ThePlayer = p
	ThePlayer.OL.SetIndex(0)
	invRestoreNexts(&ThePlayer.OL, wsLink(6))
	if GameInputMode == 1 {
		cursor := wsStatic(wsCursor)
		CursorObjPtr = ObjAlloc(false)
		setStatic(CursorObjPtr, cursor)
		if cursor.IsQuant() == 0 {
			CursorObjPtr.OL.SetIndex(0)
			invRestoreNexts(&CursorObjPtr.OL, wsLink(wsCursor+6))
		}
	}
}

// RestorePlayerInv frees the inventory and rebuilds it, from name + "player.dat" when a
// save directory is given, or from the workspace a nameless SavePlayerInv filled.
func RestorePlayerInv(name string) bool {
	ok := true
	if name != "" && GrSq >= 0 {
		ObjRem(&Level.Tiles[GrSq].Objects, ThePlayer)
	}
	PuntPlayerInv()
	if saveBuf == nil {
		saveBuf = make([]byte, 0xF000)
	}
	if name != "" {
		fd := dosOpen(name+"player.dat", O_RDWR)
		if fd >= 0 {
			readPlayerData(fd)
			var n [2]byte
			dosRead(fd, n[:])
			saveNum = int(le16(n[:], 0))
			dosRead(fd, saveBuf[:min(saveNum*8+wsObjs, len(saveBuf))])
			dosClose(fd)
			loadInventoryPix()
		} else {
			ok = false
		}
	}
	if ok {
		getPlayerInvCopy()
		FixPlayerEquips()
	}
	saveBuf = nil
	if name != "" && GrSq >= 0 {
		ObjAdd(&Level.Tiles[GrSq].Objects, ThePlayer)
	}
	return ok
}
