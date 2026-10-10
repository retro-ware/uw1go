package uw

import (
	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/host"
)

// Port of MOUSE.C: the cursor and its save-under, the one-deep click queue, the cursor
// regions, keyboard warping of the pointer, and the keyboard reader the input loop shares.
// Coordinates: x 0..319 left to right, y 0..199 from the bottom up.
//
// The DOS mouse driver gave relative motion. Here MOUSE() turns the host pointer's moves
// into the motion that puts the game's pointer under it, and reports none while the host
// pointer is still, so keyboard warps work as they did.

var (
	mouseX, mouseY int = 100, 100
	mouseShown     int
	mouseSaved     bool
	qButton        = -1
	qX, qY         int
	qRelease       int
	qTaken         bool
	lastButton     int
	inX0           = -1
	numRegions     int
	forceDepth     int
	warpX          = -1
	calledfrom3d   bool
	warpTime       uint32
	mouseFirst     = true

	mCurs        int
	rgnYlo       [20]int
	hotspotX     int
	hotspotY     int
	rgnLeft      [20]int
	conX0, conY0 int
	rgnX1        [20]int
	mWarpDx      int
	mWarpDy      int
	rgnYtop      [20]int
	curH         int
	maxX, maxY   int
	mCursorPic   int
	mWarpRate    int
	cursW        int
	warpY        int
	hitY0        int
	regIcon      [20]int
	cursStack    [3]int
	inX1, inY1   int
	m3dx, m3dy   int
	m3dw, m3dh   int
	m3dt         int
	mouseDx      int // *MouseDx, *MouseDy: the driver's last motion
	mouseDy      int
	mouseOn      = true
)

// MOUSE polls the driver: the motion since the last poll (see the file comment).
func MOUSE() {
	host.Busy()
	mouseDx, mouseDy = 0, 0
	if lookOn { // mouse-look: the captured pointer's motion
		mouseDx, mouseDy = host.LookDX, host.LookDY
		host.LookDX, host.LookDY = 0, 0
		host.MouseMoved = false
		return
	}
	if host.MouseMoved {
		host.MouseMoved = false
		mouseDx = host.MouseX - mouseX
		mouseDy = mouseY - (gr.H - 1 - host.MouseY)
	}
}

// MBUTTONS is the driver's buttons: bit 0 left, bit 1 right.
func MBUTTONS() int { return host.MouseButtons }

func initMouse() int {
	conX0, conY0 = 0, 0
	maxX, maxY = 0x13F, 0xC7
	setMouseData(ICON_CURSORS)
	gr.PenBlock = gr.NewBlock(0x28, 0x28)
	for i := range rgnLeft {
		rgnLeft[i] = 10000
	}
	return 0
}

func cursorRect() (int, int, int, int) {
	return mouseX - hotspotX, mouseY + hotspotY + 1,
		mouseX - hotspotX + cursW - 1, mouseY + hotspotY - curH + 1
}

func actualMhide() bool {
	if mouseSaved {
		gr.SetTheColor(0x101)
		gr.Rectangle(cursorRect())
	}
	return mouseSaved
}

func mouseShow() {
	mouseShown++
	if mouseShown == 1 {
		drawMouse()
	}
}

func mouseHide() {
	mouseShown--
	if mouseShown == 0 {
		if actualMhide() {
			mouseSaved = false
			gr.SetTheColor(1)
		}
	}
}

func mous3dSet(x, y, w, h int) {
	m3dx, m3dy, m3dw, m3dh = x, y, w, h
	if lookOn {
		lookCentre()
	}
}

func mousIn3dP() bool { return mouseCheckReg(m3dx, m3dy-m3dh, m3dx+m3dw, m3dy) }

func withFullWindow(f func()) {
	wl, wt, wr, wb := gr.WinLeft, gr.WinTop, gr.WinRight, gr.WinBottom
	gr.SetTheWindow(0, 0xC7, 0x13F, 0)
	f()
	gr.SetTheWindow(wl, wt, wr, wb)
}

func mous3dHide() {
	x0 := mouseX - cursW + hotspotX
	x1 := mouseX + cursW - hotspotX
	y1 := mouseY + curH - hotspotY
	y0 := mouseY - curH + hotspotY
	if m3dx+m3dw < x0 || x1 < m3dx || m3dy-m3dh > y1 || y0 > m3dy {
		m3dt = 0
		return
	}
	if x0 > m3dx && m3dx+m3dw > x1 && y1 < m3dy && m3dy-m3dh < y0 {
		m3dt = 2
	} else {
		m3dt = 1
		if inplist.mode != MODE_GAME {
			withFullWindow(mouseHide)
		}
	}
	if mouseShown == 1 {
		draw3dMouse()
	} else if mouseShown > 1 {
		mouseShown--
	}
}

