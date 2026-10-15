package uw

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/memmaker/uw1go/internal/mp"
)

// Keeping the world the same for both players. The world is the host's: a player who joins
// takes the host's LEV.ARK (the levels' maps and objects, not the automaps) in place of
// their own. From then on:
//
//   - The level a player is on is its level block (map and object records, as LEV.ARK
//     has it). Twenty times a second each player compares theirs with the last one sent
//     and sends the bytes that changed; the other patches them into their own. Their own
//     player object, and what they carry, are left out both ways (each player's own).
//   - Where both changed the same byte, the level's owner wins: the owner is the
//     lower-numbered player on the level (the host when both are there). Diffs are
//     numbered, and each says the last one it has seen from the other. The owner keeps
//     its own value for a byte it changed in a diff the other had not seen yet (or has
//     not sent yet), as the other will take that diff when it comes; the other takes
//     whatever the owner sends. Neither sends back what it took, so nothing stale returns.
//   - The owner moves the level's creatures (their AI); the other only sees them move.
//     Every other moving object (a thrown dagger, a falling stone) is moved by the player
//     who made it.
//   - The players never take the same free object slot: the host takes even ones, the
//     other odd ones. A player who frees the other's slot sends it over to be freed there.
//   - A player arriving on a level where the other is asks for it and loads it as it is
//     now; one leaving a level sends it as saved, for the other's LEV.ARK.
//
//   - Creatures see the other player by their stand-in (onlinepeer.go).

const (
	syncLen   = 0x7300 // the level block's map and object records
	syncEvery = 50 * time.Millisecond
	wantWait  = 15 * time.Second
)

var (
	syncSnap  []byte // the level block as last sent (nil: none yet for this level)
	syncLevel int    // the level syncSnap is of
	synced    bool   // this player has the world (a joiner not until the host sent it)
	syncAt    time.Time
	wantAt    time.Time // when a level was asked for; synced again after wantWait regardless
	helloSent bool
	freed     []int // the other player's slots freed here, to go with the next diff
	sentSeq   int   // the number of the last diff sent
	seenSeq   int   // the number of the last diff taken from the other
	changedIn []int // per byte of the block, the number of the last diff that changed it
	dumpAt    time.Time
	dumpN     int
)

// onlineTick runs once a main loop pass while online.
func onlineTick() {
	if online == nil || !inGame {
		return
	}
	now := time.Now()
	if !helloSent {
		helloSent = true
		synced = online.Me == 0
		if !synced {
			online.Send(mp.Msg{Op: "hello"})
			wantAt = now
		}
	}
	peersTick(now)
	if PlayerLevel != syncLevel {
		syncLevel = PlayerLevel
		resetSync()
		if synced && peerOnLevel(PlayerLevel) {
			online.Send(mp.Msg{Op: "want", Level: PlayerLevel})
			synced, wantAt = false, now
		}
	}
	if !synced && now.Sub(wantAt) > wantWait {
		synced = true
	}
	if synced && now.Sub(syncAt) >= syncEvery {
		syncAt = now
		cur := syncBlock()
		if syncSnap != nil {
			if d := diffBytes(syncSnap, cur); (len(d) > 0 || freed != nil) && peerOnLevel(PlayerLevel) {
				sentSeq++
				for i := range cur {
					if cur[i] != syncSnap[i] {
						changedIn[i] = sentSeq
					}
				}
				debugDiff(fmt.Sprintf("send %d", sentSeq), d)
				online.Send(mp.Msg{Op: "diff", Level: PlayerLevel, Data: d, Slots: freed, Seq: sentSeq, Ack: seenSeq})
			}
			freed = nil
		}
		syncSnap = cur
	}
	if synced {
		questTick(now)
	}
	for _, m := range online.Events() {
		onlineEvent(m)
	}
	if !convWatching && !convoWorkspace { // conversations open after the events taken with them
		if i := convTalkNpc; i != 0 && scrmode == 1 {
			convTalkNpc = 0
			TalkTo(&Level.Objs[i])
		}
		convWatchMaybe()
	}
	debugWorldDump(now)
}

