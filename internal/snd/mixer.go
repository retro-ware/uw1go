// Package snd is the sound hardware and driver layer under SOUND.C: a YM3812 (OPL2) FM
// emulator (github.com/memmaker/opl2), the Miles AIL XMIDI driver's job on it (timbres
// from the game's UW.AD library, voice allocation, channel locking, the 120 Hz XMIDI
// sequencer), a digitised .VOC player for the speech, and the mixer that feeds Ebiten's
// audio.
package snd

import (
	"encoding/binary"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// The mixer: the backend rendered in runs between the music's 120 Hz ticks, with the
// speech's 8-bit PCM mixed in.

const outRate = 44100

var (
	actx   *audio.Context
	player *audio.Player

	tickAcc  int
	speech   []byte // unsigned 8-bit mono
	speechHz int
	speechAt float64
	started  bool
	mixBuf   []int
)

// Startup sets up the driver on the emulated OPL2 and starts the audio output. stereo
// pans the FM voices (the dual-chip cards).
func Startup(stereo bool) {
	startWith(newFM(stereo))
}

func startWith(b backend) {
	mu.Lock()
	drv = &driver{b: b, installed: map[int]bool{}}
	mu.Unlock()
	StartOutput()
}

// StartOutput starts the audio output, once.
func StartOutput() {
	if started {
		return
	}
	started = true
	actx = audio.NewContext(outRate)
	p, err := actx.NewPlayer(stream{})
	if err != nil {
		return
	}
	p.SetBufferSize(60 * time.Millisecond)
	p.Play()
	player = p
}

// Shutdown silences everything.
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()
	if drv != nil {
		for i := range drv.seqs {
			if s := drv.seqs[i]; s != nil {
				s.stop()
			}
		}
		for c := 0; c < 16; c++ {
			drv.b.allNotesOff(c)
		}
	}
	speech = nil
}

type stream struct{}

func (stream) Read(buf []byte) (int, error) {
	frames := len(buf) / 4
	mu.Lock()
	defer mu.Unlock()
	if cap(mixBuf) < frames*2 {
		mixBuf = make([]int, frames*2)
	}
	mix := mixBuf[:frames*2]
	clear(mix)
	fade()
	if drv != nil {
		for f := 0; f < frames; {
			n := (outRate - tickAcc + 119) / 120
			if n > frames-f {
				n = frames - f
			}
			drv.b.render(mix[f*2:], n)
			f += n
			if tickAcc += n * 120; tickAcc >= outRate {
				tickAcc -= outRate
				for _, s := range drv.seqs {
					if s != nil {
						s.tick()
					}
				}
			}
		}
	}
	sg := Volume("sfx") // speech
	for f := 0; f < frames; f++ {
		l, r := mix[f*2], mix[f*2+1]
		if speech != nil {
			i := int(speechAt)
			if i >= len(speech) {
				speech = nil
			} else {
				v := int(float64((int(speech[i])-128)<<7) * sg)
				l += v
				r += v
				speechAt += float64(speechHz) / outRate
			}
		}
		binary.LittleEndian.PutUint16(buf[f*4:], uint16(clamp16(l)))
		binary.LittleEndian.PutUint16(buf[f*4+2:], uint16(clamp16(r)))
	}
	return frames * 4, nil
}

func clamp16(v int) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}

// PlayVOC plays a Creative Voice file's sound data (block types 1 and 2, 8-bit PCM),
// replacing any speech playing. False if there is none.
func PlayVOC(b []byte) bool {
	if len(b) < 0x1A || string(b[:19]) != "Creative Voice File" {
		return false
	}
	off := int(binary.LittleEndian.Uint16(b[0x14:]))
	var pcm []byte
	hz := 0
	for off+4 <= len(b) {
		typ := b[off]
		if typ == 0 {
			break
		}
		n := int(b[off+1]) | int(b[off+2])<<8 | int(b[off+3])<<16
		body := b[off+4:]
		if n > len(body) {
			n = len(body)
		}
		body = body[:n]
		switch typ {
		case 1:
			if len(body) >= 2 {
				if hz == 0 {
					hz = 1000000 / (256 - int(body[0]))
				}
				pcm = append(pcm, body[2:]...)
			}
		case 2:
			pcm = append(pcm, body...)
		}
		off += 4 + n
	}
	if len(pcm) == 0 || hz == 0 {
		return false
	}
	mu.Lock()
	speech, speechHz, speechAt = pcm, hz, 0
	mu.Unlock()
	return true
}

// SpeechPlaying is whether speech is still playing.
func SpeechPlaying() bool {
	mu.Lock()
	defer mu.Unlock()
	return speech != nil
}

// StopSpeech cuts the speech off.
func StopSpeech() {
	mu.Lock()
	speech = nil
	mu.Unlock()
}
