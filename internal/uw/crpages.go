package uw

import (
	"fmt"

	"github.com/felix/uw1go/internal/r3d"
)

// Port of CRPAGES.C (ovr113): the EMS cache of critter animation art. Critter file xx's
// pictures are CRIT\CRxxPAGE.Nyy (xx in octal), kept two EMS pages to a file page from
// crit_fpage on; the renderer (PGCACHE.ASM) loads a page when a frame needs it and evicts
// the least recently used. The tables are the renderer's, in seg051, indexed by critter
// file * 4 + page.

const (
	crAssoc   = 0xC10F // 0x80 bytes of CRIT\ASSOC.ANM from offset 0x100
	crToPg    = 0xC18F // CmaptoPg: the cache page holding a fragment, 0xFF none
	crToCmap  = 0xC20F // PgtoCmap: the fragment in a cache page, 0xFE free, 0xFF no page
	crFrm     = 0xC28F // CmapFrm[32][3]: frame counts to the end of each of the first pages
	crCache   = 0xC2EF // CmapCache: 0xFF not loaded, else an age (1 the oldest)
	crFpage   = 0xC36F
	crNlpages = 0xC371
)

func crByte(off int) *byte { return &r3d.Bytes(r3d.Seg051+off, 1)[0] }

// preloadCr empties the critter page cache and, with loadPages, reads ASSOC.ANM's table
// and the frame counts from every page file's header. False if ASSOC.ANM is missing.
func preloadCr(loadPages bool) bool {
	r3dInit()
	nl := int(r3d.R16(r3d.Seg051 + crNlpages))
	for i := 0; i < 0x80; i++ {
		v := byte(0xFF)
		if i < nl {
			v = 0xFE
		}
		*crByte(crToCmap + i) = v
		*crByte(crCache + i) = 0xFF
		*crByte(crToPg + i) = 0xFF
	}
	seg042_19B()
	if !loadPages {
		return true
	}
	b := readFile("crit\\assoc.anm")
	if b == nil {
		for i := 0; i < 0x80; i++ {
			*crByte(crAssoc + i) = 0xFF
		}
		return false
	}
	assoc := r3d.Bytes(r3d.Seg051+crAssoc, 0x80)
	clear(assoc)
	if len(b) > 0x100 {
		copy(assoc, b[0x100:])
	}
	for i := 0; i < 0x20; i++ {
		for j := 0; ; {
			frames := 0xA0
			if h := readFile(fmt.Sprintf("crit\\cr%d%dpage.n%d%d", i>>3, i&7, j/10, j%10)); len(h) >= 2 {
				frames = int(h[0]) + int(h[1])
			}
			if j < 3 {
				*crByte(crFrm + i*3 + j) = byte(frames)
			}
			j++
			if frames >= 0xA0 || j >= 4 {
				break
			}
		}
	}
	return true
}

// NightCleanCritPages drops the cached fragments that have aged right down, after sleep.
func NightCleanCritPages() {
	r3dInit()
	for i := 0; i < 0x80; i++ {
		if *crByte(crCache + i) == 1 {
			*crByte(crCache + i) = 0xFF
			page := int(*crByte(crToPg + i))
			*crByte(crToPg + i) = 0xFF
			*crByte(crToCmap + page) = 0xFE
		}
	}
	seg042_19B()
}

// puntIdleCrpage evicts the least recently used fragment and returns its page (0xFF if
// nothing is cached).
func puntIdleCrpage() int {
	lowest, frame := 0xFF, 0xFF
	for i := 0; i < 0x80; i++ {
		if c := int(*crByte(crCache + i)); c < lowest {
			lowest, frame = c, i
		}
	}
	if frame == 0xFF {
		return frame
	}
	*crByte(crCache + frame) = 0xFF
	page := int(*crByte(crToPg + frame))
	*crByte(crToPg + frame) = 0xFF
	*crByte(crToCmap + page) = 0xFE
	return page
}