func onlineEvent(m mp.Msg) {
	switch m.Op {
	case "hello": // a player joined: they take the world
		if online.Me != 0 {
			return
		}
		online.Send(mp.Msg{Op: "questall", Data: questState()})
		for l := 1; l <= 9; l++ {
			msg := mp.Msg{Op: "init", Level: l, Seq: sentSeq}
			if l == PlayerLevel {
				msg.Data, msg.Anim = liveLevelBlock()
				msg.Slots = animOurList()
			} else {
				msg.Data, msg.Anim = savedLevelBlock(l)
			}
			if msg.Data != nil {
				online.Send(msg)
			}
		}
	case "want":
		if m.Level == PlayerLevel && synced {
			data, anim := liveLevelBlock()
			online.Send(mp.Msg{Op: "level", Level: m.Level, Data: data, Anim: anim, Seq: sentSeq, Slots: animOurList()})
		}
	case "init", "level", "saved":
		// init: the host's world for a joiner; level: a level as it is now, asked for;
		// saved: a level the other left, as saved. Only the first two replace the level
		// being played.
		current := m.Level == PlayerLevel
		if current && m.Op == "saved" {
			return
		}
		writeLevelBlock(m.Level, m.Data, m.Anim)
		if current {
			reloadLevel()
			animTheirList(m.Slots)
			seenSeq = m.Seq // the sender's diffs from here on are news
			synced = true
		}
		if m.Op == "init" && m.Level == 9 {
			synced = true
		}
	case "diff":
		if synced && m.Level == PlayerLevel && syncSnap != nil {
			seenSeq = m.Seq
			debugDiff(fmt.Sprintf("take %d ack %d", m.Seq, m.Ack), m.Data)
			applyDiff(m.Data, onlineOwnsLevel(), m.Ack)
			for _, i := range m.Slots { // freed there, after the diff that unlinked them
				if i > 1 && i < NUM_OBJECTS && i%mp.MaxPlayers == online.Me {
					ObjFree(&Level.Objs[i])
				}
			}
		}
	case "animadd", "animrem", "animptr":
		animEvent(m)
	case "quest", "questall":
		questEvent(m)
	case "hurt":
		takeHurt(m)
	case "talkto":
		if len(m.Slots) == 1 && m.Level == PlayerLevel && m.Slots[0] > 1 && m.Slots[0] < NUM_MOBILE {
			convTalkNpc = m.Slots[0]
		}
	case "conv", "convpick":
		convEvent(m)
	case "left":
		editchng(2)
	}
}

// recordRange is where object i's record lies in the level block.
func recordRange(i int) (int, int) {
	if i < NUM_MOBILE {
		return 0x4000 + i*MOBILE_SIZE, MOBILE_SIZE
	}
	return 0x5B00 + (i-NUM_MOBILE)*STATIC_SIZE, STATIC_SIZE
}

// ownObjects marks this player's own objects: the player and everything carried, in
// the pack, in hand or inside those.
func ownObjects() *[NUM_OBJECTS]bool {
	var own [NUM_OBJECTS]bool
	own[1] = true
	own[proxySlot] = true
	var walk func(l Link)
	walk = func(l Link) {
		for i := l.Index(); i > 0 && i < NUM_OBJECTS && !own[i]; {
			own[i] = true
			o := &Level.Objs[i]
			if o.IsQuant() == 0 {
				walk(o.OL)
			}
			i = o.Next()
		}
	}
	walk(ThePlayer.OL)
	if c := objIndexOf(CursorObjPtr); c > 0 {
		own[c] = true
		if CursorObjPtr.IsQuant() == 0 {
			walk(CursorObjPtr.OL)
		}
	}
	return &own
}

// withoutPlayer runs f with the player object (and the other player's stand-in) out of its tile's list, as the level is
// saved. It goes back at the head of the list.
func withoutPlayer(f func()) {
	if GrSq >= 0 {
		ObjRem(&Level.Tiles[GrSq].Objects, ThePlayer)
		defer ObjAdd(&Level.Tiles[GrSq].Objects, ThePlayer)
	}
	unlinkProxy()
	defer linkProxy()
	f()
}

// syncBlock is the level block to compare: this player's own objects keep what was last
// sent, so they never go.
func syncBlock() []byte {
	var b []byte
	withoutPlayer(func() { b = Level.encode()[:syncLen] })
	for i, own := range ownObjects() {
		if own {
			off, n := recordRange(i)
			if syncSnap != nil {
				copy(b[off:off+n], syncSnap[off:off+n])
			} else {
				clear(b[off : off+n])
			}
		}
	}
	return b
}

