package host

import (
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

// The keyboard is fed to the game as PC scan codes (set 1), as the DOS keyboard interrupt
// received them: a make code when a key goes down, the code with bit 7 set when it comes up,
// and E0 before the extended keys. The game's keyboard driver (see the uw package's port of
// KEYQUEUE.ASM) reads them from KeyRing.

type scan struct {
	code byte
	e0   bool
}

var keyScan = map[ebiten.Key]scan{
	ebiten.KeyEscape: {0x01, false},
	ebiten.KeyDigit1: {0x02, false}, ebiten.KeyDigit2: {0x03, false}, ebiten.KeyDigit3: {0x04, false},
	ebiten.KeyDigit4: {0x05, false}, ebiten.KeyDigit5: {0x06, false}, ebiten.KeyDigit6: {0x07, false},
	ebiten.KeyDigit7: {0x08, false}, ebiten.KeyDigit8: {0x09, false}, ebiten.KeyDigit9: {0x0A, false},
	ebiten.KeyDigit0: {0x0B, false}, ebiten.KeyMinus: {0x0C, false}, ebiten.KeyEqual: {0x0D, false},
	ebiten.KeyBackspace: {0x0E, false}, ebiten.KeyTab: {0x0F, false},
	ebiten.KeyQ: {0x10, false}, ebiten.KeyW: {0x11, false}, ebiten.KeyE: {0x12, false}, ebiten.KeyR: {0x13, false},
	ebiten.KeyT: {0x14, false}, ebiten.KeyY: {0x15, false}, ebiten.KeyU: {0x16, false}, ebiten.KeyI: {0x17, false},
	ebiten.KeyO: {0x18, false}, ebiten.KeyP: {0x19, false}, ebiten.KeyBracketLeft: {0x1A, false},
	ebiten.KeyBracketRight: {0x1B, false}, ebiten.KeyEnter: {0x1C, false}, ebiten.KeyControlLeft: {0x1D, false},
	ebiten.KeyA: {0x1E, false}, ebiten.KeyS: {0x1F, false}, ebiten.KeyD: {0x20, false}, ebiten.KeyF: {0x21, false},
	ebiten.KeyG: {0x22, false}, ebiten.KeyH: {0x23, false}, ebiten.KeyJ: {0x24, false}, ebiten.KeyK: {0x25, false},
	ebiten.KeyL: {0x26, false}, ebiten.KeySemicolon: {0x27, false}, ebiten.KeyQuote: {0x28, false},
	ebiten.KeyBackquote: {0x29, false}, ebiten.KeyShiftLeft: {0x2A, false}, ebiten.KeyBackslash: {0x2B, false},
	ebiten.KeyZ: {0x2C, false}, ebiten.KeyX: {0x2D, false}, ebiten.KeyC: {0x2E, false}, ebiten.KeyV: {0x2F, false},
	ebiten.KeyB: {0x30, false}, ebiten.KeyN: {0x31, false}, ebiten.KeyM: {0x32, false}, ebiten.KeyComma: {0x33, false},
	ebiten.KeyPeriod: {0x34, false}, ebiten.KeySlash: {0x35, false}, ebiten.KeyShiftRight: {0x36, false},
	ebiten.KeyNumpadMultiply: {0x37, false}, ebiten.KeyAltLeft: {0x38, false}, ebiten.KeySpace: {0x39, false},
	ebiten.KeyCapsLock: {0x3A, false},
	ebiten.KeyF1:       {0x3B, false}, ebiten.KeyF2: {0x3C, false}, ebiten.KeyF3: {0x3D, false}, ebiten.KeyF4: {0x3E, false},
	ebiten.KeyF5: {0x3F, false}, ebiten.KeyF6: {0x40, false}, ebiten.KeyF7: {0x41, false}, ebiten.KeyF8: {0x42, false},
	ebiten.KeyF9: {0x43, false}, ebiten.KeyF10: {0x44, false},
	ebiten.KeyNumLock: {0x45, false}, ebiten.KeyScrollLock: {0x46, false},
	ebiten.KeyNumpad7: {0x47, false}, ebiten.KeyNumpad8: {0x48, false}, ebiten.KeyNumpad9: {0x49, false},
	ebiten.KeyNumpadSubtract: {0x4A, false}, ebiten.KeyNumpad4: {0x4B, false}, ebiten.KeyNumpad5: {0x4C, false},
	ebiten.KeyNumpad6: {0x4D, false}, ebiten.KeyNumpadAdd: {0x4E, false}, ebiten.KeyNumpad1: {0x4F, false},
	ebiten.KeyNumpad2: {0x50, false}, ebiten.KeyNumpad3: {0x51, false}, ebiten.KeyNumpad0: {0x52, false},
	ebiten.KeyNumpadDecimal: {0x53, false}, ebiten.KeyF11: {0x57, false}, ebiten.KeyF12: {0x58, false},
	ebiten.KeyNumpadEnter: {0x1C, true}, ebiten.KeyControlRight: {0x1D, true}, ebiten.KeyNumpadDivide: {0x35, true},
	ebiten.KeyAltRight: {0x38, true},
	// ISO keyboards' extra key (beside the left Shift; on a Mac the one left of 1, ^ on a German
	// board) is ` too, the mouse-look key, which non-US layouts have elsewhere or not at all
	ebiten.KeyIntlBackslash: {0x29, false},
	ebiten.KeyHome:          {0x47, true}, ebiten.KeyArrowUp: {0x48, true}, ebiten.KeyPageUp: {0x49, true},
	ebiten.KeyArrowLeft: {0x4B, true}, ebiten.KeyArrowRight: {0x4D, true}, ebiten.KeyEnd: {0x4F, true},
	ebiten.KeyArrowDown: {0x50, true}, ebiten.KeyPageDown: {0x51, true}, ebiten.KeyInsert: {0x52, true},
	ebiten.KeyDelete: {0x53, true},
}

// KeyRing holds scan code bytes the game has not read yet.
var KeyRing []byte

var (
	keysDown  = map[ebiten.Key]bool{}
	repeatKey scan
	repeatOn  bool
	repeatMs  int
	msPerTick = 1000 / TPS
)

// Mouse state, in the game's 320x200 screen (top row 0).
var (
	MouseX, MouseY int
	MouseMoved     bool
	MouseButtons   int // bit 0 left, bit 1 right
	// MouseDX, MouseDY accumulate relative motion (for a captured pointer)
	MouseDX, MouseDY int
	// MouseLook is set by the game while the mouse turns the view: the pointer is captured and
	// its motion, in screen pixels times LookSpeed/100, adds to LookDX, LookDY instead of
	// moving MouseX, MouseY.
	MouseLook      bool
	LookDX, LookDY int
	LookSpeed      = 100

	lookCaptured   bool
	lookSettle     int // ticks after a capture change whose motion is dropped
	lookFX, lookFY float64
	lastCX, lastCY int
)

func pushScan(s scan, up bool) {
	if s.e0 {
		KeyRing = append(KeyRing, 0xE0)
	}
	c := s.code
	if up {
		c |= 0x80
	}
	KeyRing = append(KeyRing, c)
}

func pollInput(g *game) {
	if runScript() {
		return
	}
	for k, s := range keyScan {
		down := ebiten.IsKeyPressed(k)
		if down != keysDown[k] {
			keysDown[k] = down
			pushScan(s, !down)
			if down {
				repeatKey, repeatOn, repeatMs = s, true, 0
			} else if repeatOn && repeatKey == s {
				repeatOn = false
			}
		}
	}
	// The keyboard's own repeat: after 500 ms, then 10.9 times a second.
	if repeatOn {
		before := repeatMs
		repeatMs += msPerTick
		if repeatMs >= 500 && (before < 500 || (repeatMs-500)/92 != (before-500)/92) {
			pushScan(repeatKey, false)
		}
	}
	if len(KeyRing) > 256 {
		KeyRing = KeyRing[len(KeyRing)-256:]
	}

	cx, cy := ebiten.CursorPosition()
	if MouseLook != lookCaptured {
		lookCaptured = MouseLook
		if MouseLook {
			ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		} else {
			ebiten.SetCursorMode(ebiten.CursorModeHidden)
		}
		lookSettle, lookFX, lookFY = 2, 0, 0
	}
	dcx, dcy := cx-lastCX, cy-lastCY
	lastCX, lastCY = cx, cy
	if lookSettle > 0 {
		lookSettle--
		dcx, dcy = 0, 0
	}
	if MouseLook {
		if g.scaleX > 0 {
			k := float64(LookSpeed) / 100
			lookFX += float64(dcx) / g.scaleX * k
			lookFY += float64(dcy) / g.scaleY * k
			ix, iy := int(lookFX), int(lookFY)
			lookFX -= float64(ix)
			lookFY -= float64(iy)
			LookDX += ix
			LookDY += iy
		}
	} else if g.scaleX > 0 {
		x := int((float64(cx) - g.offX) / g.scaleX)
		y := int((float64(cy) - g.offY) / g.scaleY)
		if x < 0 {
			x = 0
		}
		if x > ScreenW-1 {
			x = ScreenW - 1
		}
		if y < 0 {
			y = 0
		}
		if y > ScreenH-1 {
			y = ScreenH - 1
		}
		if x != MouseX || y != MouseY {
			MouseDX += x - MouseX
			MouseDY += y - MouseY
			MouseX, MouseY = x, y
			MouseMoved = true
		}
	}
	b := 0
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		b |= 1
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		b |= 2
	}
	MouseButtons = b
}

// Timers are callbacks run every host tick before the game runs, as the DOS timer
// interrupt ran them.
var timers []func()

// AddTimer registers a callback run once per host tick.
func AddTimer(f func()) { timers = append(timers, f) }

func tickTimers() {
	for _, f := range timers {
		f()
	}
}

func init() {
	// UW1GO_LOOK_SPEED: mouse-look's speed, a percentage (10 to 400)
	if n, err := strconv.Atoi(os.Getenv("UW1GO_LOOK_SPEED")); err == nil {
		LookSpeed = max(10, min(n, 400))
	}
}
