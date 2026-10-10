package uw

import (
	"io"
	"strings"

	"github.com/memmaker/uw1go/internal/gr"
)

// Port of CUTS.C: the cutscene player. A cutscene is CUTS\csNNN.n00, an opcode script, and
// .n01 onwards, Deluxe Paint LPF animations played in turn (NNN and the extension's digits
// in octal). Records are whole pictures or run/skip/dump deltas, decoded into cutsScreen.
//
// The original streamed the large pages through three EMS windows a slice per frame; here
// each large page is read whole when its turn comes, which gives the same frames. The
// scripts are all shorter than the original's 0x800-byte script buffer, so they are read
// whole too.

// cutsState is struct CutsState.
type cutsState struct {
	name       []byte // CUTS\csXXX.nXX
	x, y, w, h int
	windowed   bool
	palette    [0x300]byte
	textLines  [6]string
	color38    byte
	flag39     int
	frame3B    int
	frame3D    int
	frame3F    uint16
	repeat41   uint16
	speech43   int
	fade45     int
	fade47     int
	// flags: b0 drawing, b1 a key arrived, b2 play on in this file, b3 play on in the
	// cutscene, b4 Escape may end it, b5 speech available, b6 speech playing, b7 the
	// current wait ignores keys
	b0, b1, b2, b3, b4, b5, b6, b7 bool
}

type lpDesc struct{ base, nrecords, nbytes uint16 }

type anmCycle struct {
	started, period uint16
	first, last     byte
}

type anmHdr struct {
	nlps      int
	lastdelta bool
	rate      int
	cycles    [16]anmCycle
	palette   []byte // 256 of blue, green, red, pad
	lps       [256]lpDesc
}

type cutsOp func(code []uint16, at int, st *cutsState) int

var (
	cutsDispatch [16]cutsOp
	curFrame     int
	cutsScreen   = make([]byte, 0xFA00)
)

func init() {
	cutsDispatch = [16]cutsOp{
		cutsopTxt, cutsopErase, cutsopFunc, cutsopPause, cutsopSkip, cutsopStop,
		cutsopEnd, cutsopLoop, cutsopData, cutsopFadeout, cutsopFadein, cutsopJump,
		cutsopPunt, cutsopSay, cutsopWait, cutsopClang,
	}
}

// anmLptab sorts the large pages by their first record, giving the order to play them.
func anmLptab(desc *[256]lpDesc, n int) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	for {
		done := true
		for i := 1; i < n; i++ {
			if desc[order[i-1]].base > desc[order[i]].base {
				done = false
				order[i-1], order[i] = order[i], order[i-1]
			}
		}
		if done {
			break
		}
	}
	return order
}

func readAnimHdr(fd int) (*anmHdr, bool) {
	b := make([]byte, 0xB00)
	got := dosRead(fd, b)
	h := &anmHdr{
		nlps:      int(le16(b, 6)),
		lastdelta: b[0x1A] != 0,
		rate:      int(le16(b, 0x44)),
		palette:   b[0x100:0x500],
	}
	for i := range h.cycles {
		o := 0x80 + 8*i
		h.cycles[i] = anmCycle{le16(b, o), le16(b, o+2) >> 8 & 0x3F, b[o+6], b[o+7]}
	}
	for i := range h.lps {
		o := 0x500 + 6*i
		h.lps[i] = lpDesc{le16(b, o), le16(b, o+2), le16(b, o+4)}
	}
	if h.rate == 0 {
		h.rate = 1
	}
	return h, got == 0xB00
}

// readlp reads large page page (at 0xB00 + page * 64 KB).
func readlp(fd int, page int, d lpDesc) []byte {
	dosSeek(fd, int64(page)<<16+0xB00, io.SeekStart)
	buf := make([]byte, int(d.nbytes)+int(d.nrecords)*2+8)
	n := dosRead(fd, buf)
	if n < 0 {
		return nil
	}
	return buf[:n]
}

func convAnmpal(src []byte, dst []byte) {
	for i := 0; i < 256; i++ {
		dst[i*3] = src[i*4+2] >> 2
		dst[i*3+1] = src[i*4+1] >> 2
		dst[i*3+2] = src[i*4] >> 2
	}
}

