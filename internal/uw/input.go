package uw

// Port of INPUT.C: the input dispatcher. Screens register click areas (input_addmouse) and
// key handlers (input_addkey) with a mask of the screen modes they are live in;
// input_dispatch reads one event and calls the first entry that takes it.

// Inplist is the input state handed to a handler.
type Inplist struct {
	x, y  int // relative to the region that took the click
	mouse bool
	cmd   int // the input code; the buttons for a mouse event
	mode  int // mask of the screen modes
}

// Handler is an input handler; arg is the argument it was registered with.
type Handler func(arg any)

type mouseDispatch struct {
	hndl     int
	lrx, lry int
	ulx, uly int
	arg      any
	mask     int
	fn       Handler
	off      bool // inputEnable turned it off
}

type keyDispatch struct {
	hndl int
	key  int
	arg  any
	mask int
	fn   Handler
}

var (
	mcurhndl int
	mousDisp []mouseDispatch
	keyDisp  []keyDispatch
	inp      Inplist
	inplist  = &inp
	kcurhndl = -666
)

func initInput() {
	mousDisp = mousDisp[:0]
	keyDisp = keyDisp[:0]
	mcurhndl = 1
	kcurhndl = -1
	inplist.cmd = 0
}

func freeInput() {
	if kcurhndl != -666 {
		mousDisp, keyDisp = nil, nil
		kcurhndl = -666
	}
}

func inputAddmouse(ulx, uly, lrx, lry int, arg any, mask int, fn Handler) int {
	mousDisp = append(mousDisp, mouseDispatch{hndl: mcurhndl, lrx: lrx, lry: lry, ulx: ulx, uly: uly, arg: arg, mask: mask, fn: fn})
	mcurhndl++
	return mcurhndl - 1
}

func inputAddkey(key int, arg any, mask int, fn Handler) int {
	keyDisp = append(keyDisp, keyDispatch{hndl: kcurhndl, key: key, arg: arg, mask: mask, fn: fn})
	kcurhndl--
	return kcurhndl + 1
}

// inputEnable turns a mouse entry on or off.
func inputEnable(hndl int, on bool) {
	for i := range mousDisp {
		if mousDisp[i].hndl == hndl {
			mousDisp[i].off = !on
		}
	}
}

// inputSetRect moves a mouse entry.
func inputSetRect(hndl, ulx, uly, lrx, lry int) {
	for i := range mousDisp {
		if m := &mousDisp[i]; m.hndl == hndl && hndl != 0 {
			m.ulx, m.uly, m.lrx, m.lry = ulx, uly, lrx, lry
		}
	}
}

// inputDel removes an entry; the last entry moves into its place.
func inputDel(hndl int) {
	if hndl == 0 {
		return
	}
	if hndl > 0 {
		for i := range mousDisp {
			if mousDisp[i].hndl == hndl {
				mousDisp[i] = mousDisp[len(mousDisp)-1]
				mousDisp = mousDisp[:len(mousDisp)-1]
				return
			}
		}
		return
	}
	for i := range keyDisp {
		if keyDisp[i].hndl == hndl {
			keyDisp[i] = keyDisp[len(keyDisp)-1]
			keyDisp = keyDisp[:len(keyDisp)-1]
			return
		}
	}
	dprintf("bad kbd hndl %d\n", hndl)
}

// inputDispatch takes one input event and hands it to a handler.
func inputDispatch(in *Inplist) {
	code := mouseGetInputSp()
	if code < 0 {
		return
	}
	if code < 4 {
		x, y := mouseQgetxy()
		in.cmd = code
		in.mouse = true
		// the newest entry first, the 3D view's last: the furniture lies over it
		for pass := 0; pass < 2; pass++ {
			for i := len(mousDisp) - 1; i >= 0; i-- {
				m := &mousDisp[i]
				if (m.hndl == PMsHndle) != (pass == 1) || m.off {
					continue
				}
				if m.ulx <= x && m.uly <= y && m.lrx >= x && m.lry >= y && m.mask&in.mode != 0 && m.fn != nil {
					in.x = x - m.ulx
					in.y = y - m.uly
					fn, arg := m.fn, m.arg
					fn(arg)
					return
				}
			}
		}
		return
	}
	in.mouse = false
	dispatchKey(in, code)
}

func dispatchKey(in *Inplist, code int) {
	for i := range keyDisp {
		k := &keyDisp[i]
		if k.key == code && k.mask&in.mode != 0 && k.fn != nil {
			fn, arg := k.fn, k.arg
			fn(arg)
			return
		}
	}
}
