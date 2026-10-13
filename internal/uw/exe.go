package uw

import (
	"bytes"
	"fmt"
)

// UW.EXE holds the 3D models (doors, tables, ...), which internal/render decodes into its
// own representation. Nothing else is read from it.

var exeImage []byte

// exeShift is how far this UW.EXE's data lies past the GOG copy's: the 1992 floppy
// release has more code before it, the data itself is the same. It is found from a
// string both carry, the keyboard's character row (Esc 1 2 3 ...).
var exeShift int

const (
	exeHeader     = 800 * 16 // the MZ header, 800 paragraphs
	keyRowAtInGOG = 365648   // where that string lies in the GOG copy
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
	i := bytes.Index(b, []byte("\x00\x1b1234567890-="))
	if i < 0 {
		return fmt.Errorf("UW.EXE: no keyboard table (not UW1?)")
	}
	exeImage, exeShift = b, i-keyRowAtInGOG
	return nil
}