func anmCycleColors(c *[16]anmCycle) {
	for i := range c {
		if c[i].period == 0 {
			continue
		}
		if uint16(GameTime()-uint32(c[i].started)) < uint16(0x38E/int(c[i].period)) {
			continue
		}
		n := int(c[i].last) - int(c[i].first) + 1
		rotateBank(int(c[i].first), n, false)
		gr.LocalDoPalette(n, int(c[i].first))
		c[i].started = uint16(GameTime())
	}
}

// lpfDelta is LPFDELTA.ASM's seg002_A: the run/skip/dump decoder.
func lpfDelta(src []byte, dst []byte) {
	si := 0
	var di uint16
	next := func() byte {
		if si < len(src) {
			si++
			return src[si-1]
		}
		si++
		return 0x80
	}
	put := func(v byte) {
		if int(di) < len(dst) {
			dst[di] = v
		}
		di++
	}
	for si < len(src) {
		c := int8(next())
		switch {
		case c > 0:
			for n := int(c); n > 0; n-- {
				put(next())
			}
		case c == 0:
			n := int(next())
			v := next()
			for ; n > 0; n-- {
				put(v)
			}
		case uint8(c) != 0x80:
			di += uint16(uint8(c) - 0x80)
		default:
			w := uint16(next()) | uint16(next())<<8
			if int16(w) > 0 {
				di += w
				continue
			}
			if w == 0 {
				return
			}
			n := w - 0x8000
			if n < 0x4000 {
				for ; n > 0; n-- {
					put(next())
				}
			} else {
				v := next()
				for n -= 0x4000; n > 0; n-- {
					put(v)
				}
			}
		}
	}
}

// Opcode 0: txt(colour, string): a subtitle, split at newlines and to lines 320 wide.
func cutsopTxt(code []uint16, at int, st *cutsState) int {
	if !st.b0 {
		return 2
	}
	st.color38 = byte(code[at])
	text := getString(int(code[at+1]))
	lines := strings.SplitN(text, "\n", 7)
	if len(lines) > 6 {
		lines = lines[:6]
	}
	count, parts := 0, 0
	add := func(s string) {
		if count < 6 {
			st.textLines[count] = s
			count++
		}
		parts++
	}
	for _, s := range lines {
		if count >= 6 {
			break
		}
		start, pos, used := 0, 0, 0
		for pos >= 0 {
			t := pos
			var width int
			if sp := strings.IndexByte(s[t:], ' '); sp >= 0 {
				pos = t + sp + 1
				width = gr.StringWidth(s[t:pos])
			} else {
				pos = -1
				width = gr.StringWidth(s[t:])
				if used+width <= 320 {
					add(s[start:])
				}
			}
			if used+width > 320 && t > start {
				add(s[start : t-1])
				start, pos, used = t, t, 0
			} else {
				used += width
			}
		}
	}
	st.flag39 = min(parts, 6)
	return 2
}

func cutsopErase(code []uint16, at int, st *cutsState) int {
	st.flag39 = 0
	return 0
}

func cutsopFunc(code []uint16, at int, st *cutsState) int { return 2 }

// Opcode 3: pause(n): hold this frame for n * 256 ticks or until a key.
func cutsopPause(code []uint16, at int, st *cutsState) int {
	if !st.b0 {
		return 1
	}
	st.b1 = false
	st.frame3D = int(code[at-2])
	st.frame3F = code[at]
	return 1
}

// Opcode 14: wait(a, b).
func cutsopWait(code []uint16, at int, st *cutsState) int {
	if !st.b0 {
		return 2
	}
	st.b1 = false
	st.frame3D = int(code[at-2])
	if st.b5 {
		st.b7 = true
		st.frame3F = code[at+1]
	} else {
		st.frame3F = code[at]
	}
	return 2
}

// Opcode 4: skip(frame, n).
func cutsopSkip(code []uint16, at int, st *cutsState) int {
	if !st.b0 {
		return 2
	}
	st.b1 = false
	st.frame3B = int(code[at])
	if !st.b5 {
		st.frame3F = code[at+1]
		st.frame3D = int(code[at]) - 1
	}
	return 2
}

