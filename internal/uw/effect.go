package uw

// Port of EFFECT.C (seg044): animated objects. An animated object (MAJOR_ANIMOBJ) is an
// ordinary object in a tile's list plus an entry in animlist saying how many frames it has
// left and on which tile it is. Its class (animclassd, OBJECTS.DAT) says what each frame
// does: cycle or pick the frame (kept in the owner field), or move a door.
// updateAnimobj steps them all each frame and ends the expired ones (toastAnimobj).

// Anim is an animlist entry (6 bytes in LEV.ARK).
type Anim struct {
	link Link
	len  int16
	x, y byte
}

var (
	lengset   bool
	DoAnimO   = true
	animcount int
	animlist  [MAX_ANIMS]Anim
	// animWho says who runs each animation online (onlineanim.go)
	animWho [MAX_ANIMS]int8
)

// remAnimFromMap removes animation n's object from its tile and frees it.
func remAnimFromMap(n int) {
	obj := ObjPtrTMem(&animlist[n].link)
	t := MapGetAddr(int(animlist[n].x), int(animlist[n].y))
	ObjRem(&t.Objects, obj)
	ObjFree(obj)
}

// remAnimFromList drops the animation of object index from animlist.
func remAnimFromList(index int) {
	i := 0
	for ; i < animcount; i++ {
		if animlist[i].link.Index() == index {
			break
		}
	}
	if i < animcount {
		animDrop(i)
	}
}

// toastAnimobj ends animation n: a moving door becomes a door again (or turns round
// when something stands in a closing doorway), a removable effect leaves the map.
func toastAnimobj(n, frames int) {
	obj := ObjPtrTMem(&animlist[n].link)
	cls := obj.InClass()
	typ := animclassd[cls].Flags
	if typ&ANIMF_FINISH != 0 && animlist[n].len != 0 {
		doAnimobj(n, int(animlist[n].len))
	}
	if cls == 0xF {
		major := MAJOR_RECT
		minor := obj.Owner() >> 4
		state := obj.Owner() & 0xF
		z := obj.Z()
		if obj.Flags()&8 != 0 {
			if state >= 8 {
				state -= 8
			}
			if state&7 != 6 {
				z -= 0x18
			}
			XP, YP = int(animlist[n].x), int(animlist[n].y)
			if !canPlace(major<<6+minor<<4+state, ObjMemTPtr(obj), XP<<3+obj.FineX(), YP<<3+obj.FineY(), z, true, 8) {
				obj.SetOwner(state)
				changeDoor(obj)
				return
			}
			playEffect(0xC, XP<<3+obj.FineX(), YP<<3+obj.FineY(), 0)
		} else {
			state |= 8
		}
		obj.SetZ(z)
		obj.SetMajor(major)
		obj.SetMinor(minor)
		obj.SetInClass(state)
		obj.SetOwner(0)
		if obj.Flags()&8 != 0 {
			obj.SetFlags(obj.Flags() & 7)
		} else {
			obj.SetFlags(obj.Flags()&7 + 8)
		}
	}
	if typ&ANIMF_REMOVE != 0 {
		remAnimFromMap(n)
	}
	animDrop(n)
}

// changeAnimPtr: the animation of from now belongs to to.
func changeAnimPtr(to, from *Object) {
	toidx, fromidx := ObjMemTPtr(to), ObjMemTPtr(from)
	for i := 0; i < animcount; i++ {
		if animlist[i].link.Index() == fromidx {
			animlist[i].link.SetIndex(toidx)
			animSendMoved(i, fromidx, toidx)
			break
		}
	}
}

// addAnimobj starts an animation of length frames (-1 for ever) for object index on
// tile (x, y), from frame a. Returns the new count, or -1 when the list is full.
func addAnimobj(index, length, a, x, y int) int {
	if animcount+1 > MAX_ANIMS {
		return -1
	}
	e := &animlist[animcount]
	e.link = 0
	e.link.SetIndex(index)
	e.len = int16(length)
	e.x, e.y = byte(x), byte(y)
	obj := ObjPtrTMem(&e.link)
	ac := &animclassd[obj.InClass()]
	if ac.Start >= 0 {
		if ac.Count != 0 {
			obj.SetOwner(int(ac.Start) + (a&0xFF)%int(ac.Count))
		} else {
			obj.SetOwner(int(ac.Start))
		}
	}
	AnimObjInPipe = 1
	animWho[animcount] = animOurs
	animSendAdded(animcount)
	animcount++
	return animcount
}

