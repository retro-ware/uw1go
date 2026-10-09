package host

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Scripted input for testing, from UW1GO_INPUT: comma-separated events, each
// "tick:what:args" with the tick counted in host ticks from the start:
//
//	k:HEX        a key's scan code (E0 keys as E0xx), pressed and released
//	m:X:Y        move the pointer to (X, Y), screen pixels from the top left
//	c:X:Y        move there and click the left button
//	r:X:Y        the same with the right button
//	q            quit
//	s:NAME       save the screen as NAME.png in UW1GO_SHOT
//
// While a script is set, the real mouse and keyboard are ignored.

type scriptEvent struct {
	tick int
	what string
	args []int
	raw  []string
}

var (
	script     []scriptEvent
	scriptTick int
	scriptOn   bool
	scriptHold int // ticks left with the scripted button held
	scriptBtn  int
	scriptQuit bool
)

func init() {
	s := os.Getenv("UW1GO_INPUT")
	if s == "" {
		return
	}
	scriptOn = true
	for _, e := range strings.Split(s, ",") {
		f := strings.Split(strings.TrimSpace(e), ":")
		if len(f) < 2 {
			continue
		}
		t, _ := strconv.Atoi(f[0])
		ev := scriptEvent{tick: t, what: f[1], raw: f[2:]}
		for _, a := range f[2:] {
			base := 10
			if ev.what == "k" {
				base = 16
			}
			v, _ := strconv.ParseInt(a, base, 32)
			ev.args = append(ev.args, int(v))
		}
		script = append(script, ev)
	}
}

// runScript feeds the events due this tick; it reports whether a script is running.
func runScript() bool {
	if !scriptOn {
		return false
	}
	scriptTick++
	if scriptHold > 0 {
		scriptHold--
		if scriptHold == 0 {
			MouseButtons = 0
		}
	}
	for len(script) > 0 && script[0].tick <= scriptTick {
		ev := script[0]
		script = script[1:]
		switch ev.what {
		case "k":
			if len(ev.args) > 0 {
				c := ev.args[0]
				if c > 0xFF {
					KeyRing = append(KeyRing, 0xE0, byte(c), 0xE0, byte(c)|0x80)
				} else {
					KeyRing = append(KeyRing, byte(c), byte(c)|0x80)
				}
			}
		case "m", "c", "r":
			if len(ev.args) >= 2 {
				MouseX, MouseY = ev.args[0], ev.args[1]
				MouseMoved = true
			}
			if ev.what != "m" {
				MouseButtons = 1
				if ev.what == "r" {
					MouseButtons = 2
				}
				scriptHold = 4
			}
		case "q":
			scriptQuit = true
		case "s":
			if len(ev.raw) > 0 {
				saveShot(ev.raw[0])
			}
		}
	}
	return true
}

func saveShot(name string) {
	dir := os.Getenv("UW1GO_SHOT")
	if dir == "" || ShowFrame == nil {
		return
	}
	f := ShowFrame()
	if f == nil {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, ScreenW, ScreenH))
	for i, c := range f.Pix[:ScreenW*ScreenH] {
		r, g, b := f.Palette[int(c)*3]&63, f.Palette[int(c)*3+1]&63, f.Palette[int(c)*3+2]&63
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = r<<2|r>>4, g<<2|g>>4, b<<2|b>>4, 255
	}
	out, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		return
	}
	png.Encode(out, img)
	out.Close()
}