func cutsopStop(code []uint16, at int, st *cutsState) int {
	st.b2 = false
	return 1
}

func cutsopEnd(code []uint16, at int, st *cutsState) int {
	st.b2, st.b3 = false, false
	return 0
}

func cutsopLoop(code []uint16, at int, st *cutsState) int {
	st.repeat41 = code[at]
	st.frame3B = int(code[at-2]) + 1
	st.frame3D = 0
	st.b1 = false
	return 1
}

// Opcode 8: data(file, ext): the next file's name.
func cutsopData(code []uint16, at int, st *cutsState) int {
	st.name[7] = byte(code[at]>>6&7) + '0'
	st.name[8] = byte(code[at]>>3&7) + '0'
	st.name[9] = byte(code[at]&7) + '0'
	st.name[12] = byte(code[at+1]>>3&7) + '0'
	st.name[13] = byte(code[at+1]&7) + '0'
	return 2
}

// Opcode 13: say(colour, string, voc).
func cutsopSay(code []uint16, at int, st *cutsState) int {
	if st.b5 {
		if !st.b6 && !initVoc() {
			st.speech43 = -1
			st.b5 = false
			cutsopTxt(code, at, st)
		} else {
			st.speech43 = int(code[at+2])
			if code[at+2] == 999 {
				st.speech43 = -1
			}
		}
	} else {
		cutsopTxt(code, at, st)
	}
	return 3
}

func cutsopFadeout(code []uint16, at int, st *cutsState) int {
	if !st.windowed && st.fade47 > -2 {
		st.fade47 = int(int16(code[at]))
	}
	return 1
}

func cutsopFadein(code []uint16, at int, st *cutsState) int {
	if !st.windowed && st.fade45 > -2 {
		st.fade45 = int(int16(code[at]))
	}
	return 1
}

func cutsopJump(code []uint16, at int, st *cutsState) int {
	if code[at-2] != code[at]-1 {
		st.frame3B = int(code[at]) - 1
		st.frame3D = 0
		st.frame3F = 0
		st.b1 = true
		st.b0 = false
	}
	return 1
}

func cutsopPunt(code []uint16, at int, st *cutsState) int {
	st.b4 = code[at] != 0
	return 1
}

func cutsopClang(code []uint16, at int, st *cutsState) int {
	playEffectHere(0x11, 0x40, 0)
	return 0
}

func doSound(st *cutsState) {
	loopMusicMaybe()
	if st.b6 && st.speech43 != -1 {
		updateSpeech()
		if speechOver() {
			st.b6 = false
			st.speech43 = -1
		}
	}
}

// runOps runs the script records at code[*pc] while cond holds (each record: the frame,
// the opcode, its arguments).
func runOps(code []uint16, pc *int, st *cutsState, cond func() bool) {
	for *pc+1 < len(code) && cond() {
		*pc += 2
		if op := code[*pc-1]; op < 0x10 {
			*pc += cutsDispatch[op](code, *pc, st)
		}
	}
}

// cutsKey reads the input waiting: the last key, or a button.
func cutsKey() int {
	key := -1
	var in int
	for {
		in = mouseGetInput()
		if in <= 3 {
			break
		}
		key = in
	}
	if in > -1 && in < 4 {
		key = in
	}
	return key
}

func bumpExt(name []byte) {
	name[13]++
	if name[13] > '7' {
		name[12]++
		name[13] = '0'
	}
}

