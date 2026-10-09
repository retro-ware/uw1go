package uw

import (
	"strconv"

	"github.com/felix/uw1go/internal/gr"
)

// Port of SCROLLIO.C: the message scroll at the bottom of the game screen and the
// conversation scroll: printing with word wrap and escapes, scrolling, the animated edges,
// the [MORE] prompt, typed answers and yes-or-no questions. The printing routines work on
// NUL-terminated byte buffers, as the original does, since they cut and restore their
// text in place.

// Scroll is a message scroll's state.
type Scroll struct {
	x0, y0      int
	top, bottom int
	curX, curY  int
	left, lastY int
	morePending bool
	startLine   int
	fontColor   int
}

var (
	scroll        *Scroll
	mouseInScroll bool
	answerStart   int

	mainScroll = Scroll{0x1E, 1, 0x0F, 0x131, 0x0F, 0x1E, 0x0F, 0x1E, false, 0, 0x2E}
	npcScroll  = Scroll{0x94, 0x44, 0x38, 0xDB, 0x3B, 0x92, 0x3B, 0x92, false, 0, 0x2E}

	startLine     int
	scrollMode    int
	menusActive   bool
	didMouseInput bool
	clickTime     uint32
	edgePhase     int
	convEdgePhase int
	scrollEsc     = true
	// wrapNewline is scroll_wrap's "\n" literal, which scroll_print3 cuts to "" the first
	// time it prints it.
	wrapNewline = []byte("\n\x00")
)

func cstrlen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return len(b)
}