// doAnimobj steps animation n by frames.
func doAnimobj(n, frames int) {
	obj := ObjPtrTMem(&animlist[n].link)
	if obj.Item()&^0xF != FIRST_ANIMOBJ {
		return
	}
	cls := obj.InClass()
	ac := &animclassd[cls]
	typ := ac.Flags
	for mask := uint16(1); typ > 0; typ, mask = typ&^mask, mask<<1 {
		switch typ & mask {
		case ANIMF_CYCLE:
			if int(ac.Start)+int(ac.Count)-1 > obj.Owner() {
				obj.SetOwner(obj.Owner() + 1)
			} else {
				obj.SetOwner(int(ac.Start))
			}
		case ANIMF_RANDOM:
			if ac.Count != 0 {
				obj.SetOwner(int(ac.Start) + rand()%int(ac.Count))
			}
		case ANIMF_DOOR:
			if obj.Flags()&8 != 0 {
				frames = -frames
			}
			if obj.Owner()&7 == 6 {
				obj.SetZ(obj.Z() + frames*6)
			}
			obj.SetFlags(obj.Flags()&7 + frames + obj.Flags()&8)
			if obj.Flags()&8 != 0 {
				checkDoor(n, frames)
			}
		}
	}
}

// updateAnimobj steps every animation by frames and ends those that run out.
func updateAnimobj(frames int) {
	t := 0
	if animcount > 0 {
		t += frames
	}
	if AnimObjInPipe != 0 {
		editchng(2)
	}
	for i := 0; i < animcount; i++ {
		if !animRunsHere(i) {
			continue // the other player runs it
		}
		if animlist[i].len == -1 {
			doAnimobj(i, t)
			continue
		}
		left := int(animlist[i].len) - t
		if left < 0 {
			toastAnimobj(i, t)
		} else {
			doAnimobj(i, t)
			if lengset {
				lengset = false
			} else {
				animlist[i].len = int16(left)
			}
		}
	}
}

// fireballEffect scatters three to five short-lived flames around the explosion src.
func fireballEffect(src *Object, x, y int) {
	for n := rand()%3 + 2; n >= 0; n-- {
		cp := ObjAlloc(false)
		if cp == nil {
			return
		}
		setStatic(cp, src)
		cp.SetItem(cp.Item() + rand()&1 + 1)
		r := cp.FineX()
		off := 0
		for {
			off = rand()%5 - 2
			if r+off >= 0 && r+off <= 7 {
				break
			}
		}
		cp.SetFineX(r + off)
		r = cp.FineY()
		for {
			off = rand()%5 - 2
			if r+off >= 0 && r+off <= 7 {
				break
			}
		}
		cp.SetFineY(r + off)
		cp.SetZ(cp.Z() - 8 + rand()&0xF)
		ObjAdd(&MapGetAddr(x, y).Objects, cp)
		r = rand() % 3
		off = 2 - r + rand()%3
		if addAnimobj(ObjMemTPtr(cp), off, r, x, y) == -1 {
			ObjRem(&MapGetAddr(x, y).Objects, cp)
			ObjFree(cp)
			n = -1
		}
	}
}

// mtsDoanim: a fireball or lightning bolt struck at (x, y) becomes its explosion.
// False for any other missile.
func mtsDoanim(obj *Object, x, y, who int) bool {
	from := [2]int{ITEM_FIREBALL, ITEM_LIGHTNING_BOLT}
	to := [2]int{ITEM_EXPLOSION_1C2, ITEM_SPLASH_1C5}
	i := 0
	for ; i < 2; i++ {
		if from[i] == obj.Item() {
			break
		}
	}
	if i >= 2 {
		return false
	}
	obj.SetItem(to[i])
	if addAnimobj(ObjMemTPtr(obj), 4, 0, x, y) == -1 {
		return false
	}
	if i == 0 {
		fireballEffect(obj, x, y)
	}
	damageSquare(x, y, i+1, who&0xFF)
	return true
}