// showAnm plays cutscene cuts in the window x, y, w, h (y the top row).
func showAnm(cuts, x, y, w, h int) {
	st := &cutsState{name: []byte("CUTS\\csXXX.nXX"), x: x, y: y, w: w, h: h}
	st.windowed = !(x == 0 && y == 0xC7 && w == 0x140 && h == 0xC8)
	st.speech43 = -1
	st.b5 = speechAvailable()
	st.name[7] = byte(cuts>>6&7) + '0'
	st.name[8] = byte(cuts>>3&7) + '0'
	st.name[9] = byte(cuts&7) + '0'
	st.name[12], st.name[13] = '0', '0'

	n00 := readFile(string(st.name))
	if n00 == nil {
		return
	}
	code := make([]uint16, 0x400)
	for i := 0; i+1 < len(n00) && i/2 < len(code); i += 2 {
		code[i/2] = le16(n00, i)
	}
	if cutsPatch != nil {
		code[2], code[3], code[6] = uint16(*cutsPatch), uint16(*cutsPatch), uint16(*cutsPatch)
	}
	pc := 0
	st.b3, st.b0, st.b4 = true, true, true
	st.fade45, st.fade47 = -1, -2
	copy(st.palette[:], gr.Palette[:])
	if !st.windowed {
		fadeout(st.palette[:], 2)
	}
	gr.GrSoftPageFlip()
	gr.CopyVisibleToHidden()
	bumpExt(st.name)
	runOps(code, &pc, st, func() bool { return code[pc] == 0 })

	defer func() {
		freeSpeechStuff()
	}()
	for st.b3 {
		fd := dosOpen(string(st.name), O_RDONLY)
		dprintf("cuts: %s fd %d\n", st.name, fd)
		if fd < 0 {
			break
		}
		if !showAnmFile(fd, st, code, &pc) {
			dosClose(fd)
			goto done
		}
		dosClose(fd)
	}
done:
	if st.speech43 != -1 {
		stopSpeech()
	}
	gr.GrSoftPageFlip()
	if !st.windowed {
		if st.fade47 != -2 {
			fadeout(st.palette[:], 2)
		}
		grfxClear()
	}
}

// showAnmFile plays one LPF file; false when Escape ended the cutscene.
func showAnmFile(fd int, st *cutsState, code []uint16, pc *int) bool {
	hdr, ok := readAnimHdr(fd)
	if !ok {
		st.b3 = false
		return true
	}
	if !st.windowed {
		convAnmpal(hdr.palette, st.palette[:])
	}
	now := GameTime()
	inputTime := now
	lptab := anmLptab(&hdr.lps, hdr.nlps)
	st.repeat41 = 0
	bumpExt(st.name)
	st.flag39 = 0
restart:
	if st.b3 {
		st.b2 = true
	}
	curFrame = 1
	runOps(code, pc, st, func() bool { return code[*pc] == 0 && st.b2 && st.repeat41 == 0 })
	for i := 0; i < hdr.nlps && st.b2; i++ {
		lp := lptab[i]
		page := readlp(fd, lp, hdr.lps[lp])
		nrec := int(hdr.lps[lp].nrecords)
		if len(page) < 8+nrec*2 {
			return true
		}
		sizes := page[8 : 8+nrec*2]
		data := 8 + nrec*2
		last := 0
		if hdr.nlps-1 == i && hdr.lastdelta {
			last = 1
		}
		st.frame3D, st.frame3F = 0, 0
		st.b1 = false
		for rec := 0; nrec-last > rec && st.b2; rec++ {
			size := int(le16(sizes, rec*2))
			if data+4 > len(page) {
				break
			}
			hb := page[data+1]
			wd := (int(le16(page, data+2)) + 1) &^ 1
			off := 2
			if hb != 0 {
				off = wd + 4
			}
			frameStart := now
			if size != 0 && size-off != 0 && data+off+2 <= len(page) {
				end := min(data+size, len(page))
				switch page[data+off] {
				case 0:
					copy(cutsScreen, page[data+off+2:end])
				case 1:
					lpfDelta(page[data+off+2:end], cutsScreen)
				}
			}
			if st.b0 {
				gr.Show(st.x, st.y, cutsScreen, 0xC8, 0x140, 0x140-st.w, 0xC8-st.h)
				for {
					anmCycleColors(&hdr.cycles)
					doSound(st)
					key := cutsKey()
					if key != -1 && st.speech43 == -1 && (key > 3 || GameTime()-inputTime > 0x40) {
						st.b1 = true
						inputTime = GameTime()
					}
					if key == KEY_ESC && st.b4 {
						return false
					}
					now = GameTime()
					if now-frameStart >= uint32(0x100/hdr.rate) {
						break
					}
				}
				if curFrame < st.frame3B && st.b1 {
					if st.fade47 == -2 {
						st.b1 = false
					} else {
						st.b0 = false
						st.frame3F, st.frame3D, st.repeat41 = 0, 0, 0
					}
				}
			} else if curFrame == st.frame3B {
				st.frame3B, st.frame3D, st.frame3F = 0, 0, 0
				st.b0 = true
				st.b1 = false
			}
			runOps(code, pc, st, func() bool {
				return st.b2 && int(code[*pc]) == curFrame && st.repeat41 == 0
			})
			if st.b0 {
				if st.flag39 > 0 {
					gr.ForegroundColor = st.color38
					gr.BackgroundColor = st.color38
					ty := st.y - st.h + fontHeight()*st.flag39 + 2
					for k := 0; k < st.flag39; k++ {
						tx := (st.x + st.w - gr.StringWidth(st.textLines[k])) / 2
						gr.StringToScreen(st.textLines[k], tx, ty)
						ty -= fontHeight()
					}
				}
				frameStart = now
				gr.GrPageFlip()
				shotEvery("cuts", 25)
				if st.fade45 > -1 {
					fadein(st.palette[:], st.fade45)
					st.fade45 = -2
					st.fade47 = -1
				}
			}
			if st.b5 && !st.b6 && st.speech43 > -1 && st.speech43 < 999 {
				playSpeech(st.speech43)
				st.b6 = true
			}
			data += size
			if st.frame3B-1 == curFrame && st.repeat41 != 0 {
				st.repeat41--
				goto restart
			}
			for curFrame == st.frame3D && (st.frame3F == 999 ||
				uint16((GameTime()-frameStart)>>8) < st.frame3F || st.b7) {
				anmCycleColors(&hdr.cycles)
				doSound(st)
				if st.b7 && speechOver() {
					st.b7 = false
					st.frame3F += uint16((GameTime() - frameStart) >> 8)
				}
				key := cutsKey()
				if key == KEY_ESC && st.b4 {
					return false
				}
				if key != -1 && st.speech43 == -1 && (key > 3 || GameTime()-inputTime > 0x40) {
					st.b1 = true
				}
				if st.b1 {
					st.b1 = false
					break
				}
			}
			if st.fade47 > -1 {
				fadeout(st.palette[:], st.fade47)
				st.fade47 = -2
				st.fade45 = -1
			}
			doSound(st)
			curFrame++
		}
	}
	return true
}

