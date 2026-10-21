package main

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/retro-ware/gameimport"
)

// importGame (-import, for goplay) finds an Ultima Underworld install in src, in any
// packaging gameimport reads (folders, archives, CD images, the GOG installer): the folder
// holding UW.EXE and DATA\LEV.ARK. It copies what the engine reads into dst (UW.EXE and the
// DATA, CRIT, CUTS and SOUND folders, plus any saved games in SAVE1 to SAVE4), names
// upper-cased like the DOS originals, and writes dst/COVER.PNG from the title screen for
// the floppy label.
//
// The GOG installer holds the game as the Ultima Underworld 1 & 2 CD (game.gog, UW1 in its
// folder UW) beside UNDEROM1, the folder the CD's TSR sends the game's writes to (the
// files the game changes: DATA\UW.CFG, CUTS\*.N00); it is laid over what comes from the CD.
func importGame(src, dst string) error {
	files, err := gameimport.Find(src, func(name string) bool {
		for _, part := range strings.Split(strings.ToUpper(name), "/") {
			if part == "UW.EXE" || part == "UNDEROM1" || keepDir(part) {
				return true
			}
		}
		return false
	})
	if err != nil {
		return err
	}
	root, ok := "", false // the game's folder: "" or ".../"
	for name := range files {
		dir, base := path.Split(name)
		if strings.EqualFold(base, "UW.EXE") && hasFold(files, dir+"DATA/LEV.ARK") && (!ok || len(dir) < len(root)) {
			root, ok = dir, true
		}
	}
	if !ok {
		return errors.New("no Ultima Underworld found")
	}
	var overlay []string // UNDEROM1's files, after the game's
	for name, b := range files {
		up := strings.ToUpper(name)
		if strings.Contains(up, "UNDEROM1/") {
			overlay = append(overlay, name)
			continue
		}
		if !strings.HasPrefix(up, strings.ToUpper(root)) {
			continue
		}
		rel := up[len(root):]
		if first, _, _ := strings.Cut(rel, "/"); rel == "UW.EXE" || strings.Contains(rel, "/") && keepDir(first) {
			if err := write(filepath.Join(dst, filepath.FromSlash(rel)), b); err != nil {
				return err
			}
		}
	}
	for _, name := range overlay {
		up := strings.ToUpper(name)
		rel := up[strings.Index(up, "UNDEROM1/")+len("UNDEROM1/"):]
		if rel == "MARKER.FIL" {
			continue
		}
		if err := write(filepath.Join(dst, filepath.FromSlash(rel)), files[name]); err != nil {
			return err
		}
	}
	if err := soundOn(filepath.Join(dst, "DATA")); err != nil {
		return err
	}
	return writeCover(dst)
}

// hasFold reports whether files has name, matched without regard to case.
func hasFold(files map[string][]byte, name string) bool {
	for n := range files {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

func write(p string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

// keepDir: the game's folders that are copied (upper-case name).
func keepDir(name string) bool {
	return name == "DATA" || name == "CRIT" || name == "CUTS" || name == "SOUND" ||
		len(name) == 5 && strings.HasPrefix(name, "SAVE") && name[4] >= '1' && name[4] <= '4'
}

// soundOn sets the copied UW.CFG to a Sound Blaster for music and speech when it says no
// card (the GOG and CD copies ship it unset, and in the browser there is no install
// program or environment to choose one). The rest of the file is kept.
func soundOn(data string) error {
	name := find(data, "UW.CFG")
	if name == "" {
		name = "UW.CFG"
	}
	b, _ := os.ReadFile(filepath.Join(data, name))
	lines := strings.Split(string(b), "\r\n")
	for len(lines) < 2 {
		lines = append(lines, "")
	}
	if f := strings.Fields(lines[0]); len(f) > 0 && f[0] != "0" {
		return nil
	}
	lines[0] = "3 7 220 1 sound"
	if f := strings.Fields(lines[1]); len(f) == 0 || f[0] == "0" {
		lines[1] = "1 7 220 1 speech"
	}
	out := strings.Join(lines, "\r\n")
	if !strings.HasSuffix(out, "\r\n") {
		out += "\r\n"
	}
	return os.WriteFile(filepath.Join(data, name), []byte(out), 0o644)
}

// find is the spelling of name in dir, matched without regard to case, or "".
func find(dir, name string) string {
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.EqualFold(e.Name(), name) {
			return e.Name()
		}
	}
	return ""
}

// writeCover renders the main menu's picture, DATA\OPSCR.BYT (320x200, one byte a pixel),
// in the opening palette (2) of DATA\PALS.DAT (6 bits a component).
func writeCover(dst string) error {
	data := filepath.Join(dst, "DATA")
	pic, err := os.ReadFile(filepath.Join(data, "OPSCR.BYT"))
	if err != nil {
		return err
	}
	pals, err := os.ReadFile(filepath.Join(data, "PALS.DAT"))
	if err != nil {
		return err
	}
	if len(pic) < 320*200 || len(pals) < 3*0x300 {
		return errors.New("the title picture is short")
	}
	pal := make(color.Palette, 256)
	for i := range pal {
		c := func(v byte) uint8 { v &= 63; return v<<2 | v>>4 }
		pal[i] = color.RGBA{c(pals[0x600+i*3]), c(pals[0x600+i*3+1]), c(pals[0x600+i*3+2]), 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, 320, 200), pal)
	copy(img.Pix, pic[:320*200])
	f, err := os.Create(filepath.Join(dst, "COVER.PNG"))
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
