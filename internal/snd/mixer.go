package snd

import (
	"encoding/binary"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// The mixer: the emulated chip at its own rate, resampled to the output rate, with the
// music's 120 Hz ticks counted in chip samples, and the speech's 8-bit PCM mixed in.

const outRate = 44100

var (
	actx   *audio.Context
	player *audio.Player

	chipPos  float64 // chip samples owed to the output, fractional
	prevL    int
	prevR    int
	curL     int
	curR     int
	tickAcc  int
	speech   []byte // unsigned 8-bit mono
	speechHz int
	speechAt float64
	started  bool
)

// Startup sets up the driver and starts the audio output. stereo pans the FM voices (the
// dual-chip cards).
func Startup(stereo bool) {
	mu.Lock()
	drv = newDriver(stereo)
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
			drv.allNotesOff(c)
		}
	}
	speech = nil
}

type stream struct{}

func (stream) Read(buf []byte) (int, error) {
	frames := len(buf) / 4
	mu.Lock()
	defer mu.Unlock()
	step := float64(oplRate) / outRate
	for f := 0; f < frames; f++ {
		var l, r int
		if drv != nil {
			chipPos += step
			for chipPos >= 1 {
				chipPos--
				prevL, prevR = curL, curR
				curL, curR = drv.render()
				tickAcc += 120
				if tickAcc >= oplRate {
					tickAcc -= oplRate
					for _, s := range drv.seqs {
						if s != nil {
							s.tick()
						}
					}
				}
			}
			l = prevL + int(float64(curL-prevL)*chipPos)
			r = prevR + int(float64(curR-prevR)*chipPos)
			l, r = l*2, r*2
		}
		if speech != nil {
			i := int(speechAt)
			if i >= len(speech) {
				speech = nil
			} else {
				v := (int(speech[i]) - 128) << 7
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
