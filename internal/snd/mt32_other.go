//go:build !(darwin || linux || freebsd || js)

package snd

import "errors"

// mtOpen: munt is loaded with purego's dlopen, which this system lacks.
func mtOpen(romDir string) error { return errors.New("MT-32 emulation is not available here") }

func mtMsg(msg uint32)   {}
func mtSysex(msg []byte) {}
func mtRender(b []int16) {}
