// Package r3d is UW1's 3D renderer and the graphics library under it: the
// original's assembly modules (seg003, seg004 and parts of seg019), translated
// instruction for instruction to Go by tools/c2go from Underworld Exhumed's
// static recompilation, running on the small 16-bit machine in this file.
//
// The machine: the registers and flags are globals (the model interpreter's
// handlers pass values in registers, as on the CPU), memory is one flat array
// indexed by segment*16+offset, and UW.EXE's load image sits in it at
// paragraph LoadSeg with its relocations applied, so the code segments' data
// (patched immediates, lightabs) and the far data segments (seg048, seg051,
// seg063) are where the code expects them. Calls push the return address on
// the emulated stack and run the callee until a return pops it (asm_call).
package r3d

import (
	"fmt"
)

// LoadSeg is the paragraph UW.EXE's image is loaded at (the runtime's default,
// as in the original port).
const LoadSeg = 0x0800

const memSize = 0x110010

var mem = make([]byte, memSize)

// Mem exposes the emulated memory to the rest of the game.
func Mem() []byte { return mem }

// Registers and flags.
var AX, BX, CX, DX, SI, DI, BP, SP uint16
var CF, ZF, SF, OF, DF uint8

// Segment registers and their bases (segment*16).
var asm_ds, asm_es, asm_ss uint16
var pDS, pES, pSS int

// Code segment bases (EXE paragraphs plus LoadSeg).
var (
	CODE000 = (0x0000 + LoadSeg) * 16
	CODE001 = (0x004E + LoadSeg) * 16
	CODE002 = (0x0086 + LoadSeg) * 16
	CODE003 = (0x0090 + LoadSeg) * 16
	CODE004 = (0x06E7 + LoadSeg) * 16
	CODE015 = (0x1DAE + LoadSeg) * 16
	CODE019 = (0x1F3A + LoadSeg) * 16
)

func SET_DS(v uint16) { asm_ds = v; pDS = int(v) << 4 }
func SET_ES(v uint16) { asm_es = v; pES = int(v) << 4 }
func SET_SS(v uint16) { asm_ss = v; pSS = int(v) << 4 }

func rb(p int, a uint16) uint8 { return mem[p+int(a)] }
func rw(p int, a uint16) uint16 {
	i := p + int(a)
	return uint16(mem[i]) | uint16(mem[i+1])<<8
}
func wb(p int, a uint16, v uint8) { mem[p+int(a)] = v }
func ww(p int, a uint16, v uint16) {
	i := p + int(a)
	mem[i] = byte(v)
	mem[i+1] = byte(v >> 8)
}

func push16(v uint16) { SP -= 2; ww(pSS, SP, v) }
func pop16() uint16   { v := rw(pSS, SP); SP += 2; return v }

func STEP(n int32) int32 {
	if DF != 0 {
		return -n
	}
	return n
}

type integer interface {
	~uint8 | ~int8 | ~uint16 | ~int16 | ~int32 | ~uint32 | ~int
}

func ifv[T any](c bool, a, b T) T {
	if c {
		return a
	}
	return b
}

func b2i[T integer](b bool) T {
	if b {
		return 1
	}
	return 0
}

func b8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func memEq(a, b int, n int32) bool {
	for i := 0; i < int(n); i++ {
		if mem[a+i] != mem[b+i] {
			return false
		}
	}
	return true
}

func asm_flags() uint16 {
	return 0x7002 | uint16(CF) | uint16(ZF)<<6 | uint16(SF)<<7 | 1<<9 | uint16(DF)<<10 | uint16(OF)<<11
}

func asm_set_flags(f uint16) {
	CF = uint8(f & 1)
	ZF = uint8(f >> 6 & 1)
	SF = uint8(f >> 7 & 1)
	DF = uint8(f >> 10 & 1)
	OF = uint8(f >> 11 & 1)
}

func asm_jcc(op uint8) bool {
	switch op & 0x0F {
	case 0x0:
		return OF != 0
	case 0x1:
		return OF == 0
	case 0x2:
		return CF != 0
	case 0x3:
		return CF == 0
	case 0x4:
		return ZF != 0
	case 0x5:
		return ZF == 0
	case 0x6:
		return CF != 0 || ZF != 0
	case 0x7:
		return CF == 0 && ZF == 0
	case 0x8:
		return SF != 0
	case 0x9:
		return SF == 0
	case 0xC:
		return SF != OF
	case 0xD:
		return SF == OF
	case 0xE:
		return ZF != 0 || SF != OF
	case 0xF:
		return ZF == 0 && SF == OF
	}
	port_halt("a patched jump on the parity flag")
	return false
}

