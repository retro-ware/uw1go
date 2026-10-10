package uw

import "github.com/memmaker/uw1go/internal/gr"

// The help screen, which the DOS game does not have: F1, ? or Ctrl+H in the game shows the
// controls and every key on the map's blank parchment, three pages. A screen of its own
// (MODE_HELP, editorDispatch's fourth), entered and left as the map is, so the game stands
// still meanwhile. Space, Enter, PgDn, the right arrows or a left click turn to the next
// page, PgUp, Backspace, the left arrows or a right click back; Esc, F1, ?, Ctrl+H or the
// parchment's CLOSE scroll return to the game. F1's old meaning, the options panel, moves to
// Esc.

const MODE_HELP = 0x20

const (
	helpInk   = 0x2F // the body text: dark brown, on the parchment's 0x3B
	helpRed   = 0x26 // keys and headings
	helpLeft  = 16   // the text's left edge
	helpLineH = 7    // FONT5X6P's 6 rows and one between
)

// helpRow is a line of a page: a heading ("#" key), a key and what it does, or (no key) a
// line of text.
type helpRow struct{ key, text string }

type helpPage struct {
	title string
	cols  [][]helpRow // one column, or two side by side
	keyW  []int       // each column's key width (0: running text)
}

var helpPages = []helpPage{
	{title: "Getting Started", keyW: []int{0}, cols: [][]helpRow{{
		{"#", "Moving"},
		{"", "Walk with the keys: W runs, X walks, S backs up, A"},
		{"", "and D slide, the grey arrow keys turn. Shift jumps."},
		{"", "Mouse look (` or ^, left of 1) aims with the mouse."},
		{"#", "Acting"},
		{"", "Left-click a thing in the view to act on it: talk to"},
		{"", "people, take items, open doors and chests, pull"},
		{"", "levers. Right-click or Q only looks, E acts like a"},
		{"", "left click. The icons on the left force one action."},
		{"#", "Fighting"},
		{"", "F5 draws your weapon or puts it away. While it is out,"},
		{"", "the left button slashes, the right thrusts and Space"},
		{"", "bashes. Hold one to charge the blow, let go to strike."},
		{"#", "Inventory"},
		{"", "Left-click an item to pick it up or put it down,"},
		{"", "right-click it to look at it. Carrying something, a"},
		{"", "left click in the view drops or throws it."},
	}}},
	{title: "Keys: Moving and Acting", keyW: []int{56, 44}, cols: [][]helpRow{{
		{"#", "Moving"},
		{"W", "run forward"},
		{"X", "walk forward"},
		{"S", "walk back"},
		{"A  D", "slide left, right"},
		{"grey < >", "turn"},
		{"Ctrl+grey", "step one pace"},
		{"Shift", "jump"},
		{"Ctrl+Shift", "long jump"},
		{"Shift Ctrl", "fly up, down"},
		{"1  3", "look up, down"},
		{"grey ^ v", "look up, down"},
		{"2  pad 5", "look ahead"},
		{"`  ^", "mouse look on, off"},
		{"", "(the key left of 1)"},
		{"Alt+`  Alt+^", "mouse look axes"},
	}, {
		{"#", "Acting"},
		{"Space", "bash (hold)"},
		{".  ;  P", "thrust, slash, bash"},
		{"Q", "look at pointer"},
		{"E", "act at pointer"},
		{"F2", "talk mode"},
		{"F3", "get mode"},
		{"F4", "look mode"},
		{"F5", "draw, sheathe weapon"},
		{"F6", "use mode"},
		{"F9", "track"},
		{"F10", "sleep"},
	}}},
	{title: "Keys: Magic, Panels, Game", keyW: []int{52, 40}, cols: [][]helpRow{{
		{"#", "Magic and panels"},
		{"R", "rune bag"},
		{"F  F7", "statistics"},
		{"C", "close container"},
		{"V  B", "scroll inventory"},
		{"G", "hunger, time of day"},
		{"H", "health and mana"},
		{"F8", "cast the spell"},
		{"Ctrl+Alt+", "letter: that rune"},
		{"", "on the shelf;"},
		{"", "Backspace clears,"},
		{"", "Space casts"},
		{"#", "Map (Z)"},
		{"S  W", "level up, down"},
		{"C", "your level"},
		{"Esc", "back to the game"},
	}, {
		{"#", "Game"},
		{"Esc", "options"},
		{"Ctrl+S", "save"},
		{"Ctrl+R", "restore"},
		{"Ctrl+M", "music"},
		{"Ctrl+F", "sound"},
		{"Ctrl+D", "detail"},
		{"Ctrl+Q", "quit"},
		{"F1  ?", "this help"},
		{"Ctrl+H", "this help"},
	}}},
}

var (
	helpPageNo     int
	helpMouse      int
	helpRegistered bool
)

