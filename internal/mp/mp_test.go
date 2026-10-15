//go:build !js

package mp

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// wait polls f for up to a second.
func wait(t *testing.T, what string, f func() bool) {
	t.Helper()
	for i := 0; !f(); i++ {
		if i > 100 {
			t.Fatal(what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRelay(t *testing.T) {
	s := httptest.NewServer(NewRelay())
	defer s.Close()
	url := "ws" + strings.TrimPrefix(s.URL, "http")
	dial := func() *Client {
		c, err := Dial(url)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	a := dial()
	room, err := a.Host()
	if err != nil || a.Me != 0 {
		t.Fatal(err, a.Me)
	}
	b := dial()
	if rooms, _ := b.List(); len(rooms) != 1 || rooms[0] != room {
		t.Fatalf("rooms %v, want [%s]", rooms, room)
	}
	if err := b.Join(room); err != nil || b.Me != 1 {
		t.Fatal(err, b.Me)
	}
	if err := dial().Join(room); err == nil {
		t.Fatal("a third player got in")
	}
	want := Pos{Level: 1, X: 0x2080, Y: 0x0240, Z: 0x60, Heading: -0x4000, Moving: true}
	a.Send(Msg{Op: "pos", Pos: &want})
	wait(t, "no position arrived", func() bool { return b.Others()[0] == want })
	block := make([]byte, 0x7C08)
	block[100] = 7
	b.Send(Msg{Op: "level", Level: 3, Data: block})
	var got []Msg
	wait(t, "no level arrived", func() bool { got = append(got, a.Events()...); return len(got) > 0 })
	if m := got[0]; m.Op != "level" || m.From != 1 || m.Level != 3 || len(m.Data) != len(block) || m.Data[100] != 7 {
		t.Fatalf("got %+v", m)
	}
	a.Close()
	wait(t, "the host did not leave", func() bool { return len(b.Others()) == 0 })
}
