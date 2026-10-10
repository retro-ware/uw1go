package snd

// The MT-32 backend: munt's MT-32 emulator with the ROMs the player supplies (loaded at
// run time with purego on the desktop, mt32_dl.go; the page's mt32emu.wasm in the
// browser, mt32_js.go). It does what AIL's MT-32 driver does for UW1: banks 0 and 127 are
// the MT-32's own timbres and rhythm sounds; a timbre of any other bank (UW.MT's, the
// effects') is sent into one of the 64 slots of timbre memory, and selecting it on a
// channel points that part's temporary patch at the slot. Only channels 2 to 9 reach the
// MT-32's eight melodic parts, so only they are locked for effects.
//
// A binding provides mtOpen(romDir) (start the one synth at outRate), mtMsg (a short
// message), mtSysex and mtRender (interleaved stereo frames).

type mt32Synth struct {
	bank     [16]int
	notes    [16][128]bool
	slots    map[int]int
	nextSlot int
	buf      []int16
}

// StartupMT32 sets up the driver on the MT-32 with the ROMs in romDir and starts the
// audio output. It fails, leaving nothing set up, without the emulator or the ROMs.
func StartupMT32(romDir string) error {
	if err := mtOpen(romDir); err != nil {
		return err
	}
	startWith(&mt32Synth{slots: map[int]int{}})
	return nil
}

func (m *mt32Synth) short(status, d1, d2 int) {
	mtMsg(uint32(status) | uint32(d1)<<8 | uint32(d2)<<16)
}

// sysex sends a Roland DT1 (data set) message for address addr, packed 7 bits a byte
// (addr is a1<<14 | a2<<7 | a3).
func (m *mt32Synth) sysex(addr int, data []byte) {
	msg := []byte{0xF0, 0x41, 0x10, 0x16, 0x12, byte(addr >> 14 & 0x7F), byte(addr >> 7 & 0x7F), byte(addr & 0x7F)}
	msg = append(msg, data...)
	sum := 0
	for _, b := range msg[5:] {
		sum += int(b)
	}
	msg = append(msg, byte(-sum&0x7F), 0xF7)
	mtSysex(msg)
}

func (m *mt32Synth) message(status, d1, d2 int) {
	c := status & 15
	switch status & 0xF0 {
	case 0x80:
		m.notes[c][d1] = false
	case 0x90:
		m.notes[c][d1] = d2 != 0
	case 0xB0:
		switch d1 {
		case 0x72:
			m.bank[c] = d2
			return
		case 0x7B:
			m.notes[c] = [128]bool{}
		}
	case 0xC0:
		if m.bank[c] != 0 {
			slot, ok := m.slots[m.bank[c]<<8|d1]
			if ok && c >= 1 && c <= 8 {
				// the part's temporary patch: memory timbre slot, no key shift, no fine
				// tune, bender range 12, assign mode 0, reverb on
				m.sysex(3<<14+(c-1)*0x10, []byte{2, byte(slot), 24, 50, 12, 0, 1, 0})
			}
			return
		}
	}
	m.short(status, d1, d2)
}

func (m *mt32Synth) sounding(c int) bool {
	for _, on := range m.notes[c] {
		if on {
			return true
		}
	}
	return false
}

func (m *mt32Synth) allNotesOff(c int) {
	for n, on := range m.notes[c] {
		if on {
			m.short(0x80|c, n, 0)
		}
	}
	m.notes[c] = [128]bool{}
	m.short(0xB0|c, 0x7B, 0)
}

func (m *mt32Synth) builtIn(bank, patch int) bool { return bank == 0 || bank == 127 }

// install sends a timbre (14 bytes of common parameters and four 58-byte partials) to the
// next slot of timbre memory.
func (m *mt32Synth) install(bank, patch int, t []byte) {
	if len(t) < 2+246 {
		return
	}
	key := bank<<8 | patch
	slot, ok := m.slots[key]
	if !ok {
		slot = m.nextSlot
		m.nextSlot = (m.nextSlot + 1) & 63
		for k, s := range m.slots {
			if s == slot {
				delete(m.slots, k)
			}
		}
		m.slots[key] = slot
	}
	m.sysex(8<<14+slot<<8, t[2:2+246])
}

func (m *mt32Synth) lockOrder() []int { return []int{8, 7, 6, 5, 4, 3, 2, 1} }

func (m *mt32Synth) release(c int) {
	m.bank[c] = 0
	m.short(0xB0|c, 0x79, 0)
}

func (m *mt32Synth) render(out []int, frames int) {
	if cap(m.buf) < frames*2 {
		m.buf = make([]int16, frames*2)
	}
	b := m.buf[:frames*2]
	mtRender(b)
	for i, v := range b {
		out[i] += int(v)
	}
}
