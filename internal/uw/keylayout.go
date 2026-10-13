package uw

// The keyboard layout: what each key gives, by its PC scan code, as the game's keyboard
// driver (keyboard.go) reads them. A US 101/102-key keyboard, the one the original knew.

// Codes the game gives keys with no character beyond those in consts_gen.go.
const (
	keyNumLock    = KEY_F10 + 1
	keyScrollLock = KEY_F10 + 2
	keyPadMinus   = 173
	keyPadPlus    = 174
	keyPrintScr   = 164
	keyF11        = 154 // and the keys round it (scan codes 84 to 95) from 151 on
)

// Scan codes of the keys the driver needs by name.
const (
	scanCtrl       = 29
	scanLeftShift  = 42
	scanRightShift = 54
	scanAlt        = 56
	scanCapsLock   = 58
	scanInsert     = 82
	// Keys sent after an E0 prefix are kept at extendedBase plus their place in
	// extendedKeys.
	extendedBase = 96
)

// extendedKeys are the scan codes that follow E0 (the 101-key keyboard's extra keys), in
// the order the driver numbers them from extendedBase.
var extendedKeys = [16]byte{
	70,                                     // Ctrl-Break
	28,                                     // keypad Enter
	29,                                     // right Ctrl
	53,                                     // keypad /
	55,                                     // Print Screen
	56,                                     // right Alt
	71, 72, 73, 75, 77, 79, 80, 81, 82, 83, // the grey Home, Up, PgUp, Left, Right, End, Down, PgDn, Ins, Del
}

// The modifiers' keys, by keyOn index: Shift, Alt, Ctrl, Insert (held), keypad Enter, and
// a sixth no key gives.
var (
	modKeyA = [6]int{scanLeftShift, scanAlt, scanCtrl, scanInsert, extendedBase + 1, 0}
	modKeyB = [6]int{scanRightShift, extendedBase + 5, extendedBase + 2, 0, 0, 0}
)

// ascTab is each key's character: by keyOn index, plus 128 when Shift is down.
var ascTab = buildKeyChars()

func buildKeyChars() [256]byte {
	var t [256]byte
	plain, shifted := t[:128], t[128:]
	put := func(dst []byte, at int, s string) { copy(dst[at:], s) }

	put(plain, 1, "\x1b1234567890-=\b\tqwertyuiop[]\r")
	put(plain, 30, "asdfghjkl;'`")
	put(plain, 43, "\\zxcvbnm,./")
	put(shifted, 1, "\x1b!@#$%^&*()_+\b")
	shifted[15] = KEY_BACKTAB
	put(shifted, 16, "QWERTYUIOP{}\r")
	put(shifted, 30, "ASDFGHJKL:\"~")
	put(shifted, 43, "|ZXCVBNM<>?")
	plain[57], shifted[57] = ' ', ' '

	for _, row := range [][]byte{plain, shifted} {
		for i := 0; i < 10; i++ {
			row[59+i] = byte(KEY_F1 + i)
		}
		row[69], row[70] = keyNumLock, keyScrollLock
		for i := 0; i < 12; i++ { // scan codes 84 to 95: F11, F12 and the keys round them
			row[84+i] = byte(keyF11 - 3 + i)
		}
		// the extended keys
		copy(row[extendedBase:], []byte{
			KEY_ESC, '\r', 0, '/', keyPrintScr, 0,
			KEY_GHOME, KEY_GUP, KEY_GPGUP, KEY_GLEFT, KEY_GRIGHT, KEY_GEND, KEY_GDOWN, KEY_GPGDN,
			KEY_INS, KEY_DEL,
		})
	}
	// the keypad: cursor keys, or digits with Shift
	copy(plain[71:], []byte{KEY_HOME, KEY_UP, KEY_PGUP, keyPadMinus, KEY_LEFT, KEY_PAD5,
		KEY_RIGHT, keyPadPlus, KEY_END, KEY_DOWN, KEY_PGDN, KEY_INS, KEY_DEL})
	put(shifted, 71, "789-456+1230.")
	return t
}
