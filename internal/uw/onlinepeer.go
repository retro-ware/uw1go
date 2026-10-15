package uw

import (
	"time"

	"github.com/memmaker/uw1go/internal/mp"
)

// The players in each other's world. Each player has an avatar on the level it is on: an
// adventurer (a creature the game has pictures for but never places), in a mobile slot of
// its own, standing where the player stands, facing their way, with their hit points,
// standing or walking. It is an object of the level like any other, so the level's sync
// takes it to the other player, where it is drawn, bars the way, is what creatures go for
// and what their blows and missiles meet; what hurts it there is sent to its player to be
// suffered. In its own player's world it is left alone: the player object is the player
// there. It is never active (no AI moves it), and it leaves with its player.
//
// Where each player is, and their avatar's slot, also goes out 20 times a second while it
// changes (and once a second regardless): it says who is on which level.

const (
	posEvery     = 50 * time.Millisecond
	posKeepAlive = time.Second
	avatarStep   = 150 * time.Millisecond // a walking step of the adventurer's four
)

type peerState struct {
	mp.Pos
	gone int // their avatar's slot on this level, to take away once they left it
}

var (
	peers    = map[int]*peerState{}
	lastPos  mp.Pos
	lastPosT time.Time

	avatarSlot int // this player's avatar; 0 none
	avatarSq   int
	avatarAt   time.Time // its last walking step
	avatarHold time.Time // until then, not placed again before the other's is here
	hurtPoison int       // the poison of the creature's blow being dealt
)

// peersTick sends this player's place, takes the others' and keeps the avatars.
func peersTick(now time.Time) {
	placeAvatar(now)
	p := mp.Pos{Level: PlayerLevel, X: PN.X, Y: PN.Y, Z: PN.Z, Heading: PlayerFacing, HP: ThePlayer.HP, Avatar: avatarSlot}
	p.Moving = p.X != lastPos.X || p.Y != lastPos.Y
	if (p != lastPos && now.Sub(lastPosT) >= posEvery) || now.Sub(lastPosT) >= posKeepAlive {
		online.Send(mp.Msg{Op: "pos", Pos: &p})
		lastPos, lastPosT = p, now
	}
	others := online.Others()
	for n, ps := range peers {
		if _, ok := others[n]; !ok {
			dropAvatar(ps.gone)
			delete(peers, n)
		}
	}
	for n, o := range others {
		ps := peers[n]
		if ps == nil {
			ps = &peerState{}
			peers[n] = ps
		}
		ps.Pos = o
		here := 0
		if o.Level == PlayerLevel {
			here = o.Avatar
		}
		if ps.gone != here {
			dropAvatar(ps.gone)
			ps.gone = here
		}
		if isPeerAvatar(ps.gone) && Level.Objs[ps.gone].Item() == ITEM_ADVENTURER {
			unfree(ps.gone)
		}
	}
}

// placeAvatar keeps this player's avatar where the player is.
func placeAvatar(now time.Time) {
	if GrSq < 0 || !synced || syncSnap == nil { // made after the first comparison, so that it is sent
		return
	}
	if avatarSlot == 0 {
		if now.Before(avatarHold) && !peerAvatarHere() {
			return
		}
		avatarHold = time.Time{}
		if critptr < 0 || !onlineTakeFree(Level.MobFree[:], critptr) {
			return
		}
		avatarSlot = int(Level.MobFree[critptr])
		critptr--
		unfree(avatarSlot)
		o := &Level.Objs[avatarSlot]
		*o = Object{}
		o.SetItem(ITEM_ADVENTURER)
		o.SetPitch(0x10)
		avatarSq = -1
	}
	o := &Level.Objs[avatarSlot]
	if GrSq != avatarSq {
		dprintf("mp avatar %d: %d -> %d (player %d,%d)\n", avatarSlot, avatarSq, GrSq, PN.X>>8, PN.Y>>8)
		if avatarSq >= 0 {
			ObjRem(&Level.Tiles[avatarSq].Objects, o)
		}
		avatarSq = GrSq
		ObjAdd(&Level.Tiles[avatarSq].Objects, o)
	}
	o.SetHomeX(ThePlayer.HomeX())
	o.SetHomeY(ThePlayer.HomeY())
	o.SetFineX(ThePlayer.FineX())
	o.SetFineY(ThePlayer.FineY())
	o.SetZ(ThePlayer.Z())
	h := uint16(PlayerFacing)
	o.SetHeading(int(h >> 13))
	o.SetFineHead(int(h>>8) & 0x1F)
	o.Heading = byte(h >> 8)
	o.HP = ThePlayer.HP
	switch {
	case PN.X == lastPos.X && PN.Y == lastPos.Y:
		o.SetSeq(SEQ_STAND)
		o.SetFrame(0)
	case o.Seq() != SEQ_WALK:
		o.SetSeq(SEQ_WALK)
		o.SetFrame(0)
		avatarAt = now
	case now.Sub(avatarAt) >= avatarStep:
		o.SetFrame((o.Frame() + 1) % 4)
		avatarAt = now
	}
}

