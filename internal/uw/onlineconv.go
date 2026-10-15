package uw

import (
	"strings"

	"github.com/memmaker/uw1go/internal/mp"
)

// Conversations open for both players. The one who started it runs the conversation; what
// it prints goes to the other too, whose conversation screen shows the same NPC and the
// same lines and menus without running anything. Either may pick from a menu: the other's
// pick is sent to be taken as if the one talking had made it.

// What a "conv" message carries in Slots[0].
const (
	convStart      = iota // Slots[1]: the NPC's slot
	convNpcScroll         // Text: printed in the conversation scroll
	convPlayScroll        // Text: printed in the player's scroll
	convMenu              // Text: the menu's options, a line each
	convPicked            // the menu was answered
	convEnd
)

var (
	convWatching bool     // this player watches the other's conversation
	convWatchNpc int      // a conversation to watch, to start at the end of onlineTick
	convQueue    []mp.Msg // what came for the conversation being watched
	convTalkNpc  int      // an NPC who came up to talk to this player in the other's world
)

// convMirror sends a step of this player's conversation to the other.
func convMirror(kind int, text string) {
	if online == nil || convWatching {
		return
	}
	slots := []int{kind}
	if kind == convStart {
		slots = append(slots, ObjMemTPtr(talkingTo))
	}
	online.Send(mp.Msg{Op: "conv", Level: PlayerLevel, Slots: slots, Text: text})
}

// convMirrorMenu sends the menu being shown.
func convMirrorMenu() {
	if online == nil || convWatching {
		return
	}
	convMirror(convMenu, strings.Join(bablDisplay[1:bablOptCount], "\n"))
}

// convEvent takes a "conv" or "convpick" message.
func convEvent(m mp.Msg) {
	if len(m.Slots) == 0 {
		return
	}
	switch {
	case m.Op == "convpick":
		if !convWatching && convoMode == 1 && convoWorkspace {
			convPlayMenu(m.Slots[0])
		}
	case m.Slots[0] == convStart:
		if len(m.Slots) == 2 && m.Level == PlayerLevel && scrmode == 1 && !convoWorkspace &&
			m.Slots[1] > 1 && m.Slots[1] < NUM_MOBILE {
			convWatchNpc = m.Slots[1]
			convQueue = nil
		}
	case convWatching || convWatchNpc != 0:
		convQueue = append(convQueue, m)
	}
}

// convWatchMaybe opens the conversation screen to watch the other's conversation.
func convWatchMaybe() {
	if convWatchNpc == 0 || convWatching {
		return
	}
	npc := &Level.Objs[convWatchNpc]
	convWatchNpc = 0
	if npc.Major() != MAJOR_CREATURE {
		return
	}
	talkingTo = npc
	convWatching = true
	newscr(4)
}

// watchConverse is the watcher's Converse: show what comes until the conversation ends
// or the other player goes.
func watchConverse() {
	for convWatching {
		loopMusicMaybe()
		if changed != 0 {
			doChanges()
		}
		onlineTick()
		for len(convQueue) > 0 && convWatching {
			m := convQueue[0]
			convQueue = convQueue[1:]
			watchStep(m)
		}
		if online == nil || !peerOnLevel(PlayerLevel) {
			convWatching = false
			break
		}
		inputDispatch(inplist)
	}
	convQueue = nil
	convoMode = 0
	scrollWait(0x1F4, false)
}

func watchStep(m mp.Msg) {
	switch m.Slots[0] {
	case convNpcScroll:
		doNpcScroll()
		scrollPrint(m.Text)
		doMainScroll()
	case convPlayScroll:
		doPlayScroll()
		scrollPrint(m.Text)
		doMainScroll()
	case convMenu:
		bablOptCount = 1
		for _, s := range strings.Split(m.Text, "\n") {
			if bablOptCount < len(bablDisplay) {
				bablDisplay[bablOptCount] = s
				bablOptCount++
			}
		}
		doPlayScroll()
		scrollClear(true)
		for i := range convoLineOption {
			convoLineOption[i] = -1
		}
		used := 0
		for i := 1; i < bablOptCount; i++ {
			lines := scrollPrint(string(rune('0'+i)) + ". " + bablDisplay[i] + "\n")
			for row := used; row <= lines; row++ {
				if row >= 0 && row < len(convoLineOption) {
					convoLineOption[row] = i
				}
			}
			used = lines + 1
		}
		doMainScroll()
		convoMode = 1
	case convPicked:
		convoMode = 0
		doPlayScroll()
		scrollClear(true)
		doMainScroll()
	case convEnd:
		convWatching = false
	}
}

// watchPick sends the watcher's pick to the one talking.
func watchPick(selected int) {
	convoMode = 0
	mouseRelease(false)
	online.Send(mp.Msg{Op: "convpick", Slots: []int{selected}})
}
