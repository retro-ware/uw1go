package uw

import "github.com/memmaker/uw1go/internal/mp"

// Animations online: both players keep the same animation list. An animation a player
// starts (a door swinging, a spark, a splash) is run by that player, and the other gets it
// in its list to know of it, though it only sees what it does through the objects it
// changes; those that came with the level are run by the level's owner. When the player
// who runs one leaves the level, the one there runs it on. Starting, ending and moving an
// animation to another object go over as they happen; a player who loads the level as
// the other has it learns which of the list's animations the other runs.

const (
	animLevel  = iota // came with the level: its owner's
	animOurs          // this player's
	animTheirs        // the other player's
)

// animRunsHere says whether this player steps animation i.
func animRunsHere(i int) bool {
	switch animWho[i] {
	case animOurs:
		return true
	case animTheirs:
		if online != nil && peerOnLevel(PlayerLevel) {
			return false
		}
		animWho[i] = animLevel // the other left it to us
	}
	return onlineOwnsLevel()
}

// animDrop removes animation n from the list.
func animDrop(n int) {
	if animWho[n] == animOurs && animSending() {
		online.Send(mp.Msg{Op: "animrem", Level: PlayerLevel, Slots: []int{animlist[n].link.Index()}})
	}
	if animcount--; animcount > 0 && animcount != n {
		animlist[n] = animlist[animcount]
		animWho[n] = animWho[animcount]
	}
}

func animSending() bool { return online != nil && !animTaking && peerOnLevel(PlayerLevel) }

var animTaking bool // taking the other player's change: not to be sent back

// animSendAdded tells the other player of animation n, just started.
func animSendAdded(n int) {
	if animSending() {
		e := &animlist[n]
		online.Send(mp.Msg{Op: "animadd", Level: PlayerLevel, Slots: []int{e.link.Index(), int(e.len), int(e.x), int(e.y)}})
	}
}

// animSendMoved tells the other player that animation n is now object to's.
func animSendMoved(n, from, to int) {
	if animWho[n] == animOurs && animSending() {
		online.Send(mp.Msg{Op: "animptr", Level: PlayerLevel, Slots: []int{from, to}})
	}
}

// animOurList is the positions in the list of the animations this player runs, for a
// level sent as it is now.
func animOurList() []int {
	var l []int
	for i := 0; i < animcount; i++ {
		if animWho[i] == animOurs {
			l = append(l, i)
		}
	}
	return l
}

// animTheirList marks the animations the sender of the level just loaded runs.
func animTheirList(l []int) {
	for _, i := range l {
		if i >= 0 && i < animcount {
			animWho[i] = animTheirs
		}
	}
}

// animEvent takes the other player's start, end or move of an animation.
func animEvent(m mp.Msg) {
	if m.Level != PlayerLevel || !synced {
		return
	}
	find := func(index int) int {
		for i := 0; i < animcount; i++ {
			if animlist[i].link.Index() == index {
				return i
			}
		}
		return -1
	}
	animTaking = true
	defer func() { animTaking = false }()
	switch s := m.Slots; m.Op {
	case "animadd":
		if len(s) == 4 && find(s[0]) < 0 && animcount < MAX_ANIMS {
			e := &animlist[animcount]
			e.link = 0
			e.link.SetIndex(s[0])
			e.len = int16(s[1])
			e.x, e.y = byte(s[2]), byte(s[3])
			animWho[animcount] = animTheirs
			animcount++
		}
	case "animrem":
		if len(s) == 1 {
			if i := find(s[0]); i >= 0 {
				animDrop(i)
			}
		}
	case "animptr":
		if len(s) == 2 {
			if i := find(s[0]); i >= 0 {
				animlist[i].link.SetIndex(s[1])
			}
		}
	}
}
