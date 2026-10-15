package uw

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/memmaker/uw1go/internal/mp"
)

// The story is shared too: the quest flags and bytes, the talismans, the game variables
// the triggers and conversations keep, the story's flags in the player record (orb, key,
// cup, talismans' ok, Garamon, maze) and every conversation's own variables
// (SAVE0\bglobals.dat). Four times a second each player compares them with what was last
// sent and sends the bytes that changed, as the level is sent; the other takes them. A
// joiner takes the host's at the start.

const questEvery = 250 * time.Millisecond

const bglobalsFile = "SAVE0\\bglobals.dat"

var (
	questSnap []byte
	questAt   time.Time
)

// questState is the shared story as bytes: the player record's part, then bglobals.dat.
func questState() []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, player.Quests)
	b.Write(player.QuestBytes[:])
	b.WriteByte(player.Talismans)
	b.Write(player.GameVars[:])
	for _, v := range []int{player.Orb(), player.Key(), player.Cup(), player.TalismanOK(), player.Garamon(), player.Maze()} {
		b.WriteByte(byte(v))
	}
	b.Write(readFile(bglobalsFile))
	return b.Bytes()
}

const questHead = 4 + 4 + 1 + 0x40 + 6

// setQuestState takes the shared story from b, as questState gives it.
func setQuestState(b []byte) {
	if len(b) < questHead {
		return
	}
	player.Quests = int32(binary.LittleEndian.Uint32(b))
	copy(player.QuestBytes[:], b[4:8])
	player.Talismans = b[8]
	copy(player.GameVars[:], b[9:9+0x40])
	f := b[9+0x40:]
	player.SetOrb(int(f[0]))
	player.SetKey(int(f[1]))
	player.SetCup(int(f[2]))
	player.SetTalismanOK(int(f[3]))
	player.SetGaramon(int(f[4]))
	player.SetMaze(int(f[5]))
	if g := b[questHead:]; len(g) > 0 && !bytes.Equal(g, readFile(bglobalsFile)) {
		if h := dosOpen(bglobalsFile, O_WRONLY|O_CREAT|O_TRUNC); h >= 0 {
			dosWrite(h, g)
			dosClose(h)
		}
	}
}

// questTick sends what changed in the story since last time.
func questTick(now time.Time) {
	if now.Sub(questAt) < questEvery {
		return
	}
	questAt = now
	cur := questState()
	if questSnap != nil && len(cur) == len(questSnap) {
		if d := diffBytes(questSnap, cur); len(d) > 0 {
			online.Send(mp.Msg{Op: "quest", Data: d})
		}
	} else if questSnap != nil {
		online.Send(mp.Msg{Op: "questall", Data: cur})
	}
	questSnap = cur
}

// questEvent takes the other player's story: all of it (questall), or what changed.
func questEvent(m mp.Msg) {
	cur := questState()
	switch m.Op {
	case "questall":
		cur = m.Data
	case "quest":
		if len(cur) != len(questSnap) || !patchBytes(cur, m.Data, func(int) bool { return false }) {
			return
		}
	}
	setQuestState(cur)
	questSnap = questState() // taken, not to be sent back
}
