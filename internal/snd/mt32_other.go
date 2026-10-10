//go:build !(darwin || linux || freebsd)

package snd

import "errors"

// StartupMT32: munt is loaded with purego's dlopen, which this system lacks.
func StartupMT32(romDir string) error { return errors.New("MT-32 emulation is not available here") }
