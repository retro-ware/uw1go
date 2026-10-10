package main

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The GOG installer of today holds the game as the Ultima Underworld 1 & 2 CD: an ISO 9660
// image, game.gog, with UW1 in its folder UW. DOSBox mounts it as D: and the CD's TSR
// (cdwedge) sends the game's writes to C:\UNDEROM1, which the installer fills with the
// files the game changes (DATA\UW.CFG, the cutscene states CUTS\*.N00).

const isoSector = 2048

type isoEntry struct {
	name      string
	dir       bool
	lba, size uint32
}

// isoRoot is the root directory of the ISO 9660 image f, or false when f is not one.
func isoRoot(f io.ReaderAt) (isoEntry, bool) {
	pvd := make([]byte, isoSector)
	if _, err := f.ReadAt(pvd, 16*isoSector); err != nil || pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return isoEntry{}, false
	}
	return isoEntry{dir: true, lba: binary.LittleEndian.Uint32(pvd[156+2:]), size: binary.LittleEndian.Uint32(pvd[156+10:])}, true
}

// isoList reads directory d: its records, names without the ";1" version.
func isoList(f io.ReaderAt, d isoEntry) ([]isoEntry, error) {
	buf := make([]byte, d.size)
	if _, err := f.ReadAt(buf, int64(d.lba)*isoSector); err != nil && err != io.EOF {
		return nil, err
	}
	var out []isoEntry
	for off := 0; off < len(buf); {
		n := int(buf[off])
		if n == 0 { // no record crosses a sector: the rest of this one is padding
			off = (off/isoSector + 1) * isoSector
			continue
		}
		if n < 34 || off+n > len(buf) {
			return nil, errors.New("a broken ISO directory")
		}
		r := buf[off : off+n]
		off += n
		nl := int(r[32])
		if 33+nl > n {
			return nil, errors.New("a broken ISO directory")
		}
		name := string(r[33 : 33+nl])
		if nl == 1 && (name[0] == 0 || name[0] == 1) { // . and ..
			continue
		}
		if i := strings.IndexByte(name, ';'); i >= 0 {
			name = name[:i]
		}
		out = append(out, isoEntry{strings.TrimSuffix(name, "."), r[25]&2 != 0,
			binary.LittleEndian.Uint32(r[2:]), binary.LittleEndian.Uint32(r[10:])})
	}
	return out, nil
}

func isoFind(ents []isoEntry, name string) (isoEntry, bool) {
	for _, e := range ents {
		if strings.EqualFold(e.name, name) {
			return e, true
		}
	}
	return isoEntry{}, false
}

// isoGameDir finds, depth first, a directory of the image with UW.EXE and DATA\LEV.ARK.
func isoGameDir(f io.ReaderAt, d isoEntry, depth int) ([]isoEntry, bool) {
	ents, err := isoList(f, d)
	if err != nil {
		return nil, false
	}
	if _, ok := isoFind(ents, "UW.EXE"); ok {
		if data, ok := isoFind(ents, "DATA"); ok && data.dir {
			if de, err := isoList(f, data); err == nil {
				if _, ok := isoFind(de, "LEV.ARK"); ok {
					return ents, true
				}
			}
		}
	}
	if depth < 4 {
		for _, e := range ents {
			if e.dir {
				if g, ok := isoGameDir(f, e, depth+1); ok {
					return g, true
				}
			}
		}
	}
	return nil, false
}

func isoCopyFile(f io.ReaderAt, e isoEntry, to string) error {
	b := make([]byte, e.size)
	if _, err := f.ReadAt(b, int64(e.lba)*isoSector); err != nil && err != io.EOF {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

func isoCopyTree(f io.ReaderAt, d isoEntry, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	ents, err := isoList(f, d)
	if err != nil {
		return err
	}
	for _, e := range ents {
		to := filepath.Join(dst, strings.ToUpper(e.name))
		if e.dir {
			err = isoCopyTree(f, e, to)
		} else {
			err = isoCopyFile(f, e, to)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// importISO copies UW1 out of the CD image at path into dst as importGame does from a
// folder; false when the image holds no UW1 (or is no image).
func importISO(path, dst string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, nil
	}
	defer f.Close()
	root, ok := isoRoot(f)
	if !ok {
		return false, nil
	}
	ents, ok := isoGameDir(f, root, 0)
	if !ok {
		return false, nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return true, err
	}
	for _, e := range ents {
		name := strings.ToUpper(e.name)
		if name == "UW.EXE" && !e.dir {
			err = isoCopyFile(f, e, filepath.Join(dst, name))
		} else if e.dir && keepDir(name) {
			err = isoCopyTree(f, e, filepath.Join(dst, name))
		}
		if err != nil {
			return true, err
		}
	}
	return true, nil
}
