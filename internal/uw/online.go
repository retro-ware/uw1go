package uw

import (
	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/mp"
)

// Experimental online play (not in the original): before the main menu the player may
// host a game on the relay at RelayURL or join one hosted there. Each player runs their
// own world; only where each one is goes over the network, and the others are drawn in
// the 3D view as fighters where they stand. Nothing else is shared yet: creatures, doors
// and items are each player's own.

var (
	// RelayURL is the relay's address (ws:// or wss://); empty, no online menu.
	RelayURL  string
	online    *mp.Client
	lastSent  mp.Pos
	sentAt    uint32
	peersSeen []mp.Pos
)

// onlineMenu asks whether to play alone, host or join, before the main menu.
func onlineMenu() {
	if RelayURL == "" {
		return
	}
	var pal [0x300]byte
	readQuikpal(PAL_OPENING, pal[:])
	for {
		onlineScreen(pal[:])
		switch parseStartInput(nil, []string{"Play alone", "Host an online game", "Join an online game"}, 0) {
		case 1:
			if cl := onlineDial(pal[:]); cl != nil {
				room, err := cl.Host()
				if err != nil {
					cl.Close()
					onlineError(pal[:], err.Error())
					continue
				}
				online = cl
				onlineNotice(pal[:], "Hosting game "+room)
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

func onlineText(msg string) {
	grfxLoadFont("fontbig.sys")
	gr.ForegroundColor, gr.BackgroundColor = 0xA2, 0xA2
	mouseHide()
	gr.StringToScreen(msg, 0xA0-gr.StringWidth(msg)/2, 0x5A)
	mouseShow()
	grfxLoadFont("font5x6p.sys")
}

// onlineNotice shows msg until a click or a key.
func onlineNotice(pal []byte, msg string) {
	onlineScreen(pal)
	onlineText(msg)
	for mouseGetInput() < 0 {
		menuCycle()
	}
}

func onlineError(pal []byte, msg string) { onlineNotice(pal, msg) }

// onlineTick sends this player's place when it changed (at most 20 times a second, and
// once a second regardless) and redraws the view when another player moved.
func onlineTick() {
	if online == nil || !inGame {
		return
	}
	now := GameTime()
	p := mp.Pos{Level: PlayerLevel, X: PN.X, Y: PN.Y, Z: PN.Z, Heading: PlayerFacing}
	p.Moving = p.X != lastSent.X || p.Y != lastSent.Y
	if (p != lastSent && now-sentAt >= 0x10) || now-sentAt >= 0x100 {
		online.Send(p)
		lastSent, sentAt = p, now
	}
	others := online.Others()
	if !samePeers(others, peersSeen) {
		peersSeen = others
		editchng(2)
	}
}

func samePeers(a, b []mp.Pos) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// drawPeers draws the other players standing on the tile at tmptr, after its objects.
func drawPeers() {
	if online == nil || PickUp != 0 {
		return
	}
	for _, p := range peersSeen {
		if p.Level != PlayerLevel || int(p.X>>8)+int(p.Y>>8)*MAP_SIZE != tmptr {
			continue
		}
		fx, fy := int(p.X&0xFF), int(p.Y&0xFF)
		var x, y int
		switch quad { // the tile's place turned into the first quadrant, as doObj turns mobiles'
		case 0:
			x, y = fx, fy
		case 1:
			x, y = 0xFF-fy, fx
		case 2:
			x, y = 0xFF-fx, 0xFF-fy
		case 3:
			x, y = fy, 0xFF-fx
		}
		objxloc = (loopx-16)<<8 + x
		objzloc = loopy<<8 + y
		objyloc = int(p.Z)
		dir := int(dirtab[(int(uint16(p.Heading))>>11+0x20-int((uint16(cPlayer.heading)+headmod[quad])>>11))%0x20])
		frame := 0
		if (dir+5)&7 >= 3 {
			frame = dir + 0x20
		}
		if pic, ok := critterPic(ITEM_FIGHTER_5D&0x3F, int(sqmod)*lighton, frame, 0); ok {
			rend.CritterSprite(objPoint(), pic)
		}
	}
}