// avatarObj is this player's avatar, or nil.
func avatarObj() *Object {
	if avatarSlot == 0 {
		return nil
	}
	return &Level.Objs[avatarSlot]
}

// removeAvatar takes this player's avatar off the level, as the player leaves it.
func removeAvatar() {
	if avatarSlot == 0 {
		return
	}
	if avatarSq >= 0 {
		ObjRem(&Level.Tiles[avatarSq].Objects, &Level.Objs[avatarSlot])
	}
	if !isFree(avatarSlot) {
		ObjFree(&Level.Objs[avatarSlot])
	}
	Level.Objs[avatarSlot] = Object{} // free: no avatar there (dropStaleAvatars)
	avatarSlot = 0
}

// forgetAvatar: the level the avatar was on is being replaced.
func forgetAvatar() { avatarSlot = 0 }

// isPeerAvatar says whether slot i is another player's avatar on this level.
func isPeerAvatar(i int) bool {
	if i <= 1 || i == avatarSlot {
		return false
	}
	for _, p := range peers {
		if p.gone == i {
			return true
		}
	}
	return false
}

// dropAvatar takes away the avatar in slot i that a player left behind on this level
// (they took it away in their own world; the slot stays theirs to take again).
func dropAvatar(i int) {
	if i <= 1 || i >= NUM_MOBILE || i == avatarSlot || i%mp.MaxPlayers == onlineMe() {
		return
	}
	o := &Level.Objs[i]
	if o.Item() != ITEM_ADVENTURER {
		return
	}
	unlinkAnywhere(i)
	*o = Object{}
	editchng(2)
}

func onlineMe() int {
	if online == nil {
		return -1
	}
	return online.Me
}

// dropPeerAvatars takes the other players' avatars out of this level, as it is saved:
// they are where their players are, not in a level kept for later.
func dropPeerAvatars() {
	for _, p := range peers {
		dropAvatar(p.gone)
		p.gone = 0
	}
}

// peerOnLevel says whether another player is on level.
func peerOnLevel(level int) bool {
	for _, p := range peers {
		if p.Level == level {
			return true
		}
	}
	return false
}

// peerAvatar is another player's avatar on this level that is alive, or nil.
func peerAvatar() *Object {
	for _, p := range peers {
		if p.gone != 0 {
			if o := &Level.Objs[p.gone]; o.Item() == ITEM_ADVENTURER && o.HP > 0 {
				return o
			}
		}
	}
	return nil
}

// targetPlayer is the player a creature goes for: the nearer, or for one that fights
// from afar the weaker. meptr is the creature.
func targetPlayer() *Object {
	other := peerAvatar()
	if other == nil {
		return ThePlayer
	}
	if ThePlayer.HP == 0 {
		return other
	}
	cr := &Creatures[meptr.InMajor()]
	if (cr.Caster() > 0 || cr.ArmsItem(0)>>4 == CLASS_MISSILE) && other.HP != ThePlayer.HP {
		if other.HP < ThePlayer.HP {
			return other
		}
		return ThePlayer
	}
	if distTo(meptr, other) < distTo(meptr, ThePlayer) {
		return other
	}
	return ThePlayer
}

// distTo is the square of the distance from a to b in eighths of tiles.
func distTo(a, b *Object) int {
	dx := (b.HomeX()<<3 + b.FineX()) - (a.HomeX()<<3 + a.FineX())
	dy := (b.HomeY()<<3 + b.FineY()) - (a.HomeY()<<3 + a.FineY())
	return dx*dx + dy*dy
}

// collideSkip says whether the collision check of the object moving (index, 0 for one
// being placed) passes object i: this player's avatar always (the player object is the
// player here), and for this player and what it drops another's while this player
// stands inside it already, so that two players who start in the same place can walk
// apart.
func collideSkip(index, i int) bool {
	if i == avatarSlot && avatarSlot != 0 {
		return true
	}
	if index > 1 || !isPeerAvatar(i) {
		return false
	}
	r := 2 * ComObjData[ITEM_ADVENTURER].Radius()
	return distTo(ThePlayer, &Level.Objs[i]) < r*r
}

