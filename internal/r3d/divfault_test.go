package r3d

import "testing"

// asm_divfault runs the renderer's own translated divide fault handlers; these two resume
// after the divide with their own AX and DX.
func TestDivfaultHandlers(t *testing.T) {
	setHandler := func(h uint16) { ww(Seg063, 0x4D5, h) }

	setHandler(0x692C) // TMAPOPS: 7FFFh
	BP, AX, DX = 0x1234, 5, 9
	if c := asm_divfault(0x6E7, 0x6168, 2); c != 0 || AX != 0x7FFF || DX != 0 || BP != 0x1234 {
		t.Fatalf("692C: c %X AX %04X DX %04X BP %04X", c, AX, DX, BP)
	}

	setHandler(0x9E5)                               // SMOOTH: halve until it divides
	BP, DX, AX, CX = 0x4321, 0x0010, 0x0000, 0x0020 // 0x100000 / 0x20 overflows; 0x80000 / 0x20
	if c := asm_divfault(0x6E7, 0x400, 2); c != 0 || AX != 0x4000 || DX != 0 || BP != 0x4321 {
		t.Fatalf("9E5: c %X AX %04X DX %04X BP %04X", c, AX, DX, BP)
	}
	// Halved once it still overflows, so the handler faults in itself and runs again, as on the CPU.
	DX, AX, CX = 0x0010, 0, 0x0010 // 0x80000 / 0x10 overflows; 0x40000 / 0x10
	if c := asm_divfault(0x6E7, 0x400, 2); c != 0 || AX != 0x4000 || DX != 0 {
		t.Fatalf("9E5 twice: c %X AX %04X DX %04X", c, AX, DX)
	}
}
