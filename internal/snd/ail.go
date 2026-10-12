package snd

import (
	"encoding/binary"
	"sync"
)

// The AIL XMIDI driver's interface, as SOUND.C uses it, over a synthesiser backend (the
// emulated OPL2 with the AdLib driver's voice handling, or an MT-32 emulated by munt):
// global timbres installed by bank and patch, channels that the program locks away from
// the music for its own notes, and XMIDI sequences played at the fixed 120 Hz XMIDI rate.
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

// backend is a synthesiser under the driver.
type backend interface {
	message(status, d1, d2 int)        // a channel voice message
	sounding(c int) bool               // whether channel c has notes on
	allNotesOff(c int)                 // silence channel c
	builtIn(bank, patch int) bool      // a timbre the synthesiser has without installing
	install(bank, patch int, t []byte) // a timbre record, with its length word
	lockOrder() []int                  // the channels that may be locked, by preference
	release(c int)                     // a locked channel back to its defaults
	render(out []int, frames int)      // add frames of stereo output at outRate
}

type driver struct {
	b         backend
	locked    [16]bool
	installed map[int]bool
	seqs      []*sequence
}

var drv *driver

// musicVol is the desk's music fader (Volume) as the sequences' channel volumes last
// got it; the effects' notes are scaled by the sfx fader as they are sent.
var musicVol = 1.0

func (d *driver) has(bank, patch int) bool {
	return d.installed[bank<<8|patch] || d.b.builtIn(bank, patch)
}

func (d *driver) noteOff(c, note int) { d.b.message(0x80|c, note, 0) }

// InstallTimbre gives the driver a timbre, the whole record with its length word.
func InstallTimbre(bank, patch int, t []byte) {
	mu.Lock()
	defer mu.Unlock()
	if drv == nil {
		return
	}
	drv.installed[bank<<8|patch] = true
	drv.b.install(bank, patch, t)
}

// TimbreStatus is whether bank, patch is installed.
func TimbreStatus(bank, patch int) bool {
	mu.Lock()
	defer mu.Unlock()
	return drv != nil && drv.has(bank, patch)
}

// LockChannel takes a channel away from the music for the caller, preferring one with
// no notes sounding. Its notes are silenced. 0 if none is free.
func LockChannel() int {
	mu.Lock()
	defer mu.Unlock()
	if drv == nil {
		return 0
	}
	best := -1
	for _, c := range drv.b.lockOrder() {
		if drv.locked[c] {
			continue
		}
		if !drv.b.sounding(c) {
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
	drv.b.allNotesOff(best)
	for _, s := range drv.seqs {
		if s != nil {
			s.dropNotes(best)
		}
	}
	drv.locked[best] = true
	return best + 1
}

// ReleaseChannel gives a locked channel back to the music, silenced and with the
// music's settings for it put back.
func ReleaseChannel(ch int) {
	mu.Lock()
	defer mu.Unlock()
	c := ch - 1
	if drv == nil || c < 0 || c > 15 || !drv.locked[c] {
		return
	}
	drv.b.allNotesOff(c)
	drv.locked[c] = false
	drv.b.release(c)
	for _, s := range drv.seqs {
		if s != nil && s.status == SeqPlaying {
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
	if status&0xF0 == 0x90 && d2 != 0 {
		d2 = int(float64(d2&0x7F)*Volume("sfx") + 0.5)
	}
	drv.b.message(status&0xFF, d1&0x7F, d2&0x7F)
}

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
		if !drv.has(t[1], t[0]) {
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
		if s.shadow[c].set && !drv.locked[c] {
			drv.b.message(0xB0|c, 7, s.vol(c))
		}
	}
}

// SetRelativeTempo: the music always plays at its own tempo.
func SetRelativeTempo(h, pct, ms int) {}

func (s *sequence) stop() {
	for _, n := range s.notes {
		if !drv.locked[n.ch] {
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
	drv.b.message(0xB0|c, 0x72, sh.bank)
	drv.b.message(0xC0|c, sh.program, 0)
	drv.b.message(0xB0|c, 7, s.vol(c))
	drv.b.message(0xB0|c, 0x0B, sh.expr)
	drv.b.message(0xB0|c, 0x0A, sh.pan)
	drv.b.message(0xE0|c, sh.bend&0x7F, sh.bend>>7)
}

// vol is channel c's volume as sent: the sequence's, scaled by its relative volume and
// the music fader.
func (s *sequence) vol(c int) int {
	return int(float64(min(127, s.shadow[c].vol*s.relVol/100))*musicVol + 0.5)
}

// fade resends the sequences' channel volumes when the music fader has moved.
func fade() {
	g := Volume("music")
	if g == musicVol || drv == nil {
		return
	}
	musicVol = g
	for _, s := range drv.seqs {
		if s == nil {
			continue
		}
		for c := range s.shadow {
			if s.shadow[c].set && !drv.locked[c] {
				drv.b.message(0xB0|c, 7, s.vol(c))
			}
		}
	}
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
			d2 = s.vol(c)
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
	if !drv.locked[c] {
		drv.b.message(status, d1, d2)
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
			if !drv.locked[n.ch] {
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
			if v != 0 && !drv.locked[b&15] {
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
