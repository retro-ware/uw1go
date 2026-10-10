package r3d

// The DOS file functions the renderer calls itself: PGCACHE's critter page loader opens
// CRIT\CRxxPAGE.Nyy (the name at seg051:B0FA), reads it into the EMS frame and closes it.
// ReadFile, set by the game, reads a whole data file by its DOS name (nil if missing).

var ReadFile func(name string) []byte

var dosFiles = map[uint16]*dosHandle{}
var dosNext uint16 = 5

type dosHandle struct {
	data []byte
	pos  int
}

// dosFile does int 21h function ah if it is one of open, read and close.
func dosFile(ah uint8) bool {
	switch ah {
	case 0x3D: // open DS:DX
		var name []byte
		for a := pDS + int(DX); mem[a] != 0; a++ {
			name = append(name, mem[a])
		}
		var b []byte
		if ReadFile != nil {
			b = ReadFile(string(name))
		}
		if b == nil {
			AX, CF = 2, 1 // file not found
			return true
		}
		h := dosNext
		dosNext++
		dosFiles[h] = &dosHandle{data: b}
		AX, CF = h, 0
	case 0x3F: // read CX bytes from BX to DS:DX
		f := dosFiles[BX]
		if f == nil {
			AX, CF = 6, 1 // invalid handle
			return true
		}
		n := min(int(CX), len(f.data)-f.pos)
		copy(mem[pDS+int(DX):], f.data[f.pos:f.pos+n])
		f.pos += n
		AX, CF = uint16(n), 0
	case 0x3E: // close BX
		if dosFiles[BX] == nil {
			AX, CF = 6, 1
			return true
		}
		delete(dosFiles, BX)
		CF = 0
	default:
		return false
	}
	return true
}
