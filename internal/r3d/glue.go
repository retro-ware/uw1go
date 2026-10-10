package r3d

// Where the translated code reaches the game's Go: seg019's entries the
// renderer far-calls (the original port's x86/glue.c).

// TickRead returns the game clock's low word (TICKREAD.ASM's _C00).
var TickRead = func() uint16 { return 0 }

// MouseQ runs MousQUp(1) mid-frame (C3DENTRY's _102B, do_mouseq's).
var MouseQ = func() {}

func init() {
	glues = []glue{
		{0x1F3A, 0x0C00, func() uint32 { AX = TickRead(); return asm_glue_retf() }, "TICKREAD _C00"},
		{0x1F3A, 0x0C10, func() uint32 { return asm_glue_retf() }, "SYSLIBP _C10"},
		{0x1F3A, 0x102B, g102b, "C3DENTRY _102B (MousQUp)"},
	}
}

func g102b() uint32 {
	si, ds, es := SI, asm_ds, asm_es
	asm_run_near(0x1F3A, 0x0FDD)
	MouseQ()
	asm_run_near(0x1F3A, 0x1004)
	SI = si
	SET_DS(ds)
	SET_ES(es)
	return asm_glue_retf()
}

// scalebm_run_generated interprets the straight-line scaler SCALEBM.ASM's
// generators write at seg003:1F3A (the original port's gfx/scalebm_code.c).
func scalebm_run_generated(ip uint16) {
	code := CODE003
	at := ip
	for {
		op := mem[code+int(at)]
		op2 := mem[code+int(at)+1]
		switch op {
		case 0xAC:
			AX = AX&0xFF00 | uint16(rb(pDS, SI))
			SI += uint16(STEP(1))
			at++
		case 0xAA:
			wb(pES, DI, uint8(AX))
			DI += uint16(STEP(1))
			at++
		case 0xA4:
			wb(pES, DI, rb(pDS, SI))
			SI += uint16(STEP(1))
			DI += uint16(STEP(1))
			at++
		case 0x46:
			SI = inc16(SI)
			at++
		case 0x4E:
			SI = dec16(SI)
			at++
		case 0x83:
			if op2 != 0xC6 {
				goto bad
			}
			SI = add16(SI, uint16(int16(int8(mem[code+int(at)+2]))), 0)
			at += 3
		case 0x51:
			push16(CX)
			at++
		case 0x59:
			CX = pop16()
			at++
		case 0x36:
			if op2 != 0xD7 {
				goto bad
			}
			AX = AX&0xFF00 | uint16(rb(pSS, BX+uint16(uint8(AX))))
			at += 2
		case 0x88:
			if op2 == 0xE0 {
				AX = AX&0xFF00 | AX>>8
			} else if op2 == 0xC4 {
				AX = AX&0x00FF | AX<<8
			} else {
				goto bad
			}
			at += 2
		case 0xD2:
			if op2 != 0xE8 {
				goto bad
			}
			AX = AX&0xFF00 | uint16(shr8(uint8(AX), uint32(uint8(CX))))
			at += 2
		case 0x20:
			if op2 != 0xE8 {
				goto bad
			}
			AX = AX&0xFF00 | uint16(logic8(uint8(AX)&uint8(CX>>8)))
			at += 2
		case 0xC3:
			return
		default:
			goto bad
		}
	}
bad:
	asm_halt_at(0x0090, at, "SCALEBM's generated code: an instruction the generators do not write")
}
