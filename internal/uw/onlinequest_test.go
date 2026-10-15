//go:build !js

package uw

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/memmaker/uw1go/internal/mp"
)

// questSide is one player's share of the globals the quest sync uses.
type questSide struct {
	cl     *mp.Client
	player Player
	snap   []byte
	sent   int
	seen   int
	bits   [][8]int
	dirty  bool
}

func (s *questSide) in() {
	online, player = s.cl, &s.player
	questSnap, questSent, questSeen, questBitSeq, questDirty = s.snap, s.sent, s.seen, s.bits, s.dirty
}

func (s *questSide) out() {
	s.player = *player
	s.snap, s.sent, s.seen, s.bits, s.dirty = questSnap, questSent, questSeen, questBitSeq, questDirty
}

// TestQuestMerge: the two players change the story at the same time, each before hearing
// of the other's change. Different bits of one byte both stay; the same value ends as the
// host set it, for both.
func TestQuestMerge(t *testing.T) {
	srv := httptest.NewServer(mp.NewRelay())
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	hc, err := mp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	room, _ := hc.Host()
	jc, _ := mp.Dial(url)
	if err := jc.Join(room); err != nil {
		t.Fatal(err)
	}
	saved, savedOnline := player, online
	defer func() { player, online = saved, savedOnline }()
	host, joiner := &questSide{cl: hc, player: *player}, &questSide{cl: jc, player: *player}
	for _, s := range []*questSide{host, joiner} {
		s.in()
		questTick() // the start: nothing to send
		s.out()
	}
	step := func(s *questSide, change func()) {
		s.in()
		change()
		questChanged()
		questTick()
		s.out()
	}
	take := func(s *questSide, n int) {
		var got []mp.Msg
		for i := 0; len(got) < n && i < 200; i++ {
			got = append(got, s.cl.Events()...)
			time.Sleep(5 * time.Millisecond)
		}
		s.in()
		for _, m := range got {
			questEvent(m)
		}
		questTick() // the host answers what it kept
		s.out()
	}

	step(host, func() { player.Quests |= 1; player.GameVars[5] = 7 })
	step(joiner, func() { player.Quests |= 2; player.GameVars[5] = 9 })
	take(host, 1)
	take(joiner, 1)
	time.Sleep(50 * time.Millisecond)
	take(joiner, 0)
	take(host, 0)
	for name, s := range map[string]*questSide{"host": host, "joiner": joiner} {
		if s.player.Quests != 3 {
			t.Errorf("%s: quests %b, want 11", name, s.player.Quests)
		}
		if s.player.GameVars[5] != 7 {
			t.Errorf("%s: game variable 5 is %d, want the host's 7", name, s.player.GameVars[5])
		}
	}
}
