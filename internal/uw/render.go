package uw

import (
	"fmt"

	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/render"
)

// The bridge to the renderer (internal/render): it is loaded from UW.EXE on first use,
// draws what the database describes and asks the game, through renderArt, for the
// level's textures and the objects' and critters' pictures.

var rend *render.Renderer

// segSeg048 is the graphics library's data, where the distance shading's defaults are.
const segSeg048 = 0x3963

func initRenderer() {
	if rend != nil {
		return
	}
	if exeImage == nil {
		if err := loadExe(); err != nil {
			pfatal(err.Error())
		}
	}
	r, err := render.Load(exeImage)
	if err != nil {
		pfatal(err.Error())
	}
	rend = r
	rend.Art = renderArt{}
	render.Debugf = dprintf
	d := exeBytes(segSeg048, 0x64C, 6)
	smoothDiv = int16(le16(d, 0))
	smoothBase = int16(le16(d, 2))
	smoothLowpass = int16(le16(d, 4))
	rend.SetLightTabs(lightabs[:])
	rend.SetSmooth(int(smoothDiv), int(smoothBase), int(smoothLowpass))
}

// rmem is the renderer's data segment, which the database writers address by offset.
func rmem() []byte {
	initRenderer()
	return rend.Mem
}

// syncRender gives the renderer the frame's camera and window.
func syncRender() {
	initRenderer()
	c := &cPlayer
	rend.SetCamera(render.Camera{X: c.x, Y: c.y, Z: c.z, Heading: c.heading, Pitch: c.pitch, Bank: c.bank})
}

// levelTex is the renderer's texture table: walls 0..47 and floors 48..57 at full size,
// then the same at 16 pixels.
func levelTex(i int) []byte {
	n := TXM_WALLS + TXM_FLOORS
	switch {
	case i < 0:
	case i < TXM_WALLS:
		return wallTex64[i]
	case i < n:
		return floorTex32[i-TXM_WALLS]
	case i < n+TXM_WALLS:
		return wallTex16[i-n]
	case i < 2*n:
		return floorTex16[i-n-TXM_WALLS]
	}
	return nil
}

func texFirstPixel(i int) byte {
	if t := levelTex(i); len(t) > 0 {
		return t[0]
	}
	return 0
}

// renderArt hands the renderer the game's pictures.
type renderArt struct{}

func (renderArt) Texture(i int) []byte { return levelTex(i) }
func (renderArt) GrPic(i int) []byte   { return slotPic(i) }
func (renderArt) MouseQ()              { mousQUp(true) }

// ObjectPic is do_uwobj's picture: below 1C0h an object's (its slot from obj_tab), from
// there the animations; decoded with its auxiliary palette (numbers from 20h fold down to
// (n-8)*2) and shaded.
func (renderArt) ObjectPic(typ, shade int) (render.Pic, bool) {
	slot := firstAnim + typ - 0x1C0
	if typ < 0x1C0 {
		if typ < 0 || 2*typ >= len(objTab) {
			return render.Pic{}, false
		}
		slot = int(objTab[2*typ] & 0x3FF)
	}
	bm := slotPic(slot)
	if len(bm) < 5 {
		return render.Pic{}, false
	}
	n := int(bm[3])
	if n >= 0x20 {
		n = (n - 8) * 2 & 0xFF
	}
	pal := make([]byte, 0, 32)
	for i := n; i < n+2 && i < len(Palettes); i++ {
		pal = append(pal, Palettes[i][:]...)
	}
	return decodePic(bm, pal, shade&0xFF, 0, 0)
}

func decodePic(bm, pal []byte, shade, hotX, hotY int) (render.Pic, bool) {
	w, h := int(bm[1]), int(bm[2])
	px := cFrmtoRaw(bm, pal, shade)
	if w == 0 || h == 0 || len(px) < w*h {
		return render.Pic{}, false
	}
	return render.Pic{Pix: append([]byte(nil), px[:w*h]...), W: w, H: h, HotX: hotX, HotY: hotY}, true
}

