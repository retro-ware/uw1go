package uw

// Port of OBJECTS.C: the object store in the level block, allocating and freeing objects,
// the lists, the conversions between links and objects, searches, the consistency checks,
// and garbage collection. Every object lives in Level.Objs: indices 0..0xFF are mobile
// (critdata; 0 unused, 1 the player), 0x100..0x3FF static (objdata). The free stacks
// critbot..critptr and objbot..objptr are Level.MobFree and Level.StaticFree with critptr
// and objptr the index of the top entry (-1 empty); ActiveMob..LastActiveMob is
// Level.Active[:nActive].

var (
	critptr, objptr int
	nActive         int
	ObjFindHead     *Link
	mobcount        int
	objcount        int
	cullRange       int
)

// memtptr is MEMTPTR: an object's index.
func memtptr(obj *Object) int { return objIndexOf(obj) }

// MapObjFix empties the object store (Map_ObjFix).
func MapObjFix() {
	for i := range Level.Tiles {
		Level.Tiles[i].Objects.SetIndex(0)
	}
	for i := 2; i < NUM_OBJECTS; i++ {
		if i < NUM_MOBILE {
			Level.MobFree[i-2] = uint16(i)
		} else {
			Level.StaticFree[i-NUM_MOBILE] = uint16(i)
		}
	}
	critptr = 0xFD
	objptr = 0x2FF
	if ThePlayer != nil {
		ThePlayer.SetLinkF(0)
		ThePlayer.SetNext(0)
		ClearInventory()
	}
	animcount = 0
	nActive = 0
}

// ObjCheck is Obj_Check: whether fn holds for obj or anything after it, depth first.
func ObjCheck(obj *Object, fn func(*Object) bool) bool {
	for !fn(obj) {
		if obj.IsQuant() == 0 && obj.LinkF() > 0 {
			if ObjCheck(ObjPtrTMem(&obj.OL), fn) {
				return true
			}
		}
		if obj.Next() == 0 {
			return false
		}
		obj = ObjPtrTMem(&obj.QN)
	}
	return true
}

func chkTenacious(obj *Object) bool {
	if obj.DoorDir() != 0 {
		return true
	}
	extra := 0
	if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
		extra = obj.LinkF() - 1
	}
	return ComObjData[obj.Item()].Tenacity()+extra/2 > cullRange
}

// ObjElemFate decides whether an object may be deleted.
func ObjElemFate(rng int, obj *Object) bool {
	if obj == nil {
		return false
	}
	if rng != 0 {
		rng += rand() * 3 / 0x8000
	}
	cullRange = rng
	if chkTenacious(obj) {
		return false
	}
	if obj.IsQuant() == 0 && obj.LinkF() > 0 {
		if ObjCheck(ObjPtrTMem(&obj.OL), chkTenacious) {
			return false
		}
	}
	return rand()*10/0x8000 < cullRange
}

func objFate(rng int, head *Link) bool {
	if head.Index() == 0 {
		return false
	}
	return ObjElemFate(rng, ObjPtrTMem(head))
}

// ObjGarbageCollect deletes up to count objects far from the player.
func ObjGarbageCollect(rng, count int) {
	px := ThePlayer.HomeX()
	py := ThePlayer.HomeY()
	freed := 0
	for y := 0; y < MAP_SIZE; y++ {
		dy := abs(py - y)
		for x := 0; x < MAP_SIZE; x++ {
			t := &Level.Tiles[y*MAP_SIZE+x]
			if dy+abs(px-x) > 10-rng {
				tilehead := &t.Objects
				link := *tilehead
				for link.Index() > 0 {
					next := ObjPtrTMem(&link).Next()
					if objFate(rng, &link) {
						ObjFreeLinkChain(tilehead, ObjIntTMem(link.Index()))
						freed++
						if freed >= count {
							return
						}
					}
					link.SetIndex(next)
				}
			}
		}
	}
}

