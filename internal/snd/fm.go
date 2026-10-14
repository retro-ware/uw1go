package snd

import (
	"math"

	"github.com/memmaker/opl2"
)

// The AdLib backend: the AIL AdLib driver's work on the emulated OPL2. MIDI channels are
// mapped onto the 9 FM voices; a note takes a free voice, else the oldest released one,
// else the oldest note (a locked channel's last). Timbres are the 14-byte AIL AdLib
// records of UW.AD: transpose, then the modulator's and carrier's registers.

const (
	defPitchRange = 12 // semitones of the pitch wheel, AIL's default
	percChannel   = 9  // MIDI channel 10, the percussion
	percBank      = 127
)

// velGraph is AIL's velocity sensitivity curve, indexed by velocity / 8.
var velGraph = [16]int{82, 85, 88, 91, 94, 97, 100, 103, 106, 109, 112, 115, 118, 121, 124, 127}

type midiChan struct {
	program, bank int
	vol, expr     int
	pan           int
	bend          int // 0 to 0x3FFF, 0x2000 centred
	sustain       bool
}

type voice struct {
	active    bool
	keyed     bool
	sustained bool
	ch, note  int
	vel       int
	timbre    []byte
	age       uint32
}

type fmSynth struct {
	chip                     *opl2.Chip
	ch                       [16]midiChan
	v                        [9]voice
	timbres                  map[int][]byte
	ageCount                 uint32
	stereo                   bool
	pos                      float64
	prevL, prevR, curL, curR int
}

func newFM(stereo bool) *fmSynth {
	d := &fmSynth{chip: opl2.New(), timbres: map[int][]byte{}, stereo: stereo}
	d.chip.Write(0x01, 0x20)
	d.chip.Write(0x08, 0x00)
	d.chip.Write(0xBD, 0x00)
	for i := range d.ch {
		d.resetChannel(i)
		d.ch[i].vol = 127
	}
	return d
}

func (d *fmSynth) resetChannel(c int) {
	ch := &d.ch[c]
	ch.expr = 127
	ch.pan = 64
	ch.bend = 0x2000
	ch.sustain = false
}

func (d *fmSynth) sounding(c int) bool {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c && d.v[i].keyed {
			return true
		}
	}
	return false
}

// message carries out a channel voice message.
func (d *fmSynth) message(status, d1, d2 int) {
	c := status & 15
	ch := &d.ch[c]
	switch status & 0xF0 {
	case 0x80:
		d.noteOff(c, d1)
	case 0x90:
		if d2 == 0 {
			d.noteOff(c, d1)
		} else {
			d.noteOn(c, d1, d2)
		}
	case 0xB0:
		switch d1 {
		case 7:
			ch.vol = d2
			d.updateLevels(c)
		case 0x0B:
			ch.expr = d2
			d.updateLevels(c)
		case 0x0A:
			ch.pan = d2
		case 0x40:
			ch.sustain = d2 >= 64
			if !ch.sustain {
				for i := range d.v {
					if v := &d.v[i]; v.active && v.ch == c && v.sustained {
						d.keyOff(i)
					}
				}
			}
		case 0x72:
			ch.bank = d2
		case 0x79:
			d.resetChannel(c)
			d.updateLevels(c)
			d.updatePitch(c)
		case 0x7B:
			d.allNotesOff(c)
		}
	case 0xC0:
		ch.program = d1
	case 0xE0:
		ch.bend = d1 | d2<<7
		d.updatePitch(c)
	}
}

func (d *fmSynth) allNotesOff(c int) {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c {
			d.keyOff(i)
		}
	}
}

func (d *fmSynth) noteOn(c, note, vel int) {
	ch := &d.ch[c]
	key := ch.bank<<8 | ch.program
	if c == percChannel {
		key = percBank<<8 | note
	}
	t, ok := d.timbres[key]
	if !ok {
		return
	}
	i := d.allocVoice(c)
	if i < 0 {
		return
	}
	d.ageCount++
	d.v[i] = voice{active: true, keyed: true, ch: c, note: note, vel: vel, timbre: t, age: d.ageCount}
	d.keyOffNow(i)
	d.setTimbre(i)
	d.setPitch(i, true)
}

// allocVoice picks a voice for a note on channel c: a free one, else the oldest released
// one, else the oldest note of the music (a locked channel's notes are stolen last).
func (d *fmSynth) allocVoice(c int) int {
	best := -1
	for i := range d.v {
		if !d.v[i].active {
			return i
		}
	}
	for i := range d.v {
		if !d.v[i].keyed && (best < 0 || d.v[i].age < d.v[best].age) {
			best = i
		}
	}
	if best >= 0 {
		return best
	}
	for pass := 0; pass < 2 && best < 0; pass++ {
		for i := range d.v {
			if pass == 0 && drv.locked[d.v[i].ch] {
				continue
			}
			if best < 0 || d.v[i].age < d.v[best].age {
				best = i
			}
		}
	}
	return best
}

func (d *fmSynth) noteOff(c, note int) {
	for i := range d.v {
		v := &d.v[i]
		if v.active && v.keyed && v.ch == c && v.note == note {
			if d.ch[c].sustain {
				v.sustained = true
			} else {
				d.keyOff(i)
			}
		}
	}
}

// keyOff releases voice i: its envelope runs out and the voice becomes free to take.
func (d *fmSynth) keyOff(i int) {
	v := &d.v[i]
	v.keyed = false
	v.sustained = false
	d.chip.Write(0xB0+i, d.chip.Reg(0xB0+i)&^0x20)
}

