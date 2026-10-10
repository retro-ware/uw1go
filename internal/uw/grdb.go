package uw

import "github.com/memmaker/uw1go/internal/r3d"

// Port of GRDB.C: the label table of the 3D view's render database, and SETPNT.ASM. The
// database is the word bytecode VIEW3D.C, GRIDDB.C and DRAWOBJ.C write into the buffer at
// cDbase (seg051:2938, in the renderer's memory) and cRender runs. dbptr is the byte
// offset in seg051 of the next word.

const (
	cDbase     = 0x2938 // the database buffer, in seg051
	cEntryStrt = 0x73F2 // where the bytecode starts
	cDbbase    = 0x161C // the word holding the entry point's offset
)

var (
	lstrel    int
	prelblnum int
	lstpos    int
	freelblp  int
	refs      [32]int
	dbptr     int
	lblrefs   [32][16]int
	loc       [160]int
	freelbls  [32]int
	ptnuminq  int
)

func dbAddr(off int) int { return r3d.Seg051 + off }

// dbw writes word v at dbptr and moves on (*dbptr++ = v).
func dbw(v int) {
	r3d.W16(dbAddr(dbptr), uint16(v))
	dbptr += 2
}

// dbWord is the word at word index i of cDbase.
func dbWord(i int) int16 { return int16(r3d.R16(dbAddr(cDbase + 2*i))) }

func dbSetWord(i int, v int) { r3d.W16(dbAddr(cDbase+2*i), uint16(v)) }

// dbIndex is dbptr - cDbase in words.
func dbIndex() int { return (dbptr - cDbase) >> 1 }

func grdbBlank() {
	dbptr = cDbase
	i := 0
	for ; i < 32; i++ {
		loc[i] = 0xFFFF
		freelbls[i] = i
		lblrefs[i][0] = 0xFFFF
		refs[i] = 0
	}
	for ; i < 96; i++ {
		loc[i] = 0xFFFF
	}
	for ; i < 160; i++ {
		if v := dbWord(i - 172); v != -1 {
			loc[i] = int(uint16(v >> 1))
		} else {
			loc[i] = 0xFFFF
		}
	}
	freelblp = 31
	prelblnum = 32
}

// Clk is the offset of word n of the table 0x18 bytes before the database.
func Clk(n int) int { return cDbase - 0x18 + 2*n }

func grEntry()  { r3d.W16(dbAddr(cDbbase), uint16(dbptr)) }
func grTostrt() { dbptr = cEntryStrt }

// Ref emits the jump word for label lab.
func Ref(lab int, rel int) {
	switch {
	case lab == 0xA0:
		lstpos = dbIndex()
		lstrel = rel
		dbw(0)
	case loc[lab] == 0xFFFF:
		if refs[lab] == 16 || lab >= 32 {
			panic(fatalError{"", -20})
		}
		lblrefs[lab][refs[lab]] = dbIndex()
		refs[lab]++
		dbw(0)
	default:
		dbw((loc[lab] - dbIndex() - 1) * 2)
	}
}

// grPutlab places label lab at dbptr and patches the references to it.
func grPutlab(lab int) {
	if lab == 0xA0 {
		dbSetWord(lstpos, (dbIndex()-lstpos-lstrel)*2)
		return
	}
	loc[lab] = dbIndex()
	if lab < 32 {
		for i := 0; i < refs[lab]; i++ {
			dbSetWord(lblrefs[lab][i], (loc[lab]-lblrefs[lab][i]-1)*2)
		}
	}
}

// SetPnt appends a do_minires point record (x, y a grid cell, z a tile height) and
// returns the point's number.
func SetPnt(x, y, z int) int {
	dbw(0xB6)
	dbw(int(uint8(x-0x10)<<3) | int(uint8(y)<<3)<<8)
	dbw(int(uint8(z)<<1) | int(uint8(ptnuminq))<<8)
	n := ptnuminq
	ptnuminq++
	return n
}
