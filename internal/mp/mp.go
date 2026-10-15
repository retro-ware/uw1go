// Package mp is the experimental online multiplayer: a relay that keeps rooms of players
// and passes each player's messages to the others in the room, and the client the game
// talks to it with. The relay knows nothing of the game: each player runs their own world
// and only tells the others where they are.
package mp

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Msg is every message between a client and the relay. Op says which fields count:
//
//	host  → room (Room)        the client opens a room and is in it
//	list  → rooms (Rooms)      the open rooms
//	join  (Room) → joined, or error (Err)
//	pos   (Pos) relayed to the room with From set
//	left  (From)               a player of the room has gone
type Msg struct {
	Op    string   `json:"op"`
	Room  string   `json:"room,omitempty"`
	Rooms []string `json:"rooms,omitempty"`
	From  int      `json:"from,omitempty"`
	Pos   *Pos     `json:"pos,omitempty"`
	Err   string   `json:"err,omitempty"`
}

// Pos is where a player is: the level, the place in the game's units (x and y in 1/256
// tiles, z in 1/8 of an object's z unit) and the heading.
type Pos struct {
	Level   int   `json:"l"`
	X       int16 `json:"x"`
	Y       int16 `json:"y"`
	Z       int16 `json:"z"`
	Heading int16 `json:"h"`
	Moving  bool  `json:"m,omitempty"`
}

// Client is one connection to the relay.
type Client struct {
	c    *websocket.Conn
	in   chan Msg
	mu   sync.Mutex
	pos  map[int]Pos // the other players by id
	Room string
}

// Dial connects to the relay at url (ws:// or wss://).
func Dial(url string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	cl := &Client{c: c, in: make(chan Msg, 8), pos: map[int]Pos{}}
	go cl.read()
	return cl, nil
}

func (cl *Client) read() {
	defer close(cl.in)
	for {
		var m Msg
		if err := wsjson.Read(context.Background(), cl.c, &m); err != nil {
			return
		}
		switch m.Op {
		case "pos":
			if m.Pos != nil {
				cl.mu.Lock()
				cl.pos[m.From] = *m.Pos
				cl.mu.Unlock()
			}
		case "left":
			cl.mu.Lock()
			delete(cl.pos, m.From)
			cl.mu.Unlock()
		default:
			select {
			case cl.in <- m:
			default:
			}
		}
	}
}

func (cl *Client) send(m Msg) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return wsjson.Write(ctx, cl.c, m)
}

// ask sends m and waits for the relay's answer.
func (cl *Client) ask(m Msg) (Msg, error) {
	if err := cl.send(m); err != nil {
		return Msg{}, err
	}
	select {
	case r, ok := <-cl.in:
		if !ok {
			return Msg{}, errors.New("the relay closed the connection")
		}
		if r.Err != "" {
			return r, errors.New(r.Err)
		}
		return r, nil
	case <-time.After(5 * time.Second):
		return Msg{}, errors.New("the relay does not answer")
	}
}

// Host opens a room and returns its name.
func (cl *Client) Host() (string, error) {
	r, err := cl.ask(Msg{Op: "host"})
	cl.Room = r.Room
	return r.Room, err
}

// List gives the open rooms.
func (cl *Client) List() ([]string, error) {
	r, err := cl.ask(Msg{Op: "list"})
	return r.Rooms, err
}

// Join enters a room.
func (cl *Client) Join(room string) error {
	_, err := cl.ask(Msg{Op: "join", Room: room})
	if err == nil {
		cl.Room = room
	}
	return err
}

// Send tells the room where this player is; it does not wait for the network.
func (cl *Client) Send(p Pos) {
	go cl.send(Msg{Op: "pos", Pos: &p})
}

// Others gives the other players' last places.
func (cl *Client) Others() []Pos {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	ps := make([]Pos, 0, len(cl.pos))
	for _, p := range cl.pos {
		ps = append(ps, p)
	}
	return ps
}

// Close leaves the relay.
func (cl *Client) Close() { cl.c.Close(websocket.StatusNormalClosure, "") }

// RoomName makes a room name of four letters easy to read out.
func RoomName() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 4)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}
