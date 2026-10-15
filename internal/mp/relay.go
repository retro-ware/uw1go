//go:build !js

package mp

import (
	"context"
	"log"
	"net/http"
	"sort"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type peer struct {
	id   int
	conn *websocket.Conn
	out  chan Msg
}

// Relay keeps the rooms.
type Relay struct {
	mu     sync.Mutex
	nextID int
	rooms  map[string]map[*peer]bool
}

// Serve runs a relay on addr (such as ":7777") until it fails.
func Serve(addr string) error {
	r := &Relay{rooms: map[string]map[*peer]bool{}}
	log.Printf("uw1 relay on %s", addr)
	return http.ListenAndServe(addr, r)
}

func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true}) // any page may play
	if err != nil {
		return
	}
	r.mu.Lock()
	r.nextID++
	p := &peer{id: r.nextID, conn: c, out: make(chan Msg, 64)}
	r.mu.Unlock()
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	go func() {
		for m := range p.out {
			if wsjson.Write(ctx, c, m) != nil {
				cancel()
				return
			}
		}
	}()
	room := ""
	defer func() {
		r.mu.Lock()
		if room != "" {
			delete(r.rooms[room], p)
			if len(r.rooms[room]) == 0 {
				delete(r.rooms, room)
			}
			r.broadcast(room, p, Msg{Op: "left", From: p.id})
		}
		r.mu.Unlock()
		close(p.out)
		c.CloseNow()
	}()
	for {
		var m Msg
		if wsjson.Read(ctx, c, &m) != nil {
			return
		}
		r.mu.Lock()
		switch m.Op {
		case "host":
			if room == "" {
				room = RoomName()
				for r.rooms[room] != nil {
					room = RoomName()
				}
				r.rooms[room] = map[*peer]bool{p: true}
			}
			r.reply(p, Msg{Op: "room", Room: room})
		case "list":
			rooms := []string{}
			for name := range r.rooms {
				rooms = append(rooms, name)
			}
			sort.Strings(rooms)
			r.reply(p, Msg{Op: "rooms", Rooms: rooms})
		case "join":
			if room != "" || r.rooms[m.Room] == nil {
				r.reply(p, Msg{Op: "error", Err: "no such game"})
				break
			}
			room = m.Room
			r.rooms[room][p] = true
			r.reply(p, Msg{Op: "joined", Room: room})
		case "pos":
			if room != "" && m.Pos != nil {
				r.broadcast(room, p, Msg{Op: "pos", From: p.id, Pos: m.Pos})
			}
		}
		r.mu.Unlock()
	}
}

// reply and broadcast drop messages for a peer too slow to take them.
func (r *Relay) reply(p *peer, m Msg) {
	select {
	case p.out <- m:
	default:
	}
}

func (r *Relay) broadcast(room string, from *peer, m Msg) {
	for q := range r.rooms[room] {
		if q != from {
			r.reply(q, m)
		}
	}
}
