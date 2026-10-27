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
	"math"
	"os"
	"runtime"
	"strconv"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	ScreenW = 320
	ScreenH = 200
	// MaxW is the widest frame: a 32:9 screen.
	MaxW = 864
	// TPS is the host's tick rate; the game clock advances 256 ticks a second.
	TPS = 70
)

// Frame is what the host shows: a W by 200 paletted picture (top row first) and its
// palette of 6-bit VGA values. It is 320 wide, or wider for the game screen in a wide window.
type Frame struct {
	Pix     []byte
	W       int // 0 is ScreenW
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

// WantW is the frame width that fills the window with the CRT's pixels (the 200 lines
// shown 240 high): ScreenW for the window, which keeps the 4:3 shape, and as wide as the
// screen's shape for fullscreen, up to MaxW. UW1GO_WIDTH (or a script's w event) fixes it.
var (
	WantW  = ScreenW
	fixedW bool
)

func init() {
	if w, err := strconv.Atoi(os.Getenv("UW1GO_WIDTH")); err == nil {
		WantW, fixedW = clampW(w), true
	}
}

func clampW(w int) int { return min(max(w&^1, ScreenW), MaxW) }

func (f *Frame) width() int {
	if f.W <= 0 {
		return ScreenW
	}
	return f.W
}

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
			if r := recover(); r != nil {
				crashed(r)
			}
			finished.Store(true)
			yielded <- struct{}{}
		}()
		gameMain()
	}()
	ebiten.SetFullscreen(os.Getenv("GOPLAY_FULLSCREEN") == "1") // the goplay launcher asks for fullscreen
	return ebiten.RunGame(&game{})
}

type game struct {
	img    *ebiten.Image
	rgba   *image.RGBA
	frameW int // the shown frame's width
	// the frame's placement on the window, for mapping the pointer
	offX, offY, scaleX, scaleY float64
}

func (g *game) Update() error {
	if finished.Load() {
		return ebiten.Termination
	}
	if (ebiten.IsKeyPressed(ebiten.KeyAlt) && inpututil.IsKeyJustPressed(ebiten.KeyEnter)) ||
		inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	keepWindowShape()
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
	fw := f.width()
	if g.img == nil || fw != g.frameW {
		if g.img != nil {
			g.img.Deallocate()
		}
		g.img = ebiten.NewImage(fw, ScreenH)
		g.rgba = image.NewRGBA(image.Rect(0, 0, fw, ScreenH))
		g.frameW = fw
	}
	pix := g.rgba.Pix
	for i, c := range f.Pix[:fw*ScreenH] {
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
				if sx < 0 || sx >= fw {
					continue
				}
				o := (sy*fw + sx) * 4
				copy(pix[o:o+4], lut[c][:])
			}
		}
	}
	g.img.WritePixels(pix)

	// The picture fills the window, without bars: the game screen is as wide as the
	// window's shape asks (WantW), the other screens, 320 wide, are stretched in a wider one.
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	if !fixedW && sw > 0 && sh > 0 {
		WantW = clampW(int(math.Round(float64(ScreenH) * 1.2 * float64(sw) / float64(sh))))
	}
	g.scaleX = float64(sw) / float64(fw)
	g.scaleY = float64(sh) / float64(ScreenH)
	g.offX, g.offY = 0, 0
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(g.scaleX, g.scaleY)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.img, op)
}

// keepWindowShape gives the window back the 4:3 shape once a resize has kept still for a
// sixth of a second: its width stays, its height follows.
var (
	lastWinW, lastWinH int
	winStill           int
)

func keepWindowShape() {
	if runtime.GOOS == "js" || ebiten.IsFullscreen() || scriptOn { // a page sizes its canvas itself
		return
	}
	w, h := ebiten.WindowSize()
	if w != lastWinW || h != lastWinH {
		lastWinW, lastWinH, winStill = w, h, 0
		return
	}
	if winStill++; winStill == TPS/6 {
		if want := (w*3 + 2) / 4; w > 0 && (h-want > 1 || want-h > 1) {
			ebiten.SetWindowSize(w, want)
		}
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	s := ebiten.Monitor().DeviceScaleFactor()
	return int(float64(outsideWidth) * s), int(float64(outsideHeight) * s)
}

// Exit ends the program once the game goroutine returns.
func Exit(code int) {
	os.Exit(code)
}
