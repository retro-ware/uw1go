package uw

import "fmt"

// The game's own data in UW.EXE: tables the C reads from the data segments (the keyboard's
// scan code tables, the 3D models, ...), read from the user's copy at start-up so the port
// holds no game data. A segment:offset of the listing is file offset
// header + segment*16 + offset, the MZ header being 800 paragraphs.

var exeImage []byte

const exeHeader = 800 * 16

// Segments of UW1's EXE used here.
const (
	segSeg063 = 0x5624 // seg019's data: the keyboard tables
	segDGROUP = 0x5AAC
)

func loadExe() error {
	b := readFile("UW.EXE")
	if b == nil {
		return fmt.Errorf("cannot read UW.EXE in %s", GameDir)
	}
	if len(b) < exeHeader || b[0] != 'M' || b[1] != 'Z' {
		return fmt.Errorf("UW.EXE is not an MZ executable")
	}
	if hdr := int(le16(b, 8)) * 16; hdr != exeHeader {
		return fmt.Errorf("UW.EXE: unexpected header size %d (not UW1 1.x?)", hdr)
	}
	exeImage = b
	return nil
}

// exeBytes returns n bytes of the EXE at seg:off.
func exeBytes(seg, off, n int) []byte {
	o := exeHeader + seg*16 + off
	if o < 0 || o+n > len(exeImage) {
		panic(fmt.Sprintf("exeBytes %04X:%04X+%d outside UW.EXE", seg, off, n))
	}
	return exeImage[o : o+n]
}