// ---------------------------------------------------------------- arithmetic

func add16(a, b uint16, c uint32) uint16 {
	r := uint32(a) + uint32(b) + c
	t := uint16(r)
	CF = uint8(r >> 16 & 1)
	OF = uint8(((a ^ t) & (b ^ t)) >> 15 & 1)
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func sub16(a, b uint16, c uint32) uint16 {
	r := uint32(a) - uint32(b) - c
	t := uint16(r)
	CF = uint8(r >> 16 & 1)
	OF = uint8(((a ^ b) & (a ^ t)) >> 15 & 1)
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func add8(a, b uint8, c uint32) uint8 {
	r := uint32(a) + uint32(b) + c
	t := uint8(r)
	CF = uint8(r >> 8 & 1)
	OF = ((a ^ t) & (b ^ t)) >> 7 & 1
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func sub8(a, b uint8, c uint32) uint8 {
	r := uint32(a) - uint32(b) - c
	t := uint8(r)
	CF = uint8(r >> 8 & 1)
	OF = ((a ^ b) & (a ^ t)) >> 7 & 1
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func logic16(t uint16) uint16 {
	CF, OF = 0, 0
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func logic8(t uint8) uint8 {
	CF, OF = 0, 0
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func inc16(a uint16) uint16 {
	t := a + 1
	OF = b8(t == 0x8000)
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func dec16(a uint16) uint16 {
	t := a - 1
	OF = b8(a == 0x8000)
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func inc8(a uint8) uint8 {
	t := a + 1
	OF = b8(t == 0x80)
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func dec8(a uint8) uint8 {
	t := a - 1
	OF = b8(a == 0x80)
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func neg16(a uint16) uint16 { t := sub16(0, a, 0); CF = b8(a != 0); return t }
func neg8(a uint8) uint8    { t := sub8(0, a, 0); CF = b8(a != 0); return t }

func shl16(a uint16, n uint32) uint16 {
	n &= 31
	if n == 0 {
		return a
	}
	var t uint16
	if n < 16 {
		t = a << n
	}
	if n > 16 {
		CF = 0
	} else {
		CF = uint8(uint32(a) >> (16 - n) & 1)
	}
	OF = uint8(t>>15&1) ^ CF
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func shr16(a uint16, n uint32) uint16 {
	n &= 31
	if n == 0 {
		return a
	}
	var t uint16
	if n < 16 {
		t = a >> n
	}
	if n > 16 {
		CF = 0
	} else {
		CF = uint8(uint32(a) >> (n - 1) & 1)
	}
	OF = uint8(a >> 15 & 1)
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func sar16(a uint16, n uint32) uint16 {
	n &= 31
	if n == 0 {
		return a
	}
	s := int32(int16(a))
	var t uint16
	if n >= 16 {
		t = uint16(s >> 15)
		CF = uint8(a >> 15 & 1)
	} else {
		t = uint16(s >> n)
		CF = uint8(s >> (n - 1) & 1)
	}
	OF = 0
	ZF = b8(t == 0)
	SF = uint8(t >> 15)
	return t
}

func shl8(a uint8, n uint32) uint8 {
	n &= 31
	if n == 0 {
		return a
	}
	var t uint8
	if n < 8 {
		t = a << n
	}
	if n > 8 {
		CF = 0
	} else {
		CF = uint8(uint32(a) >> (8 - n) & 1)
	}
	OF = t>>7&1 ^ CF
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func shr8(a uint8, n uint32) uint8 {
	n &= 31
	if n == 0 {
		return a
	}
	var t uint8
	if n < 8 {
		t = a >> n
	}
	if n > 8 {
		CF = 0
	} else {
		CF = uint8(uint32(a) >> (n - 1) & 1)
	}
	OF = a >> 7 & 1
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func sar8(a uint8, n uint32) uint8 {
	n &= 31
	if n == 0 {
		return a
	}
	s := int32(int8(a))
	var t uint8
	if n >= 8 {
		t = uint8(s >> 7)
		CF = a >> 7 & 1
	} else {
		t = uint8(s >> n)
		CF = uint8(s >> (n - 1) & 1)
	}
	OF = 0
	ZF = b8(t == 0)
	SF = t >> 7
	return t
}

func rol16(a uint16, n uint32) uint16 {
	n &= 31
	if n == 0 {
		return a
	}
	k := n % 16
	if k != 0 {
		a = a<<k | a>>(16-k)
	}
	CF = uint8(a & 1)
	OF = uint8(a>>15&1) ^ CF
	return a
}

func ror16(a uint16, n uint32) uint16 {
	n &= 31
	if n == 0 {
		return a
	}
	k := n % 16
	if k != 0 {
		a = a>>k | a<<(16-k)
	}
	CF = uint8(a >> 15 & 1)
	OF = uint8((a>>15 ^ a>>14) & 1)
	return a
}

func rol8(a uint8, n uint32) uint8 {
	n &= 31
	if n == 0 {
		return a
	}
	k := n % 8
	if k != 0 {
		a = a<<k | a>>(8-k)
	}
	CF = a & 1
	OF = a>>7&1 ^ CF
	return a
}

func ror8(a uint8, n uint32) uint8 {
	n &= 31
	if n == 0 {
		return a
	}
	k := n % 8
	if k != 0 {
		a = a>>k | a<<(8-k)
	}
	CF = a >> 7 & 1
	OF = (a>>7 ^ a>>6) & 1
	return a
}

func rcl16(a uint16, n uint32) uint16 {
	n &= 31
	n %= 17
	for ; n > 0; n-- {
		c := uint8(a >> 15 & 1)
		a = a<<1 | uint16(CF)
		CF = c
		OF = uint8(a>>15&1) ^ CF
	}
	return a
}

func rcr16(a uint16, n uint32) uint16 {
	n &= 31
	n %= 17
	for ; n > 0; n-- {
		c := uint8(a & 1)
		OF = uint8(a>>15&1) ^ CF
		a = a>>1 | uint16(CF)<<15
		CF = c
	}
	return a
}

func rcl8(a uint8, n uint32) uint8 {
	n &= 31
	n %= 9
	for ; n > 0; n-- {
		c := a >> 7 & 1
		a = a<<1 | CF
		CF = c
		OF = a>>7&1 ^ CF
	}
	return a
}

func rcr8(a uint8, n uint32) uint8 {
	n &= 31
	n %= 9
	for ; n > 0; n-- {
		c := a & 1
		OF = a>>7&1 ^ CF
		a = a>>1 | CF<<7
		CF = c
	}
	return a
}

func shld16(d, s uint16, n uint32) uint16 {
	w := uint32(d)<<16 | uint32(s)
	n &= 31
	if n == 0 {
		return d
	}
	if n > 16 {
		port_halt("shld with a count above 16")
	}
	t := w << n >> 16 & 0xFFFF
	CF = uint8(d >> (16 - n) & 1)
	OF = uint8((t ^ uint32(d)) >> 15 & 1)
	ZF = b8(t == 0)
	SF = uint8(t >> 15 & 1)
	return uint16(t)
}

func shrd16(d, s uint16, n uint32) uint16 {
	w := uint32(s)<<16 | uint32(d)
	n &= 31
	if n == 0 {
		return d
	}
	if n > 16 {
		port_halt("shrd with a count above 16")
	}
	t := w >> n & 0xFFFF
	CF = uint8(d >> (n - 1) & 1)
	OF = uint8((t ^ uint32(d)) >> 15 & 1)
	ZF = b8(t == 0)
	SF = uint8(t >> 15 & 1)
	return uint16(t)
}

func mul8(s uint8) {
	AX = uint16(uint8(AX)) * uint16(s)
	CF = b8(AX>>8 != 0)
	OF = CF
}

func mul16(s uint16) {
	r := uint32(AX) * uint32(s)
	AX = uint16(r)
	DX = uint16(r >> 16)
	CF = b8(DX != 0)
	OF = CF
}

func imul8(s uint8) {
	r := int16(int8(AX)) * int16(int8(s))
	AX = uint16(r)
	CF = b8(r != int16(int8(r)))
	OF = CF
}

func imul16(s uint16) {
	r := int32(int16(AX)) * int32(int16(s))
	AX = uint16(r)
	DX = uint16(uint32(r) >> 16)
	CF = b8(r != int32(int16(r)))
	OF = CF
}

func imul16x(a, b uint16) uint16 {
	r := int32(int16(a)) * int32(int16(b))
	CF = b8(r != int32(int16(r)))
	OF = CF
	return uint16(r)
}

// The divides return true when the CPU would raise the divide fault.
func asm_div8(d uint8) bool {
	if d == 0 {
		return true
	}
	q := AX / uint16(d)
	if q > 0xFF {
		return true
	}
	AX = (AX%uint16(d))<<8 | q
	return false
}

func asm_idiv8(d uint8) bool {
	n := int32(int16(AX))
	if d == 0 {
		return true
	}
	q := n / int32(int8(d))
	if q > 127 || q < -128 {
		return true
	}
	AX = uint16(uint8(n%int32(int8(d))))<<8 | uint16(uint8(q))
	return false
}

func asm_div16(d uint16) bool {
	n := uint32(DX)<<16 | uint32(AX)
	if d == 0 {
		return true
	}
	q := n / uint32(d)
	if q > 0xFFFF {
		return true
	}
	DX = uint16(n % uint32(d))
	AX = uint16(q)
	return false
}

func asm_idiv16(d uint16) bool {
	n := int64(int32(uint32(DX)<<16 | uint32(AX)))
	if d == 0 {
		return true
	}
	q := n / int64(int16(d))
	if q > 32767 || q < -32768 {
		return true
	}
	DX = uint16(int16(n % int64(int16(d))))
	AX = uint16(q)
	return false
}

// ---------------------------------------------------------------- control flow

const (
	ASM_RET  = 4
	ASM_RETF = 5
	ASM_IRET = 6
)

func ASM_JMP(seg, off uint16) uint32 { return 0x80000000 | uint32(seg)<<16 | uint32(off) }
func isJmp(c uint32) bool            { return c&0x80000000 != 0 }
func jmpSeg(c uint32) uint16         { return uint16(c >> 16 & 0x7FFF) }
func retKind(c uint32) uint32        { return c & 0xFF }
func retExtra(c uint32) uint32       { return c >> 8 & 0xFFFF }

type module struct {
	seg, lo, hi uint16
	fn          func(uint16) uint32
	name        string
}

type glue struct {
	seg, off uint16
	fn       func() uint32
	name     string
}

var glues []glue

func findModule(seg, off uint16) *module {
	for i := range modules {
		m := &modules[i]
		if m.seg == seg && off >= m.lo && off < m.hi {
			return m
		}
	}
	return nil
}

// Trace, when set, logs every entry into translated code.
var Trace bool

func asm_exec(target uint32) uint32 {
	seg, off := jmpSeg(target), uint16(target)
	if Trace {
		fmt.Printf("asm %04X:%04X ax=%04X bx=%04X cx=%04X dx=%04X si=%04X di=%04X bp=%04X sp=%04X ds=%04X es=%04X ss=%04X\n",
			seg, off, AX, BX, CX, DX, SI, DI, BP, SP, asm_ds, asm_es, asm_ss)
	}
	for i := range glues {
		if glues[i].seg == seg && glues[i].off == off {
			return glues[i].fn()
		}
	}
	if m := findModule(seg, off); m != nil {
		return m.fn(off)
	}
	asm_halt_at(seg, off, "no translated code and no glue here")
	return 0
}

func run(target uint32, s uint16, size int) uint32 {
	c := target
	for {
		c = asm_exec(c)
		d := int16(SP - s)
		if !isJmp(c) {
			want := size + int(retExtra(c))
			if int(d) == want && (retKind(c) == ASM_RET) == (size == 2) {
				return 0
			}
			if int(d) > want {
				return c
			}
			port_halt(fmt.Sprintf("a return (kind %d) with SP %d bytes from the call's %d", retKind(c), d, want))
		}
		if d > 0 {
			return c
		}
	}
}

func asm_call(target uint32, retip uint16) uint32 {
	push16(retip)
	return run(target, SP, 2)
}

func asm_callf(target uint32, retcs, retip uint16) uint32 {
	push16(retcs)
	push16(retip)
	return run(target, SP, 4)
}

func asm_run_far(seg, off uint16) {
	if c := asm_callf(ASM_JMP(seg, off), 0xFFFF, 0xFFFF); c != 0 {
		port_halt("translated code returned past the Go that called it")
	}
}

func asm_run_near(seg, off uint16) {
	if c := asm_call(ASM_JMP(seg, off), 0xFFFF); c != 0 {
		port_halt("translated code returned past the Go that called it")
	}
}

func asm_glue_ret() uint32  { SP += 2; return ASM_RET }
func asm_glue_retf() uint32 { SP += 4; return ASM_RETF }

type state struct {
	ax, bx, cx, dx, si, di, bp, sp uint16
	ds, es, ss                     uint16
	cf, zf, sf, of, df             uint8
}

func save() state {
	return state{AX, BX, CX, DX, SI, DI, BP, SP, asm_ds, asm_es, asm_ss, CF, ZF, SF, OF, DF}
}

func restore(s state) {
	AX, BX, CX, DX, SI, DI, BP, SP = s.ax, s.bx, s.cx, s.dx, s.si, s.di, s.bp, s.sp
	SET_DS(s.ds)
	SET_ES(s.es)
	SET_SS(s.ss)
	CF, ZF, SF, OF, DF = s.cf, s.zf, s.sf, s.of, s.df
}

// Halt is raised (as a panic) when the translated code does something the
// machine does not model.
type Halt string

func (h Halt) Error() string { return string(h) }

func port_halt(why string) { panic(Halt("r3d: " + why)) }

func asm_halt_at(seg, ip uint16, why string) {
	port_halt(fmt.Sprintf("%04X:%04X: %s", seg, ip, why))
}

func asm_bad_entry(module string, entry uint16) {
	port_halt(fmt.Sprintf("the translated %s has no entry at %04X", module, entry))
}

// ---------------------------------------------------------------- divide faults

// asm_divfault runs the divide fault handler the renderer installed (its offset in seg004
// is the word at seg063:04D5) as the CPU would: flags, CS and IP on the stack, then the
// handler's own translated code. A handler either resumes after the divide (it moves the
// saved IP past the instruction and IRETs: the faulting code carries on, with the handler's
// AX and DX) or drops the frame and carries on in the procedure itself (run hands that jump
// back, and the faulting code passes it up).
func asm_divfault(seg, ip uint16, length uint32) uint32 {
	h := rw(Seg063, 0x4D5)
	push16(asm_flags())
	push16(seg + LoadSeg)
	push16(ip)
	frame := SP
	if c := run(ASM_JMP(0x06E7, h), frame, 6); c != 0 {
		return c
	}
	if resume := rw(pSS, frame); resume != ip+uint16(length) {
		asm_halt_at(seg, ip, fmt.Sprintf("divide fault handler %04X resumed at %04X, not after the divide", h, resume))
	}
	return 0
}

// ---------------------------------------------------------------- interrupts and ports

func asm_int(n uint8) uint32 {
	ah := uint8(AX >> 8)
	switch n {
	case 0x02:
		return 0
	case 0x10:
		switch ah {
		case 0x00: // set mode: the screen is the game's
			return 0
		case 0x1A: // display combination: a VGA with a colour display
			if uint8(AX) == 0 {
				AX = AX&0xFF00 | 0x1A
				BX = 0x0008
				return 0
			}
		}
	case 0x21:
		if ah == 0x25 && uint8(AX) == 0 { // set int 0's vector: divides go to asm_divfault
			return 0
		}
		if dosFile(ah) {
			return 0
		}
	case 0x67:
		switch ah {
		case 0x44: // map logical page BX into physical page AL
			AX = AX&0x00FF | uint16(emsMap(int(uint8(AX)), int(BX)))<<8
			return 0
		case 0x50: // map CX pages from the (logical, physical) pairs at DS:SI
			st := uint8(0)
			for i := 0; i < int(CX); i++ {
				lg := rw(pDS, SI+uint16(4*i))
				ph := rw(pDS, SI+uint16(4*i+2))
				if s := emsMap(int(ph), int(lg)); s != 0 {
					st = s
				}
			}
			AX = AX&0x00FF | uint16(st)<<8
			return 0
		}
	}
	port_halt(fmt.Sprintf("int %02Xh function %02Xh is not emulated", n, ah))
	return 0
}

// The VGA DAC as the translated code programs it (port 3C8h the index, 3C9h
// three writes per colour).
var DAC [768]uint8
var dacIndex int
var dacDirty bool

func asm_in8(port uint16) uint8 {
	if port == 0x3DA {
		return 0x09 // in retrace
	}
	return 0
}

func asm_out8(port uint16, v uint8) {
	switch port {
	case 0x3C8:
		dacIndex = int(v) * 3
	case 0x3C9:
		DAC[dacIndex%768] = v
		dacIndex++
		dacDirty = true
	}
}

func asm_out16(port uint16, v uint16) {
	asm_out8(port, uint8(v))
	asm_out8(port+1, uint8(v>>8))
}
