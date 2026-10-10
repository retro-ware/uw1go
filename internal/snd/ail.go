package snd

import (
	"encoding/binary"
	"math"
	"sync"
)

// The AIL XMIDI driver's interface, as SOUND.C uses it, over the emulated OPL2: global
// timbres installed by bank and patch (the 14-byte AIL AdLib timbres of UW.AD), 16 MIDI
// channels mapped onto the 9 FM voices, channels that the program locks away from the
// music for its own notes, and XMIDI sequences played at the fixed 120 Hz XMIDI rate.
// Channels are numbered 1 to 16 where the API takes or returns one, as AIL's are.
//
// The driver's state is shared with the audio thread: every entry point takes mu.

var mu sync.Mutex

// Sequence statuses, as AIL_sequence_status returns them.
const (
	SeqStopped = 0
	SeqPlaying = 1
	SeqDone    = 2
)

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
	locked        bool
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

type driver struct {
	chip     *opl
	ch       [16]midiChan
	v        [9]voice
	timbres  map[int][]byte
	ageCount uint32
	stereo   bool
	seqs     []*sequence
}

var drv *driver

func newDriver(stereo bool) *driver {
	d := &driver{chip: newOPL(), timbres: map[int][]byte{}, stereo: stereo}
	d.chip.write(0x01, 0x20)
	d.chip.write(0x08, 0x00)
	d.chip.write(0xBD, 0x00)
	for i := range d.ch {
		d.resetChannel(i)
		d.ch[i].vol = 127
	}
	return d
}

func (d *driver) resetChannel(c int) {
	ch := &d.ch[c]
	ch.expr = 127
	ch.pan = 64
	ch.bend = 0x2000
	ch.sustain = false
}

// InstallTimbre gives the driver a timbre, the whole record with its length word.
func InstallTimbre(bank, patch int, t []byte) {
	mu.Lock()
	defer mu.Unlock()
	if drv == nil || len(t) < 14 {
		return
	}
	drv.timbres[bank<<8|patch] = append([]byte(nil), t...)
}

// TimbreStatus is whether bank, patch is installed.
func TimbreStatus(bank, patch int) bool {
	mu.Lock()
	defer mu.Unlock()
	if drv == nil {
		return false
	}
	_, ok := drv.timbres[bank<<8|patch]
	return ok
}

// LockChannel takes a channel away from the music for the caller: the highest-numbered
// one (not the percussion) that is unlocked, preferring one with no notes sounding. Its
// notes are silenced. 0 if none is free.
func LockChannel() int {
	mu.Lock()
	defer mu.Unlock()
	if drv == nil {
		return 0
	}
	best := -1
	for c := 15; c >= 1; c-- {
		if c == percChannel || drv.ch[c].locked {
			continue
		}
		if !drv.channelSounding(c) {
			best = c
			break
		}
		if best < 0 {
			best = c
		}
	}
	if best < 0 {
		return 0
	}
	drv.allNotesOff(best)
	for _, s := range drv.seqs {
		s.dropNotes(best)
	}
	drv.ch[best].locked = true
	return best + 1
}

// ReleaseChannel gives a locked channel back to the music, silenced and with the
// music's settings for it put back.
func ReleaseChannel(ch int) {
	mu.Lock()
	defer mu.Unlock()
	c := ch - 1
	if drv == nil || c < 0 || c > 15 || !drv.ch[c].locked {
		return
	}
	drv.allNotesOff(c)
	drv.ch[c].locked = false
	drv.resetChannel(c)
	drv.ch[c].vol = 127
	for _, s := range drv.seqs {
		if s.status == SeqPlaying {
			s.restore(c)
		}
	}
}

// SendVoice sends a channel voice message: status 0x80 to 0xEF, channel in its low nibble.
func SendVoice(status, d1, d2 int) {
	mu.Lock()
	defer mu.Unlock()
	if drv == nil {
		return
	}
	drv.message(status&0xFF, d1&0x7F, d2&0x7F)
}

func (d *driver) channelSounding(c int) bool {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c && d.v[i].keyed {
			return true
		}
	}
	return false
}

// message carries out a channel voice message.
func (d *driver) message(status, d1, d2 int) {
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

func (d *driver) allNotesOff(c int) {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c {
			d.keyOff(i)
		}
	}
}

func (d *driver) noteOn(c, note, vel int) {
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
func (d *driver) allocVoice(c int) int {
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
			if pass == 0 && d.ch[d.v[i].ch].locked {
				continue
			}
			if best < 0 || d.v[i].age < d.v[best].age {
				best = i
			}
		}
	}
	return best
}

