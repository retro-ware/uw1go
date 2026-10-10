// Package snd is the sound hardware and driver layer under SOUND.C: a YM3812 (OPL2) FM
// emulator, the Miles AIL XMIDI driver's job on it (timbres from the game's UW.AD library,
// voice allocation, channel locking, the 120 Hz XMIDI sequencer), a digitised .VOC player
// for the speech, and the mixer that feeds Ebiten's audio.
package snd

import "math"

// OPL2 emulation: 9 two-operator channels, the four waveforms, the envelope generator with
// its rate tables, key scaling of level and rate, tremolo and vibrato, feedback and both
// connections. Rhythm mode is not emulated: the AIL AdLib driver does not use it. Levels
// are worked in the chip's log domain (a log-sine table and an exponent table), so
// attenuations add.

const oplRate = 49716 // the chip's sample rate: a 14.31818 MHz clock / 288

var (
	logsinTab [256]int
	expTab    [256]int
	multTab   = [16]int{1, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 20, 24, 24, 30, 30} // twice the multiple
	kslTab    = [16]int{0, 32, 40, 45, 48, 51, 53, 55, 56, 58, 59, 60, 61, 62, 63, 64}
	kslShift  = [4]uint{8, 1, 2, 0}
	// the envelope increments, eight steps a row, as the chip's
	egInc = [15][8]int{
		{0, 1, 0, 1, 0, 1, 0, 1}, {0, 1, 0, 1, 1, 1, 0, 1}, {0, 1, 1, 1, 0, 1, 1, 1}, {0, 1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1, 1}, {1, 1, 1, 2, 1, 1, 1, 2}, {1, 2, 1, 2, 1, 2, 1, 2}, {1, 2, 2, 2, 1, 2, 2, 2},
		{2, 2, 2, 2, 2, 2, 2, 2}, {2, 2, 2, 4, 2, 2, 2, 4}, {2, 4, 2, 4, 2, 4, 2, 4}, {2, 4, 4, 4, 2, 4, 4, 4},
		{4, 4, 4, 4, 4, 4, 4, 4}, {8, 8, 8, 8, 8, 8, 8, 8}, {0, 0, 0, 0, 0, 0, 0, 0},
	}
	// the register offset of each operator slot, and each channel's first slot
	slotReg = [18]int{0, 1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 13, 16, 17, 18, 19, 20, 21}
	chSlot  = [9]int{0, 1, 2, 6, 7, 8, 12, 13, 14}
)

func init() {
	for i := range logsinTab {
		v := -math.Log2(math.Sin((float64(i)+0.5)*math.Pi/512)) * 256
		logsinTab[i] = int(math.Round(v))
	}
	for i := range expTab {
		expTab[i] = int(math.Round(math.Pow(2, float64(255-i)/256)*1024)) - 0
	}
}

const (
	egAttack = iota
	egDecay
	egSustain
	egRelease
)

type oplOp struct {
	// registers
	am, vib, egt, ksr bool
	mult              int
	ksl, tl           int
	ar, dr, sl, rr    int
	ws                int
	// state
	phase     uint32
	eg        int // attenuation, 0 to 511 in 0.1875 dB steps
	state     int
	keyOn     bool
	out, out1 int // the last two outputs (feedback)
}

type oplCh struct {
	fnum, block int
	key         bool
	fb          int
	additive    bool
	op          [2]*oplOp
}

// opl is the chip.
type opl struct {
	ops      [18]oplOp
	ch       [9]oplCh
	wse      bool
	dam, dvb bool
	egCnt    uint32
	tremPos  int
	tremDir  int
	vibPos   int
	sampleNo uint32
	regs     [256]byte
}

func newOPL() *opl {
	o := &opl{tremDir: 1}
	for c := 0; c < 9; c++ {
		o.ch[c].op[0] = &o.ops[chSlot[c]]
		o.ch[c].op[1] = &o.ops[chSlot[c]+3]
	}
	for i := range o.ops {
		o.ops[i].eg = 511
		o.ops[i].state = egRelease
	}
	return o
}

// slotOf is the operator slot a register's offset names, or -1.
func slotOf(off int) int {
	for i, r := range slotReg {
		if r == off {
			return i
		}
	}
	return -1
}

// write sets a register.
func (o *opl) write(reg int, val byte) {
	reg &= 0xFF
	o.regs[reg] = val
	switch {
	case reg == 0x01:
		o.wse = val&0x20 != 0
	case reg == 0xBD:
		o.dam = val&0x80 != 0
		o.dvb = val&0x40 != 0
	case reg >= 0x20 && reg <= 0x35, reg >= 0x40 && reg <= 0x55, reg >= 0x60 && reg <= 0x75,
		reg >= 0x80 && reg <= 0x95, reg >= 0xE0 && reg <= 0xF5:
		s := slotOf(reg & 0x1F)
		if s < 0 {
			return
		}
		op := &o.ops[s]
		switch reg & 0xE0 {
		case 0x20:
			op.am = val&0x80 != 0
			op.vib = val&0x40 != 0
			op.egt = val&0x20 != 0
			op.ksr = val&0x10 != 0
			op.mult = int(val & 15)
		case 0x40:
			op.ksl = int(val >> 6)
			op.tl = int(val & 0x3F)
		case 0x60:
			op.ar = int(val >> 4)
			op.dr = int(val & 15)
		case 0x80:
			op.sl = int(val >> 4)
			op.rr = int(val & 15)
		case 0xE0:
			op.ws = int(val & 3)
		}
	case reg >= 0xA0 && reg <= 0xA8:
		c := &o.ch[reg-0xA0]
		c.fnum = c.fnum&0x300 | int(val)
	case reg >= 0xB0 && reg <= 0xB8:
		c := &o.ch[reg-0xB0]
		c.fnum = c.fnum&0xFF | int(val&3)<<8
		c.block = int(val >> 2 & 7)
		key := val&0x20 != 0
		if key != c.key {
			c.key = key
			for _, op := range c.op {
				if key {
					op.keyOn = true
					op.state = egAttack
					op.phase = 0
				} else {
					op.keyOn = false
					op.state = egRelease
				}
			}
		}
	case reg >= 0xC0 && reg <= 0xC8:
		c := &o.ch[reg-0xC0]
		c.fb = int(val >> 1 & 7)
		c.additive = val&1 != 0
	}
}

