package uw

import (
	"io"

	"github.com/felix/uw1go/internal/gr"
)

// Port of LOADGR.C: art loading, .GR picture files into graphics slots and .TR textures.
//
// The original keeps EMS pictures as paragraph addresses in grs_off and video memory
// pictures as video offsets. Here a slot holds either the picture's stored bytes (grsPic,
// for the EMS ranges) or a video memory block handle (grsOff, from first_vram on). Slot
// numbers, obj_tab and the first_* bases are as in the original.

var (
	firstObj    int
	firstAnim   int
	firstButton int
	firstTmobj  int
	firstVram   int

	grsPic [][]byte // per slot: the .GR bitmap (EMS pictures)
	grsOff []int    // per slot: the video memory block (pictures from firstVram)
	objTab [1024]uint16

	grIndex    int
	reloadBase = -1
	gsize      byte

	// Palettes are ALLPALS.DAT's 32 auxiliary 16-colour palettes.
	Palettes [32][16]byte
)

var grExt = [6]string{"", ".gr", ".tr", ".cr", ".sr", ".ar"}

// grFile is an open picture file (_ld_open's grfp, tmpoffs and tmpcnt).
type grFile struct {
	fd    int
	offs  []uint32
	count int
}

func setSlot(i int, pic []byte, vram int) {
	for len(grsPic) <= i {
		grsPic = append(grsPic, nil)
		grsOff = append(grsOff, 0)
	}
	if pic != nil {
		grsPic[i] = pic
	}
	if vram != 0 {
		grsOff[i] = vram
	}
}

func slotPic(i int) []byte {
	if i < 0 || i >= len(grsPic) {
		return nil
	}
	return grsPic[i]
}

func slotVram(i int) int {
	if i < 0 || i >= len(grsOff) {
		return 0
	}
	return grsOff[i]
}

// ldOpen opens DATA\<art><ext>, checks its type and reads its header and offset table.
func ldOpen(art string, typ int) *grFile {
	path := "data\\" + art + grExt[typ]
	fd := dosOpen(path, O_RDONLY)
	if fd < 0 {
		dprintf("gr no parse data %s\n", path)
		return nil
	}
	bad := func() *grFile {
		dosClose(fd)
		dprintf("gr no parse data %s\n", path)
		return nil
	}
	var b [4]byte
	if dosRead(fd, b[:1]) != 1 || int(b[0]) != typ {
		return bad()
	}
	if typ == 2 {
		if dosRead(fd, b[:1]) != 1 {
			return bad()
		}
		gsize = b[0]
	}
	if dosRead(fd, b[:2]) != 2 {
		return bad()
	}
	f := &grFile{fd: fd, count: int(le16(b[:], 0))}
	if typ == 3 {
		// get_pals: skip the palettes (PalStore is never set)
		if dosRead(fd, b[:1]) != 1 {
			return bad()
		}
		dosSeek(fd, int64(b[0])<<5, io.SeekCurrent)
	}
	tab := make([]byte, 4*(f.count+1))
	if dosRead(fd, tab) != len(tab) {
		return bad()
	}
	f.offs = make([]uint32, f.count+1)
	for i := range f.offs {
		f.offs[i] = le32(tab, 4*i)
	}
	return f
}

func (f *grFile) close() { dosClose(f.fd) }

// size is gronk_gr's picture size: 16-bit, as the original computes it.
func (f *grFile) size(image int) int {
	return int(uint16(f.offs[image+1]) - uint16(f.offs[image]))
}

// ldGr reads one picture; it returns nil and -1 on failure.
func (f *grFile) ldGr(image int) ([]byte, int) {
	var size int
	if image != f.count-1 {
		size = f.size(image)
	} else {
		end := dosSeek(f.fd, 0, io.SeekEnd)
		size = int(uint16(end) - uint16(f.offs[image]))
	}
	if dosSeek(f.fd, int64(f.offs[image]), io.SeekStart) < 0 {
		return nil, -1
	}
	if size == 0 {
		return nil, 0
	}
	buf := make([]byte, size)
	if dosRead(f.fd, buf) != size {
		return nil, -1
	}
	return buf, size
}

// gronkGr loads count pictures of art from start (all of them if count < 0) and hands each
// to move, which records it (index counts from 0).
func gronkGr(art string, start, count int, move func(img []byte, size, index int) bool) bool {
	f := ldOpen(art, 1)
	if f == nil {
		return false
	}
	defer f.close()
	if count < 0 {
		count = f.count - start
	}
	ok := true
	for i := 0; i < count && ok; i++ {
		n := start + i
		if n >= f.count {
			return false
		}
		size := f.size(n)
		img, got := f.ldGr(n)
		if got != size {
			ok = false
		} else if move != nil {
			ok = move(img, size, i) && ok
		}
	}
	lastGrCount = f.count
	return ok
}

// lastGrCount is tmpcnt after a load (load_gr_ems advances gr_index by it).
var lastGrCount int

func movenewEms(img []byte, size, index int) bool {
	setSlot(index+grIndex, img, 0)
	return true
}