// helpKeys binds the help keys in the game and, once, the help screen's own.
func helpKeys() {
	for _, k := range []int{KEY_F1, '?', KEY_CTRL | 'h'} {
		inputAddkey(k, MODE_HELP, MODE_GAME, func(a any) { newscr(a.(int)) })
	}
	inputAddkey(KEY_ESC, 5, MODE_GAME, ih(dealWithIcons)) // the options panel, F1's before
}

// HelpScreen opens the help screen (editorDispatch[3]'s bit 1).
func HelpScreen() {
	if !helpRegistered {
		for _, k := range []int{' ', KEY_ENTER, KEY_PGDN, KEY_RIGHT, KEY_GRIGHT} {
			inputAddkey(k, 1, MODE_HELP, ih(helpTurn))
		}
		for _, k := range []int{KEY_PGUP, KEY_BACKSPACE, KEY_LEFT, KEY_GLEFT} {
			inputAddkey(k, -1, MODE_HELP, ih(helpTurn))
		}
		for _, k := range []int{KEY_ESC, KEY_F1, '?', KEY_CTRL | 'h'} {
			inputAddkey(k, MODE_GAME, MODE_HELP, func(a any) { newscr(a.(int)) })
		}
		helpRegistered = true
	}
	helpPageNo = 0
	helpMouse = inputAddmouse(0, 0, 0x13F, 0xC7, nil, MODE_HELP, func(any) { helpClick() })
	mouseFreereign()
	forceMouseCursor(ICON_CURSORS)
	showHelpPage()
}

// ExitHelp leaves the help screen (editorDispatch[3]'s bit 15).
func ExitHelp() {
	mouseHide()
	inputDel(helpMouse)
	unforceMouseCursor(0)
	grfxClear()
	grfxQuikpal(PAL_GAME)
	mouseShow()
}

func helpScr() { loopMusicMaybe() }

func helpTurn(by int) {
	helpPageNo = (helpPageNo + by + len(helpPages)) % len(helpPages)
	showHelpPage()
}

// helpClick: the CLOSE scroll closes, a left click elsewhere turns on, a right click back
// (inplist.y counts up from the bottom of the screen).
func helpClick() {
	x, y, cmd := inplist.x, inplist.y, inplist.cmd
	mouseRelease(true)
	switch {
	case x >= 0x106 && x <= 0x134 && y >= 0x1B && y <= 0x31:
		newscr(MODE_GAME)
	case cmd&2 != 0:
		helpTurn(-1)
	default:
		helpTurn(1)
	}
}

// showHelpPage draws the current page on the blank map, its level number, compass and
// ERASE button covered with the parchment beside them.
func showHelpPage() {
	buf := make([]byte, 0xFA00)
	if !bltfromdrive("DATA\\blnkmap.byt", buf) {
		newscr(MODE_GAME)
		return
	}
	for _, r := range [][4]int{{0xF6, 4, 0x134, 0x36}, {0x10A, 0x70, 0x134, 0x94}} { // x0, y0, x1, y1
		for y := r[1]; y <= r[3]; y++ {
			copy(buf[y*0x140+r[0]:y*0x140+r[2]+1], buf[y*0x140+r[0]-0x6E:])
		}
	}
	mouseHide()
	gr.GrSoftPageFlip()
	gr.SetTheWindow(0, 0xC7, 0x13F, 0)
	gr.Show(0, 0xC7, buf, 0xC8, 0x140, 0, 0)
	p := &helpPages[helpPageNo]
	top := func(y int) int { return 0xC7 - y } // the text's top, y down the screen
	grfxLoadFont("fontbig.sys")
	gr.ForegroundColor = helpRed
	gr.StringToScreen(p.title, helpLeft, top(9))
	grfxLoadFont("font5x6p.sys")
	colW := (0x12C - helpLeft) / len(p.cols) // the full width: the decorations are covered
	for c, col := range p.cols {
		x, y := helpLeft+c*colW, 0x20
		for _, r := range col {
			switch {
			case r.key == "#":
				if y > 0x20 {
					y += 3
				}
				gr.ForegroundColor = helpRed
				gr.StringToScreen(r.text, x, top(y))
				y += helpLineH + 1
				continue
			case r.key != "":
				gr.ForegroundColor = helpRed
				gr.StringToScreen(r.key, x, top(y))
			}
			gr.ForegroundColor = helpInk
			gr.StringToScreen(r.text, x+p.keyW[c], top(y))
			y += helpLineH
		}
	}
	gr.ForegroundColor = helpInk
	foot := "Page " + string(rune('1'+helpPageNo)) + " of " + string(rune('0'+len(helpPages))) +
		".  Space: next page.  Esc: back to game."
	gr.StringToScreen(foot, helpLeft, top(0xB8))
	grfxQuikpal(PAL_MAP)
	gr.GrPageFlip()
	gr.GrSoftPageFlip()
	gr.CopyHiddenToVisible()
	mouseShow()
}
