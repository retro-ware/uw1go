package uw

import (
	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/r3d"
)

// The bridge to the renderer (internal/r3d, the translated seg003/seg004/seg019): it is
// loaded from UW.EXE on first use, and before each frame gets the camera, the clock and
// the aux palettes; the level's textures go into its memory when they are loaded.

var r3dReady bool

func r3dInit() {
	if r3dReady {
		return
	}
	if exeImage == nil {
		if err := loadExe(); err != nil {
			pfatal(err.Error())
		}
	}
	if err := r3d.Load(exeImage); err != nil {
		pfatal(err.Error())
	}
	r3dReady = true
	r3d.ReadFile = readFile
	r3d.TickRead = func() uint16 { return uint16(GameTime()) }
	r3d.MouseQ = func() { mousQUp(true) }
	smoothDiv = int16(r3d.R16(r3d.Seg048 + 0x64C))
	smoothBase = int16(r3d.R16(r3d.Seg048 + 0x64E))
	smoothLowpass = int16(r3d.R16(r3d.Seg048 + 0x650))
	r3d.SetLightTabs(lightabs[:])
}

// syncRender gives the renderer the frame's camera, clock and palettes.
func syncRender() {
	r3dInit()
	c := &cPlayer
	r3d.SetCamera(c.x, c.y, c.z, c.heading, c.pitch, c.bank)
	r3d.SetClock(GameTime())
	r3d.SetPalettes(&Palettes)
}

// The texture blocks in the renderer's memory: index i of its tables (seg051:C3EC, which
// runs on into C460) is walls 0..47 and floors 48..57 at full size, then 58.. the same
// at 16 pixels.
var texSegs [2 * (TXM_WALLS + TXM_FLOORS)]uint16

// texToR3d copies the level's textures into the renderer's memory and fills its tables.
func texToR3d() {
	r3dInit()
	n := TXM_WALLS + TXM_FLOORS
	if texSegs[0] == 0 {
		for i := range texSegs {
			size := 64 * 64
			switch {
			case i >= n:
				size = 16 * 16
			case i >= TXM_WALLS:
				size = 32 * 32
			}
			texSegs[i] = r3d.Alloc(size)
		}
	}
	for i := range texSegs {
		var src []byte
		switch {
		case i < TXM_WALLS:
			src = wallTex64[i]
		case i < n:
			src = floorTex32[i-TXM_WALLS]
		case i < n+TXM_WALLS:
			src = wallTex16[i-n]
		default:
			src = floorTex16[i-n-TXM_WALLS]
		}
		size := 16 * 16
		switch {
		case i < TXM_WALLS:
			size = 64 * 64
		case i < n:
			size = 32 * 32
		}
		dst := r3d.Bytes(int(texSegs[i])<<4, size)
		clear(dst)
		copy(dst, src)
		r3d.W16(r3d.Seg051+0xC3EC+2*i, texSegs[i])
	}
	clear(r3d.Bytes(r3d.Seg051+0xC378, 2*n))
}

// fbFromR3d copies the renderer's frame buffer into gr.FB.
func fbFromR3d() {
	if len(gr.FB) == gr.FBW*gr.FBH && gr.FBW > 0 {
		r3d.CopyFB(gr.FB, gr.FBW, gr.FBH)
	}
}

// cFillFB fills the frame buffer with colour c.
func cFillFB(c byte) {
	r3dInit()
	r3d.CFillFB(int(c))
	fbFromR3d()
}

// GRFX.C's 3D view fades and flash.

// fillFB fills the 3D view with one colour and shows it at once.
func fillFB(c byte) {
	cFillFB(c)
	mouseHide()
	ShowStupidFirstPersonWeapon = false
	sendFB()
	ShowStupidFirstPersonWeapon = true
	mouseShow()
}

// dimStep is seg019_CB7, cDimFB: darken the frame buffer through lightabs row i.
func dimStep(i int) {
	r3d.CDimFB(i)
	fbFromR3d()
}

// out3d calls callback(0..count), showing the view after each, then fills it with colour.
func out3d(count int, callback func(int), colour byte) {
	mouseHide()
	ShowStupidFirstPersonWeapon = false
	for i := 0; i <= count; i++ {
		callback(i)
		sendFB()
	}
	cFillFB(colour)
	sendFB()
	ShowStupidFirstPersonWeapon = true
	mouseShow()
}

// in3d is out3d's reverse: the rendered view is kept, colour shown, and the copy
// restored after each callback(count..1).
func in3d(count int, callback func(int), colour byte) {
	mouseHide()
	r3dInit()
	fb := r3d.Bytes(int(r3d.R16(r3d.Seg048+0x95A))<<4, 0x4BEC)
	saved := append([]byte(nil), fb...)
	ShowStupidFirstPersonWeapon = false
	cFillFB(colour)
	sendFB()
	for i := count; i > 0; i-- {
		callback(i)
		sendFB()
		copy(fb, saved)
		fbFromR3d()
	}
	sendFB()
	ShowStupidFirstPersonWeapon = true
	mouseShow()
}

// fadeout3d and fadein3d ignore n and take 12 steps.
func fadeout3d(n int) { r3dInit(); out3d(12, dimStep, 0xF1) }
func fadein3d(n int)  { r3dInit(); in3d(12, dimStep, 0xF1) }

// pickPixel is the pick frame's colour at view position x, y (pick_3d's stdat read).
func pickPixel(x, y int) int {
	if x < 0 || y < 0 || x >= gr.FBW || y >= gr.FBH {
		return 0
	}
	return int(gr.FB[(gr.FBH-1-y)*gr.FBW+x]) // y counts up from the bottom row
}

// seg042_19B forgets what the EMS frame holds (TMPALLOC.C).
func seg042_19B() {
	r3dInit()
	r3d.ForgetPages()
}