// ObjAlloc pops a free mobile or static object.
func ObjAlloc(mobile bool) *Object {
	if mobile {
		if critptr < 0 || !onlineTakeFree(Level.MobFree[:], critptr) {
			ObjGarbageCollect(3, 5)
			if critptr < 0 || !onlineTakeFree(Level.MobFree[:], critptr) {
				return nil
			}
		}
		i := int(Level.MobFree[critptr])
		activeCritter(i)
		critptr--
		return &Level.Objs[i]
	}
	if objptr < 0 || !onlineTakeFree(Level.StaticFree[:], objptr) {
		ObjGarbageCollect(3, 10)
		if objptr < 0 || !onlineTakeFree(Level.StaticFree[:], objptr) {
			return nil
		}
	}
	i := int(Level.StaticFree[objptr])
	objptr--
	return &Level.Objs[i]
}

// ObjFree pushes an object back on its free stack.
func ObjFree(obj *Object) {
	i := memtptr(obj)
	if onlineFreeElsewhere(i) { // the other player's slot: freed there
		if i < NUM_MOBILE {
			freeCritter(i)
		}
		return
	}
	if i < NUM_MOBILE {
		critptr++
		Level.MobFree[critptr] = uint16(i)
		if obj == UsPtr {
			releaseCamera(i)
		}
		freeCritter(i)
	} else {
		objptr++
		Level.StaticFree[objptr] = uint16(i)
	}
}

func ObjAdd(head *Link, obj *Object) {
	obj.SetNext(head.Index())
	head.SetIndex(memtptr(obj))
}

func ObjAddEnd(head *Link, obj *Object) {
	p := head
	for {
		o := ObjPtrTMem(p)
		if o == nil {
			break
		}
		p = &o.QN
	}
	obj.SetNext(0)
	p.SetIndex(memtptr(obj))
}

func ObjRem(head *Link, obj *Object) {
	if obj == nil {
		return
	}
	guard := 0
	for p := head; ; {
		o := ObjPtrTMem(p)
		if o == nil {
			return
		}
		g := guard
		guard++
		if g >= 0x401 {
			return
		}
		if o == obj {
			p.SetIndex(obj.Next())
			obj.SetNext(0)
			return
		}
		p = &o.QN
	}
}

// ObjPunt deletes obj and its contents when force is set or ObjElemFate allows; nil when
// deleted, else obj.
func ObjPunt(head *Link, obj *Object, force bool) *Object {
	if force || ObjElemFate(10, obj) {
		var link Link
		link.SetIndex(memtptr(obj))
		if obj.Major() == MAJOR_ANIMOBJ {
			remAnimFromList(link.Index())
		}
		if head != nil {
			ObjFreeLinkChain(head, obj)
		} else {
			ObjFreeChain(&link)
		}
		return nil
	}
	return obj
}

func ObjFreeChain(head *Link) {
	obj := ObjPtrTMem(head)
	if obj == nil {
		return
	}
	if obj.Major() == MAJOR_TRAP {
		trapObjDel(head, obj)
	} else {
		if obj.Next() > 0 {
			ObjFreeChain(&obj.QN)
		}
		if obj.IsQuant() == 0 && obj.LinkF() > 0 {
			ObjFreeChain(&obj.OL)
		}
		ObjRem(head, obj)
		ObjFree(obj)
	}
}

func ObjFreeLinkChain(head *Link, obj *Object) {
	if obj.IsQuant() == 0 && obj.LinkF() > 0 {
		ObjFreeChain(&obj.OL)
	}
	if head != nil {
		ObjRem(head, obj)
	}
	ObjFree(obj)
}

// ObjPtrTMem is the object a link names.
func ObjPtrTMem(link *Link) *Object {
	if link == nil || link.Index() == 0 {
		return nil
	}
	return &Level.Objs[link.Index()]
}

func ObjMemTPtr(obj *Object) int {
	if obj == nil {
		return 0
	}
	return memtptr(obj)
}

func ObjIntTMem(index int) *Object {
	if index == 0 {
		return nil
	}
	return &Level.Objs[index&0x3FF]
}

// ObjFind finds object index in a list and its contents.
func ObjFind(head *Link, recurse bool, index int) *Object {
	if head.Index() == 0 {
		return nil
	}
	ObjFindHead = head
	obj := ObjPtrTMem(head)
	for ObjMemTPtr(obj) != index {
		if obj.IsQuant() == 0 && obj.LinkF() != 0 {
			if found := ObjFind(&obj.OL, recurse, index); found != nil {
				return found
			}
		}
		if obj.Next() == 0 {
			return nil
		}
		obj = ObjPtrTMem(&obj.QN)
	}
	ObjFindHead = head
	return obj
}