func (d *fmSynth) keyOffNow(i int) {
	d.chip.Write(0xB0+i, d.chip.Reg(0xB0+i)&^0x20)
}

// opBase is the register offset of voice i's modulator; the carrier's is 3 more.
func opBase(i int) int { return slotReg[chSlot[i]] }

var (
	// the register offset of each operator slot, and each channel's first slot
	slotReg = [18]int{0, 1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 13, 16, 17, 18, 19, 20, 21}
	chSlot  = [9]int{0, 1, 2, 6, 7, 8, 12, 13, 14}
)

func (d *fmSynth) setTimbre(i int) {
	t := d.v[i].timbre
	m := opBase(i)
	d.chip.Write(0x20+m, t[3])
	d.chip.Write(0x60+m, t[5])
	d.chip.Write(0x80+m, t[6])
	d.chip.Write(0xE0+m, t[7])
	d.chip.Write(0xC0+i, t[8])
	d.chip.Write(0x20+m+3, t[9])
	d.chip.Write(0x60+m+3, t[11])
	d.chip.Write(0x80+m+3, t[12])
	d.chip.Write(0xE0+m+3, t[13])
	d.setLevel(i)
}

// setLevel scales voice i's output level (the carrier's, and the modulator's too when
// the connection is additive) by velocity, channel volume and expression.
func (d *fmSynth) setLevel(i int) {
	v := &d.v[i]
	t := v.timbre
	ch := &d.ch[v.ch]
	scale := velGraph[v.vel>>3] * ch.vol * ch.expr / (127 * 127)
	lvl := func(ksltl byte) byte {
		tl := int(ksltl & 0x3F)
		tl = 0x3F - (0x3F-tl)*scale/127
		return ksltl&0xC0 | byte(tl)
	}
	m := opBase(i)
	if t[8]&1 != 0 {
		d.chip.Write(0x40+m, lvl(t[4]))
	} else {
		d.chip.Write(0x40+m, t[4])
	}
	d.chip.Write(0x40+m+3, lvl(t[10]))
}

func (d *fmSynth) updateLevels(c int) {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c {
			d.setLevel(i)
		}
	}
}

func (d *fmSynth) updatePitch(c int) {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c {
			d.setPitch(i, d.v[i].keyed)
		}
	}
}

// setPitch sets voice i's frequency from its note, the timbre's transpose (on the
// percussion, the fixed note the timbre plays) and the channel's pitch wheel.
func (d *fmSynth) setPitch(i int, key bool) {
	v := &d.v[i]
	note := float64(v.note + int(int8(v.timbre[2])))
	if v.ch == percChannel {
		note = float64(v.timbre[2])
	}
	note += float64(d.ch[v.ch].bend-0x2000) / 0x2000 * defPitchRange
	f := 440 * math.Pow(2, (note-69)/12)
	block := 0
	fnum := f * (1 << 20) / opl2.Rate
	for fnum >= 1024 && block < 7 {
		block++
		fnum /= 2
	}
	if fnum > 1023 {
		fnum = 1023
	}
	n := int(fnum)
	d.chip.Write(0xA0+i, byte(n))
	b := byte(n>>8&3) | byte(block<<2)
	if key {
		b |= 0x20
	}
	d.chip.Write(0xB0+i, b)
}

// render runs the chip one sample: the mono sum, and its left and right when stereo.
func (d *fmSynth) sample() (l, r int) {
	var out [opl2.Channels]int
	d.chip.SampleChannels(&out)
	for i, s := range out {
		if !d.stereo {
			l += s
			continue
		}
		p := d.ch[d.v[i].ch].pan
		l += s * (127 - p) / 64
		r += s * p / 64
	}
	if !d.stereo {
		r = l
	}
	for i := range d.v {
		v := &d.v[i]
		if v.active && !v.keyed {
			if d.chip.Silent(i) {
				v.active = false
			}
		}
	}
	return
}

// The XMIDI sequencer: one FORM XMID's TIMB list and EVNT stream. Intervals are counts
// of 120 Hz ticks; a note on carries its duration, so the sequencer keeps the notes
// sounding and ends them. Controllers 0x74 and 0x75 are AIL's FOR and NEXT loops.

func (d *fmSynth) builtIn(bank, patch int) bool { return false }

func (d *fmSynth) install(bank, patch int, t []byte) {
	if len(t) >= 14 {
		d.timbres[bank<<8|patch] = append([]byte(nil), t...)
	}
}

// lockOrder: any channel but the percussion, highest first.
func (d *fmSynth) lockOrder() []int {
	return []int{15, 14, 13, 12, 11, 10, 8, 7, 6, 5, 4, 3, 2, 1}
}

func (d *fmSynth) release(c int) {
	d.resetChannel(c)
	d.ch[c].vol = 127
}

// render adds frames of output, resampled from the chip's rate, into out (stereo pairs).
func (d *fmSynth) render(out []int, frames int) {
	step := float64(opl2.Rate) / outRate
	for f := 0; f < frames; f++ {
		d.pos += step
		for d.pos >= 1 {
			d.pos--
			d.prevL, d.prevR = d.curL, d.curR
			d.curL, d.curR = d.sample()
		}
		out[f*2] += (d.prevL + int(float64(d.curL-d.prevL)*d.pos)) * 2
		out[f*2+1] += (d.prevR + int(float64(d.curR-d.prevR)*d.pos)) * 2
	}
}