// diffBytes encodes where b differs from a: runs of (offset from the last run's end,
// length, bytes), runs closer than 8 bytes merged.
func diffBytes(a, b []byte) []byte {
	var out []byte
	last := 0
	for i := 0; i < len(b); {
		if a[i] == b[i] {
			i++
			continue
		}
		j, same := i, 0
		for ; j < len(b) && same < 8; j++ {
			if a[j] == b[j] {
				same++
			} else {
				same = 0
			}
		}
		j -= same
		out = binary.AppendUvarint(out, uint64(i-last))
		out = binary.AppendUvarint(out, uint64(j-i))
		out = append(out, b[i:j]...)
		last, i = j, j
	}
	return out
}

// patchBytes applies diff to b, leaving the bytes for which keep is true alone.
func patchBytes(b, diff []byte, keep func(int) bool) bool {
	at := 0
	for len(diff) > 0 {
		skip, n1 := binary.Uvarint(diff)
		if n1 <= 0 {
			return false
		}
		size, n2 := binary.Uvarint(diff[n1:])
		if n2 <= 0 {
			return false
		}
		diff = diff[n1+n2:]
		at += int(skip)
		if at+int(size) > len(b) || int(size) > len(diff) {
			return false
		}
		for k := 0; k < int(size); k++ {
			if !keep(at + k) {
				b[at+k] = diff[k]
			}
		}
		diff = diff[size:]
		at += int(size)
	}
	return true
}

// applyDiff takes the other player's changes to this level, but not to this player's own
// objects. owner: this player owns the level; ack: the last of this player's diffs the
// other had seen.
func applyDiff(d []byte, owner bool, ack int) {
	own := ownObjects()
	withoutPlayer(func() {
		b := Level.encode()
		// both as they were: keep must decide the same for both patches
		live, snap := bytes.Clone(b), bytes.Clone(syncSnap)
		keep := func(at int) bool {
			if owner && (changedIn[at] > ack || live[at] != snap[at]) {
				dprintf("mp keep %#x: changed in %d, ack %d, live %#x snap %#x\n", at, changedIn[at], ack, live[at], snap[at])
				return true // the owner's word, on its way or still to go
			}
			var i int
			switch {
			case at < 0x4000:
				return false
			case at < 0x5B00:
				i = (at - 0x4000) / MOBILE_SIZE
			default:
				i = NUM_MOBILE + (at-0x5B00)/STATIC_SIZE
			}
			return i < NUM_OBJECTS && own[i]
		}
		if patchBytes(b[:syncLen], d, keep) {
			patchBytes(syncSnap, d, keep)
			Level.decode(b)
		}
	})
	editchng(2)
}

// resetSync starts the level's comparison afresh.
func resetSync() {
	syncSnap = nil
	sentSeq, seenSeq = 0, 0
	changedIn = make([]int, syncLen)
}

// liveLevelBlock is the current level as MapSave would write it, without this player's
// own objects, and its animation list.
func liveLevelBlock() (data, anim []byte) {
	Level.NActive = uint16(nActive)
	Level.NMobFree = uint16(critptr)
	Level.NStatFree = uint16(objptr)
	Level.Magic = LEVEL_MAGIC
	withoutPlayer(func() { data = Level.encode() })
	for i, own := range ownObjects() {
		if own {
			off, n := recordRange(i)
			clear(data[off : off+n])
		}
	}
	return data, animBytes()
}

// savedLevelBlock is level as SAVE0's LEV.ARK has it.
func savedLevelBlock(level int) (data, anim []byte) {
	var arc Arc
	if !openArc(&arc, "SAVE0\\lev.ark") {
		return nil, nil
	}
	defer closeArc(&arc)
	return getArc(&arc, level-1), getArc(&arc, level+8)
}

// writeLevelBlock puts a level into SAVE0's LEV.ARK; for the level being played, its
// automap is saved first, as reloading it reads the automap back.
func writeLevelBlock(level int, data, anim []byte) {
	if len(data) != LEVEL_SIZE {
		return
	}
	var arc Arc
	if !openArc(&arc, "SAVE0\\lev.ark") {
		return
	}
	if level == PlayerLevel {
		SaveAutoMapLevel(&arc, level)
	}
	putArc(&arc, level-1, data)
	if len(anim) == ANIM_BLOCK_LEN {
		putArc(&arc, level+8, anim)
	}
	closeArc(&arc)
}