func mous3dShow() {
	if m3dt == 1 && inplist.mode != MODE_GAME {
		withFullWindow(mouseShow)
	}
}

func mouseGetxy() (int, int) { return mouseX, mouseY }

func mouseQgetxy() (int, int) {
	if qTaken {
		return qX, qY
	}
	return mouseX, mouseY
}

func mouseClearQ() { qButton = -1 }

func flushKeys() {
	for KEY() != 0 {
	}
}

func mousePutxy(x, y int) {
	mouseHide()
	checkMouse()
	if mouseOn {
		MOUSE()
	}
	mouseX, mouseY = x, y
	mouseShow()
}

func mouseGetbut() int {
	b := mouseBtns()
	if b == 0 {
		qButton = -1
	}
	lastButton = b
	return b
}

// mouseRelease waits for the last button seen down to come up; with how set the game goes
// on updating meanwhile.
func mouseRelease(how bool) {
	qRelease = -1
	want := 2
	if lastButton == 1 {
		want = 1
	}
	for mouseGetInputSp()&(want|^3) == want && qRelease == -1 {
		if how {
			doChanges()
		}
		keyboardMouse(doKeyboardInput(true))
		moveMouse()
	}
	if qRelease == -1 {
		if b := mouseBtns(); b != 0 {
			qButton, qX, qY = b, mouseX, mouseY
		}
	}
}

// mouseDragged reports whether the pointer moved more than six pixels before the button
// came up.
func mouseDragged(how bool) bool {
	x0, y0 := mouseGetxy()
	moved := false
	for mouseGetbut() != 0 && !moved {
		if how {
			doChanges()
		}
		keyboardMouse(doKeyboardInput(true))
		moveMouse()
		x1, y1 := mouseGetxy()
		if abs(x1-x0)+abs(y1-y0) > 6 {
			moved = true
		}
	}
	return moved
}

func mouseConstrain(x0, y0, x1, y1 int) { conX0, conY0, maxX, maxY = x0, y0, x1, y1 }

func mouseFreereign() { conX0, conY0, maxX, maxY = 0, 0, 0x13F, 0xC7 }

func doMouseInput() int {
	moveMouse()
	var b int
	if qButton == -1 {
		qTaken = false
		b = mouseBtns()
	} else {
		if b = mouseBtns(); b == 0 {
			b = qButton
			qTaken = true
		}
		qButton = -1
	}
	lastButton = b
	if b == 0 {
		return -1
	}
	return b
}

// The game screen's keys (array): the game read no key events there but looked at the keys
// held every 30 ticks, so a tap between two looks was lost and a key held past one came
// twice, a toggle undone. A press now counts once, from its event; only the keys meant for
// holding (looking up and down, stepping, scrolling, the pointer keys) repeat while held,
// after 0x80 ticks (half a second), then every 30 as the looks came.
var (
	heldKey  byte // keyOn's index of the repeating key, or 0
	heldChar int
	heldNext uint32
)

func keyRepeats(ch int) bool {
	switch ch {
	case 'b', 'n':
		return true
	}
	return false
}

// keyArrayInput takes KEY's result (the character, the key in the high byte) and gives the
// character of a press, or of the held key's repeat, or 0.
func keyArrayInput(c int) int {
	now := GameTime()
	if c&0xFF != 0 {
		heldKey = 0
		if keyRepeats(c & 0xFF) {
			heldKey, heldChar, heldNext = byte(c>>8)&0x7F, c&0xFF, now+0x80
		}
		return c & 0xFF
	}
	if heldKey == 0 || keyOn[heldKey] == 0 {
		heldKey = 0
		return 0
	}
	if now < heldNext {
		return 0
	}
	heldNext = now + 30
	return heldChar
}

// doKeyboardInput reads a key and returns its input code (with KEY_SHIFT, KEY_ALT and
// KEY_CTRL), or -1.
// keyLShift is the code a lone left Shift gives (UltimaHacks' jump), unused by the game.
const keyLShift = 0xBF

var (
	lshiftWas   bool
	lshiftAlone bool // no other key went down while the left Shift was held
)