// putEffect creates effect cls on tile (x, y) for length frames from frame: at who's fine
// position, at height -z when z is negative, else who's z plus z eighths of its height.
func putEffect(who *Object, cls, length, frame, z, x, y int) bool {
	p := CreateObj(cls+FIRST_ANIMOBJ, false)
	if p == nil {
		return false
	}
	if who != nil {
		p.SetFineX(who.FineX())
		p.SetFineY(who.FineY())
	}
	if z < 0 {
		p.SetZ(-z)
	} else if who != nil {
		h := byte(ComObjData[who.Item()].Height() >> 3)
		if h == 0 {
			h = 1
		}
		h *= byte(z)
		p.SetZ(who.Z() + int(h))
	}
	if addAnimobj(ObjMemTPtr(p), length, frame, x, y) == -1 {
		ObjFree(p)
		return false
	}
	ObjAddEnd(&MapGetAddr(x, y).Objects, p)
	return true
}

// findAnim is obj's animlist entry, or -1.
func findAnim(obj *Object) int {
	index := ObjMemTPtr(obj)
	for i := 0; i < animcount; i++ {
		if animlist[i].link.Index() == index {
			return i
		}
	}
	return -1
}

// getAnimlen is the frames left in obj's animation, -1 for ever, -2 when it has none.
func getAnimlen(obj *Object) int {
	i := findAnim(obj)
	if i < 0 {
		return -2
	}
	return int(animlist[i].len)
}

// setAnimlen sets the frames left in obj's animation.
func setAnimlen(obj *Object, length int) {
	if i := findAnim(obj); i >= 0 {
		animlist[i].len = int16(length)
	}
}

// checkDoor: a closing door checks its doorway and turns round when blocked. True when
// the way is clear.
func checkDoor(n, frames int) bool {
	limit := 5
	obj := ObjPtrTMem(&animlist[n].link)
	minor := obj.Owner() & 0xF
	item := obj.Owner()>>4<<4 + minor + FIRST_RECT
	z := obj.Z()
	if minor&7 != 6 {
		z -= 0x18
	}
	XP, YP = int(animlist[n].x), int(animlist[n].y)
	if canPlace(item, ObjMemTPtr(obj), XP<<3+obj.FineX(), YP<<3+obj.FineY(), z, true, 8) {
		return true
	}
	if obj.Major() == MAJOR_RECT && obj.ID&7 == 6 || obj.Major() == MAJOR_ANIMOBJ && obj.Owner()&7 == 6 {
		limit = 4
	}
	obj.SetFlags(obj.Flags() & 7)
	obj.SetFlags(obj.Flags()&8 + (obj.Flags()&7 - frames - 1))
	if length := getAnimlen(obj); length >= 0 {
		animlist[n].len = int16(limit - length + 1)
		lengset = true
	}
	return false
}

// AnimLoad reads the animation list of level from LEV.ARK block level+8.
func AnimLoad(arc *Arc, level int) bool {
	b := getArc(arc, level+8)
	if len(b) != ANIM_BLOCK_LEN {
		animcount = 0
		return false
	}
	count := MAX_ANIMS
	for i := 0; i < MAX_ANIMS; i++ {
		e := b[i*6:]
		animlist[i] = Anim{Link(le16(e, 0)), int16(le16(e, 2)), e[4], e[5]}
		if animlist[i].link.Index() == 0 && count == MAX_ANIMS {
			count = i
		}
	}
	animcount = count
	animWho = [MAX_ANIMS]int8{}
	return true
}

// AnimSave writes the animation list as LEV.ARK block level+8.
func AnimSave(arc *Arc, level int) bool {
	return putArc(arc, level+8, animBytes())
}

// animBytes is the animation list as LEV.ARK keeps it.
func animBytes() []byte {
	if animcount < MAX_ANIMS {
		animlist[animcount].link.SetIndex(0)
	}
	b := make([]byte, ANIM_BLOCK_LEN)
	for i, e := range animlist {
		o := b[i*6:]
		o[0], o[1] = byte(e.link), byte(e.link>>8)
		o[2], o[3] = byte(e.len), byte(uint16(e.len)>>8)
		o[4], o[5] = e.x, e.y
	}
	return b
}
