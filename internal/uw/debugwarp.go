package uw

import (
	"fmt"
	"os"
)

var debugWarpDone bool

// debugWarp, for debugging, teleports the player once to UW1GO_WARP="x,y" (a square of
// this level) once the game is running.
func debugWarp() {
	if debugWarpDone {
		return
	}
	debugWarpDone = true
	var x, y int
	if v := os.Getenv("UW1GO_WARP"); v != "" {
		if n, _ := fmt.Sscanf(v, "%d,%d", &x, &y); n == 2 {
			r := doTeleport(ThePlayer, x, y, 0)
			dprintf("WARP %d,%d -> %d (%d,%d)\n", x, y, r, NewPlayerX, NewPlayerY)
			editchng(8) // new_player_pos
		}
	}
}

var debugTalkCount int

// debugTalk, for debugging: with UW1GO_TALK set, talks to the nearest active critter a
// while after the game starts.
func debugTalk() {
	if os.Getenv("UW1GO_TALK") == "" || debugTalkCount < 0 {
		return
	}
	debugTalkCount++
	if debugTalkCount < 300 {
		return
	}
	debugTalkCount = -1
	var best *Object
	bd := 1 << 30
	for p := 1; p < nActive; p++ {
		npc := ObjIntTMem(int(Level.Active[p]))
		if npc == nil || npc == ThePlayer {
			continue
		}
		dx, dy := npc.HomeX()-ThePlayer.HomeX(), npc.HomeY()-ThePlayer.HomeY()
		if d := dx*dx + dy*dy; d < bd {
			bd, best = d, npc
		}
	}
	if best != nil {
		dprintf("TALK to %d whoami %d\n", ObjMemTPtr(best), best.Whoami)
		TalkTo(best)
	}
}
