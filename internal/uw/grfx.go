package uw

import (
	"io"

	"github.com/felix/uw1go/internal/gr"
)

// Ports of GRFX.C (graphics start-up, fonts, palettes and fades), COLCYCLE.C (palette
// colour cycling) and SHOWPIC.C (full-screen pictures).

var fadeBuffer [0x300]byte

func grfxInit() bool {
	gr.InitGraphics()
	gr.SetTheWindow(0, 0xC7, 0x13F, 0)
	gr.GrSoftPageFlip()
	return grfxLoadFont("font5x6p.sys")
}

// grfxLoadFont loads DATA\name as the current font.
func grfxLoadFont(name string) bool {
	b := readFile("DATA\\" + name)
	if b == nil {
		return false
	}
	gr.CurFont = gr.LoadFont(b)
	return gr.CurFont != nil
}

func grfxClose() {
	mouseHide()
}

// grfxClear clears the whole screen to colour 0.
func grfxClear() {
	mouseHide()
	gr.SetTheWindow(0, 0xC7, 0x13F, 0)
	gr.SetTheColor(0)
	gr.ClearWindow()
	mouseShow()
}

// readQuikpal reads palette n of PALS.DAT (768 bytes, 6-bit RGB) into dest.
func readQuikpal(n int, dest []byte) bool {
	fd := dosOpen("data/pals.dat", O_RDONLY)
	if fd < 0 {
		return false
	}
	defer dosClose(fd)
	dosSeek(fd, int64(n*0x300), io.SeekStart)
	return dosRead(fd, dest[:0x300]) == 0x300
}

// grfxQuikpal loads palette n of PALS.DAT into the VGA.
func grfxQuikpal(n int) bool {
	if readQuikpal(n, gr.Palette[:]) {
		gr.LocalDoPalette(0x100, 0)
		return true
	}
	return false
}

func grfxSetpal(src []byte) {
	copy(gr.Palette[:], src[:0x300])
	gr.LocalDoPalette(0x100, 0)
}

// fadeout fades from palette src to black in count*8 steps, 8 ticks apart.
func fadeout(src []byte, count int) {
	buf := fadeBuffer[:]
	var acc [0x300]uint16
	start := GameTime()
	if count == 0 {
		clear(buf)
		grfxSetpal(buf)
		return
	}
	scale := count << 3
	for i := range acc {
		acc[i] = uint16(scale * int(src[i]))
	}
	for step := 0; step < scale; step++ {
		for i := range acc {
			acc[i] -= uint16(src[i])
			buf[i] = byte(acc[i] / uint16(scale))
		}
		for GameTime()-start < 8 {
		}
		grfxSetpal(buf)
		start = GameTime()
	}
}

// fadein fades from black to palette src.
func fadein(src []byte, count int) {
	buf := fadeBuffer[:]
	var acc [0x300]uint16
	start := GameTime()
	if count == 0 {
		grfxSetpal(src)
		return
	}
	scale := count << 3
	for step := 0; step < scale; step++ {
		for i := range acc {
			acc[i] += uint16(src[i])
			buf[i] = byte(acc[i] / uint16(scale))
		}
		for GameTime()-start < 8 {
		}
		grfxSetpal(buf)
		start = GameTime()
	}
}

// COLCYCLE.C

var (
	ccLastPhase byte
	ccLastHalf  byte
)

// rotateBank rotates count palette entries from first by one place, upward when up.
func rotateBank(first, count int, up bool) {
	p := gr.Palette[:]
	if count < 1 {
		return
	}
	lo, hi := first*3, (first+count)*3
	var saved [3]byte
	if up {
		copy(saved[:], p[hi-3:hi])
		copy(p[lo+3:hi], p[lo:hi-3])
		copy(p[lo:lo+3], saved[:])
	} else {
		copy(saved[:], p[lo:lo+3])
		copy(p[lo:hi-3], p[lo+3:hi])
		copy(p[hi-3:hi], saved[:])
	}
}

// cycleColors rotates the colour banks every 64 ticks of t.
func cycleColors(t byte) {
	if t>>5 == ccLastPhase {
		return
	}
	ccLastPhase = t >> 5
	if ccLastPhase>>1 == ccLastHalf {
		return
	}
	rotateBank(0x30, 4, false)
	rotateBank(0x34, 4, false)
	rotateBank(0x38, 4, false)
	rotateBank(0x3C, 4, false)
	gr.LocalDoPalette(0x10, 0x30)
	rotateBank(0x10, 5, true)
	rotateBank(0x15, 3, true)
	gr.LocalDoPalette(8, 0x10)
	ccLastHalf = ccLastPhase >> 1
}

// SHOWPIC.C

// loadBitMap shows a full-screen 320x200 .BYT picture. pal >= 0 clears the screen, draws
// on the hidden page, loads palette pal and flips; pal < 0 draws on the current page.
func loadBitMap(pal int, name string) bool {
	buf := make([]byte, 0xFA00)
	if bltfromdrive(name, buf) {
		if pal >= 0 {
			grfxClear()
			gr.GrSoftPageFlip()
		}
		gr.SetTheWindow(0, 0xC7, 0x13F, 0)
		t := gr.Transparency
		gr.Transparency = false
		gr.Show(0, 0xC7, buf, 0xC8, 0x140, 0, 0)
		gr.Transparency = t
		if pal >= 0 {
			grfxQuikpal(pal)
			gr.GrPageFlip()
			gr.GrSoftPageFlip()
		}
	}
	return true
}

// camerasFade is the crystal ball's view: fade to the detached camera, stay while the
// mouse button is held, then fade back to the player's eye.
func camerasFade() {
	renderFB()
	attachEye(-1)
	fadeout3d(5)
	renderFB()
	fadein3d(5)
	mouseRelease(true)
	renderFB()
	fadeout3d(5)
	attachEye(1)
	renderFB()
	fadein3d(5)
}
