// Package host runs the game on Ebiten.
//
// The original program is written in a blocking style: its main loop, menus, fades and
// cutscenes all spin until something happens. Rather than turning that inside out, the game
// runs on its own goroutine in lockstep with Ebiten: each Ebiten tick hands control to the
// game goroutine, which runs until it calls Yield (at a page flip, an input poll or a busy
// wait), and then Ebiten draws the screen the game left behind. Only one of the two ever runs
// at a time, so the game's state needs no locks.
package host

import (
	"image"
	"image/color"
	"os"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	ScreenW = 320
	ScreenH = 200
	// TPS is the host's tick rate; the game clock advances 256 ticks a second.
	TPS = 70
)

// Frame is what the host shows: a 320x200 paletted picture (top row first) and its palette
// of 6-bit VGA values.
type Frame struct {
	Pix     []byte
	Palette *[768]byte
	// Cursor is drawn over the frame at the pointer when CursorOn is set; transparent where 0.
	Cursor     []byte
	CursorW    int
	CursorH    int
	CursorHotX int
	CursorHotY int
	CursorOn   bool
	CursorAtX  int // where the game's cursor is, in screen pixels (top-down)
	CursorAtY  int
}

// ShowFrame returns the frame to present. Set by the game.
var ShowFrame func() *Frame

var (
	tick     = make(chan struct{})
	yielded  = make(chan struct{})
	finished atomic.Bool
	started  bool

	// Clock is the game clock in 1/256 seconds.
	Clock    uint32
	clockAcc int

	// yieldsSinceTick counts busy calls, see Busy.
	busyCount int
)

// Yield hands control back to Ebiten until its next tick. Only the game goroutine calls it.
func Yield() {
	busyCount = 0
	yielded <- struct{}{}
	<-tick
}

// Busy is called from routines the game spins on (the clock, input polls): after enough
// calls without a Yield, it yields, so a busy wait lets time pass.
func Busy() {
	busyCount++
	if busyCount > 64 {
		Yield()
	}
}

// Run starts Ebiten with the game's entry point running on its own goroutine.
func Run(title string, scale int, gameMain func()) error {
	ebiten.SetWindowTitle(title)
	ebiten.SetWindowSize(ScreenW*scale, ScreenH*scale*6/5)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(TPS)
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	go func() {
		<-tick
		defer func() {
			finished.Store(true)
			yielded <- struct{}{}
		}()
		gameMain()
	}()
	g := &game{}
	g.img = ebiten.NewImage(ScreenW, ScreenH)
	g.rgba = image.NewRGBA(image.Rect(0, 0, ScreenW, ScreenH))
	return ebiten.RunGame(g)
}

type game struct {
	img  *ebiten.Image
	rgba *image.RGBA
	// the 320x200 picture's placement on the window, for mapping the pointer
	offX, offY, scaleX, scaleY float64
}

func (g *game) Update() error {
	if finished.Load() {
		return ebiten.Termination
	}
	pollInput(g)
	if scriptQuit {
		return ebiten.Termination
	}
	clockAcc += 256
	Clock += uint32(clockAcc / TPS)
	clockAcc %= TPS
	tickTimers()
	tick <- struct{}{}
	<-yielded
	if finished.Load() {
		return ebiten.Termination
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	if ShowFrame == nil {
		return
	}
	f := ShowFrame()
	if f == nil {
		return
	}
	pal := f.Palette
	var lut [256][4]byte
	for i := 0; i < 256; i++ {
		r, gg, b := pal[i*3]&63, pal[i*3+1]&63, pal[i*3+2]&63
		lut[i] = [4]byte{r<<2 | r>>4, gg<<2 | gg>>4, b<<2 | b>>4, 0xFF}
	}
	pix := g.rgba.Pix
	for i, c := range f.Pix[:ScreenW*ScreenH] {
		copy(pix[i*4:i*4+4], lut[c][:])
	}
	if f.CursorOn && f.Cursor != nil {
		for y := 0; y < f.CursorH; y++ {
			sy := f.CursorAtY - f.CursorHotY + y
			if sy < 0 || sy >= ScreenH {
				continue
			}
			for x := 0; x < f.CursorW; x++ {
				c := f.Cursor[y*f.CursorW+x]
				if c == 0 {
					continue
				}
				sx := f.CursorAtX - f.CursorHotX + x
				if sx < 0 || sx >= ScreenW {
					continue
				}
				o := (sy*ScreenW + sx) * 4
				copy(pix[o:o+4], lut[c][:])
			}
		}
	}
	g.img.WritePixels(pix)

	// Fit a 4:3 picture (320x200 shown as 320x240, the CRT's shape) in the window.
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	aspectW, aspectH := float64(ScreenW), float64(ScreenH)*1.2
	k := float64(sw) / aspectW
	if float64(sh)/aspectH < k {
		k = float64(sh) / aspectH
	}
	g.scaleX = k
	g.scaleY = k * 1.2
	g.offX = (float64(sw) - aspectW*k) / 2
	g.offY = (float64(sh) - aspectH*k) / 2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(g.scaleX, g.scaleY)
	op.GeoM.Translate(g.offX, g.offY)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.img, op)
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	s := ebiten.Monitor().DeviceScaleFactor()
	return int(float64(outsideWidth) * s), int(float64(outsideHeight) * s)
}

// Exit ends the program once the game goroutine returns.
func Exit(code int) {
	os.Exit(code)
}

// for inpututil, kept referenced
var _ = inpututil.IsKeyJustPressed
