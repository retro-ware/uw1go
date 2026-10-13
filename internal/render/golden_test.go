package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModelsGolden draws every model from the user's UW.EXE (gamedata/UW) from several
// sides, distances and model variable settings, with made-up textures, and compares each
// frame with the hashes in RENDER_GOLDEN (a directory; RENDER_GOLDEN_UPDATE=1 writes
// them). The frames show game data, so they stay out of the repository.
func TestModelsGolden(t *testing.T) {
	dir := os.Getenv("RENDER_GOLDEN")
	if dir == "" {
		t.Skip("RENDER_GOLDEN not set")
	}
	r := goldenRenderer(t)
	got := map[string]string{}
	frames := map[string][]byte{}
	for n := 0; n < 64; n++ {
		for _, vars := range goldenVars {
			for side := 0; side < 8; side++ {
				for _, dist := range []int{350, 900} {
					for _, cam := range goldenCams {
						r.SetCamera(cam.c)
						r.SetLightTabs(goldenLight)
						r.Begin(false, false, true)
						for v, x := range vars.set {
							r.SetVar(v, x)
						}
						for s := 0; s < 6; s++ {
							r.SetTexture(s, goldenTex(s))
							r.SetTextureHeight(s, []int{255, 255, 1023, 1023, 4095, 4095}[s])
						}
						if vars.door {
							goldenDoor(r, 2)
							r.SetVar(7, 700)
							r.SetContext(2)
						}
						r.SetObjectTexture(goldenObjTex)
						r.Model(n, 0, 0, dist, int16(side*0x2000))
						k := fmt.Sprintf("m%02d_%s_s%d_d%d_%s", n, vars.name, side, dist, cam.name)
						h := sha256.Sum256(r.FB)
						got[k] = hex.EncodeToString(h[:8])
						frames[k] = append([]byte(nil), r.FB...)
					}
				}
			}
		}
	}
	file := filepath.Join(dir, "hashes.txt")
	if os.Getenv("RENDER_GOLDEN_UPDATE") != "" {
		var keys []string
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			fmt.Fprintf(&b, "%s %s\n", k, got[k])
		}
		if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		writeFrame(t, filepath.Join(dir, "sheet.png"), keys, frames, r.W, r.H)
		return
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, want, _ := strings.Cut(line, " ")
		if got[k] != want {
			bad = append(bad, k)
		}
	}
	if len(bad) > 0 {
		writeFrame(t, filepath.Join(dir, "bad.png"), bad, frames, r.W, r.H)
		t.Fatalf("%d of %d frames differ, first %v (drawn now in bad.png)", len(bad), len(got), bad[:min(8, len(bad))])
	}
}

var goldenCams = []struct {
	name string
	c    Camera
}{
	{"level", Camera{Z: 40}},
	{"down", Camera{Z: 40, Pitch: -0x0C00}},
	{"above", Camera{Z: 300, Pitch: -0x1800}},
}

var goldenVars = []struct {
	name string
	set  map[int]int
	door bool
}{
	{"base", map[int]int{0: 0x30, 1: 0x50, 3: 0, 5: 0, 8: 0, 10: 2, 11: 0x60}, false},
	{"open", map[int]int{0: 0x40, 1: 0x20, 3: 2, 5: 0x3000, 8: 1, 10: 3, 11: 0x70}, false},
	{"far", map[int]int{0: 0x70, 1: 0x10, 3: 5, 5: 0x7000, 8: 0, 10: 6, 11: 0x20}, false},
	{"door", map[int]int{0: 0x30, 3: 1, 5: 0x2000}, true},
}

// goldenLight is shading maps where each row darkens differently.
var goldenLight = func() []byte {
	b := make([]byte, 16*256)
	for row := 0; row < 16; row++ {
		for c := 0; c < 256; c++ {
			b[row*256+c] = byte(c*(16-row)/16 + row)
		}
	}
	return b
}()

func goldenTex(s int) []byte {
	b := make([]byte, 64*64)
	for i := range b {
		b[i] = byte(16 + s*24 + (i>>6^i)&15)
	}
	return b
}

var goldenObjTex = func() []byte {
	b := []byte{2, 16, 16, 0, 1}
	for i := 0; i < 256; i++ {
		b = append(b, byte(64+i%32))
	}
	return b
}()

func goldenRenderer(t *testing.T) *Renderer {
	exe, err := os.ReadFile("../../gamedata/UW/UW.EXE")
	if err != nil {
		t.Skip(err)
	}
	r, err := loadForTest(exe)
	if err != nil {
		t.Fatal(err)
	}
	r.Resize(172, 113)
	return r
}

// writeFrame puts the frames side by side, 8 to a row, in a grey ramp palette.
func writeFrame(t *testing.T, path string, keys []string, frames map[string][]byte, w, h int) {
	if len(keys) > 512 {
		keys = keys[:512]
	}
	cols := 8
	img := image.NewRGBA(image.Rect(0, 0, cols*w, (len(keys)+cols-1)/cols*h))
	for i, k := range keys {
		ox, oy := i%cols*w, i/cols*h
		for p, c := range frames[k] {
			v := c
			img.Set(ox+p%w, oy+p/w, color.RGBA{v, v * 3, v * 7, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, img)
}