// reloadLevel loads the level being played again from LEV.ARK, as a level change does;
// the player's inventory goes into this player's free slots.
func reloadLevel() {
	if GameInputMode == 2 && CursorObjPtr != nil {
		GameInputMode = 0
		CursorObjPtr = nil
		unforceMouseCursor(3)
	}
	if GrSq >= 0 {
		ObjRem(&Level.Tiles[GrSq].Objects, ThePlayer)
	}
	GrSq = -1
	GetLevel(PlayerLevel)
	playerChangeSq()
	resetSync()
	realDScheck = true
	editchng(0x7FFE)
}

// onlineLeftLevel sends the level just saved on leaving it.
func onlineLeftLevel(level int) {
	if online == nil {
		return
	}
	data, anim := savedLevelBlock(level)
	if data != nil {
		online.Send(mp.Msg{Op: "saved", Level: level, Data: data, Anim: anim})
	}
}

// onlineOwnsLevel says whether this player moves the level's creatures.
func onlineOwnsLevel() bool {
	if online == nil {
		return true
	}
	for n, p := range peers {
		if p.Level == PlayerLevel && n < online.Me {
			return false
		}
	}
	return true
}

// onlineMoves says whether this player moves the active mobile object in slot i.
func onlineMoves(o *Object, i int) bool {
	if online == nil {
		return true
	}
	if o.Major() == MAJOR_CREATURE {
		return onlineOwnsLevel()
	}
	return i%mp.MaxPlayers == online.Me || !peerOnLevel(PlayerLevel)
}

// onlineTakeFree moves a free slot of this player's to the top of a free stack; false if
// there is none.
func onlineTakeFree(stack []uint16, top int) bool {
	if online == nil {
		return true
	}
	for k := top; k >= 0; k-- {
		if int(stack[k])%mp.MaxPlayers == online.Me {
			stack[k], stack[top] = stack[top], stack[k]
			return true
		}
	}
	return false
}

// onlineFreeElsewhere keeps the other player's slot i, freed here, to be freed there
// with the next diff; true if it was kept.
func onlineFreeElsewhere(i int) bool {
	if online == nil || i%mp.MaxPlayers == online.Me || !peerOnLevel(PlayerLevel) {
		return false
	}
	freed = append(freed, i)
	return true
}

// debugWorldDump, for debugging: with UW1GO_MPDUMP set, writes every tile's objects
// (item, place, quality) there every two seconds, so that two players' worlds can be
// compared.
func debugWorldDump(now time.Time) {
	name := os.Getenv("UW1GO_MPDUMP")
	if name == "" || now.Sub(dumpAt) < 2*time.Second {
		return
	}
	dumpAt = now
	var b strings.Builder
	fmt.Fprintf(&b, "level %d player %d,%d\n", PlayerLevel, PN.X>>8, PN.Y>>8)
	for t := range Level.Tiles {
		l := Level.Tiles[t].Objects
		line := ""
		for i, n := l.Index(), 0; i > 0 && n < 100; n++ {
			o := &Level.Objs[i]
			if i != 1 {
				line += fmt.Sprintf(" %d:%#x@%d,%d,%d q%d", i, o.Item(), o.FineX(), o.FineY(), o.Z(), o.Quality())
			}
			i = o.Next()
		}
		if line != "" {
			fmt.Fprintf(&b, "%d,%d:%s\n", t%MAP_SIZE, t/MAP_SIZE, line)
		}
	}
	os.WriteFile(name, []byte(b.String()), 0o644)
	dumpN++
	os.WriteFile(fmt.Sprintf("%s.%03d", name, dumpN), []byte(b.String()), 0o644)
}

// debugDiff logs a diff's runs.
func debugDiff(what string, d []byte) {
	if os.Getenv("UW1GO_DEBUG") == "" {
		return
	}
	at := 0
	for len(d) > 0 {
		skip, n1 := binary.Uvarint(d)
		size, n2 := binary.Uvarint(d[n1:])
		at += int(skip)
		dprintf("mp %s %#x+%d % x\n", what, at, size, d[n1+n2:n1+n2+int(size)])
		d = d[n1+n2+int(size):]
		at += int(size)
	}
}
