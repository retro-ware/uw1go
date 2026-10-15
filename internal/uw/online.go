package uw

import (
	"strconv"
	"strings"
	"time"

	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/mp"
)

// Experimental online play (not in the original): before the main menu the player may
// host a game on the relay at RelayURL or join one hosted there. Each player has their
// own character; the world is the host's and is kept the same for both (onlinesync.go),
// and each sees the other as an adventurer walking about (onlinepeer.go).

var (
	// RelayURL is the server's address (host:port, ws:// or wss://); empty or "off", games
	// are hosted and joined between the players' machines (the host's game is the meeting
	// point).
	RelayURL string
	// DefaultRelay is the public server, the one the invite links' page knows.
	DefaultRelay string
	// AutoHost and AutoJoin skip the menu: host a game, or join the game with that code.
	AutoHost bool
	AutoJoin string
	// Invite is the link others join a hosted game with (code: the game's); Share hands
	// it on where it can (the browser's clipboard). Either may be nil.
	Invite func(code string) string
	Share  func(link string) bool
	online *mp.Client
)

// onlineMenu asks whether to play alone, host or join, before the main menu. With a
// server set (RelayURL) games are hosted and joined there; without, the host's game is
// the meeting point itself and the joiner types its address.
func onlineMenu() {
	var pal [0x300]byte
	readQuikpal(PAL_OPENING, pal[:])
	if RelayURL == "off" {
		RelayURL = ""
	}
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
		server := "Server: none (this network)"
		if RelayURL != "" {
			server = "Server: " + serverName(RelayURL)
		}
		onlineScreen(pal[:])
		switch parseStartInput(nil, []string{"Play alone", "Host a game", "Join a game", server}, 0) {
		case 1:
			if onlineHost(pal[:]) {
				return
			}
		case 2:
			if onlineJoin(pal[:]) {
				return
			}
		case 3:
			onlineScreen(pal[:])
			if a, ok := onlineAsk(pal[:], "Server (host:port, empty for none):", serverName(RelayURL)); ok {
				RelayURL = serverURL(a)
			}
		default: // play alone (Escape too)
			return
		}
	}
}

// serverName shows a server URL as the player would type it: its host[:port].
func serverName(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		url = url[i+3:]
	}
	if i := strings.Index(url, "/"); i >= 0 {
		url = url[:i]
	}
	return url
}

// serverURL is the address of the server the player typed: host[:port] (plain TCP), or a
// ws(s):// URL (the browser's way in).
func serverURL(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || strings.EqualFold(s, "none"):
		return ""
	case strings.Contains(s, "://") || strings.Contains(s, ":"):
		return s
	}
	return s + ":" + strconv.Itoa(mp.Port)
}

// onlineHost hosts a game: on the server, or, without one, here, telling the network.
func onlineHost(pal []byte) bool {
	if RelayURL == "" {
		url, err := mp.ServeLAN()
		if err != nil {
			onlineError(pal, "Cannot host here: "+err.Error())
			return false
		}
		RelayURL = url
		ok := onlineHostOn(pal, "(from elsewhere, once port 7781 is forwarded: -join "+url+"/ROOM)")
		RelayURL = ""
		if ok {
			mp.Announce(Protocol, online.Room)
		}
		return ok
	}
	return onlineHostOn(pal)
}

// onlineJoin joins a game on the server, or, without one, on the host the player names
// (a host announcing itself on this network is offered).
func onlineJoin(pal []byte) bool {
	if RelayURL != "" {
		return onlineJoinOn(pal)
	}
	onlineScreen(pal)
	onlineText("Looking for a game...")
	found, _, _ := mp.Find(Protocol, 2*time.Second)
	onlineScreen(pal)
	a, ok := onlineAsk(pal, "Host (ip:port):", found)
	if !ok || serverURL(a) == "" {
		return false
	}
	RelayURL = serverURL(a)
	ok = onlineJoinOn(pal)
	RelayURL = ""
	return ok
}

// onlineAsk shows prompt and lets the player type (Enter takes it, Escape gives up).
func onlineAsk(pal []byte, prompt, text string) (string, bool) {
	for {
		onlineScreen(pal)
		onlineText(prompt, text+"_")
		key := mouseGetInput()
		switch {
		case key < 0:
			menuCycle()
		case key == KEY_ENTER:
			return text, true
		case key == KEY_ESC || key == 2:
			return "", false
		case key == KEY_BACKSPACE && len(text) > 0:
			text = text[:len(text)-1]
		case key >= 0x20 && key < 0x7F && len(text) < 60:
			text += strings.ToLower(string(rune(key)))
		}
	}
}

// onlineHostOn opens a game on the server at RelayURL and shows how others join it:
// with the invite link where there is a page for it (the public server), else by the
// server's name and the code.
func onlineHostOn(pal []byte, more ...string) bool {
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
	if Invite != nil && RelayURL == DefaultRelay {
		link := Invite(room)
		if Share != nil && Share(link) {
			lines = append(lines, "Invite link copied:")
		} else {
			lines = append(lines, "Invite link:")
		}
		lines = append(lines, link)
	} else {
		lines = append(lines, "Others join at "+serverName(RelayURL)+" with code "+room)
	}
	onlineNotice(pal, append(lines, more...)...)
	return true
}

// onlineJoinCode joins the game with the code (or the invite link) code. A direct
// address, HOST:PORT/CODE, names the server too (a LAN host reached over the internet).
func onlineJoinCode(pal []byte, code string) bool {
	if i := strings.LastIndex(code, "/"); i > 0 && !strings.Contains(code, "join=") {
		RelayURL, code = code[:i], code[i+1:]
	}
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

// onlineJoinOn lets the player pick one of the open games on the server at RelayURL.
func onlineJoinOn(pal []byte) bool {
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
	cl, err := mp.Dial(RelayURL, Protocol)
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