// rate is an envelope rate register's effective rate (0 to 63) for the channel's key.
func (op *oplOp) rate(r int, c *oplCh) int {
	if r == 0 {
		return 0
	}
	kc := c.block<<1 | c.fnum>>9&1
	if !op.ksr {
		kc >>= 2
	}
	r = r*4 + kc
	if r > 63 {
		r = 63
	}
	return r
}

// envStep advances the operator's envelope one sample.
func (o *opl) envStep(op *oplOp, c *oplCh) {
	var r int
	switch op.state {
	case egAttack:
		r = op.rate(op.ar, c)
	case egDecay:
		r = op.rate(op.dr, c)
	case egSustain:
		if op.egt {
			return
		}
		r = op.rate(op.rr, c)
	case egRelease:
		r = op.rate(op.rr, c)
	}
	if r == 0 {
		return
	}
	g := r >> 2
	var sel, shift int
	switch {
	case g <= 12:
		sel, shift = r&3, 12-g
	case g == 13:
		sel = 4 + r&3
	case g == 14:
		sel = 8 + r&3
	default:
		sel = 12
	}
	if o.egCnt&(1<<uint(shift)-1) != 0 {
		return
	}
	inc := egInc[sel][(o.egCnt>>uint(shift))&7]
	switch op.state {
	case egAttack:
		if r >= 60 {
			op.eg = 0
		} else {
			op.eg += (-(op.eg + 1) * inc) >> 3
		}
		if op.eg <= 0 {
			op.eg = 0
			op.state = egDecay
		}
	case egDecay:
		op.eg += inc
		sl := op.sl << 4
		if op.sl == 15 {
			sl = 511
		}
		if op.eg >= sl {
			op.eg = sl
			op.state = egSustain
		}
	default:
		op.eg += inc
		if op.eg > 511 {
			op.eg = 511
		}
	}
}

// opOut is one operator's output for a phase modulation of mod.
func (o *opl) opOut(op *oplOp, c *oplCh, mod int, trem int) int {
	att := op.eg + op.tl<<2
	if op.ksl != 0 {
		k := kslTab[c.fnum>>6]<<2 - (8-c.block)<<5
		if k > 0 {
			att += k >> kslShift[op.ksl]
		}
	}
	if op.am {
		att += trem
	}
	if att > 511 {
		att = 511
	}
	ph := int(op.phase>>10) + mod
	ws := op.ws
	if !o.wse {
		ws = 0
	}
	neg := false
	idx := ph & 0xFF
	if ph&0x100 != 0 {
		idx = ^idx & 0xFF
	}
	var level int
	switch ws {
	case 0:
		neg = ph&0x200 != 0
		level = logsinTab[idx]
	case 1:
		if ph&0x200 != 0 {
			level = 0x1000
		} else {
			level = logsinTab[idx]
		}
	case 2:
		level = logsinTab[idx]
	case 3:
		if ph&0x100 != 0 {
			level = 0x1000
		} else {
			level = logsinTab[ph&0xFF]
		}
	}
	level += att << 3
	if level > 0x1FFF {
		return 0
	}
	v := expTab[level&0xFF] << 1 >> uint(level>>8)
	if neg {
		return -v
	}
	return v
}

// sample runs the chip one sample and returns each channel's output into out.
func (o *opl) sample(out *[9]int) {
	o.sampleNo++
	o.egCnt++
	if o.sampleNo&63 == 0 {
		o.tremPos += o.tremDir
		if o.tremPos >= 105 {
			o.tremDir = -1
		} else if o.tremPos <= 0 {
			o.tremDir = 1
		}
	}
	if o.sampleNo&1023 == 0 {
		o.vibPos = (o.vibPos + 1) & 7
	}
	trem := o.tremPos >> 4
	if o.dam {
		trem = o.tremPos >> 2
	}
	for ci := range o.ch {
		c := &o.ch[ci]
		m, k := c.op[0], c.op[1]
		if m.eg >= 511 && k.eg >= 511 && m.state == egRelease && k.state == egRelease {
			out[ci] = 0
			continue
		}
		for _, op := range c.op {
			o.envStep(op, c)
			f := c.fnum
			if op.vib {
				r := f >> 7 & 7
				switch {
				case o.vibPos&3 == 0:
					r = 0
				case o.vibPos&1 != 0:
					r >>= 1
				}
				if !o.dvb {
					r >>= 1
				}
				if o.vibPos&4 != 0 {
					r = -r
				}
				f += r
			}
			op.phase += uint32((f << uint(c.block)) * multTab[op.mult] >> 1)
		}
		fbmod := 0
		if c.fb != 0 {
			fbmod = (m.out + m.out1) >> uint(9-c.fb)
		}
		mo := o.opOut(m, c, fbmod, trem)
		m.out1, m.out = m.out, mo
		if c.additive {
			out[ci] = mo + o.opOut(k, c, 0, trem)
		} else {
			out[ci] = o.opOut(k, c, mo, trem)
		}
	}
}
