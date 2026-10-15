package uw

import (
	"time"

	"github.com/memmaker/uw1go/internal/mp"
)

// The other player: where they are goes out 20 times a second while it changes (and once
// a second regardless), and they are drawn as the adventurer (a creature the game has
// pictures for but never places), standing or walking in eight directions.

const (
	peerItem     = ITEM_ADVENTURER
	posEvery     = 50 * time.Millisecond
	posKeepAlive = time.Second
	peerStepTime = 150 * time.Millisecond // a walking step of the adventurer's four
	peerNear     = 0x30                   // closer than this (1/256 tiles) is not drawn
)

type peerState struct {
	mp.Pos
	step   int
	stepAt time.Time
}

var (
	peers    = map[int]*peerState{}
	lastPos  mp.Pos
	lastPosT time.Time
)

// peersTick sends this player's place and takes the others'; the view is redrawn when
// one of them moved or stepped.
func peersTick(now time.Time) {
	p := mp.Pos{Level: PlayerLevel, X: PN.X, Y: PN.Y, Z: PN.Z, Heading: PlayerFacing, HP: ThePlayer.HP}
	p.Moving = p.X != lastPos.X || p.Y != lastPos.Y
	if (p != lastPos && now.Sub(lastPosT) >= posEvery) || now.Sub(lastPosT) >= posKeepAlive {
		online.Send(mp.Msg{Op: "pos", Pos: &p})
		lastPos, lastPosT = p, now
	}
	others := online.Others()
	redraw := false
	for n := range peers {
		if _, ok := others[n]; !ok {
			delete(peers, n)
			redraw = true
		}
	}
	for n, o := range others {
		ps := peers[n]
		if ps == nil {
			ps = &peerState{}
			peers[n] = ps
		}
		if ps.Pos != o {
			ps.Pos = o
			redraw = true
		}
		if o.Moving && now.Sub(ps.stepAt) >= peerStepTime {
			ps.step = (ps.step + 1) % 4
			ps.stepAt = now
			redraw = true
		} else if !o.Moving && ps.step != 0 {
			ps.step = 0
			redraw = true
		}
	}
	placeProxy()
	if redraw {
		editchng(2)
	}
}

// The other player in this player's world: a creature record of the player's own type,
// in a mobile slot of this player's, linked into the tile where the other stands but not
// active (nothing moves it but placeProxy) and never sent (onlinesync.go leaves it out
// like the player). It is what creatures take aim at, what their blows and missiles meet
// and what bars the way; the adventurer drawn there is drawPeers'. What hurts it is sent
// to the other player to be suffered there.

var (
	proxySlot  int // 0: none
	proxySq    int
	hurtPoison int // the poison of the creature's blow being dealt
)

// isProxy says whether slot i is the other player.
func isProxy(i int) bool { return proxySlot != 0 && i == proxySlot }

// placeProxy keeps the other player's stand-in where they are, while they are on this
// level.
func placeProxy() {
	var p *peerState
	for _, ps := range peers {
		if ps.Level == PlayerLevel && ps.HP > 0 {
			p = ps
		}
	}
	if p == nil || GrSq < 0 {
		removeProxy()
		return
	}
	if proxySlot == 0 {
		if critptr < 0 || !onlineTakeFree(Level.MobFree[:], critptr) {
			return
		}
		proxySlot = int(Level.MobFree[critptr])
		critptr--
		o := &Level.Objs[proxySlot]
		*o = Object{}
		o.ID = ThePlayer.ID
		proxySq = -1
	}
	o := &Level.Objs[proxySlot]
	x, y := int(p.X>>8)&MAP_MASK, int(p.Y>>8)&MAP_MASK
	if sq := x + y<<6; sq != proxySq {
		if proxySq >= 0 {
			ObjRem(&Level.Tiles[proxySq].Objects, o)
		}
		proxySq = sq
		ObjAdd(&Level.Tiles[sq].Objects, o)
	}
	o.SetHomeX(x)
	o.SetHomeY(y)
	o.SetFineX(int(p.X>>5) & 7)
	o.SetFineY(int(p.Y>>5) & 7)
	o.SetZ(int(p.Z >> 3))
	h := uint16(p.Heading)
	o.SetHeading(int(h >> 13))
	o.SetFineHead(int(h>>8) & 0x1F)
	o.Heading = byte(h >> 8)
	o.HP = p.HP
}

