package snd

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// TestRender renders a theme offline to $UW1GO_WAV (if set) and checks it is not silent.
func TestRender(t *testing.T) {
	lib, err := os.ReadFile("../../gamedata/UW/SOUND/UW.AD")
	if err != nil {
		t.Skip("no game data")
	}
	name := os.Getenv("UW1GO_XMI")
	if name == "" {
		name = "AW01.XMI"
	}
	xmi, _ := os.ReadFile("../../gamedata/UW/SOUND/" + name)
	drv = &driver{b: newFM(false), installed: map[int]bool{}}
	h := RegisterSequence(xmi)
	if h < 0 {
		t.Fatal("not a sequence")
	}
	for r := TimbreRequest(h); r != 0xFFFF; r = TimbreRequest(h) {
		bank, patch := r>>8, r&0xFF
		for i := 0; i+6 <= len(lib) && lib[i+1] != 0xFF; i += 6 {
			if int(lib[i+1]) == bank && int(lib[i]) == patch {
				off := int(binary.LittleEndian.Uint32(lib[i+2:]))
				size := int(binary.LittleEndian.Uint16(lib[off:]))
				InstallTimbre(bank, patch, lib[off:off+size])
			}
		}
	}
	StartSequence(h)
	SetRelativeVolume(h, 0x60, 0)
	pcm := renderSecs(t, 20)
	if out := os.Getenv("UW1GO_WAV"); out != "" {
		writeWAV(out, pcm)
	}
}

func renderSecs(t *testing.T, secs int) []byte {
	pcm := make([]byte, secs*outRate*4)
	for s := 0; s < secs; s++ {
		stream{}.Read(pcm[s*outRate*4 : (s+1)*outRate*4])
		var sum float64
		peak := 0
		for i := s * outRate * 4; i < (s+1)*outRate*4; i += 4 {
			v := int(int16(binary.LittleEndian.Uint16(pcm[i:])))
			sum += float64(v * v)
			if v > peak {
				peak = v
			}
		}
		t.Logf("sec %2d rms %6.0f peak %5d", s, math.Sqrt(sum/outRate), peak)
	}
	return pcm
}

func writeWAV(out string, pcm []byte) {
	hdr := make([]byte, 44)
	copy(hdr, "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(pcm)))
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1)
	binary.LittleEndian.PutUint16(hdr[22:], 2)
	binary.LittleEndian.PutUint32(hdr[24:], outRate)
	binary.LittleEndian.PutUint32(hdr[28:], outRate*4)
	binary.LittleEndian.PutUint16(hdr[32:], 4)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(pcm)))
	os.WriteFile(out, append(hdr, pcm...), 0o644)
}

// TestPitch plays A4 on a plain sine timbre and counts zero crossings.
func TestPitch(t *testing.T) {
	fm := newFM(false)
	drv = &driver{b: fm, installed: map[int]bool{}}
	// mult 1, sustained; no modulation (modulator silent, FM connection), carrier full level
	tim := []byte{14, 0, 0, 0x21, 0x3F, 0xF0, 0x0F, 0, 0, 0x21, 0x00, 0xF0, 0x0F, 0}
	InstallTimbre(0, 5, tim)
	SendVoice(0xC0, 5, 0)
	SendVoice(0x90, 69, 127)
	var prev, cross int
	n := oplRate
	for i := 0; i < n; i++ {
		l, _ := fm.sample()
		if i > 1000 && (prev < 0) != (l < 0) {
			cross++
		}
		prev = l
	}
	f := float64(cross) / 2 * float64(oplRate) / float64(n-1000)
	t.Logf("A4 measured %.1f Hz", f)
	if f < 435 || f > 445 {
		t.Fail()
	}
}
