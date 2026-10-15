//go:build !js

package mp

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// peer is a connection; what goes out to it is JSON, the game's messages as they came.
type peer struct {
	num int
	out chan []byte
}

// Relay keeps the rooms: each a map of player number to peer.
type Relay struct {
	mu    sync.Mutex
	rooms map[string]map[int]*peer
	conns int
	idle  time.Time // when the last connection went
}

func NewRelay() *Relay { return &Relay{rooms: map[string]map[int]*peer{}, idle: time.Now()} }

// Serve runs a relay on addr (such as "127.0.0.1:7781"), or on the socket systemd passes
// (LISTEN_FDS). With idle above zero it returns once no one has been connected for that
// long; systemd starts it again on the next connection.
func Serve(addr string, idle time.Duration) error {
	var l net.Listener
	var err error
	if os.Getenv("LISTEN_FDS") == "1" {
		l, err = net.FileListener(os.NewFile(3, "systemd"))
	} else {
		l, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return err
	}
	r := NewRelay()
	if idle > 0 {
		go func() {
			for range time.Tick(time.Minute) {
				r.mu.Lock()
				done := r.conns == 0 && time.Since(r.idle) > idle
				r.mu.Unlock()
				if done {
					log.Print("idle, stopping")
					l.Close()
					return
				}
			}
		}()
	}
	log.Printf("uw1 relay on %s", l.Addr())
	if err := http.Serve(l, r); err != nil && idle > 0 {
		return nil // closed for idleness
	} else {
		return err
	}
}

func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true}) // any page may play
	if err != nil {
		return
	}
	c.SetReadLimit(readLimit)
	p := &peer{out: make(chan []byte, 512)}
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	go func() {
		for m := range p.out {
			if c.Write(ctx, websocket.MessageText, m) != nil {
				cancel()
				return
			}
		}
	}()
	r.mu.Lock()
	r.conns++
	r.mu.Unlock()
	room := ""
	defer func() {
		r.mu.Lock()
		r.conns--
		r.idle = time.Now()
		if room != "" {
			delete(r.rooms[room], p.num)
			if len(r.rooms[room]) == 0 {
				delete(r.rooms, room)
			}
			r.broadcast(room, p, encode(Msg{Op: "left", From: p.num}))
		}
		r.mu.Unlock()
		close(p.out)
		c.CloseNow()
	}()
	for {
		_, raw, err := c.Read(ctx)
		if err != nil {
			return
		}
		raw = bytes.TrimSpace(raw)
		var m struct{ Op, Room string }
		if json.Unmarshal(raw, &m) != nil || len(raw) < 2 || raw[len(raw)-1] != '}' {
			continue
		}
		r.mu.Lock()
		switch m.Op {
		case "host":
			if room == "" {
				room = RoomName()
				for r.rooms[room] != nil {
					room = RoomName()
				}
				p.num = 0
				r.rooms[room] = map[int]*peer{0: p}
			}
			r.reply(p, Msg{Op: "room", Room: room, From: p.num})
		case "list":
			rooms := []string{}
			for name, ps := range r.rooms {
				if len(ps) < MaxPlayers {
					rooms = append(rooms, name)
				}
			}
			sort.Strings(rooms)
			r.reply(p, Msg{Op: "rooms", Rooms: rooms})
		case "join":
			ps := r.rooms[m.Room]
			switch {
			case room != "" || ps == nil:
				r.reply(p, Msg{Op: "error", Err: "No such game"})
			case len(ps) >= MaxPlayers:
				r.reply(p, Msg{Op: "error", Err: "That game is full"})
			default:
				for ps[p.num] != nil {
					p.num++
				}
				room = m.Room
				ps[p.num] = p
				r.reply(p, Msg{Op: "joined", Room: room, From: p.num})
			}
		default: // the game's: passed on as it is, with the sender's number (the last "from" counts)
			if room != "" {
				from, _ := json.Marshal(p.num)
				fwd := append(append(append(raw[:len(raw)-1:len(raw)-1], `,"from":`...), from...), '}')
				r.broadcast(room, p, fwd)
			}
		}
		r.mu.Unlock()
	}
}

// reply and broadcast drop messages for a peer too slow to take them.
func (r *Relay) reply(p *peer, m Msg) { r.send(p, encode(m)) }

func encode(m Msg) []byte {
	b, _ := json.Marshal(m)
	return b
}

func (r *Relay) send(p *peer, m []byte) {
	select {
	case p.out <- m:
	default:
	}
}

func (r *Relay) broadcast(room string, from *peer, m []byte) {
	for _, q := range r.rooms[room] {
		if q != from {
			r.send(q, m)
		}
	}
}
