package uw

import "github.com/memmaker/uw1go/internal/host"

// Port of KEYQUEUE.ASM (with KBDINT.ASM's Caps Lock toggle): the keyboard driver. Scan codes
// come from host.KeyRing; E0-prefixed keys become 60h plus their place in the table at
// seg063:0288; each key's down count is kept in keyOn; the six modifier states are worked
// out from the layout's key tables; and characters come from the Asc table at seg063:0010
// (scan code, plus 80h when Shift is down). The tables are read from UW.EXE.

var (
	ascTab    [256]byte // seg063:0010
	e0Tab     [16]byte  // seg063:0288
	modKeyA   [6]int    // seg063:0299 + layout, per modifier
	modKeyB   [6]int    // seg063:02A1 + layout
	keyOn     [256]byte // seg063:01EB, indexed by scan code
	modifiers [6]byte   // seg063:01E2: Shift, Alt, Ctrl, and three more
	capsLock  byte      // seg063:01E8, bit 0
	keyRepeat bool      // seg063:01E0: let the hardware repeat through
)

// kbdLayout is seg063:01E9: 2, an enhanced 101/102-key keyboard, as every keyboard now is.
const kbdLayout = 2

func initKeyboard() {
	copy(ascTab[:], exeBytes(segSeg063, 0x10, 256))
	copy(e0Tab[:], exeBytes(segSeg063, 0x288, 16))
	t := exeBytes(segSeg063, 0x299, 0x70)
	for m := 0; m < 6; m++ {
		modKeyA[m] = int(le16(t, kbdLayout+0x10*m))
		modKeyB[m] = int(le16(t, 8+kbdLayout+0x10*m))
	}
}

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
	for m := 0; m < 6; m++ {
		modifiers[m] = keyOn[modKeyA[m]&0xFF] | keyOn[modKeyB[m]&0xFF]
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
		if c == 0xE1 {
			kbdNext()
			kbdNext()
			c = 0xE0
		} else if c == 0xE0 {
			b, ok := kbdNext()
			if !ok {
				return 0
			}
			k := b & 0x7F
			if k == 0x2A || k == 0x36 {
				continue
			}
			i := -1
			for j, e := range e0Tab {
				if e == k {
					i = j
					break
				}
			}
			if i < 0 {
				continue
			}
			c = b&0x80 | byte(0x60+i)
		} else if c == 0x3A {
			capsLock ^= 1
		}
		bx := int(c & 0x7F)
		if c&0x80 != 0 {
			keyOn[bx] = 0
			updateModifiers()
			continue
		}
		if !keyRepeat && keyOn[bx] != 0 {
			continue
		}
		keyOn[bx]++
		if modifiers[0] != 0 {
			bx |= 0x80
		}
		ch := ascTab[bx]
		updateModifiers()
		return int(ch) | int(c)<<8
	}
}