// runcutscene plays cutscene n: below 100h full screen, from 100h in the 3D view's window.
func runcutscene(n int) {
	x, y, w, h := 0, 0xC7, 0x140, 0xC8
	if n >= 0x100 {
		x, y, w, h = 0x34, 0xB4, 0xAC, 0x70
	}
	if n == 1 || n == 2 || n == 3 {
		loadNewMusic(4, true)
	}
	grfxLoadFont("fontbig.sys")
	CutsceneOrConversationStringBlock = n + STRBLK_CUTSCENE
	mouseHide()
	if n == 0x103 {
		y += 9
		h += 0xB
	}
	showAnm(n, x, y, w, h)
	grfxLoadFont("font5x6p.sys")
	if inGame {
		loadTxtmaps()
	}
	if n < 0x100 {
		if inplist.mode == MODE_GAME {
			newscr(MODE_GAME)
		} else if inplist.mode != 0 {
			grfxQuikpal(PAL_GAME)
			editchng(0x7FFE)
		}
	} else {
		editchng(2)
	}
	mouseShow()
}

func initCutscene() {}

// valueCuts shows cutscene n with value written over the words at byte offsets 4, 6 and 12
// of its script (the dungeon level, a grave's number). The original wrote them into the
// file; here the script is patched as it is read, so the game files stay as they are.
func valueCuts(n, value int) {
	cutsPatch = &value
	defer func() { cutsPatch = nil }()
	runCutsceneChecked(n)
}

var cutsPatch *int

func runCutsceneChecked(n int) {
	name := []byte("CUTS\\csXXX.n00")
	name[7] = byte(n>>6&7) + '0'
	name[8] = byte(n>>3&7) + '0'
	name[9] = byte(n&7) + '0'
	if !fileExists(string(name)) {
		return
	}
	runcutscene(n)
}
