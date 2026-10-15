// Package mp is the game's side of the experimental online multiplayer: its messages,
// and the connection to the other player through a rendezvous server
// (github.com/memmaker/rendezvous), which introduces the two and relays for them until
// they connect directly. The server knows nothing of the game; what the messages mean
// is the game's.
package mp

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/memmaker/rendezvous"
	"github.com/memmaker/rendezvous/client"
)

// MaxPlayers is the players a room takes. The host is player 0.
const MaxPlayers = 2

// Port is where a hosting game listens without a server (TCP and UDP), and where the
// rendezvous server listens by default.
const Port = 7781

// Msg is every message between a client and the relay. Op says which fields count. To the
// server and back:
//
//	host  → room (Room, From: the player's number)
//	list  → rooms (Rooms)
//	join  (Room) → joined (Room, From), or error (Err)
//	left  (From)  from the relay: a player of the room has gone
//
// Any other op is the game's: the server passes it to the rest of the room with From set
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

// Client is one connection to the server and, through it, to the other player.
type Client struct {
	c   *client.Conn
	pos map[int]Pos // the other players by number
	// Room is the game's code and Me this player's number in it.
	Room string
	Me   int
}

// Dial connects to the server at url (host:port, ws:// or wss://) for games of this
// protocol version: only players with the same one meet.
func Dial(url, protocol string) (*Client, error) {
	c, err := client.Dial(url, protocol)
	if err != nil {
		return nil, err
	}
	return &Client{c: c, pos: map[int]Pos{}}, nil
}

// Host opens a room and returns its name.
func (cl *Client) Host() (string, error) {
	room, err := cl.c.Host()
	cl.Room, cl.Me = cl.c.Room, cl.c.Me
	return room, err
}

// List gives the open rooms.
func (cl *Client) List() ([]string, error) { return cl.c.List() }

// Join enters a room.
func (cl *Client) Join(room string) error {
	err := cl.c.Join(room)
	cl.Room, cl.Me = cl.c.Room, cl.c.Me
	return err
}

// Send passes m to the other players, in order, without waiting for the network. With
// the network that far behind it is dropped.
func (cl *Client) Send(m Msg) { cl.c.Send(m) }

// Events takes the messages that came in since the last call, the other players'
// positions kept aside for Others.
func (cl *Client) Events() []Msg {
	var ev []Msg
	for _, raw := range cl.c.Recv() {
		var m Msg
		if json.Unmarshal(raw.Data, &m) != nil {
			continue
		}
		m.From = raw.From
		switch m.Op {
		case "pos":
			if m.Pos != nil {
				cl.pos[m.From] = *m.Pos
			}
		case "left":
			delete(cl.pos, m.From)
			ev = append(ev, m)
		default:
			ev = append(ev, m)
		}
	}
	return ev
}

// Others gives the other players' last places by number, as of the last Events.
func (cl *Client) Others() map[int]Pos {
	ps := make(map[int]Pos, len(cl.pos))
	for k, p := range cl.pos {
		ps[k] = p
	}
	return ps
}

// Direct tells whether the messages go straight to the other player.
func (cl *Client) Direct() bool { return cl.c.Direct() }

// Close leaves the game.
func (cl *Client) Close() { cl.c.Close() }

// NewRelay is a server as an HTTP handler, for tests.
func NewRelay() http.Handler { return rendezvous.New() }

// ServeLAN runs a rendezvous server in this game on Port, for hosting without a server,
// and returns its address on the network for this game to dial and others to join by.
func ServeLAN() (string, error) {
	if _, _, err := rendezvous.Start(":" + strconv.Itoa(Port)); err != nil {
		return "", err
	}
	return net.JoinHostPort(client.LocalIP(), strconv.Itoa(Port)), nil
}

// Announce tells the local network about the room this game hosts, so Find hears it.
func Announce(protocol, room string) { client.Announce(protocol, room, Port) }

// Find waits up to timeout for a host on the local network and returns its address and room.
func Find(protocol string, timeout time.Duration) (addr, room string, err error) {
	return client.Find(protocol, Port, timeout)
}