// removeProxy takes the stand-in out of the level, as before the level is saved.
func removeProxy() {
	if proxySlot == 0 {
		return
	}
	unlinkProxy()
	critptr++
	Level.MobFree[critptr] = uint16(proxySlot)
	proxySlot = 0
}

// unlinkProxy takes the stand-in out of its tile; linkProxy puts it back.
func unlinkProxy() {
	if proxySlot != 0 && proxySq >= 0 {
		ObjRem(&Level.Tiles[proxySq].Objects, &Level.Objs[proxySlot])
	}
}

func linkProxy() {
	if proxySlot != 0 && proxySq >= 0 {
		ObjAdd(&Level.Tiles[proxySq].Objects, &Level.Objs[proxySlot])
	}
}

// forgetProxy drops the stand-in when the level it was in is replaced.
func forgetProxy() { proxySlot = 0 }

// targetPlayer is the player a creature goes for: the nearer, or for one that fights
// from afar the weaker. meptr is the creature.
func targetPlayer() *Object {
	if proxySlot == 0 {
		return ThePlayer
	}
	other := &Level.Objs[proxySlot]
	if ThePlayer.HP == 0 {
		return other
	}
	if other.HP == 0 {
		return ThePlayer
	}
	cr := &Creatures[meptr.InMajor()]
	if cr.Caster() > 0 || cr.ArmsItem(0)>>4 == CLASS_MISSILE {
		if other.HP != ThePlayer.HP {
			if other.HP < ThePlayer.HP {
				return other
			}
			return ThePlayer
		}
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

// hurtProxy sends the other player the damage a creature's blow or missile did to them.
func hurtProxy(damage, typ int) {
	dprintf("mp hurt the other: %d (type %d) by %d\n", damage, typ, fromwho)
	online.Send(mp.Msg{Op: "hurt", Level: PlayerLevel, Slots: []int{damage, typ, hurtPoison}})
}

// takeHurt suffers what a creature did to this player in the other player's world.
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

// peerOnLevel says whether another player is on level.
func peerOnLevel(level int) bool {
	for _, p := range peers {
		if p.Level == level {
			return true
		}
	}
	return false
}

// drawPeers draws the other players standing on the tile at tmptr, after its objects.
func drawPeers() {
	if online == nil || PickUp != 0 {
		return
	}
	for _, p := range peers {
		if p.Level != PlayerLevel || int(p.X>>8)+int(p.Y>>8)*MAP_SIZE != tmptr {
			continue
		}
		if iabs(int(p.X-PN.X)) < peerNear && iabs(int(p.Y-PN.Y)) < peerNear {
			continue // standing in each other: the eye would be inside the picture
		}
		fx, fy := int(p.X&0xFF), int(p.Y&0xFF)
		var x, y int
		switch quad { // the tile's place turned into the first quadrant, as doObj turns mobiles'
		case 0:
			x, y = fx, fy
		case 1:
			x, y = 0xFF-fy, fx
		case 2:
			x, y = 0xFF-fx, 0xFF-fy
		case 3:
			x, y = fy, 0xFF-fx
		}
		objxloc = (loopx-16)<<8 + x
		objzloc = loopy<<8 + y
		objyloc = int(p.Z)
		// as doObj picks a critter's frame: the sequence's picture for the direction it
		// faces from the eye, and the step of the walk
		dir := int(dirtab[(int(uint16(p.Heading))>>11+0x20-int((uint16(cPlayer.heading)+headmod[quad])>>11))%0x20])
		frame, step := SEQ_STAND+dir, 0
		if p.Moving {
			frame, step = (SEQ_WALK-0x20)<<3+dir+0x20, p.step
		}
		if pic, ok := critterPic(peerItem&0x3F, int(sqmod)*lighton, frame, step); ok {
			rend.CritterSprite(objPoint(), pic)
		}
	}
}

// proxyTalkTo has the other player talked to by the creature in slot i, which came up to
// them.
func proxyTalkTo(i int) {
	online.Send(mp.Msg{Op: "talkto", Level: PlayerLevel, Slots: []int{i}})
}
