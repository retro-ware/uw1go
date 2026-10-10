package r3d

// Expanded memory as the renderer and LOADGR.C use it: a handle of logical 16 KB
// pages, four of which at a time are mapped into the page frame at EmsBuff. The
// frame is real memory here; mapping a page writes the frame's old page back to
// its store and copies the new one in.

const (
	emsPageSize = 0x4000
	EMSPages    = 0x80
	EMSFrameSeg = 0xE000
)

var (
	emsStore  [EMSPages][emsPageSize]byte
	emsMapped = [4]int{-1, -1, -1, -1}
)

func emsFrame(phys int) []byte {
	a := EMSFrameSeg*16 + phys*emsPageSize
	return mem[a : a+emsPageSize]
}

// emsMap maps logical page lg into physical page phys (lg 0xFFFF unmaps); it
// returns the EMS status (0 for success).
func emsMap(phys, lg int) uint8 {
	if phys < 0 || phys > 3 {
		return 0x8B
	}
	if lg != 0xFFFF && (lg < 0 || lg >= EMSPages) {
		return 0x8A
	}
	if lg == 0xFFFF {
		lg = -1
	}
	if emsMapped[phys] == lg {
		return 0
	}
	f := emsFrame(phys)
	if old := emsMapped[phys]; old >= 0 {
		copy(emsStore[old][:], f)
	}
	emsMapped[phys] = lg
	if lg >= 0 {
		// the same page may be mapped twice: take the other mapping's bytes
		for p, m := range emsMapped {
			if p != phys && m == lg {
				copy(emsStore[lg][:], emsFrame(p))
			}
		}
		copy(f, emsStore[lg][:])
	}
	return 0
}

// EMSWrite stores b at offset off of logical page lg, wherever it is mapped.
func EMSWrite(lg, off int, b []byte) {
	for len(b) > 0 && lg < EMSPages {
		n := copy(emsStore[lg][off:], b)
		for p, m := range emsMapped {
			if m == lg {
				copy(emsFrame(p)[off:], b[:n])
			}
		}
		b = b[n:]
		lg++
		off = 0
	}
}

// EMSRead is the n bytes at offset off of logical page lg.
func EMSRead(lg, off, n int) []byte {
	for p, m := range emsMapped {
		if m == lg {
			copy(emsStore[lg][:], emsFrame(p))
		}
	}
	return append([]byte(nil), emsStore[lg][off:off+n]...)
}

// EMSMap maps logical page lg into physical page phys (EMS.C's seg012_10F).
func EMSMap(phys, lg int) { emsMap(phys, lg) }

// initEMS points the renderer at the page frame (TMPALLOC.C's mem_setup).
func initEMS() {
	W16(Seg051+0xC4D5, EMSFrameSeg)                   // EmsBuff
	W16(Seg051+0xC4D7, 1)                             // the handle
	W16(Seg051+0xC36F, 0x16)                          // crit_fpage
	W16(Seg051+0xC371, uint16(len(emsStore)-0x16)>>1) // crit_nlpages, the pairs from crit_fpage on
	ForgetPages()
}

// ForgetPages marks the frame's physical pages unknown, so the next user maps its
// own (TMPALLOC.C's seg042_19B: crit_inpage, obj_inpage1, tmap_inpage).
func ForgetPages() {
	mem[Seg051+0xC373] = 0xFF
	mem[Seg051+0xC4D4] = 0xFF
	mem[Seg051+0xC374] = 0xFF
}