func (d *driver) noteOff(c, note int) {
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
func (d *driver) keyOff(i int) {
	v := &d.v[i]
	v.keyed = false
	v.sustained = false
	d.chip.write(0xB0+i, d.chip.regs[0xB0+i]&^0x20)
}

func (d *driver) keyOffNow(i int) {
	d.chip.write(0xB0+i, d.chip.regs[0xB0+i]&^0x20)
}

// opBase is the register offset of voice i's modulator; the carrier's is 3 more.
func opBase(i int) int { return slotReg[chSlot[i]] }

func (d *driver) setTimbre(i int) {
	t := d.v[i].timbre
	m := opBase(i)
	d.chip.write(0x20+m, t[3])
	d.chip.write(0x60+m, t[5])
	d.chip.write(0x80+m, t[6])
	d.chip.write(0xE0+m, t[7])
	d.chip.write(0xC0+i, t[8])
	d.chip.write(0x20+m+3, t[9])
	d.chip.write(0x60+m+3, t[11])
	d.chip.write(0x80+m+3, t[12])
	d.chip.write(0xE0+m+3, t[13])
	d.setLevel(i)
}

// setLevel scales voice i's output level (the carrier's, and the modulator's too when
// the connection is additive) by velocity, channel volume and expression.
func (d *driver) setLevel(i int) {
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
		d.chip.write(0x40+m, lvl(t[4]))
	} else {
		d.chip.write(0x40+m, t[4])
	}
	d.chip.write(0x40+m+3, lvl(t[10]))
}

func (d *driver) updateLevels(c int) {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c {
			d.setLevel(i)
		}
	}
}

func (d *driver) updatePitch(c int) {
	for i := range d.v {
		if d.v[i].active && d.v[i].ch == c {
			d.setPitch(i, d.v[i].keyed)
		}
	}
}

// setPitch sets voice i's frequency from its note, the timbre's transpose (on the
// percussion, the fixed note the timbre plays) and the channel's pitch wheel.
func (d *driver) setPitch(i int, key bool) {
	v := &d.v[i]
	note := float64(v.note + int(int8(v.timbre[2])))
	if v.ch == percChannel {
		note = float64(v.timbre[2])
	}
	note += float64(d.ch[v.ch].bend-0x2000) / 0x2000 * defPitchRange
	f := 440 * math.Pow(2, (note-69)/12)
	block := 0
	fnum := f * (1 << 20) / oplRate
	for fnum >= 1024 && block < 7 {
		block++
		fnum /= 2
	}
	if fnum > 1023 {
		fnum = 1023
	}
	n := int(fnum)
	d.chip.write(0xA0+i, byte(n))
	b := byte(n>>8&3) | byte(block<<2)
	if key {
		b |= 0x20
	}
	d.chip.write(0xB0+i, b)
}

// render runs the chip one sample: the mono sum, and its left and right when stereo.
func (d *driver) render() (l, r int) {
	var out [9]int
	d.chip.sample(&out)
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
			m := &d.chip.ops[chSlot[i]+3]
			if m.eg >= 511 {
				v.active = false
			}
		}
	}
	return
}

// The XMIDI sequencer: one FORM XMID's TIMB list and EVNT stream. Intervals are counts
// of 120 Hz ticks; a note on carries its duration, so the sequencer keeps the notes
// sounding and ends them. Controllers 0x74 and 0x75 are AIL's FOR and NEXT loops.

type seqNote struct{ ch, note, left int }

type sequence struct {
	timb   [][2]int // patch, bank
	tnext  int
	evnt   []byte
	pos    int
	delay  int
	status int
	notes  []seqNote
	loops  []struct{ pos, count int }
	relVol int
	shadow [16]struct {
		program, bank, vol, expr, pan, bend int
		set                                 bool
	}
}

// RegisterSequence parses an XMI file's first sequence; -1 if it is not one.
func RegisterSequence(xmi []byte) int {
	s := parseXMI(xmi)
	if s == nil {
		return -1
	}
	mu.Lock()
	defer mu.Unlock()
	if drv == nil {
		return -1
	}
	for i, old := range drv.seqs {
		if old == nil {
			drv.seqs[i] = s
			return i
		}
	}
	drv.seqs = append(drv.seqs, s)
	return len(drv.seqs) - 1
}