func moveObjEms(base int) func([]byte, int, int) bool {
	return func(img []byte, size, index int) bool {
		if size == 0 {
			objTab[(base+index)*2] = 0
		} else {
			objTab[(base+index)*2] = uint16(grIndex)
			setSlot(grIndex, img, 0)
			grIndex++
		}
		return true
	}
}

// movenewVram copies an 8-bit picture to a new video memory block.
func movenewVram(img []byte, size, index int) bool {
	if len(img) < 5 {
		return false
	}
	w, h := int(img[1]), int(img[2])
	hd := gr.Valloc(w, h)
	if hd == 0 {
		return false
	}
	copyToVram(img, hd)
	setSlot(index+grIndex, nil, hd)
	return true
}

func moveVram(img []byte, size, index int) bool {
	copyToVram(img, slotVram(index+grIndex))
	return true
}

func copyToVram(img []byte, hd int) {
	b := gr.VBlock(hd)
	if b == nil || len(img) < 5 {
		return
	}
	b.W, b.H = int(img[1]), int(img[2])
	if img[0] == 4 {
		copy(b.Pix, img[5:])
	} else {
		pal := Palettes[img[3]&0x1F][:]
		copy(b.Pix, cFrmtoRaw(img, pal, noShade))
	}
}

func loadGrEms(art string) bool {
	ok := gronkGr(art, 0, -1, movenewEms)
	grIndex += lastGrCount
	return ok
}

func loadObjEms(art string) bool {
	return gronkGr(art, 0, -1, moveObjEms(0))
}

func reloadObjEms(art string, base, count int) bool {
	reloadBase = base
	return gronkGr(art, 0, count, moveObjEms(base))
}

// loadTrEms loads the level's wall textures TxmID[12..47] from art (w64 or w16).
func loadTrEms(art string) bool {
	f := ldOpen(art, 2)
	if f == nil {
		return false
	}
	defer f.close()
	bytes := int(gsize) * int(gsize)
	i := 0x0C
	for ; i < 0x30; i++ {
		id := int(TxmID[i])
		if id < 0 || id >= f.count || dosSeek(f.fd, int64(f.offs[id]), io.SeekStart) < 0 {
			break
		}
		buf := make([]byte, bytes)
		if dosRead(f.fd, buf) != bytes {
			break
		}
		if gsize == 64 {
			wallTex64[i] = buf
		} else {
			wallTex16[i] = buf
		}
	}
	return i == 0x30
}

func loadGrVideo(art string) bool {
	ok := gronkGr(art, 0, -1, movenewVram)
	grIndex += lastGrCount
	return ok
}

// grLoadAt reloads count pictures of art from start into the blocks of the slots from icon
// offset on.
func grLoadAt(offset int, art string, start, count int) bool {
	old := grIndex
	grIndex = offset - ICON_VRAM + firstVram
	ok := gronkGr(art, start, count, moveVram)
	grIndex = old
	return ok
}

func reloadGrVpic(offset int, art string, image int) {
	old := grIndex
	grIndex = offset - ICON_VRAM + firstVram
	gronkGr(art, image, 1, moveVram)
	grIndex = old
}

// readGrFar reads one picture of art (its stored bytes).
func readGrFar(art string, image int) ([]byte, bool) {
	var got []byte
	ok := gronkGr(art, image, 1, func(img []byte, size, index int) bool {
		got = img
		return true
	})
	return got, ok
}

// loadAllGr is the start-up load. It returns 0 or an ERR_READ code.
func loadAllGr() int {
	b := readFile("data/allpals.dat")
	if b == nil {
		return ERR_READ | 0x008
	}
	for i := 0; i < 32; i++ {
		copy(Palettes[i][:], b[min(i*16, len(b)):])
	}
	ok := true
	ok = loadGrEms("question") && ok
	ok = loadGrEms("views") && ok
	firstObj = grIndex
	ok = loadObjEms("objects") && ok
	firstAnim = grIndex
	ok = loadGrEms("animo") && ok
	firstButton = grIndex
	ok = loadGrEms("buttons") && ok
	ok = loadGrEms("cursors") && ok
	ok = loadGrEms("3dwin") && ok
	firstTmobj = grIndex
	ok = reloadObjEms("tmflat", 0x170, 0x10) && ok
	ok = loadGrEms("tmobj") && ok
	firstVram = grIndex
	for _, n := range []string{"lfti", "flasks", "compass", "dragons", "inv", "power", "eyes", "chains", "spells", "scrledge", "optb"} {
		ok = loadGrVideo(n) && ok
	}
	if !ok {
		return ERR_READ | 0x004
	}
	if preloadCr(true) {
		return 0
	}
	return ERR_READ | 0x009
}

// loadDoors loads the level's six door pictures into the slots from first_tmobj + 30h.
func loadDoors() {
	old := grIndex
	grIndex = firstTmobj + 0x30
	for i := 0; i < 6; i, grIndex = i+1, grIndex+1 {
		slot := grIndex
		gronkGr("doors", int(ActDoors[i]), 1, func(img []byte, size, index int) bool {
			setSlot(slot, img, 0)
			return true
		})
	}
	grIndex = old
	seg042_19B()
}
