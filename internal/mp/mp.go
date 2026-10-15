// Package mp is the experimental online multiplayer: a relay that keeps rooms of two
// players and passes each one's messages to the other, and the client the game talks to
// it with. The relay knows nothing of the game; what the messages mean is the game's.
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

// MaxPlayers is the players a room takes. The host is player 0.
const MaxPlayers = 2

// readLimit takes a whole level block in one message.
const readLimit = 1 << 20

// Msg is every message between a client and the relay. Op says which fields count. To the
// relay and back:
//
//	host  → room (Room, From: the player's number)
//	list  → rooms (Rooms)
//	join  (Room) → joined (Room, From), or error (Err)
//	left  (From)  from the relay: a player of the room has gone
//
// Any other op is the game's: the relay passes it to the rest of the room with From set
// to the sender's number.
type Msg struct {
	Op    string   `json:"op"`
	Room  string   `json:"room,omitempty"`
	Rooms []string `json:"rooms,omitempty"`
	From  int      `json:"from"`
	Err   string   `json:"err,omitempty"`
	Pos   *Pos     `json:"pos,omitempty"`
	Level int      `json:"level,omitempty"`
	Slots []int    `json:"slots,omitempty"`
	Seq   int      `json:"seq,omitempty"`
	Ack   int      `json:"ack,omitempty"`
	Data  []byte   `json:"data,omitempty"`
	Anim  []byte   `json:"anim,omitempty"`
	Text  string   `json:"text,omitempty"`
}

// Pos is where a player is: the level, the place in the game's units (x and y in 1/256
// tiles, z in 1/8 of an object's z unit), the heading, and whether they are walking.
type Pos struct {
	Level   int   `json:"l"`
	X       int16 `json:"x"`
	Y       int16 `json:"y"`
	Z       int16 `json:"z"`
	Heading int16 `json:"h"`
	Moving  bool  `json:"m,omitempty"`
	HP      byte  `json:"hp,omitempty"`
	Avatar  int   `json:"a,omitempty"` // the player's avatar's object slot on the level`
}

// Client is one connection to the relay.
type Client struct {
	c      *websocket.Conn
	answer chan Msg
	out    chan Msg
	mu     sync.Mutex
	pos    map[int]Pos // the other players by number
	events []Msg
	Room   string
	Me     int // this player's number in the room
}

// Dial connects to the relay at url (ws:// or wss://).
func Dial(url string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	c.SetReadLimit(readLimit)
	cl := &Client{c: c, answer: make(chan Msg, 1), out: make(chan Msg, 256), pos: map[int]Pos{}}
	go cl.read()
	go cl.write()
	return cl, nil
}

func (cl *Client) read() {
	defer close(cl.answer)
	for {
		var m Msg
		if err := wsjson.Read(context.Background(), cl.c, &m); err != nil {
			return
		}
		cl.mu.Lock()
		switch m.Op {
		case "room", "rooms", "joined", "error":
			select {
			case cl.answer <- m:
			default:
			}
		case "pos":
			if m.Pos != nil {
				cl.pos[m.From] = *m.Pos
			}
		case "left":
			delete(cl.pos, m.From)
			cl.events = append(cl.events, m)
		default:
			cl.events = append(cl.events, m)
		}
		cl.mu.Unlock()
	}
}

func (cl *Client) write() {
	for m := range cl.out {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		wsjson.Write(ctx, cl.c, m)
		cancel()
	}
}

// ask sends m and waits for the relay's answer.
func (cl *Client) ask(m Msg) (Msg, error) {
	cl.out <- m
	select {
	case r, ok := <-cl.answer:
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
	cl.Room, cl.Me = r.Room, r.From
	return r.Room, err
}

// List gives the open rooms.
func (cl *Client) List() ([]string, error) {
	r, err := cl.ask(Msg{Op: "list"})
	return r.Rooms, err
}

// Join enters a room.
func (cl *Client) Join(room string) error {
	r, err := cl.ask(Msg{Op: "join", Room: room})
	if err == nil {
		cl.Room, cl.Me = room, r.From
	}
	return err
}

// Send passes m to the other players, in order, without waiting for the network. With
// the network that far behind it is dropped.
func (cl *Client) Send(m Msg) {
	select {
	case cl.out <- m:
	default:
	}
}

// Events takes the messages that came in since the last call.
func (cl *Client) Events() []Msg {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	ev := cl.events
	cl.events = nil
	return ev
}

// Others gives the other players' last places by number.
func (cl *Client) Others() map[int]Pos {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	ps := make(map[int]Pos, len(cl.pos))
	for k, p := range cl.pos {
		ps[k] = p
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