func parseXMI(b []byte) *sequence {
	s := &sequence{relVol: 100}
	found := false
	var walk func(b []byte)
	walk = func(b []byte) {
		for len(b) >= 8 && !found {
			id := string(b[:4])
			n := int(binary.BigEndian.Uint32(b[4:8]))
			if n > len(b)-8 {
				n = len(b) - 8
			}
			body := b[8 : 8+n]
			switch id {
			case "FORM", "CAT ":
				if len(body) >= 4 {
					if id == "FORM" && string(body[:4]) == "XMID" {
						s.chunks(body[4:])
						found = s.evnt != nil
					} else {
						walk(body[4:])
					}
				}
			}
			b = b[8+n+n&1:]
		}
	}
	walk(b)
	if !found {
		return nil
	}
	return s
}

func (s *sequence) chunks(b []byte) {
	for len(b) >= 8 {
		id := string(b[:4])
		n := int(binary.BigEndian.Uint32(b[4:8]))
		if n > len(b)-8 {
			n = len(b) - 8
		}
		body := b[8 : 8+n]
		switch id {
		case "TIMB":
			if len(body) >= 2 {
				cnt := int(binary.LittleEndian.Uint16(body))
				for i := 0; i < cnt && 2+i*2+1 < len(body); i++ {
					s.timb = append(s.timb, [2]int{int(body[2+i*2]), int(body[3+i*2])})
				}
			}
		case "EVNT":
			s.evnt = body
		}
		b = b[8+n+n&1:]
	}
}

func seqOf(h int) *sequence {
	if drv == nil || h < 0 || h >= len(drv.seqs) {
		return nil
	}
	return drv.seqs[h]
}

// ReleaseSequence forgets sequence h, silencing it.
func ReleaseSequence(h int) {
	mu.Lock()
	defer mu.Unlock()
	if s := seqOf(h); s != nil {
		s.stop()
		drv.seqs[h] = nil
	}
}

// TimbreRequest is the next timbre sequence h needs that is not installed, bank<<8|patch,
// or 0xFFFF.
func TimbreRequest(h int) int {
	mu.Lock()
	defer mu.Unlock()
	s := seqOf(h)
	if s == nil {
		return 0xFFFF
	}
	for s.tnext < len(s.timb) {
		t := s.timb[s.tnext]
		s.tnext++
		if _, ok := drv.timbres[t[1]<<8|t[0]]; !ok {
			return t[1]<<8 | t[0]
		}
	}
	return 0xFFFF
}

// StartSequence plays sequence h from its beginning.
func StartSequence(h int) {
	mu.Lock()
	defer mu.Unlock()
	if s := seqOf(h); s != nil {
		s.stop()
		s.pos, s.delay, s.loops = 0, 0, nil
		s.status = SeqPlaying
	}
}

// StopSequence stops sequence h and silences its notes.
func StopSequence(h int) {
	mu.Lock()
	defer mu.Unlock()
	if s := seqOf(h); s != nil && s.status == SeqPlaying {
		s.stop()
		s.status = SeqStopped
	}
}

// SequenceStatus is SeqStopped, SeqPlaying or SeqDone.
func SequenceStatus(h int) int {
	mu.Lock()
	defer mu.Unlock()
	if s := seqOf(h); s != nil {
		return s.status
	}
	return SeqDone
}

// SetRelativeVolume scales sequence h's channel volumes to pct percent. The fade time is
// not kept: the change is at once.
func SetRelativeVolume(h, pct, ms int) {
	mu.Lock()
	defer mu.Unlock()
	s := seqOf(h)
	if s == nil {
		return
	}
	s.relVol = pct
	for c := range s.shadow {
		if s.shadow[c].set && !drv.ch[c].locked {
			drv.message(0xB0|c, 7, s.shadow[c].vol*pct/100)
		}
	}
}

// SetRelativeTempo: the music always plays at its own tempo.
func SetRelativeTempo(h, pct, ms int) {}

func (s *sequence) stop() {
	for _, n := range s.notes {
		if !drv.ch[n.ch].locked {
			drv.noteOff(n.ch, n.note)
		}
	}
	s.notes = s.notes[:0]
}

func (s *sequence) dropNotes(c int) {
	k := s.notes[:0]
	for _, n := range s.notes {
		if n.ch != c {
			k = append(k, n)
		}
	}
	s.notes = k
}