func doKeyboardInput(array bool) int {
	// UltimaHacks binds the left Shift alone. It has no character, so KEY never gives it; its
	// release is seen in keyOn (0x2A, its scan code) and given with the modifiers held, unless
	// another key went down meanwhile (Shift+/ is ? and no jump). It goes on the release, not
	// the press.
	now := keyOn[0x2A] != 0
	if now && !lshiftWas {
		lshiftAlone = true
	}
	released := !now && lshiftWas && lshiftAlone
	lshiftWas = now
	if released {
		c := keyLShift | KEY_SHIFT
		if altDown() {
			c |= KEY_ALT
		}
		if ctrlDown() {
			c |= KEY_CTRL
		}
		return c
	}
	c := KEY()
	if c>>8 != 0 && keyOn[0x2A] != 0 && !isModifierKey(c>>8&0x7F) {
		lshiftAlone = false // a key pressed while it was down: keys held from before are W's run
	}
	if array {
		c = keyArrayInput(c)
	}
	c &= 0xFF
	if c == 0 {
		return -1
	}
	if c&0x80 != 0 {
		if shiftDown() {
			c |= KEY_SHIFT
		}
	} else if capsLock&1 != 0 && isAlpha(c) {
		if !shiftDown() {
			c -= 0x20
		} else {
			c += 0x20
		}
	}
	if altDown() {
		c |= KEY_ALT
	}
	if ctrlDown() {
		c |= KEY_CTRL
	}
	return c
}

func isAlpha(c int) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func getInput(array bool) int {
	didMouseInput = true
	if mouseFirst {
		mouseFirst = false
		c := doMouseInput()
		if c < 0 {
			return doKeyboardInput(array)
		}
		return c
	}
	mouseFirst = true
	c := doKeyboardInput(array)
	if c < 0 {
		return doMouseInput()
	}
	return c
}

func mouseGetInput() int   { return getInput(false) }
func mouseGetInputSp() int { return getInput(true) }

func defineMouseRegion(x0, y0, x1, y1, id int) int {
	i := 0
	for ; i < 20; i++ {
		if rgnLeft[i] == 10000 {
			break
		}
	}
	if i == 20 {
		return -1
	}
	rgnLeft[i], rgnX1[i], rgnYtop[i], rgnYlo[i], regIcon[i] = x0, x1, y1, y0, id
	if i >= numRegions {
		numRegions = i + 1
	}
	checkMouse()
	return i
}

func undefineMouseRegion(handle int) {
	if handle < 0 || handle >= numRegions {
		return
	}
	rgnLeft[handle] = 10000
	if regIcon[handle] == mCursorPic {
		inX0 = 10000
	}
	if numRegions-1 == handle {
		i := numRegions - 2
		for ; i >= 0; i-- {
			if rgnLeft[i] != 10000 {
				break
			}
		}
		numRegions = i + 1
	}
	checkMouse()
}

func forceMouseCursor(id int) {
	if forceDepth != 3 {
		mouseHide()
		cursStack[forceDepth] = mCurs
		forceDepth++
		setMouseData(id)
		mouseShow()
	}
}

func unforceMouseCursor(how int) {
	if how&1 != 0 {
		mouseHide()
	}
	forceDepth--
	if forceDepth < 0 {
		cursStack[0] = ICON_CURSORS
		forceDepth = 0
	}
	setMouseData(cursStack[forceDepth])
	checkMouse()
	if how&2 != 0 {
		mouseShow()
	}
}

func mouseCheckReg(x0, y0, x1, y1 int) bool {
	dy := (curH + 1) >> 1
	dx := (cursW + 1) >> 1
	return y0-dy <= mouseY && y1+dy >= mouseY && x0-dx <= mouseX && x1+dx >= mouseX
}

func setMouseData(id int) {
	actualMhide()
	cursW, curH = bmSize(slotPic(grsWhich1(id)))
	hotspotX = cursW/2 - 1
	hotspotY = curH / 2
	mCursorPic = id
	mCurs = id
	if mouseSaved {
		mousReSave()
	}
}

func checkMouse() {
	if forceDepth > 0 {
		return
	}
	if inX0 != -1 && mouseX >= inX0 && mouseX <= inX1 && mouseY >= hitY0 && mouseY <= inY1 {
		return
	}
	i := 0
	for ; i < numRegions; i++ {
		if rgnLeft[i] <= mouseX && rgnX1[i] >= mouseX && rgnYlo[i] <= mouseY && rgnYtop[i] >= mouseY {
			inX0, inX1, hitY0, inY1 = rgnLeft[i], rgnX1[i], rgnYlo[i], rgnYtop[i]
			setMouseData(regIcon[i])
			break
		}
	}
	if inX0 != -1 && i == numRegions {
		inX0 = -1
		setMouseData(ICON_CURSORS)
	}
}

