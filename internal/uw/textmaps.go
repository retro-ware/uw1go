package uw

import "io"

// Port of TEXTMAPS.C: the level's texture map (which wall, floor and door textures it
// uses), their terrain types, and loading the textures. The DOS build kept the first 12
// walls and the floors in conventional memory and the rest of the walls in EMS; here every
// texture is a slice: wallTex64/wallTex16 for TxmID's walls (64 and 16 pixels square),
// floorTex32/floorTex16 for floorIDs.

var (
	floorIDs [TXM_FLOORS]int16
	TxmTerr  [TXM_FLOORS]uint16
	floorNum int
	TxmID    [TXM_WALLS]int16
	w64Types [TXM_WALLS]uint16
	w64Num   int
	ActDoors [TXM_DOORS]byte

	wallTex64  [TXM_WALLS][]byte
	wallTex16  [TXM_WALLS][]byte
	floorTex32 [TXM_FLOORS][]byte
	floorTex16 [TXM_FLOORS][]byte
)

// initTxtlib sets up the default texture map and loads it.
func initTxtlib() bool {
	for i := 0; i < TXM_WALLS; i++ {
		TxmID[i] = int16(i % 12)
		w64Types[i] = 0
	}
	for i := 0; i < TXM_FLOORS; i++ {
		floorIDs[i] = int16(i)
		TxmTerr[i] = 0
	}
	for i := 0; i < TXM_DOORS; i++ {
		ActDoors[i] = byte(i)
	}
	w64Num = TXM_WALLS
	floorNum = TXM_FLOORS
	loadTxtmaps()
	return true
}

// TxmLoad reads level lev's texture block into TxmID, floorIDs and ActDoors, reads the
// terrain types and loads the textures.
func TxmLoad(arc *Arc, lev int) bool {
	ok := true
	b := getArc(arc, lev+0x11)
	if len(b) != TXM_BLOCK_SIZE {
		dprintf("bad tmap ids size\n")
		ok = false
	}
	buf := make([]byte, 0x80)
	copy(buf, b)
	w := func(i int) int16 { return int16(le16(buf, i*2)) }
	for i := 0; i < TXM_WALLS; i++ {
		TxmID[i] = w(i)
		w64Types[i] = 0
	}
	for i := 0; i < TXM_FLOORS; i++ {
		TxmTerr[i] = 0
		floorIDs[i] = w(i + TXM_WALLS)
	}
	loadTerrains()
	for i := 0; i < 3; i++ {
		v := uint16(w(i + TXM_WALLS + TXM_FLOORS))
		ActDoors[i*2] = byte(v)
		ActDoors[i*2+1] = byte(v >> 8)
	}
	loadTxtmaps()
	return ok
}

// TxmSave writes the texture map back as level lev's block.
func TxmSave(arc *Arc, lev int) bool {
	buf := make([]byte, TXM_BLOCK_SIZE)
	for i := 0; i < TXM_WALLS; i++ {
		put16(buf, i*2, uint16(TxmID[i]))
	}
	for i := 0; i < TXM_FLOORS; i++ {
		put16(buf, (i+TXM_WALLS)*2, uint16(floorIDs[i]))
	}
	for i := 0; i < 3; i++ {
		put16(buf, (i+TXM_WALLS+TXM_FLOORS)*2, uint16(ActDoors[i*2+1])<<8|uint16(ActDoors[i*2]))
	}
	return putArc(arc, lev+0x11, buf)
}

// loadTxtmaps loads the level's textures. At most 60 fit in the original's memory, so
// floorNum may be cut, as there.
func loadTxtmaps() {
	n := 12
	loadTrMem("DATA\\w64.tr", TxmID[:], &n, wallTex64[:])
	if floorNum+n > 0x3C {
		floorNum = 0x3C - w64Num
	}
	loadTrMem("DATA\\f32.tr", floorIDs[:], &floorNum, floorTex32[:])
	loadTrMem("DATA\\w16.tr", TxmID[:], &n, wallTex16[:])
	loadTrMem("DATA\\f16.tr", floorIDs[:], &floorNum, floorTex16[:])
	loadTrEms("w64")
	loadTrEms("w16")
	loadDoors()
}

// loadTrMem reads textures ids[0..*count-1] of the .tr file name into dst; it stops at a
// negative id and leaves in *count the number read.
func loadTrMem(name string, ids []int16, count *int, dst [][]byte) {
	fd := dosOpen(name, O_RDONLY)
	if fd < 0 {
		pfatalCode(ERR_READ | 0xF)
	}
	defer dosClose(fd)
	var hdr [4]byte
	dosRead(fd, hdr[:])
	if hdr[0] != 2 {
		pfatalCode(ERR_READ | 0x10)
	}
	size := int(hdr[1]) * int(hdr[1])
	cnt := int(le16(hdr[:], 2))
	offs := make([]byte, cnt*4)
	if dosRead(fd, offs) < 0 {
		pfatalCode(ERR_READ | 0x11)
	}
	i := 0
	for ; *count > i && ids[i] >= 0; i++ {
		id := int(ids[i])
		if id >= cnt {
			break
		}
		dosSeek(fd, int64(le32(offs, id*4)), io.SeekStart)
		buf := make([]byte, size)
		if dosRead(fd, buf) != size {
			pfatalCode(ERR_READ | 0x12)
		}
		dst[i] = buf
	}
	*count = i
}

// loadTerrains reads each wall's and floor's terrain word from DATA\TERRAIN.DAT.
func loadTerrains() {
	b := readFile("DATA\\terrain.dat")
	if b == nil {
		return
	}
	for i := 0; i < TXM_WALLS; i++ {
		if o := int(TxmID[i]) * 2; o >= 0 && o+2 <= len(b) {
			w64Types[i] = le16(b, o)
		}
	}
	for i := 0; i < TXM_FLOORS; i++ {
		if o := int(floorIDs[i])*2 + 0x200; o >= 0 && o+2 <= len(b) {
			TxmTerr[i] = le16(b, o)
		}
	}
}
