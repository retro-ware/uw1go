package uw

import "github.com/memmaker/uw1go/internal/host"

// Mouse look (UltimaHacks' toggleMouseLook, setMouseLookState,
// mouseLookOrMoveCursor, orientMouseLook and setInterfaceMode, as underworld-exhumed's
// MOUSE.C has them): ` turns it on and off in the 3D view. While it is on the host captures
// the pointer, its motion turns the view instead of moving the cursor, which stays at the
// view's centre as a crosshair, where clicks and the q and e keys act. It goes off while the
// 3D view gives way (the map, a conversation) and comes back with it.

const (
	lookInvertY = 1 << iota
	lookInvertX
)

var (
	lookOn       bool
	lookWas      = true // it was on when the 3D view gave way; it starts on
	lookX, lookY int    // the pointer before it went on
	lookOrient   int    // lookInvertY, lookInvertX
)

func mouseLookSet(on bool) {
	if on == lookOn {
		return
	}
	lookOn = on
	host.MouseLook = on
	host.LookDX, host.LookDY = 0, 0
	if on {
		if actualMhide() { // the cursor off the screen where it was
			mouseSaved = false
		}
		lookX, lookY = mouseX, mouseY
		lookCentre()
		mousReSave()
		if GameInputMode == 0 { // not while carrying something: its picture stays
			setMouseData(ICON_CURSORS)
		}
	} else {
		mouseX, mouseY = lookX, lookY // the view's next frame clears the crosshair
		mousReSave()
		inX0 = -1
		checkMouse()
		drawMouse()
	}
}

// lookCentre puts the crosshair in the view's centre (m3dy is its bottom edge), again when
// the view is placed: mouse look is on from the start, before the game screen has a view.
func lookCentre() {
	mouseX = m3dx + m3dw/2
	mouseY = m3dy - m3dh/2
	inX0, inX1 = mouseX, mouseX // a region of one point, so checkMouse keeps the cursor
	hitY0, inY1 = mouseY, mouseY
}

// mouseLookToggle is the ` key.
func mouseLookToggle() {
	mouseLookSet(!lookOn)
	if lookOn {
		scrollPrint("Mouse look enabled.\n")
	} else {
		scrollPrint("Mouse look disabled.\n")
	}
}

// mouseLookOrient is Alt+`: the next of the four ways round the axes go.
func mouseLookOrient() {
	lookOrient = (lookOrient + 1) & 3
	x, y := "+", "+"
	if lookOrient&lookInvertX != 0 {
		x = "-"
	}
	if lookOrient&lookInvertY != 0 {
		y = "-"
	}
	scrollPrint("Mouse-look orientation: " + x + "X " + y + "Y\n")
}

// mouseLookScreen is called from changeScreen around the mode change: off while the 3D view
// gives way, and back as it was when it returns (the modes with a bit of 0x11, as
// setInterfaceMode tests).
func mouseLookScreen(leaving bool) {
	if inplist.mode&0x11 == 0 {
		return
	}
	if leaving {
		lookWas = lookOn
		mouseLookSet(false)
	} else {
		mouseLookSet(lookWas)
	}
}

// mouseLookStep turns the view by the mouse's motion (dy counts down the screen).
func mouseLookStep(dx, dy int) {
	if lookOrient&lookInvertX != 0 {
		dx = -dx
	}
	if lookOrient&lookInvertY != 0 {
		dy = -dy
	}
	bound := pitchBound()
	PlayerFacing += int16(dx << 6)
	PlayerPitch = int16(max(-bound, min(int(PlayerPitch)-dy*128, bound)))
	ThePlayer.SetHeading(int(PlayerFacing >> 13)) // as PHYSICS.C keeps them
	ThePlayer.SetFineHead(int(PlayerFacing >> 8))
	editchng(2) // redraw the view
}
