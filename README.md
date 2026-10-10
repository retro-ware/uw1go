# uw1go

Ultima Underworld: The Stygian Abyss (1992), ported to Go and
[Ebiten](https://ebitengine.org). It runs on the original game files and
reads all of its data from them, UW.EXE's tables included. No game data is
in this repository: you need your own copy of the DOS game (the GOG version
works).

The port follows the decompiled source of
[underworld-exhumed](https://github.com/memmaker/underworld-exhumed)
function by function: one Go file per C file in `internal/uw`, the 3D
engine machine-translated into `internal/r3d` (see `tools/c2go`).

## Running

```sh
go run ./cmd/uw1 -data /path/to/UW
```

`-data` is the game directory (the one with `UW.EXE`, `DATA`, `CRIT`,
`CUTS` and `SOUND`); it defaults to `gamedata/UW`. Saved games go to your
user config directory unless you pass `-saves DIR`.

## Sound

The game reads its sound card from `DATA\UW.CFG`, as the original does. The
copies sold today ship that file unset (no sound), so the card can also be
chosen with environment variables:

| Variable | Values |
|---|---|
| `UW1GO_SOUND` | `2` AdLib, `3` Sound Blaster, `4` SB Pro and `5` PAS (stereo), `6` Roland MT-32 |
| `UW1GO_SPEECH` | `1` to `3`: the cutscene speech (Sound Blaster digital) |
| `UW1GO_MT32_ROMS` | the folder with the MT-32 control and PCM ROMs (default `./roms`) |

The AdLib cards run on an OPL2 emulator with the game's own instruments
from `SOUND\UW.AD`. The MT-32 uses [munt](https://github.com/munt/munt)'s
libmt32emu, loaded at run time (`brew install mt32emu`; no cgo needed), with
ROMs you supply. Without the library or the ROMs it falls back to AdLib.
The PC speaker is not emulated.

## In the browser

`-import SRC` finds the game anywhere under `SRC`, copies what it needs to
`-data` with a cover picture, and exits. This mode is for
[goplay](https://ruzzoli.de/goplay/), which runs the WebAssembly build
(`GOOS=js GOARCH=wasm go build ./cmd/uw1`) on the player's own files, with
`GOPLAY_DATA` naming the game's folder.

## Debugging

`UW1GO_DEBUG=1` logs to stderr. `UW1GO_INPUT` plays a script of timed key
presses and clicks (and ignores the real keyboard and mouse);
`UW1GO_SHOT=DIR` is where its screenshots go. `UW1GO_WARP=x,y`,
`UW1GO_LIGHT` and `UW1GO_TALK=1` help get to a place or a conversation
quickly.