func IsMobElem(obj *Object) bool {
	if obj == nil {
		return false
	}
	i := objIndexOf(obj)
	return i >= 0 && i < NUM_MOBILE
}

func activeCritter(index int) {
	Level.Active[nActive] = byte(index)
	nActive++
}

func freeCritter(index int) {
	for p := 0; p < nActive; p++ {
		if Level.Active[p] == byte(index) {
			nActive--
			if p < nActive {
				Level.Active[p] = Level.Active[nActive]
			}
			return
		}
	}
}

// ObjInList finds the first object in *head's list of the class given (-1 any); with
// recurse it searches contents, and *head becomes the container's list.
func ObjInList(head **Link, recurse bool, major, minor, index int) *Object {
	for obj := ObjPtrTMem(*head); obj != nil; obj = ObjPtrTMem(&obj.QN) {
		if (major == -1 || obj.Major() == major) && (minor == -1 || obj.Minor() == minor) &&
			(index == -1 || obj.InClass() == index) {
			return obj
		}
		if recurse && obj.IsQuant() == 0 && obj.OL.Index() != 0 {
			list := &obj.OL
			if found := ObjInList(&list, recurse, major, minor, index); found != nil {
				*head = list
				return found
			}
		}
	}
	return nil
}

func HasOrIsObj(obj *Object, id int) bool {
	if obj.Item() == id {
		return true
	}
	if obj.IsQuant() == 0 {
		list := &obj.OL
		if ObjInList(&list, true, id>>6, (id&ID_MINOR)>>4, id&ID_INCLASS) != nil {
			return true
		}
	}
	return false
}

// ObjFindInMap scans the map from (*x, *y) on for a matching object.
func ObjFindInMap(major, minor, index int, x, y *int) *Object {
	if *x >= MAP_SIZE {
		*x = 0
		*y++
	}
	for ; *y < MAP_SIZE; *y++ {
		for ; *x < MAP_SIZE; *x++ {
			head := &Level.Tiles[*y<<6+*x].Objects
			if head.Index() > 0 {
				if found := ObjInList(&head, true, major, minor, index); found != nil {
					return found
				}
			}
		}
		*x = 0
	}
	return nil
}

func flogList(x, y int, counts []byte, head *Link) bool {
	bad := false
	for obj := ObjPtrTMem(head); obj != nil; obj = ObjPtrTMem(&obj.QN) {
		i := ObjMemTPtr(obj)
		counts[i]++
		if counts[i] > 1 {
			dprintf("====ERROR==== @%d,%d: Object %d occured for time #%d.\n", x, y, i, counts[i])
			bad = true
			if counts[i] > 10 {
				return true
			}
		}
		if obj.IsQuant() == 0 && obj.LinkF() > 0 {
			bad = flogList(x, y, counts, &obj.OL)
		}
		if IsMobElem(obj) {
			mobcount++
		} else {
			objcount++
		}
	}
	return bad
}

func countList(obj *Object, counts []byte) bool {
	bad := false
	for ; obj != nil; obj = ObjPtrTMem(&obj.QN) {
		i := ObjMemTPtr(obj)
		counts[i]++
		if counts[i] > 1 {
			dprintf("====ERROR==== Object %d occured for time #%d.\n", i, counts[i])
			bad = true
			if counts[i] > 10 {
				return true
			}
		}
		if obj.IsQuant() == 0 && obj.LinkF() > 0 {
			bad = countList(ObjPtrTMem(&obj.OL), counts)
		}
		if IsMobElem(obj) {
			mobcount++
		} else {
			objcount++
		}
	}
	return bad
}

var objlistComplained bool

// checkObjList complains once when the object store is inconsistent.
func checkObjList() {
	if !ObjListOkay(false) {
		if !objlistComplained {
			errmsg("Problems in", "object list")
			objlistComplained = true
		}
	} else {
		objlistComplained = false
	}
}

