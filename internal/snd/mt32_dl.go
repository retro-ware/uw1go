//go:build darwin || linux || freebsd

package snd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ebitengine/purego"
)

// The MT-32 binding on the desktop (see mt32.go): munt's libmt32emu, loaded at run time
// with purego (no cgo).

var mt32Libs = []string{
	"libmt32emu.dylib", "/opt/homebrew/lib/libmt32emu.dylib", "/usr/local/lib/libmt32emu.dylib",
	"libmt32emu.so.2", "libmt32emu.so",
}

var mtAPI struct {
	loaded        bool
	createCtx     func(handler, data uintptr) uintptr
	addROMFile    func(ctx uintptr, name string) int32
	addMachineROM func(ctx uintptr, machine, name string) int32
	freeCtx       func(ctx uintptr)
	setRate       func(ctx uintptr, rate float64)
	openSynth     func(ctx uintptr) int32
	playMsg       func(ctx uintptr, msg uint32)
	playSysex     func(ctx uintptr, sysex *byte, n uint32)
	render        func(ctx uintptr, out *int16, frames uint32)
	version       func() string
}

func loadMT32Lib() error {
	if mtAPI.loaded {
		return nil
	}
	var lib uintptr
	var err error
	for _, name := range mt32Libs {
		if lib, err = purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
			break
		}
	}
	if lib == 0 {
		return fmt.Errorf("libmt32emu not found: %v", err)
	}
	purego.RegisterLibFunc(&mtAPI.createCtx, lib, "mt32emu_create_context")
	purego.RegisterLibFunc(&mtAPI.addROMFile, lib, "mt32emu_add_rom_file")
	purego.RegisterLibFunc(&mtAPI.addMachineROM, lib, "mt32emu_add_machine_rom_file")
	purego.RegisterLibFunc(&mtAPI.freeCtx, lib, "mt32emu_free_context")
	purego.RegisterLibFunc(&mtAPI.setRate, lib, "mt32emu_set_stereo_output_samplerate")
	purego.RegisterLibFunc(&mtAPI.openSynth, lib, "mt32emu_open_synth")
	purego.RegisterLibFunc(&mtAPI.playMsg, lib, "mt32emu_play_msg")
	purego.RegisterLibFunc(&mtAPI.playSysex, lib, "mt32emu_play_sysex")
	purego.RegisterLibFunc(&mtAPI.render, lib, "mt32emu_render_bit16s")
	purego.RegisterLibFunc(&mtAPI.version, lib, "mt32emu_get_library_version_string")
	mtAPI.loaded = true
	return nil
}

var mtCtx uintptr

// mtOpen starts the synth on the ROMs in romDir: the first machine of mtMachines whose
// control and PCM ROM are both there, else whatever pair munt finds. The original MT-32
// comes first: UW1 was written for it, and its music uses none of the CM-32L's extra drum
// keys.
func mtOpen(romDir string) error {
	if err := loadMT32Lib(); err != nil {
		return err
	}
	ents, err := os.ReadDir(romDir)
	if err != nil {
		return fmt.Errorf("MT-32 ROMs: %v", err)
	}
	names := []string{}
	for _, e := range ents {
		if !e.IsDir() {
			names = append(names, filepath.Join(romDir, e.Name()))
		}
	}
	sort.Strings(names)
	rc := int32(0)
	for _, machine := range append(mtMachines, "") {
		ctx := mtAPI.createCtx(0, 0)
		if ctx == 0 {
			return errors.New("mt32emu: no context")
		}
		for _, n := range names {
			if machine != "" {
				mtAPI.addMachineROM(ctx, machine, n)
			} else {
				mtAPI.addROMFile(ctx, n)
			}
		}
		mtAPI.setRate(ctx, outRate)
		if rc = mtAPI.openSynth(ctx); rc == 0 {
			mtCtx = ctx
			return nil
		}
		mtAPI.freeCtx(ctx)
	}
	return fmt.Errorf("mt32emu %s: cannot open the synth with the ROMs in %s (code %d)", mtAPI.version(), romDir, rc)
}

// mtMachines are munt's machine ids, best first: the original MT-32 (control ROM 1.0x),
// then the CM-32L and the MT-32 2.x.
var mtMachines = []string{"mt32_1_07", "mt32_1_06", "mt32_1_05", "mt32_1_04", "mt32_bluer", "cm32l_1_02", "cm32l_1_00", "cm32ln_1_00", "mt32_2_07", "mt32_2_06", "mt32_2_04", "mt32_2_03"}

func mtMsg(msg uint32)   { mtAPI.playMsg(mtCtx, msg) }
func mtSysex(msg []byte) { mtAPI.playSysex(mtCtx, &msg[0], uint32(len(msg))) }
func mtRender(b []int16) { mtAPI.render(mtCtx, &b[0], uint32(len(b)/2)) }
