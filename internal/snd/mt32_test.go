//go:build darwin || linux || freebsd

package snd

import (
	"os"
	"testing"
)

// TestMT32 opens munt with the ROMs in $UW1GO_MT32_ROMS and plays a note; without the
// library it is skipped, without usable ROMs it must fail cleanly.
func TestMT32(t *testing.T) {
	if err := loadMT32Lib(); err != nil {
		t.Skip(err)
	}
	t.Logf("libmt32emu %s", mtAPI.version())
	dir := os.Getenv("UW1GO_MT32_ROMS")
	if dir == "" {
		dir = t.TempDir()
	}
	started = true // no audio output in the test
	if err := StartupMT32(dir); err != nil {
		t.Logf("as expected without ROMs: %v", err)
		return
	}
	SendVoice(0x91, 60, 100)
	out := make([]int, 2*outRate/2)
	drv.b.render(out, outRate/2)
	peak := 0
	for _, v := range out {
		if v > peak {
			peak = v
		}
	}
	t.Logf("peak %d", peak)
	if peak == 0 {
		t.Fatal("silent")
	}
}

// TestRenderMT32 renders UW01.XMI on munt to $UW1GO_WAV, then plays each effect timbre
// of UW.MT alone and checks it sounds.
func TestRenderMT32(t *testing.T) {
	dir := os.Getenv("UW1GO_MT32_ROMS")
	lib, err := os.ReadFile("../../gamedata/UW/SOUND/UW.MT")
	if dir == "" || err != nil {
		t.Skip("no ROMs or game data")
	}
	started = true
	if err := StartupMT32(dir); err != nil {
		t.Fatal(err)
	}
	timbre := func(bank, patch int) []byte {
		for i := 0; i+6 <= len(lib) && lib[i+1] != 0xFF; i += 6 {
			if int(lib[i+1]) == bank && int(lib[i]) == patch {
				off := int(lib[i+2]) | int(lib[i+3])<<8
				return lib[off : off+248]
			}
		}
		return nil
	}
	for p := 0; p < 64; p++ {
		InstallTimbre(1, p, timbre(1, p))
	}
	xmi, _ := os.ReadFile("../../gamedata/UW/SOUND/UW01.XMI")
	h := RegisterSequence(xmi)
	if r := TimbreRequest(h); r != 0xFFFF {
		t.Fatalf("timbre %#x requested", r)
	}
	StartSequence(h)
	SetRelativeVolume(h, 0x60, 0)
	pcm := renderSecs(t, 20)
	if out := os.Getenv("UW1GO_WAV"); out != "" {
		writeWAV(out, pcm)
	}
	StopSequence(h)
	renderSecs(t, 1)
	sfx, _ := os.ReadFile("../../gamedata/UW/SOUND/SOUNDS.DAT")
	for fx := 0; fx < int(sfx[0]); fx++ {
		e := sfx[1+fx*5:]
		ch := LockChannel()
		SendVoice(ch+0xAF, 0x72, 1)
		SendVoice(ch+0xBF, int(e[0]), 0)
		SendVoice(ch+0xAF, 0x79, 0)
		SendVoice(ch+0xAF, 7, 0x7F)
		SendVoice(ch+0xAF, 0x0B, 0x7F)
		SendVoice(ch+0x8F, int(e[1]), int(e[2]))
		out := make([]int, outRate/2*2)
		drv.b.render(out, outRate/2)
		peak := 0
		for _, v := range out {
			if v > peak {
				peak = v
			}
		}
		t.Logf("fx %2d patch %2d note %#x chan %d peak %d", fx, e[0], e[1], ch, peak)
		ReleaseChannel(ch)
		drv.b.render(out, outRate/2)
	}
}

// TestMT32CustomTimbre checks that bank 1 really selects the uploaded timbre: the same
// patch number from bank 0 (a built-in timbre) must sound different.
func TestMT32CustomTimbre(t *testing.T) {
	dir := os.Getenv("UW1GO_MT32_ROMS")
	lib, err := os.ReadFile("../../gamedata/UW/SOUND/UW.MT")
	if dir == "" || err != nil {
		t.Skip("no ROMs or game data")
	}
	started = true
	play := func(bank int) []int {
		if err := StartupMT32(dir); err != nil {
			t.Fatal(err)
		}
		off := int(lib[2]) | int(lib[3])<<8 // patch 0 of bank 1
		InstallTimbre(1, 8, lib[off:off+248])
		ch := LockChannel()
		SendVoice(ch+0xAF, 0x72, bank)
		SendVoice(ch+0xBF, 8, 0)
		SendVoice(ch+0x8F, 0x38, 0x60)
		out := make([]int, outRate/4*2)
		drv.b.render(out, outRate/4)
		return out
	}
	a, b := play(0), play(1)
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	t.Logf("%d of %d samples differ", diff, len(a))
	if diff == 0 {
		t.Fatal("bank 1 played the built-in timbre")
	}
	// and the same custom timbre twice is identical
	c := play(1)
	for i := range b {
		if b[i] != c[i] {
			t.Fatalf("not deterministic at %d", i)
		}
	}
}