// hurtPeer sends the other player the damage a creature's blow or missile did to their
// avatar.
func hurtPeer(damage, typ int) {
	dprintf("mp hurt the other: %d (type %d) by %d\n", damage, typ, fromwho)
	online.Send(mp.Msg{Op: "hurt", Level: PlayerLevel, Slots: []int{damage, typ, hurtPoison}})
}

// takeHurt suffers what a creature did to this player's avatar in the other's world.
func takeHurt(m mp.Msg) {
	if len(m.Slots) < 3 || m.Level != PlayerLevel || ThePlayer.HP == 0 {
		return
	}
	damage, typ, poison := m.Slots[0], m.Slots[1], m.Slots[2]
	dprintf("mp hurt: %d (type %d), hp %d\n", damage, typ, ThePlayer.HP)
	playEffectHere(3, 0x40, damage<<2)
	if player.Easy != 0 {
		damage >>= 1
	}
	damageItem(ThePlayer, nil, -1, -1, damage, typ)
	if poison > player.Poison() && checkRes(ThePlayer, 1, DMG_POISON) != 0 {
		player.SetPoison(poison)
	}
	setNewMusic(MUSIC_COMBAT)
	lastcombattime = GameTime()
}

// peerTalkTo has the other player talked to by the creature in slot i, which came up to
// their avatar.
func peerTalkTo(i int) {
	online.Send(mp.Msg{Op: "talkto", Level: PlayerLevel, Slots: []int{i}})
}

// linkedIn says whether object i is in the list at head.
func linkedIn(head *Link, i int) bool {
	for j, n := head.Index(), 0; j > 0 && j < NUM_OBJECTS && n < 0x400; n++ {
		if j == i {
			return true
		}
		j = Level.Objs[j].Next()
	}
	return false
}

// unlinkAnywhere takes object i out of its square's list: the one it says it is on, or,
// where a diff moved it without its list, whichever has it.
func unlinkAnywhere(i int) {
	o := &Level.Objs[i]
	if head := &Level.Tiles[o.HomeX()+o.HomeY()<<6].Objects; linkedIn(head, i) {
		ObjRem(head, o)
		return
	}
	for t := range Level.Tiles {
		if head := &Level.Tiles[t].Objects; linkedIn(head, i) {
			ObjRem(head, o)
			return
		}
	}
}

// avatarAgain takes this player's avatar off the level just sent, and starts the
// comparison anew, so that the avatar is placed again and goes with the next diff.
//
// The other player's avatar goes too: they load the level without it and place it
// again. Theirs is placed first, so that the two are not linked into a square at once.
func avatarAgain() {
	removeAvatar()
	dropPeerAvatars()
	syncSnap = nil
	avatarHold = time.Now().Add(avatarHoldMax)
}

// avatarHoldMax is how long avatarAgain waits for the other player's avatar at most.
const avatarHoldMax = 3 * time.Second

// peerAvatarHere says whether another player's avatar is linked into its square.
func peerAvatarHere() bool {
	for _, p := range peers {
		if p.gone > 1 && p.gone < NUM_MOBILE {
			o := &Level.Objs[p.gone]
			if o.Item() == ITEM_ADVENTURER && linkedIn(&Level.Tiles[o.HomeX()+o.HomeY()<<6].Objects, p.gone) {
				return true
			}
		}
	}
	return false
}

// dropStaleAvatars clears the avatars a level was loaded with: levels are kept and sent
// without them in their squares (a saved game keeps their records), so any is left over,
// and its player places theirs again. A slot of this player's goes back on the free
// stack; the other player's stays theirs to take again.
func dropStaleAvatars() {
	if online == nil {
		return
	}
	for i := 2; i < NUM_MOBILE; i++ {
		o := &Level.Objs[i]
		if o.Item() != ITEM_ADVENTURER {
			continue
		}
		unlinkAnywhere(i)
		if i%mp.MaxPlayers == online.Me && !isFree(i) {
			ObjFree(o)
		}
		*o = Object{}
	}
	for _, p := range peers {
		p.gone = 0
	}
}

// isFree says whether mobile slot i is on the free stack.
func isFree(i int) bool {
	for k := critptr; k >= 0; k-- {
		if int(Level.MobFree[k]) == i {
			return true
		}
	}
	return false
}

// unfree takes mobile slot i, an avatar in use, off the free stack: a level saved or
// sent while it was freed can have it there still, to be handed out twice.
func unfree(i int) {
	for k := critptr; k >= 0; k-- {
		if int(Level.MobFree[k]) == i {
			copy(Level.MobFree[k:critptr], Level.MobFree[k+1:critptr+1])
			critptr--
		}
	}
}
