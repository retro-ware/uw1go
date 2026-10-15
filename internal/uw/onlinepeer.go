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
	p := mp.Pos{Level: PlayerLevel, X: PN.X, Y: PN.Y, Z: PN.Z, Heading: PlayerFacing}
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
	if redraw {
		editchng(2)
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