// ObjListOkay checks the object store; true when consistent.
func ObjListOkay(how bool) bool {
	counts := make([]byte, NUM_OBJECTS)
	bad := false
	nmobile := b2i(GrSq < 0)
	for p := critptr; p >= 0; p-- {
		if counts[Level.MobFree[p]] != 0 {
			bad = true
		}
		counts[Level.MobFree[p]]++
		nmobile++
	}
	nstatic := 0
	for p := objptr; p >= 0; p-- {
		if counts[Level.StaticFree[p]] != 0 {
			bad = true
		}
		counts[Level.StaticFree[p]]++
		nstatic++
	}
	objcount, mobcount = 0, 0
	for y := 0; y < MAP_SIZE; y++ {
		for x := 0; x < MAP_SIZE; x++ {
			head := &Level.Tiles[y*MAP_SIZE+x].Objects
			if head.Index() > 0 {
				bad = flogList(x, y, counts, head) || bad
			}
		}
	}
	if (GameInputMode == 1 || GameInputMode == 0) && CursorObjPtr != nil {
		bad = countList(CursorObjPtr, counts) || bad
	}
	if online != nil { // the other player's things are in neither the lists nor the free stacks
		return !bad
	}
	if nmobile+mobcount != 0xFF {
		bad = true
	}
	if nstatic+objcount != NUM_STATIC {
		bad = true
	}
	return !bad
}

// MAPADDR.C

// MapGetAddr is the tile at x, y, or nil outside the map.
func MapGetAddr(x, y int) *Tile {
	if (x&^0x3F)+(y&^0x3F) == 0 {
		return &Level.Tiles[x+y<<6]
	}
	return nil
}

// CreateObj makes a new object of type item from a free list, not placed anywhere.
func CreateObj(item int, mobile bool) *Object {
	obj := ObjAlloc(mobile)
	com := &ComObjData[item]
	if obj == nil {
		return nil
	}
	obj.SetQuality(40)
	obj.SetItem(item)
	obj.SetZ(0)
	obj.SetDoorDir(0)
	obj.SetInvis(0)
	obj.SetEnchanted(0)
	obj.SetFlag11(0)
	obj.SetFineX(3)
	obj.SetFineY(3)
	obj.SetHeading(0)
	obj.SetFlags(0)
	obj.SetNext(0)
	obj.SetOwner(0)
	if com.Stack() == 0 || com.Stack() == 2 {
		obj.SetLinkF(1)
		obj.SetIsQuant(1)
	} else {
		obj.SetIsQuant(0)
		obj.SetLinkF(0)
	}
	return obj
}

// MAP.C

var (
	hgtVal   = [17]int16{0x000, 0x040, 0x080, 0x0C0, 0x100, 0x140, 0x180, 0x1C0, 0x200, 0x240, 0x280, 0x2C0, 0x300, 0x340, 0, 0, 0x400}
	MapDirty bool
)

func MapInit() bool {
	MapObjFix()
	return true
}

// MapLoad reads level (1-based) from arc, or SAVE0\LEV.ARK with none.
func MapLoad(arc *Arc, level int) bool {
	var a Arc
	if arc == nil {
		if !openArc(&a, "SAVE0\\lev.ark") {
			return false
		}
		arc = &a
		defer closeArc(&a)
	}
	Level.Magic = 0
	b := getArc(arc, level-1)
	if len(b) >= LEVEL_SIZE {
		Level.decode(b)
	}
	if Level.Magic != LEVEL_MAGIC {
		pfatalCode(3)
	} else {
		critptr = int(int16(Level.NMobFree))
		objptr = int(int16(Level.NStatFree))
		nActive = int(Level.NActive)
		MapDirty = false
	}
	result := AnimLoad(arc, level)
	return result
}

// MapSave writes the level block back as level.
func MapSave(arc *Arc, level int) bool {
	var a Arc
	if arc == nil {
		if !openArc(&a, "SAVE0\\lev.ark") {
			return false
		}
		arc = &a
		defer closeArc(&a)
	}
	Level.NActive = uint16(nActive)
	Level.NMobFree = uint16(critptr)
	Level.NStatFree = uint16(objptr)
	Level.Magic = LEVEL_MAGIC
	MapDirty = false
	result := putArc(arc, level-1, Level.encode())
	if result {
		result = AnimSave(arc, level)
	}
	return result
}
