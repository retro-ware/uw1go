package uw

import (
	"bytes"
	"encoding/binary"

	"github.com/memmaker/uw1go/internal/mp"
)

// The story is shared too: the quest flags and bytes, the talismans, the game variables
// the triggers and conversations keep, the story's flags in the player record (orb, key,
// cup, talismans' ok, Garamon, maze) and every conversation's own variables
// (SAVE0\bglobals.dat). A joiner takes the host's at the start.
//
// Whatever changes them calls questChanged; the next pass compares them with what was
// last sent and sends what changed, each byte with its old and new value. Flags are
// taken bit by bit, so two players setting different flags of one byte both keep theirs;
// values (counters, the conversations' variables) are taken whole. Where both changed
// the same flag or value before hearing of the other's change, the host's word stands: the host keeps its value for bits it changed in a message the
// joiner had not seen yet (the joiner takes that message when it comes), and the joiner
// takes whatever the host sends. Messages are numbered, and each says the last one it has
// seen from the other, as the level's diffs do.

const bglobalsFile = "SAVE0\\bglobals.dat"

var (
	questDirty  bool
	questSnap   []byte   // the story as last sent or taken
	questSent   int      // the number of the last message sent
	questSeen   int      // the number of the last message taken
	questBitSeq [][8]int // the host's: per bit, the number of the last message that changed it
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

// questField is the field byte i of the story is part of, and whether it is flags (each
// bit a player's own) or a value (taken whole): the quest flags and the record's flags
// are flags, the quest bytes, the talismans and the game variables bytes, and
// bglobals.dat's variables words.
func questField(i int) (start, end int, flags bool) {
	switch {
	case i < 4:
		return i, i + 1, true
	case i < questHead-6:
		return i, i + 1, false
	case i < questHead:
		return i, i + 1, true
	}
	s := questHead + (i-questHead)&^1
	return s, s + 2, false
}

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

// questChanged notes that the story may have changed, to be compared at the next pass.
func questChanged() { questDirty = true }

// questStart takes the story as it is now as the last sent: on starting online play, and
// on taking the host's.
func questStart(cur []byte) {
	questSnap = cur
	questSent, questSeen = 0, 0
	questBitSeq = make([][8]int, len(cur))
	questDirty = false
}

// questTick sends what changed in the story since last time, if something may have.
func questTick() {
	if questSnap == nil {
		questStart(questState())
		return
	}
	if !questDirty {
		return
	}
	questDirty = false
	cur := questState()
	if len(cur) != len(questSnap) {
		return
	}
	var d []byte
	last := 0
	for i := 0; i < len(cur); {
		s, e, _ := questField(i)
		e = min(e, len(cur))
		if bytes.Equal(cur[s:e], questSnap[s:e]) {
			i = e
			continue
		}
		for ; i < e; i++ { // the whole field, so that a value goes whole
			d = binary.AppendUvarint(d, uint64(i-last))
			d = append(d, questSnap[i], cur[i])
			last = i
		}
	}
	if d == nil {
		return
	}
	questSent++
	if online.Me == 0 {
		for i := range cur {
			for bit, m := 0, cur[i]^questSnap[i]; m != 0; bit, m = bit+1, m>>1 {
				if m&1 != 0 {
					questBitSeq[i][bit] = questSent
				}
			}
		}
	}
	online.Send(mp.Msg{Op: "quest", Data: d, Seq: questSent, Ack: questSeen})
	questSnap = cur
}

// questEvent takes the host's story (questall) or the other player's changes to it.
func questEvent(m mp.Msg) {
	if m.Op == "questall" {
		setQuestState(m.Data)
		questStart(questState())
		return
	}
	if questSnap == nil {
		return
	}
	questSeen = m.Seq
	cur := questState()
	host := online.Me == 0
	type change struct{ old, new byte }
	got := map[int]change{}
	d := m.Data
	for i := 0; len(d) > 0; {
		gap, n := binary.Uvarint(d)
		if n <= 0 || len(d) < n+2 {
			return
		}
		i += int(gap)
		if i >= len(cur) {
			return
		}
		got[i] = change{d[n], d[n+1]}
		d = d[n+2:]
	}
	// the host's own: the bits it changed in what the joiner had not seen
	hostKept := func(i int) (mask byte) {
		if host {
			for bit := 0; bit < 8; bit++ {
				if questBitSeq[i][bit] > m.Ack {
					mask |= 1 << bit
				}
			}
		}
		return mask
	}
	for i, c := range got {
		s, e, flags := questField(i)
		mask := c.old ^ c.new
		if flags {
			mask &^= hostKept(i)
		} else {
			mask = 0xFF // a value goes whole, unless the host changed any of it
			for k := s; k < min(e, len(cur)); k++ {
				if hostKept(k) != 0 {
					mask = 0
				}
			}
		}
		cur[i] = cur[i]&^mask | c.new&mask
		questSnap[i] = questSnap[i]&^mask | c.new&mask // taken, not to be sent back
	}
	setQuestState(cur)
}
