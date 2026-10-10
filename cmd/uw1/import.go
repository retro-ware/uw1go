package main

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// importGame (-import, for goplay) finds an Ultima Underworld install anywhere under src
// (the folder holding UW.EXE and DATA\LEV.ARK), copies what the engine reads into dst
// (UW.EXE and the DATA, CRIT, CUTS and SOUND folders, plus any saved games in SAVE1 to
// SAVE4), and writes dst/COVER.PNG from the title screen for the floppy label. Without
// such a folder it looks in the CD images under src (iso.go), with the CD's write folder
// UNDEROM1 laid over what comes from the CD.
func importGame(src, dst string) error {
	var dir, underom string
	var images []string
	filepath.WalkDir(src, func(p string, d fs.DirEntry, _ error) error {
		if d == nil {
			return nil
		}
		if d.IsDir() {
			if underom == "" && strings.EqualFold(d.Name(), "UNDEROM1") {
				underom = p
			}
			return nil
		}
		if strings.EqualFold(d.Name(), "UW.EXE") {
			if data := find(filepath.Dir(p), "DATA"); data != "" && find(filepath.Join(filepath.Dir(p), data), "LEV.ARK") != "" {
				dir = filepath.Dir(p)
				return fs.SkipAll
			}
		}
		if info, err := d.Info(); err == nil && info.Size() > 17*isoSector {
			images = append(images, p)
		}
		return nil
	})
	if dir == "" {
		for _, img := range images {
			ok, err := importISO(img, dst)
			if err != nil {
				return err
			}
			if ok {
				if underom != "" {
					if err := copyTree(underom, dst); err != nil {
						return err
					}
					os.Remove(filepath.Join(dst, "MARKER.FIL"))
				}
				if err := soundOn(filepath.Join(dst, "DATA")); err != nil {
					return err
				}
				return writeCover(dst)
			}
		}
		return errors.New("no Ultima Underworld found")
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(dir, find(dir, "UW.EXE")), filepath.Join(dst, "UW.EXE")); err != nil {
		return err
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if name := strings.ToUpper(e.Name()); e.IsDir() && keepDir(name) {
			if err := copyTree(filepath.Join(dir, e.Name()), filepath.Join(dst, name)); err != nil {
				return err
			}
		}
	}
	if err := soundOn(filepath.Join(dst, "DATA")); err != nil {
		return err
	}
	return writeCover(dst)
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

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

// copyTree copies a folder's files and subfolders, names upper-cased like the DOS
// originals.
func copyTree(dir, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		from, to := filepath.Join(dir, e.Name()), filepath.Join(dst, strings.ToUpper(e.Name()))
		var err error
		if e.IsDir() {
			err = copyTree(from, to)
		} else {
			err = copyFile(from, to)
		}
		if err != nil {
			return err
		}
	}
	return nil
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
