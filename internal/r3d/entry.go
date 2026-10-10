package r3d

import "fmt"

// Segment bases in the emulated memory (paragraph*16) of the far data the
// renderer and the game's C share.
const (
	Seg048 = (0x3963 + LoadSeg) * 16 // the graphics library's data
	Seg051 = (0x4723 + LoadSeg) * 16 // the renderer's data: the database, the models
	Seg063 = (0x5624 + LoadSeg) * 16 // seg019's data: the camera, the private stack
)

const dgroupPara = 0x5AAC

// The C stack the game's calls into translated code run on (DGROUP's, in DOS),
// a block of its own, as the original port's x86/entry.c.
const (
	cstackSeg  = dgroupPara + LoadSeg + 0x0800
	cstackSize = 0x1000
)

// The far heap: blocks the game allocates for the renderer (textures, the
// palettes it reads) start where DOS's heap did.
const heapFirst = 0x6955 + LoadSeg

var heapNext = heapFirst

var loaded bool

// Loaded reports whether Load has run.
func Loaded() bool { return loaded }

// Load copies UW.EXE's load image into the emulated memory at LoadSeg and
// applies its relocations.
func Load(exe []byte) error {
	le16 := func(b []byte, i int) int { return int(b[i]) | int(b[i+1])<<8 }
	if len(exe) < 0x20 || exe[0] != 'M' || exe[1] != 'Z' {
		return fmt.Errorf("r3d: not an MZ executable")
	}
	hdr := le16(exe, 8) * 16
	size := le16(exe, 4) * 512
	if last := le16(exe, 2); last != 0 {
		size -= 512 - last
	}
	size -= hdr
	if hdr+size > len(exe) || LoadSeg*16+size > heapFirst*16 {
		return fmt.Errorf("r3d: UW.EXE's image does not fit")
	}
	copy(mem[LoadSeg*16:], exe[hdr:hdr+size])
	nrel := le16(exe, 6)
	relo := le16(exe, 0x18)
	for i := 0; i < nrel; i++ {
		off := le16(exe, relo+4*i)
		seg := le16(exe, relo+4*i+2)
		a := (seg+LoadSeg)*16 + off
		w := uint16(mem[a]) | uint16(mem[a+1])<<8
		w += LoadSeg
		mem[a] = byte(w)
		mem[a+1] = byte(w >> 8)
	}
	loaded = true
	initEMS()
	return nil
}

// Alloc gives a block of n bytes of the far heap, paragraph aligned: its
// segment (offset 0).
func Alloc(n int) uint16 {
	seg := heapNext
	heapNext += (n + 15) >> 4
	if heapNext*16 > EMSFrameSeg*16 {
		panic("r3d: far heap exhausted")
	}
	return uint16(seg)
}

// Addr is the flat address of seg:off.
func Addr(seg, off uint16) int { return int(seg)<<4 + int(off) }

// R16 and W16 read and write a word of the emulated memory.
func R16(a int) uint16    { return uint16(mem[a]) | uint16(mem[a+1])<<8 }
func W16(a int, v uint16) { mem[a] = byte(v); mem[a+1] = byte(v >> 8) }

var depth int

// farEntry calls seg:off as Turbo C's far call did, with the word arguments
// pushed right to left on the C stack; it returns DX:AX.
func farEntry(seg, off uint16, args ...uint16) uint32 {
	sp := uint16(cstackSize - 0x200*depth)
	if sp < 0x200 {
		port_halt("Go calls into translated code nested too deep")
	}
	s := save()
	depth++
	SET_SS(cstackSeg)
	SET_DS(cstackSeg)
	SET_ES(cstackSeg)
	SP = sp
	for i := 0; i < 4; i++ {
		push16(0)
	}
	for i := len(args) - 1; i >= 0; i-- {
		push16(args[i])
	}
	asm_run_far(seg, off)
	r := uint32(DX)<<16 | uint32(AX)
	depth--
	restore(s)
	return r
}

// cstack slots for near pointer arguments (cSinCos's results).
func nearSlot(i int) uint16 { return uint16(0x10 + 2*i) }
func nearGet(i int) int16   { return int16(R16(cstackSeg*16 + 0x10 + 2*i)) }

const seg019 = 0x1F3A

// The C entry points into the renderer (C3DENTRY.ASM).

