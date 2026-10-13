package uw

import "github.com/memmaker/uw1go/internal/host"

// The keyboard driver (the original's KEYQUEUE.ASM, with its Caps Lock toggle): scan
// codes come from host.KeyRing as a PC keyboard sends them (set 1); keys sent after an
// E0 prefix are kept from extendedBase on; each key's down count is kept in keyOn; the
// modifiers' states come from their keys (keylayout.go); and characters from ascTab.

var (
	keyOn     [256]byte // by scan code (extended keys from extendedBase)
	modifiers [6]byte   // Shift, Alt, Ctrl, and three more (modKeyA, modKeyB)
	capsLock  byte      // bit 0
	keyRepeat bool      // let the keyboard's own repeat through
)

// The scan code protocol.
const (
	prefixExtended = 224 // E0: an extended key follows
	prefixPause    = 225 // E1: Pause, two more bytes
	releaseBit     = 128 // set on a key's release
	shiftedChars   = 128 // ascTab's shifted half
)

func shiftDown() bool { return modifiers[0] != 0 }
func altDown() bool   { return modifiers[1] != 0 }
func ctrlDown() bool  { return modifiers[2] != 0 }

func kbdNext() (byte, bool) {
	if len(host.KeyRing) == 0 {
		return 0, false
	}
	c := host.KeyRing[0]
	host.KeyRing = host.KeyRing[1:]
	return c, true
}

func updateModifiers() {
	for m := range modifiers {
		modifiers[m] = keyOn[modKeyA[m]] | keyOn[modKeyB[m]]
	}
}

// KEY returns the next key event: the character in the low byte and the scan code in the
// high byte, or 0 when no key is waiting.
func KEY() int {
	host.Busy()
	for {
		c, ok := kbdNext()
		if !ok {
			return 0
		}
		switch c {
		case prefixPause:
			kbdNext()
			kbdNext()
			c = prefixExtended
		case prefixExtended:
			b, ok := kbdNext()
			if !ok {
				return 0
			}
			k := b &^ releaseBit
			if k == scanLeftShift || k == scanRightShift { // the fake shifts some keys send
				continue
			}
			i := -1
			for j, e := range extendedKeys {
				if e == k {
					i = j
					break
				}
			}
			if i < 0 {
				continue
			}
			c = b&releaseBit | byte(extendedBase+i)
		case scanCapsLock:
			capsLock ^= 1
		}
		key := int(c &^ releaseBit)
		if c&releaseBit != 0 {
			keyOn[key] = 0
			updateModifiers()
			continue
		}
		if !keyRepeat && keyOn[key] != 0 {
			continue
		}
		keyOn[key]++
		ch := ascTab[key]
		if shiftDown() {
			ch = ascTab[key+shiftedChars]
		}
		updateModifiers()
		return int(ch) | int(c)<<8
	}
}

// isModifierKey: keyOn's index i is one of the modifiers (Shift, Alt, Ctrl, ...).
func isModifierKey(i int) bool {
	for m := range modKeyA {
		if i == modKeyA[m] || i == modKeyB[m] {
			return true
		}
	}
	return false
}
