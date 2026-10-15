package uw

import (
	"strings"

	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/mp"
)

// Experimental online play (not in the original): before the main menu the player may
// host a game on the relay at RelayURL or join one hosted there. Each player has their
// own character; the world is the host's and is kept the same for both (onlinesync.go),
// and each sees the other as an adventurer walking about (onlinepeer.go).

var (
	// RelayURL is the relay's address (ws:// or wss://); empty or "off", no online menu.
	RelayURL string
	// AutoHost and AutoJoin skip the menu: host a game, or join the game with that code.
	AutoHost bool
	AutoJoin string
	// Invite is the link others join a hosted game with (code: the game's); Share hands
	// it on where it can (the browser's clipboard). Either may be nil.
	Invite func(code string) string
	Share  func(link string) bool
	online *mp.Client
)

// onlineMenu asks whether to play alone, host or join, before the main menu.
func onlineMenu() {
	if RelayURL == "" || RelayURL == "off" {
		return
	}
	var pal [0x300]byte
	readQuikpal(PAL_OPENING, pal[:])
	if code := AutoJoin; code != "" {
		AutoJoin = ""
		if onlineJoinCode(pal[:], code) {
			return
		}
	}
	if AutoHost {
		AutoHost = false
		if onlineHost(pal[:]) {
			return
		}
	}
	for {
		onlineScreen(pal[:])
		switch parseStartInput(nil, []string{"Play alone", "Host an online game", "Join an online game"}, 0) {
		case 1:
			if onlineHost(pal[:]) {
				return
			}
		case 2:
			if onlineJoin(pal[:]) {
				return
			}
		default: // play alone (Escape too)
			return
		}
	}
}

// onlineHost opens a game on the relay and shows how others join it.
func onlineHost(pal []byte) bool {
	cl := onlineDial(pal)
	if cl == nil {
		return false
	}
	room, err := cl.Host()
	if err != nil {
		cl.Close()
		onlineError(pal, err.Error())
		return false
	}
	online = cl
	dprintf("mp hosting %s\n", room)
	lines := []string{"Hosting game " + room}
	if Invite != nil {
		link := Invite(room)
		if Share != nil && Share(link) {
			lines = append(lines, "Invite link copied:")
		} else {
			lines = append(lines, "Invite link:")
		}
		lines = append(lines, link)
	} else {
		lines = append(lines, "Others join with code "+room)
	}
	onlineNotice(pal, lines...)
	return true
}

// onlineJoinCode joins the game with the code (or the invite link) code.
func onlineJoinCode(pal []byte, code string) bool {
	code = JoinCode(code)
	cl := onlineDial(pal)
	if cl == nil {
		return false
	}
	if err := cl.Join(code); err != nil {
		cl.Close()
		onlineError(pal, "Game "+code+": "+err.Error())
		return false
	}
	online = cl
	onlineNotice(pal, "Joined game "+code)
	return true
}

// JoinCode is the game's code in s, a code or an invite link (...?join=CODE).
func JoinCode(s string) string {
	if i := strings.LastIndex(s, "join="); i >= 0 {
		s = s[i+5:]
	}
	if i := strings.IndexAny(s, "&#/ "); i >= 0 {
		s = s[:i]
	}
	return strings.ToUpper(strings.TrimSpace(s))
}

// onlineJoin lets the player pick one of the relay's open games.
func onlineJoin(pal []byte) bool {
	cl := onlineDial(pal)
	if cl == nil {
		return false
	}
	rooms, err := cl.List()
	if err == nil && len(rooms) == 0 {
		err = errString("No games are open")
	}
	if err != nil {
		cl.Close()
		onlineError(pal, err.Error())
		return false
	}
	if len(rooms) > 4 {
		rooms = rooms[:4]
	}
	text := []string{"Back"}
	for _, r := range rooms {
		text = append(text, "Join game "+r)
	}
	onlineScreen(pal)
	i := parseStartInput(nil, text, 1)
	if i < 1 {
		cl.Close()
		return false
	}
	if err := cl.Join(rooms[i-1]); err != nil {
		cl.Close()
		onlineError(pal, err.Error())
		return false
	}
	online = cl
	onlineNotice(pal, "Joined game "+rooms[i-1])
	return true
}

type errString string

func (e errString) Error() string { return string(e) }

func onlineDial(pal []byte) *mp.Client {
	onlineScreen(pal)
	onlineText("Connecting...")
	cl, err := mp.Dial(RelayURL)
	if err != nil {
		onlineError(pal, "No connection to the relay")
		return nil
	}
	return cl
}

// onlineScreen shows the menu's backdrop without its buttons.
func onlineScreen(pal []byte) {
	mouseHide()
	loadBitMap(-1, "DATA\\opscr.byt")
	mouseShow()
	grfxSetpal(pal)
}

// onlineText writes lines in the middle of the screen, the first big, the rest small
// (an invite link is long).
func onlineText(lines ...string) {
	gr.ForegroundColor, gr.BackgroundColor = 0xA2, 0xA2
	mouseHide()
	y := 0x5A - 6*(len(lines)-1)
	for i, msg := range lines {
		if i == 0 {
			grfxLoadFont("fontbig.sys")
		} else {
			grfxLoadFont("font5x6p.sys")
		}
		gr.StringToScreen(msg, 0xA0-gr.StringWidth(msg)/2, y)
		y += 14
	}
	mouseShow()
	grfxLoadFont("font5x6p.sys")
}

// onlineNotice shows lines until a click or a key.
func onlineNotice(pal []byte, lines ...string) {
	onlineScreen(pal)
	onlineText(lines...)
	for mouseGetInput() < 0 {
		menuCycle()
	}
}

func onlineError(pal []byte, msg string) { onlineNotice(pal, msg) }
