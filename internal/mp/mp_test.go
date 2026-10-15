package mp

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRelay(t *testing.T) {
	s := httptest.NewServer(&Relay{rooms: map[string]map[*peer]bool{}})
	defer s.Close()
	url := "ws" + strings.TrimPrefix(s.URL, "http")
	a, err := Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	room, err := a.Host()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	if rooms, _ := b.List(); len(rooms) != 1 || rooms[0] != room {
		t.Fatalf("rooms %v, want [%s]", rooms, room)
	}
	if err := b.Join(room); err != nil {
		t.Fatal(err)
	}
	want := Pos{Level: 1, X: 0x2080, Y: 0x0240, Z: 0x60, Heading: -0x4000}
	a.Send(want)
	for i := 0; ; i++ {
		if o := b.Others(); len(o) == 1 {
			if o[0] != want {
				t.Fatalf("got %+v, want %+v", o[0], want)
			}
			break
		}
		if i > 100 {
			t.Fatal("no position arrived")
		}
		time.Sleep(10 * time.Millisecond)
	}
	a.Close()
	for i := 0; len(b.Others()) != 0; i++ {
		if i > 100 {
			t.Fatal("the host did not leave")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