// moveMouse moves the pointer by the driver's motion or a step of a keyboard warp, and
// redraws the cursor.
func moveMouse() {
	dx, dy := 0, 0
	if mouseOn {
		MOUSE()
		dx, dy = mouseDx, mouseDy
	}
	if dx == 0 && dy == 0 {
		if warpX < 0 {
			return
		}
		if GameTime()-warpTime < 10 {
			return
		}
		mWarpRate = min(mWarpRate+8, 40)
		warpTime = GameTime()
		axes := 3
		for n := mWarpRate; n > 0 && axes != 0; n-- {
			if mouseX+dx > warpX-5 && mouseX+dx < warpX+5 && axes&1 != 0 {
				axes ^= 1
				dx = warpX - mouseX
			} else if axes&1 != 0 {
				dx += mWarpDx
			}
			if mouseY-dy > warpY-5 && mouseY-dy < warpY+5 && axes&2 != 0 {
				axes ^= 2
				dy = mouseY - warpY
			} else if axes&2 != 0 {
				dy += mWarpDy
			}
		}
	} else {
		warpX = -1
	}
	wl, wt, wr, wb := gr.WinLeft, gr.WinTop, gr.WinRight, gr.WinBottom
	if calledfrom3d {
		gr.SetTheWindow(0, 0xC7, 0x13F, 0)
	}
	if actualMhide() {
		mouseSaved = false
	}
	if lookOn && inplist.mode&0x11 == 0 {
		// the view gone without changeScreen (realDeath sets the mode itself, for the main
		// menu): mouse-look off, remembered for the view's return, so the pointer moves
		lookWas = true
		mouseLookSet(false)
	}
	if lookOn { // mouse-look turns the view instead
		mouseLookStep(dx, dy)
		dx, dy = 0, 0
	}
	mouseX += dx
	mouseY -= dy
	mouseX = max(conX0, min(mouseX, maxX))
	mouseY = max(conY0, min(mouseY, maxY))
	if warpX == mouseX && warpY == mouseY {
		warpX = -1
	}
	checkMouse()
	if mouseShown > 0 {
		drawMouse()
	}
	if calledfrom3d {
		gr.SetTheWindow(wl, wt, wr, wb)
	}
}

// mousQUp keeps the pointer moving while the game is busy and remembers a press.
func mousQUp(from3d bool) {
	calledfrom3d = from3d
	moveMouse()
	calledfrom3d = false
	if b := mouseBtns(); b != 0 {
		if qButton == -1 {
			qButton, qX, qY = b, mouseX, mouseY
		}
	} else {
		qRelease = 0
	}
}

func mousReSave() {
	gr.SetTheColor(0x100)
	gr.Rectangle(cursorRect())
	mouseSaved = true
}

func mousReSave3d() {
	switch {
	case m3dt == 2:
		gr.FbufSetcolor(0x100)
		gr.Rectangle(mouseX-hotspotX-m3dx, mouseY+hotspotY-m3dy+m3dh,
			mouseX+cursW-hotspotX-m3dx-1, mouseY-curH+hotspotY-m3dy+m3dh)
	case m3dt == 1 && inplist.mode == MODE_GAME:
		gr.ShowClip = true
		gr.PenBlock.CopyFB(mouseX-hotspotX-m3dx, mouseY+hotspotY-m3dy+m3dh, cursW, curH+1)
		gr.ShowClip = false
	default:
		return
	}
	mouseSaved = true
}

func drawMouse() {
	mousReSave()
	gr.ShowClip, gr.Transparency = true, true
	picToScreen(mCursorPic, mouseX-hotspotX, mouseY+hotspotY)
	gr.ShowClip, gr.Transparency = false, false
	gr.SetTheColor(0)
}

func draw3dMouse() {
	mousReSave3d()
	x := mouseX - hotspotX - m3dx
	y := mouseY + hotspotY - m3dy + m3dh - 1
	gr.ShowClip, gr.Transparency = true, true
	picToFbuf(mCursorPic, x, y)
	gr.ShowClip, gr.Transparency = false, false
}

func warpMouse(x, y int) {
	mWarpRate = 1
	x = max(conX0, min(x, maxX))
	warpX = x
	y = max(conY0, min(y, maxY))
	warpY = y
	mWarpDx = -1
	if x > mouseX {
		mWarpDx = 1
	}
	mWarpDy = 1
	if y > mouseY {
		mWarpDy = -1
	}
}

// keyboardMouse moves the pointer from the keyboard: Tab and Shift+Tab hop between the view
// and the panels.
func keyboardMouse(key int) {
	x, y := mouseX, mouseY
	switch key {
	case KEY_SHIFT | KEY_BACKTAB:
		if warpX >= 0 || lookOn { // mouse-look holds the pointer
			return
		}
		y = 0x82
		switch {
		case mouseX < 0x2E:
			x = 0x118
		case mouseX < 0xE1:
			x = 0x14
		default:
			x = 0x82
		}
	case 9:
		if warpX >= 0 || lookOn {
			return
		}
		y = 0x82
		switch {
		case mouseX < 0x2E:
			x = 0x82
		case mouseX < 0xE1:
			x = 0x118
		default:
			x = 0x14
		}
	default:
		return
	}
	if x != mouseX || y != mouseY {
		warpMouse(x, y)
	}
}

// mouseBtns is the buttons held.
func mouseBtns() int {
	if !mouseOn {
		return 0
	}
	return MBUTTONS()
}
