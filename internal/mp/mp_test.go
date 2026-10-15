//go:build !js

package mp

import (
	"testing"
	"time"
)

// A game hosting without a server: another finds it on the network and the messages cross.
func TestLAN(t *testing.T) {
	addr, err := ServeLAN()
	if err != nil {
		t.Skip(err) // the port is taken (a game is running)
	}
	h, err := Dial(addr, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	room, err := h.Host()
	if err != nil {
		t.Fatal(err)
	}
	Announce("test", room)
	found, got, err := Find("test", 3*time.Second)
	if err != nil {
		t.Skip(err) // no broadcast on this network (CI runners)
	}
	if got != room || found != addr {
		t.Fatalf("found %s %s, hosting %s %s", found, got, addr, room)
	}
	j, err := Dial(found, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if err := j.Join(got); err != nil {
		t.Fatal(err)
	}
	j.Send(Msg{Op: "hello", Text: "hi"})
	for i := 0; ; i++ {
		for _, m := range h.Events() {
			if m.Op == "hello" && m.Text == "hi" && m.From == 1 {
				return
			}
		}
		if i > 100 {
			t.Fatal("the host got nothing")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
