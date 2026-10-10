package snd

import (
	"errors"
	"syscall/js"
)

// The MT-32 binding in the browser (see mt32.go): goplay's page loads munt as
// mt32emu.wasm and offers it as goplayMT32, which picks the control and PCM ROM among
// the files in romDir of the page's file system.

var mtJS js.Value

func mtOpen(romDir string) error {
	g := js.Global().Get("goplayMT32")
	if g.IsUndefined() || g.Get("ready").IsUndefined() || !g.Call("ready").Bool() {
		return errors.New("the page has no MT-32 emulator")
	}
	if err := g.Call("open", romDir, outRate); err.Truthy() {
		return errors.New(err.String())
	}
	mtJS = g
	return nil
}

func mtMsg(msg uint32) { mtJS.Call("playMsg", msg) }

func mtSysex(msg []byte) {
	a := js.Global().Get("Uint8Array").New(len(msg))
	js.CopyBytesToJS(a, msg)
	mtJS.Call("playSysex", a)
}

var mtBytes []byte

// mtRender: goplayMT32.render returns the frames as a Uint8Array (16-bit little endian).
func mtRender(b []int16) {
	if cap(mtBytes) < len(b)*2 {
		mtBytes = make([]byte, len(b)*2)
	}
	buf := mtBytes[:len(b)*2]
	js.CopyBytesToGo(buf, mtJS.Call("render", len(b)/2))
	for i := range b {
		b[i] = int16(uint16(buf[2*i]) | uint16(buf[2*i+1])<<8)
	}
}