// restore sends channel c the settings the sequence last gave it.
func (s *sequence) restore(c int) {
	sh := &s.shadow[c]
	if !sh.set {
		return
	}
	drv.message(0xB0|c, 0x72, sh.bank)
	drv.message(0xC0|c, sh.program, 0)
	drv.message(0xB0|c, 7, sh.vol*s.relVol/100)
	drv.message(0xB0|c, 0x0B, sh.expr)
	drv.message(0xB0|c, 0x0A, sh.pan)
	drv.message(0xE0|c, sh.bend&0x7F, sh.bend>>7)
}

func (s *sequence) varlen() int {
	v := 0
	for s.pos < len(s.evnt) {
		b := s.evnt[s.pos]
		s.pos++
		v = v<<7 | int(b&0x7F)
		if b&0x80 == 0 {
			break
		}
	}
	return v
}

func (s *sequence) byte() int {
	if s.pos >= len(s.evnt) {
		return 0
	}
	b := s.evnt[s.pos]
	s.pos++
	return int(b)
}

// send passes a message on unless its channel is locked, keeping the sequence's view of
// the channel either way.
func (s *sequence) send(status, d1, d2 int) {
	c := status & 15
	sh := &s.shadow[c]
	if !sh.set {
		sh.set = true
		sh.vol, sh.expr, sh.pan, sh.bend = 127, 127, 64, 0x2000
	}
	switch status & 0xF0 {
	case 0xB0:
		switch d1 {
		case 7:
			sh.vol = d2
			d2 = d2 * s.relVol / 100
			if d2 > 127 {
				d2 = 127
			}
		case 0x0B:
			sh.expr = d2
		case 0x0A:
			sh.pan = d2
		case 0x72:
			sh.bank = d2
		}
	case 0xC0:
		sh.program = d1
	case 0xE0:
		sh.bend = d1 | d2<<7
	}
	if !drv.ch[c].locked {
		drv.message(status, d1, d2)
	}
}

// tick runs one 120 Hz tick of the sequence.
func (s *sequence) tick() {
	if s.status != SeqPlaying {
		return
	}
	k := s.notes[:0]
	for _, n := range s.notes {
		n.left--
		if n.left <= 0 {
			if !drv.ch[n.ch].locked {
				drv.noteOff(n.ch, n.note)
			}
			continue
		}
		k = append(k, n)
	}
	s.notes = k
	if s.delay > 0 {
		s.delay--
		if s.delay > 0 {
			return
		}
	}
	for s.status == SeqPlaying {
		if s.pos >= len(s.evnt) {
			s.end()
			return
		}
		b := int(s.evnt[s.pos])
		if b < 0x80 {
			for s.pos < len(s.evnt) && s.evnt[s.pos] < 0x80 {
				s.delay += int(s.evnt[s.pos])
				s.pos++
			}
			if s.delay > 0 {
				return
			}
			continue
		}
		s.pos++
		switch b & 0xF0 {
		case 0x80:
			n, _ := s.byte(), s.byte()
			s.send(b, n, 0)
		case 0x90:
			n, v := s.byte(), s.byte()
			dur := s.varlen()
			s.send(b, n, v)
			if v != 0 && !drv.ch[b&15].locked {
				if dur <= 0 {
					drv.noteOff(b&15, n)
				} else {
					s.notes = append(s.notes, seqNote{b & 15, n, dur})
				}
			}
		case 0xA0, 0xE0:
			d1, d2 := s.byte(), s.byte()
			s.send(b, d1, d2)
		case 0xB0:
			d1, d2 := s.byte(), s.byte()
			switch d1 {
			case 0x74:
				s.loops = append(s.loops, struct{ pos, count int }{s.pos, d2})
			case 0x75:
				if n := len(s.loops); n > 0 && d2 >= 64 {
					l := &s.loops[n-1]
					if l.count == 0 {
						s.pos = l.pos
					} else if l.count--; l.count > 0 {
						s.pos = l.pos
					} else {
						s.loops = s.loops[:n-1]
					}
				}
			default:
				s.send(b, d1, d2)
			}
		case 0xC0, 0xD0:
			d1 := s.byte()
			s.send(b, d1, 0)
		case 0xF0:
			switch b {
			case 0xFF:
				typ := s.byte()
				n := s.varlen()
				s.pos += n
				if typ == 0x2F {
					s.end()
					return
				}
			case 0xF0, 0xF7:
				s.pos += s.varlen()
			}
		}
	}
}

func (s *sequence) end() {
	s.stop()
	s.status = SeqDone
}