func cstrrchr(b []byte, c byte) int {
	n := cstrlen(b)
	for i := n - 1; i >= 0; i-- {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func cstrchr(b []byte, from int, c byte) int {
	for i := from; i < len(b) && b[i] != 0; i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func cstring(s string) []byte { return append([]byte(s), 0) }

func fontHeight() int {
	if gr.CurFont == nil {
		return 0
	}
	return gr.CurFont.Height
}

func cwidth(b []byte) int { return gr.StringWidth(string(b[:cstrlen(b)])) }

func initScroll() {
	scroll = &mainScroll
	scrollMode = 0
	drawScroll(0x0F, 0x1E, 0x131, 1, false)
	drawEdges()
}

func setMouseIn() {
	mouseInScroll = mouseCheckReg(scroll.top, scroll.y0, scroll.bottom, scroll.x0)
}

func doMainScroll() {
	scrollMode = 0
	didMouseInput = true
	scroll = &mainScroll
}

func doNpcScroll() {
	scrollMode = 1
	didMouseInput = true
	scroll = &npcScroll
}

func doPlayScroll() {
	scrollMode = 2
	didMouseInput = true
	scroll = &mainScroll
}

// scrollWait waits for the input to change, or ticks of the clock when ticks is not 0.
func scrollWait(ticks int, mouse bool) {
	mouseRelease(true)
	key := mouseGetInput()
	end := GameTime() + uint32(ticks)
	if mouseInScroll && mouse {
		mouseShow()
	}
	for mouseGetInput() == key {
		if ticks != 0 && GameTime() > end {
			break
		}
	}
	mouseRelease(true)
	setMouseIn()
	if mouseInScroll && mouse {
		mouseHide()
	}
}

func drawEdges() {
	picToScreen(edgePhase+ICON_SCRLEDGE, 0x0B, 0x1E)
	picToScreen(edgePhase+ICON_SCRLEDGE+5, 0x132, 0x1E)
	edgePhase++
	if edgePhase == 5 {
		edgePhase = 0
	}
}

func drawConvEdges() {
	for i := 0; i < 3; i++ {
		picToScreen(convEdgePhase+ICON_SCRLEDGE+0xA, 0x34, 0x94-i*0x1B)
		picToScreen(convEdgePhase+ICON_SCRLEDGE+0x10, 0xDC, 0x94-i*0x1B)
	}
	convEdgePhase++
	if convEdgePhase == 6 {
		convEdgePhase = 0
	}
}

func scrollUp(n int) {
	h := fontHeight()
	gr.VCopy(scroll.top, scroll.lastY-h, scroll.bottom-scroll.top+1, scroll.lastY-h-n+1, scroll.top, scroll.lastY)
	gr.SetTheColor(0x2A)
	gr.Rectangle(scroll.top, scroll.curY, scroll.bottom, n+1)
	if scroll == &mainScroll {
		setScreenFrame(SCR_DRAGON, 1)
		drawEdges()
	} else {
		drawConvEdges()
	}
}

func scrollMore() {
	y := scroll.curY - fontHeight()
	scrollUp(y)
	y = scroll.curY
	color := gr.ForegroundColor
	gr.ForegroundColor = 0x60
	gr.StringToScreen("[MORE]", scroll.left, y)
	scrollWait(0, true)
	gr.SetTheColor(0x2A)
	gr.Rectangle(scroll.left, y, scroll.bottom, scroll.y0)
	gr.ForegroundColor = color
	startLine = scroll.startLine - 1
	scroll.curX = scroll.left
}

// scrollPrint prints s to the current scroll and returns its line count.
func scrollPrint(s string) int {
	if scrmode != MODE_GAME && scrmode != MODE_CONV {
		return -1
	}
	setMouseIn()
	if mouseInScroll {
		mouseHide()
	}
	if menusActive && scrollMode != 1 {
		menusActive = false
		scrollClear(false)
	}
	startLine = scroll.startLine
	didMouseInput = false
	gr.ForegroundColor = byte(scroll.fontColor)
	gr.BackgroundColor = 0x2A
	src := []byte(s)
	for len(src) > 0x31 {
		cp := make([]byte, 0x32)
		copy(cp, src[:0x31])
		found := cstrrchr(cp, ' ')
		if found <= 0 {
			found = 0x31
		}
		cp[found] = 0
		scrollPrint1(cp, 1)
		src = src[found:]
	}
	scrollPrint1(cstring(string(src)), 0)
	clickTime = GameTime()
	if mouseInScroll {
		mouseShow()
	}
	return scroll.startLine
}

// scrollPrint1 splits at backslash escapes, so each escape starts a piece.
func scrollPrint1(text []byte, flag int) {
	di := 0
	for {
		found := cstrchr(text, di+1, '\\')
		if found < 0 {
			break
		}
		text[found] = 0
		at := func(i int) byte {
			if i < len(text) {
				return text[i]
			}
			return 0
		}
		if at(found+2) != 0 || at(found+1) == 'm' {
			scrollPrint2(text[di:], 1)
		} else {
			scrollPrint2(text[di:], flag)
		}
		text[found] = '\\'
		di = found
	}
	scrollPrint2(text[di:], flag)
}

// scrollPrint2 splits at newlines, keeping each newline at the end of its piece.
func scrollPrint2(text []byte, flag int) {
	di := 0
	for {
		si := cstrchr(text, di, '\n')
		if si < 0 || si+1 >= len(text) || text[si+1] == 0 {
			break
		}
		saved := text[si+1]
		text[si+1] = 0
		scrollPrint3(text[di:], 1)
		text[si+1] = saved
		di = si + 1
	}
	scrollPrint3(text[di:], flag)
}

// scrollPrint3 prints one piece.
func scrollPrint3(text []byte, flag int) {
	si := text
	code := 0
	if scrollEsc && cstrlen(si) > 0 && si[0] == '\\' {
		var e byte
		if len(si) > 1 {
			e = si[1]
		}
		si = si[min(2, len(si)):]
		switch e {
		case 'p':
			code = 0x190
			fallthrough
		case 'P':
			code += 0xC8
			scrollWait(code, true)
		case 'm':
			scrollMore()
		case '0':
			gr.ForegroundColor = 0x2E
		case '1':
			gr.ForegroundColor = 0x26
		case '2':
			gr.ForegroundColor = 0xF1
		case '3':
			gr.ForegroundColor = 0x60
		case '4':
			gr.ForegroundColor = 0xB4
		case '5':
			gr.ForegroundColor = 0xC4
		case '6':
			gr.ForegroundColor = 0xD4
		}
		scroll.fontColor = int(gr.ForegroundColor)
	}
	if scroll.morePending {
		h := fontHeight()
		y := scroll.curY - h
		if startLine-flag < 0 {
			if y-h < scroll.y0-1 {
				scrollMore()
				y = scroll.curY
			} else {
				scroll.startLine++
			}
		} else {
			if y-h < scroll.y0-1 {
				scrollUp(y)
				y = scroll.curY
				startLine--
			} else {
				scroll.startLine++
			}
		}
		scroll.curY = y
		scroll.curX = scroll.left
		scroll.morePending = false
	}
	width := scroll.curX + cwidth(si)
	if scroll.bottom <= width {
		scrollWrap(si, flag)
		return
	}
	n := cstrlen(si)
	if n > 0 && si[n-1] == '\n' {
		si[n-1] = 0
		scroll.morePending = true
	}
	gr.StringToScreen(string(si[:cstrlen(si)]), scroll.curX, scroll.curY)
	scroll.curX += cwidth(si)
}

// scrollWrap breaks the text once, at the last space that fits, or by cutting characters
// off the end of a word longer than the line.
func scrollWrap(text []byte, flag int) {
	si := cstrrchr(text, ' ')
	for si >= 0 {
		text[si] = 0
		if scroll.curX+cwidth(text) < scroll.bottom {
			text[si] = ' '
			break
		}
		next := cstrrchr(text, ' ')
		text[si] = ' '
		si = next
	}
	if si < 0 {
		si = cstrlen(text) - 1
		saved := text[si]
		si--
		for {
			text[si+1] = saved
			si--
			saved = text[max(si, 0)]
			if si >= 0 {
				text[si] = 0
			}
			if !(si > 0 && scroll.curX+cwidth(text) >= scroll.bottom) {
				break
			}
		}
		if si <= 0 {
			scrollPrint3(wrapNewline, 1)
			scrollPrint3(text, flag)
			return
		}
		text[si] = saved
	}
	saved := text[si]
	saved2 := text[si+1]
	text[si] = '\n'
	text[si+1] = 0
	scrollPrint3(text, 1)
	text[si] = saved
	text[si+1] = saved2
	if saved == ' ' {
		si++
	}
	scrollPrint3(text[si:], flag)
}

// drawScroll clears a scroll box; with flag a frame in colour F1h is drawn first.
func drawScroll(x, y, w, h int, flag bool) {
	if flag {
		gr.SetTheColor(0xF1)
		gr.Rectangle(x+0xE, y-1, w-0xF, h+1)
	}
	gr.SetTheColor(0x2A)
	gr.Rectangle(x, y, w, h)
}

func scrollClear(redraw bool) {
	if redraw {
		setMouseIn()
		if mouseInScroll {
			mouseHide()
		}
	}
	gr.SetTheColor(0x2A)
	gr.Rectangle(scroll.top, scroll.x0, scroll.bottom, scroll.y0)
	scroll.curY = scroll.lastY
	scroll.curX = scroll.left
	scroll.morePending = false
	scroll.startLine = 0
	if scroll == &mainScroll {
		setScreenFrame(SCR_DRAGON, 1)
		drawEdges()
	} else {
		drawConvEdges()
	}
	if redraw && mouseInScroll {
		mouseShow()
	}
}

func eraseAnswer() (int, int) {
	x := answerStart
	y := scroll.curY
	doMainScroll()
	gr.ForegroundColor = byte(scroll.fontColor)
	gr.SetTheColor(0x2A)
	gr.Rectangle(x, y, scroll.curX, y-fontHeight()+1)
	scroll.curX = answerStart
	return x, y
}

func wdReplace(n int) {
	eraseAnswer()
	scrollPrint(strconv.Itoa(n))
}

func wdBool(yes bool) {
	eraseAnswer()
	if yes {
		scrollPrint("Yes")
	} else {
		scrollPrint("No")
	}
}

// wdialog reads a line typed into the scroll after prompt (or ">"). initial, when given,
// is shown and replaced by the first character typed. It returns the key that ended the
// input and the text.
func wdialog(prompt string, initial *string, anychar bool, maxlen int) (int, string) {
	maxlen = min(maxlen, 50)
	doMainScroll()
	gr.ForegroundColor = byte(scroll.fontColor)
	var width int
	if prompt != "" {
		width = scroll.bottom - (gr.StringWidth(prompt) + scroll.left) - 20
		scrollPrint(prompt)
	} else {
		width = scroll.bottom - (gr.StringWidth(">") + scroll.left) - 20
		scrollPrint(">")
	}
	answerStart = scroll.curX
	var text []byte
	pos := 0
	if initial != nil {
		scrollEsc = false
		scrollPrint(*initial)
		scrollEsc = true
		text = []byte(*initial)
		pos = -len(*initial)
	}
	apos := func() int { return abs(pos) }
	blink := 3000
	x := answerStart + gr.StringWidth(string(text))
	y := scroll.curY
	h := fontHeight()
	mouseHide()
	mouseRelease(false)
	var key int
	for {
		key = mouseGetInput()
		if key == 13 || key == 27 || key == 1 || key == 2 || key == 3 {
			break
		}
		x = answerStart + gr.StringWidth(string(text[:min(apos(), len(text))]))
		if blink >= 2000 && blink < 4000 {
			gr.SetTheColor(scroll.fontColor)
			gr.Rectangle(x, y, x+4, y-h+1)
		} else if blink == 4000 {
			gr.SetTheColor(0x2A)
			blink = 0
			gr.Rectangle(x, y, x+4, y-h+1)
			if len(text) > pos {
				gr.StringToScreen(string(text), answerStart, scroll.curY)
			}
		}
		blink++
		switch key {
		case -1:
			continue
		case KEY_CTRL | 'k':
			pos = apos()
			text = text[:pos]
		case KEY_HOME, KEY_GHOME, KEY_CTRL | 'a':
			pos = 0
		case KEY_END, KEY_GEND, KEY_CTRL | 'e':
			pos = len(text)
		case KEY_RIGHT, KEY_GRIGHT, KEY_CTRL | 'f':
			pos = apos()
			if len(text) > pos {
				pos++
			}
		case KEY_LEFT, KEY_GLEFT, KEY_CTRL | 'b':
			pos = apos()
			if pos > 0 {
				pos--
			}
		case KEY_BACKSPACE:
			pos = apos()
			if pos > 0 {
				text = append(text[:pos-1], text[pos:]...)
				pos--
			}
		case KEY_DEL, KEY_CTRL | 'd':
			pos = apos()
			if len(text) > pos {
				text = append(text[:pos], text[pos+1:]...)
			}
		default:
			if pos < 0 {
				pos = 0
				text = text[:0]
			}
			if key >= 0x20 && key <= 0x7E && gr.StringWidth(string(text)) < width &&
				len(text) < maxlen && (anychar || key >= '0' && key <= '9') {
				text = append(text, 0)
				copy(text[pos+1:], text[pos:])
				text[pos] = byte(key)
				pos++
			}
		}
		gr.SetTheColor(0x2A)
		gr.Rectangle(answerStart, scroll.curY, scroll.bottom, scroll.curY-h+1)
		gr.ForegroundColor = byte(scroll.fontColor)
		gr.StringToScreen(string(text), answerStart, scroll.curY)
	}
	if blink >= 2000 {
		gr.SetTheColor(0x2A)
		gr.Rectangle(x, y, x+4, y-h+1)
		if len(text) > pos {
			gr.StringToScreen(string(text), answerStart, scroll.curY)
		}
	}
	mouseShow()
	if key == KEY_ESC {
		result := ""
		if initial != nil {
			result = *initial
		}
		gr.SetTheColor(0x2A)
		gr.Rectangle(answerStart, scroll.curY, scroll.bottom, scroll.curY-h+1)
		scroll.curX = answerStart
		scrollPrint("-")
		return key, result
	}
	scroll.curX = x
	return key, string(text)
}

// wyorn asks a yes or no question (the text, or game string id); *answer is the default
// and the result. It returns the key that ended it, -1 for Escape.
func wyorn(question string, id int, answer *bool) int {
	cur, sel := *answer, *answer
	doMainScroll()
	gr.ForegroundColor = byte(scroll.fontColor)
	if question == "" {
		gameSprint(id)
	} else {
		scrollPrint(question)
	}
	answerStart = scroll.curX
	if *answer {
		scrollPrint("Yes")
	} else {
		scrollPrint("No")
	}
	mouseHide()
	mouseRelease(false)
	var key int
	for {
		key = mouseGetInput()
		if key == 13 || key == 27 || key == 1 || key == 2 || key == 3 {
			break
		}
		switch key {
		case 'Y', 'y':
			sel = true
		case 'N', 'n':
			sel = false
		default:
			continue
		}
		if sel != cur {
			wdBool(sel)
			cur = sel
		}
	}
	mouseShow()
	if key == KEY_ESC {
		wdBool(false)
		*answer = false
		return -1
	}
	*answer = cur
	return key
}
