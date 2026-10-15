// Package uw is the game: a port of UW1's C sources (underworld-exhumed's uw1/src) to Go,
// file by file, with the original's own data files read at run time.
//
// The files of this package carry the names of the C files they port. The game's globals
// are package variables, its functions are package functions with the original names in
// Go style, and its 16-bit arithmetic is kept where wrap-around or truncation matters.
package uw

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GameDir is the game's directory (the one holding DATA, CRIT, CUTS, SOUND and UW.EXE).
// SaveDir holds the save directories SAVE0..SAVE4, which DOS kept in the game directory.
var (
	GameDir string
	SaveDir string
)

// dosPath turns a DOS path as the game spells it ("DATA\\strings.pak", "SAVE0\\lev.ark",
// "crit\\cr00page.n00") into a host path: names are matched without regard to case, the
// save directories live under SaveDir, and DOS's 8.3 truncation of the extension is kept.
func dosPath(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	parts := strings.Split(name, "/")
	base := GameDir
	if len(parts) > 0 && strings.HasPrefix(strings.ToUpper(parts[0]), "SAVE") {
		base = SaveDir
		parts[0] += workSuffix // a second copy's own working folder and saves
	}
	for i, p := range parts {
		if i == len(parts)-1 {
			// DOS keeps only three characters of an extension
			if dot := strings.LastIndexByte(p, '.'); dot >= 0 && len(p)-dot-1 > 3 {
				p = p[:dot+4]
			}
		}
		parts[i] = p
	}
	return resolve(base, parts)
}

// resolve finds each part of a path in its directory without regard to case; parts that
// do not exist keep the spelling given, upper-cased (a file to be created).
func resolve(base string, parts []string) string {
	cur := base
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		found := ""
		if ents, err := os.ReadDir(cur); err == nil {
			for _, e := range ents {
				if strings.EqualFold(e.Name(), p) {
					found = e.Name()
					break
				}
			}
		}
		if found == "" {
			found = strings.ToUpper(p)
		}
		cur = filepath.Join(cur, found)
	}
	return cur
}

// DOS open flags, as io.h has them.
const (
	O_RDONLY = 0
	O_WRONLY = 1
	O_RDWR   = 2
	O_CREAT  = 0x100
	O_TRUNC  = 0x200
)

// File handles: a DOS handle is an index into this table, so the game's code can keep
// its int handles. 0..4 are DOS's standard handles and never given out.
var files = map[int]*os.File{}
var nextFD = 5

func dosOpen(name string, flags int) int {
	var of int
	switch flags & 3 {
	case O_RDONLY:
		of = os.O_RDONLY
	case O_WRONLY:
		of = os.O_WRONLY
	default:
		of = os.O_RDWR
	}
	if flags&O_CREAT != 0 {
		of |= os.O_CREATE
	}
	if flags&O_TRUNC != 0 {
		of |= os.O_TRUNC
	}
	p := dosPath(name)
	if flags&O_CREAT != 0 {
		os.MkdirAll(filepath.Dir(p), 0o755)
	}
	f, err := os.OpenFile(p, of, 0o644)
	if err != nil {
		return -1
	}
	fd := nextFD
	nextFD++
	files[fd] = f
	return fd
}

func dosClose(fd int) int {
	f, ok := files[fd]
	if !ok {
		return -1
	}
	delete(files, fd)
	if f.Close() != nil {
		return -1
	}
	return 0
}

// dosRead reads up to len(buf) bytes; the count read, or -1.
func dosRead(fd int, buf []byte) int {
	f, ok := files[fd]
	if !ok {
		return -1
	}
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return -1
	}
	return n
}

func dosWrite(fd int, buf []byte) int {
	f, ok := files[fd]
	if !ok {
		return -1
	}
	n, err := f.Write(buf)
	if err != nil {
		return -1
	}
	return n
}

// dosSeek is lseek: whence 0 from the start, 1 from here, 2 from the end.
func dosSeek(fd int, off int64, whence int) int64 {
	f, ok := files[fd]
	if !ok {
		return -1
	}
	p, err := f.Seek(off, whence)
	if err != nil {
		return -1
	}
	return p
}

func dosUnlink(name string) int {
	if os.Remove(dosPath(name)) != nil {
		return -1
	}
	return 0
}

func dosRename(from, to string) int {
	if os.Rename(dosPath(from), dosPath(to)) != nil {
		return -1
	}
	return 0
}

// readFile reads a whole data file; nil if it cannot be read.
func readFile(name string) []byte {
	b, err := os.ReadFile(dosPath(name))
	if err != nil {
		return nil
	}
	return b
}

func fileExists(name string) bool {
	_, err := os.Stat(dosPath(name))
	return err == nil
}

func dirExist(name string) bool {
	st, err := os.Stat(dosPath(name))
	return err == nil && st.IsDir()
}

// le16 and friends read little-endian values from a byte slice.
func le16(b []byte, o int) uint16 { return uint16(b[o]) | uint16(b[o+1])<<8 }
func le32(b []byte, o int) uint32 {
	return uint32(b[o]) | uint32(b[o+1])<<8 | uint32(b[o+2])<<16 | uint32(b[o+3])<<24
}
func put16(b []byte, o int, v uint16) { b[o] = byte(v); b[o+1] = byte(v >> 8) }
func put32(b []byte, o int, v uint32) {
	b[o] = byte(v)
	b[o+1] = byte(v >> 8)
	b[o+2] = byte(v >> 16)
	b[o+3] = byte(v >> 24)
}

// cstr returns the C string at the start of b.
func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// setCstr copies s into b as a C string, cut to fit.
func setCstr(b []byte, s string) {
	n := copy(b[:len(b)-1], s)
	b[n] = 0
}
