package uw

import (
	"encoding/binary"
	"time"

	"github.com/memmaker/uw1go/internal/mp"
)

// Giving: what the player carries, dropped on the other player's avatar, goes to them,
// with what is inside it. The giver sends its records; the taker makes them anew in its
// own object slots and puts the thing in its pack, or in hand when the pack is full,
// or at its feet when its hands are full or it is too heavy. The giver keeps it aside
// until the taker says it has it, and gets it back at its feet if no word comes.

const giveWait = 5 * time.Second

type giving struct {
	obj *Object
	at  time.Time
}

var (
	givings = map[int]giving{} // what this player gave away, by number, until taken
	giveN   int
)

// giveToOther gives obj, carried, to the other player when it is dropped on their
// avatar within reach; true if it went.
func giveToOther(obj *Object) bool {
	if online == nil {
		return false
	}
	av := pick3d(2)
	if av == nil || !isPeerAvatar(ObjMemTPtr(av)) || !InPickRange(PickDist, av, PickMap) {
		return false
	}
	giveN++
	givings[giveN] = giving{obj, time.Now()}
	online.Send(mp.Msg{Op: "give", Level: PlayerLevel, Seq: giveN, Data: packObject(obj)})
	dprintf("mp give %#x\n", obj.Item())
	name, _ := getName(obj, true, false)
	scrollPrint("You hand " + name + " over.\n")
	return true
}

// packObject is obj and everything inside it: its record, then the number of things in
// it and those, in order.
func packObject(obj *Object) []byte {
	var b [STATIC_SIZE]byte
	obj.encode(b[:], false)
	out := b[:]
	var in []*Object
	if obj.IsQuant() == 0 {
		for o := ObjPtrTMem(&obj.OL); o != nil; o = ObjPtrTMem(&o.QN) {
			in = append(in, o)
		}
	}
	out = binary.AppendUvarint(out, uint64(len(in)))
	for _, o := range in {
		out = append(out, packObject(o)...)
	}
	return out
}

// unpackObject makes the object b packs in this player's slots; nil when there is no room
// (what was made is freed again).
func unpackObject(b []byte) (*Object, []byte) {
	if len(b) < STATIC_SIZE {
		return nil, nil
	}
	obj := ObjAlloc(false)
	if obj == nil {
		return nil, nil
	}
	obj.decode(b[:STATIC_SIZE], false)
	obj.SetNext(0)
	b = b[STATIC_SIZE:]
	n, k := binary.Uvarint(b)
	if k <= 0 {
		ObjFree(obj)
		return nil, nil
	}
	b = b[k:]
	if obj.IsQuant() == 0 {
		obj.OL.SetIndex(0)
	}
	var last *Object
	for ; n > 0; n-- {
		var o *Object
		if o, b = unpackObject(b); o == nil {
			ObjFreeChain(objLink(obj))
			return nil, nil
		}
		if last == nil {
			obj.OL.SetIndex(ObjMemTPtr(o))
		} else {
			last.SetNext(ObjMemTPtr(o))
		}
		last = o
	}
	return obj, b
}

func objLink(obj *Object) *Link {
	var l Link
	l.SetIndex(ObjMemTPtr(obj))
	return &l
}

// takeGift takes what the other player gave.
func takeGift(m mp.Msg) {
	obj, _ := unpackObject(m.Data)
	if obj == nil {
		return // no room: no word, and the giver gets it back
	}
	online.Send(mp.Msg{Op: "given", Seq: m.Seq})
	dprintf("mp given %#x\n", obj.Item())
	name, _ := getName(obj, true, false)
	scrollPrint("You are handed " + name + ".\n")
	if CursorObjPtr == nil && GameInputMode == 0 && scrmode == 1 && EncumCheck(obj) {
		GameInputMode = GIM_CARRY
		CursorObjPtr = obj
		forceMouseCursor(obj.Item())
		stowCarried() // into the pack; with the pack full it stays in hand
		return
	}
	dropAtFeet(obj)
}

// dropAtFeet puts obj on the floor where the player stands.
func dropAtFeet(obj *Object) {
	if GrSq < 0 {
		ObjFreeChain(objLink(obj))
		return
	}
	obj.SetFineX(ThePlayer.FineX())
	obj.SetFineY(ThePlayer.FineY())
	obj.SetZ(ThePlayer.Z())
	ObjAddEnd(&Level.Tiles[GrSq].Objects, obj)
	editchng(2)
}

// givenEvent: the other player has what this one gave; givesTick takes back what was
// not taken.
func givenEvent(m mp.Msg) {
	if g, ok := givings[m.Seq]; ok {
		delete(givings, m.Seq)
		ObjFreeChain(objLink(g.obj))
	}
}

func givesTick(now time.Time) {
	for n, g := range givings {
		if now.Sub(g.at) > giveWait || !peerOnLevel(PlayerLevel) {
			delete(givings, n)
			scrollPrint("Nobody took it.\n")
			dropAtFeet(g.obj)
		}
	}
}