// CritterPic is do_uwcrit's and parse_cpg's frame: the critter's animation file and
// palette from ASSOC.ANM, the page the frame is on from the pages' frame counts, and in
// the page the frame's slot, its picture for the variant and the palette.
func (renderArt) CritterPic(typ, shade, frame, variant int) (render.Pic, bool) {
	if typ < 0 || 2*typ+1 >= len(crAssoc) || crAssoc[2*typ] == 0xFF {
		return render.Pic{}, false
	}
	cmap, palette := int(crAssoc[2*typ]), int(crAssoc[2*typ+1])
	if cmap >= len(crFrm) {
		return render.Pic{}, false
	}
	page := 0
	for page < 3 && frame >= int(crFrm[cmap][page]) {
		page++
	}
	p := critPage(cmap, page)
	at := func(i int) int {
		if i < 0 || i >= len(p) {
			return -1
		}
		return int(p[i])
	}
	if len(p) < 2 {
		return render.Pic{}, false
	}
	n := at(1)
	slot := at(2 + frame - at(0))
	if slot < 0 || slot == 0xFF || frame < at(0) {
		return render.Pic{}, false
	}
	di := 2 + n
	m := at(di)
	di++
	idx := at(di + slot*8 + variant&0xFF)
	if idx == 0xFF {
		idx = 0
	}
	di += m * 8
	np := at(di)
	di++
	if np <= palette || idx < 0 || m < 0 {
		return render.Pic{}, false
	}
	pal := p[min(di+palette*32, len(p)):min(di+palette*32+32, len(p))]
	di += np*32 + 2
	if di+2*idx+2 > len(p) {
		return render.Pic{}, false
	}
	rec := int(le16(p, di+2*idx))
	if rec+7 > len(p) {
		return render.Pic{}, false
	}
	// The frame record: width, height, hot spot x and y, format, then the data the
	// decoders read (size and pixels); cFrmtoRaw wants the .GR header round it.
	r := p[rec:]
	bm := append([]byte{r[4], r[0], r[1], 0}, r[5:]...)
	if r[4] == 4 {
		bm = append([]byte{r[4], r[0], r[1]}, r[5:]...)
	}
	return decodePic(bm, pal, shade&0xFF, int(r[2]), int(r[3]))
}

// copyFB copies the renderer's frame buffer into gr.FB.
func copyFB() {
	if len(gr.FB) != gr.FBW*gr.FBH || gr.FBW <= 0 {
		return
	}
	w, h := min(rend.W, gr.FBW), min(rend.H, gr.FBH)
	for y := 0; y < h; y++ {
		copy(gr.FB[y*gr.FBW:y*gr.FBW+w], rend.FB[y*rend.W:])
	}
}

// cFillFB fills the frame buffer with colour c.
func cFillFB(c byte) {
	initRenderer()
	rend.Fill(c)
	copyFB()
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

// dimStep is cDimFB: darken the frame buffer through lightabs row i.
func dimStep(i int) {
	rend.Dim(i)
	copyFB()
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
	initRenderer()
	saved := append([]byte(nil), rend.FB...)
	ShowStupidFirstPersonWeapon = false
	cFillFB(colour)
	sendFB()
	for i := count; i > 0; i-- {
		callback(i)
		sendFB()
		copy(rend.FB, saved)
		copyFB()
	}
	sendFB()
	ShowStupidFirstPersonWeapon = true
	mouseShow()
}

// fadeout3d and fadein3d ignore n and take 12 steps.
func fadeout3d(n int) { initRenderer(); out3d(12, dimStep, 0xF1) }
func fadein3d(n int)  { initRenderer(); in3d(12, dimStep, 0xF1) }

// pickPixel is the pick frame's colour at view position x, y (pick_3d's stdat read).
func pickPixel(x, y int) int {
	if x < 0 || y < 0 || x >= gr.FBW || y >= gr.FBH {
		return 0
	}
	return int(gr.FB[(gr.FBH-1-y)*gr.FBW+x]) // y counts up from the bottom row
}

// The critter pages: CRIT\CRxxPAGE.Nyy (xx the animation file in octal), read when a
// frame first needs them and kept until NightCleanCritPages.

var (
	crAssoc [0x80]byte    // ASSOC.ANM from 100h: animation file and palette per critter
	crFrm   [0x20][3]byte // the frame counts to the end of each file's first pages
	crPages = map[[2]int][]byte{}
)

func critPage(cmap, page int) []byte {
	k := [2]int{cmap, page}
	if p, ok := crPages[k]; ok {
		return p
	}
	p := readFile(fmt.Sprintf("crit\\cr%d%dpage.n%d%d", cmap>>3, cmap&7, page/10, page%10))
	crPages[k] = p
	return p
}

// preloadCr empties the critter page cache and, with loadPages, reads ASSOC.ANM's table
// and the frame counts from every page file's header. False if ASSOC.ANM is missing.
func preloadCr(loadPages bool) bool {
	clear(crPages)
	if !loadPages {
		return true
	}
	b := readFile("crit\\assoc.anm")
	if b == nil {
		for i := range crAssoc {
			crAssoc[i] = 0xFF
		}
		return false
	}
	clear(crAssoc[:])
	if len(b) > 0x100 {
		copy(crAssoc[:], b[0x100:])
	}
	for i := range crFrm {
		for j := 0; j < 4; j++ {
			frames := 0xA0
			if h := critPage(i, j); len(h) >= 2 {
				frames = int(h[0]) + int(h[1])
			}
			if j < 3 {
				crFrm[i][j] = byte(frames)
			}
			if frames >= 0xA0 {
				break
			}
		}
	}
	return true
}

// NightCleanCritPages drops the cached critter pages, after sleep.
func NightCleanCritPages() { clear(crPages) }

func rmemR16(off int) int {
	m := rmem()
	return int(m[off]) | int(m[off+1])<<8
}

func rmemW16(off, v int) {
	m := rmem()
	m[off] = byte(v)
	m[off+1] = byte(v >> 8)
}