func CZoom(zoom uint16) { farEntry(seg019, 0x0C34, zoom) }
func CFBtoScreen()      { farEntry(seg019, 0x0C7E) }
func CFillFB(c int)     { farEntry(seg019, 0x0C84, uint16(c)) }
func CDimFB(shade int)  { farEntry(seg019, 0x0CB7, uint16(shade)) }
func CLiteFB(count int) { farEntry(seg019, 0x0CEA, uint16(count)) }
func CPlaceFB(x, y, w, h int) {
	farEntry(seg019, 0x0D1D, uint16(x), uint16(y), uint16(w), uint16(h))
}
func CRender() uint32 { return farEntry(seg019, 0x0D7A) }
func CInit3d()        { farEntry(seg019, 0x0E1A) }

// CSinCos is the table sine and cosine of angle.
func CSinCos(angle int) (int16, int16) {
	farEntry(seg019, 0x0EAE, uint16(angle), nearSlot(0), nearSlot(1))
	return nearGet(0), nearGet(1)
}

// The aux palettes (LOADGR.C's Palettes) as the translated do_uwobj reads
// them: a far block the game keeps up to date (SetPalettes).
var palSeg uint16

func palettesOff() uint16 { return 0 }
func palettesSeg() uint16 {
	if palSeg == 0 {
		palSeg = Alloc(32 * 16)
	}
	return palSeg
}

// SetPalettes copies the 32 aux palettes to where the renderer reads them.
func SetPalettes(p *[32][16]byte) {
	a := int(palettesSeg()) << 4
	for i := range p {
		copy(mem[a+i*16:], p[i][:])
	}
}

// SetClock stores the game clock where cRender reads it (seg019:0710).
func SetClock(t uint32) {
	W16(CODE019+0x710, uint16(t))
	W16(CODE019+0x712, uint16(t>>16))
}

// SetTheWindow sets the graphics library's clip window, which render_3d also
// reads (GRCORE's set_the_window: left, top, right, bottom, y counting up).
func SetTheWindow(x0, y0, x1, y1 int) {
	farEntry(0x0090, 0x52BA, uint16(x0), uint16(y0), uint16(x1), uint16(y1))
}

// Camera field offsets in seg063 (struct Camera at 0970).
const camBase = Seg063 + 0x970

// SetCamera stores the view's camera where the renderer reads it.
func SetCamera(x, y, z, heading, pitch, bank int16) {
	W16(camBase+0x0A, uint16(x))
	W16(camBase+0x0E, uint16(z))
	W16(camBase+0x12, uint16(y))
	W16(camBase+0x28, uint16(pitch))
	W16(camBase+0x2A, uint16(bank))
	W16(camBase+0x2C, uint16(heading))
}

// Light tables (cLightTabs, 16 maps of 256) and the pixel-transfer tables (cXfer).
var (
	lightTabs = CODE004 + 0x696E
	xferTab   = CODE003 + 0x1201
)

// SetLightTabs copies the 16 shading maps to where the renderer reads them.
func SetLightTabs(b []byte) { copy(mem[lightTabs:lightTabs+0x1000], b) }

// SetXfer copies XFER.DAT's tables (cXfer).
func SetXfer(b []byte) { copy(mem[xferTab:xferTab+0x600], b) }

// SetSmooth stores the distance shading parameters (smooth_div, smooth_base,
// smooth_lowpass).
func SetSmooth(div, base, lowpass int16) {
	W16(Seg048+0x64C, uint16(div))
	W16(Seg048+0x64E, uint16(base))
	W16(Seg048+0x650, uint16(lowpass))
}

// SetPixXfer selects pixel-transfer mode n (GRIDDB.C's set_pix_xfer).
func SetPixXfer(n int) {
	p := Seg048 + 0xB12
	W16(p, R16(p+2*n))
}

// CopyFB copies the renderer's frame buffer, w by h, into dst top row first. The renderer's
// y counts up: its row 0 (the first in memory) is the bottom one, as cFBtoScreen copies it.
func CopyFB(dst []byte, w, h int) {
	base := int(R16(Seg048+0x95A)) << 4
	for y := 0; y < h; y++ {
		o := base + int(R16(Seg048+0x95E+2*(h-1-y)))
		copy(dst[y*w:y*w+w], mem[o:o+w])
	}
}

// Bytes is a window onto the emulated memory at flat address a.
func Bytes(a, n int) []byte { return mem[a : a+n] }
