package uw

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/memmaker/uw1go/internal/gr"
)

// Debugging: with UW1GO_SHOT set to a directory, shot writes the shown page as a PNG there.

var shotDir = os.Getenv("UW1GO_SHOT")

func shot(name string) {
	if shotDir == "" {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, gr.W, gr.H))
	pix := gr.Shown()
	for y := 0; y < gr.H; y++ {
		for x := 0; x < gr.W; x++ {
			c := int(pix[y*gr.W+x]) * 3
			r, g, b := gr.DAC[c]&63, gr.DAC[c+1]&63, gr.DAC[c+2]&63
			img.Set(x, y, color.RGBA{r<<2 | r>>4, g<<2 | g>>4, b<<2 | b>>4, 255})
		}
	}
	f, err := os.Create(filepath.Join(shotDir, name+".png"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer f.Close()
	png.Encode(f, img)
}

var shotCount int

// shotEvery writes every nth call's screen as name-<k>.png.
func shotEvery(name string, n int) {
	if shotDir == "" {
		return
	}
	shotCount++
	if shotCount%n == 0 {
		shot(fmt.Sprintf("%s-%04d", name, shotCount/n))
	}
}
