# uw1go

Ultima Underworld: The Stygian Abyss (1992), ported to Go and
[Ebiten](https://ebitengine.org). It runs on the original game files and
reads all of its data from them, UW.EXE's tables included. No game data is
in this repository: you need your own copy of the DOS game (the GOG version
works).

The port follows the decompiled source of
[underworld-exhumed](https://github.com/memmaker/underworld-exhumed)
function by function: one Go file per C file in `internal/uw`, the 3D
engine machine-translated into `internal/r3d` (see `tools/c2go`). It is not a
faithful replica, though: it always plays as a modernized game, with the changes below.

## What is modern

Most of it is John Glassmyer's
[UltimaHacks](https://github.com/JohnGlassmyer/UltimaHacks) (MIT) for
UW.EXE, as underworld-exhumed has it.

- The game starts at the main menu, without the title sequence or the introduction (the menu still offers it), and the menu's cursor wraps around.
- Textures are mapped perspective-correct (`internal/r3d/persp.go`, after SiENcE's uwpatch), so walls no longer bend and floors no longer swim.
- You can look three times as far up and down, and the squares beside and behind you are drawn too. Creatures and objects keep their size when the view pitches.
- Mouse look: `` ` `` turns it on and off (on ISO keyboards also the extra key beside the left `Shift`, which on a Mac is the key left of `1`, `^` on a German one). While it is on, the mouse turns the view and the cursor stays in the middle as a crosshair. `` Alt+` `` cycles through the axis inversions. `UW1GO_LOOK_SPEED` sets its speed in percent (10 to 400, default 100).
- Sliding along a wall no longer turns the view, and the right-hand panel slides twice as fast.
- The mouse acts and the keys walk. A left click in the view talks to a creature, takes an item in reach, and looks at anything else. A right click uses: it opens doors and chests, pulls levers, presses buttons. While something is carried, a left click in the view drops or throws it. The icons on the left still force one action.
- `F5` draws the weapon or puts it away. While it is drawn the left button slashes, the right thrusts and `Space` bashes; hold one to charge the blow and let go to strike.
- Looking at gear shows its stats after Angband: a weapon's damage by attack type, (slash/bash/thrust); the damage of a shot, (n), on ammunition and on the launcher that fires it, before the missile skill scales it; armour's protection at its quality, [n]. An identified enchantment adds (+accuracy,+damage) to a weapon and [n,+toughness] to armour, or (+n) for Protection, which makes the wearer harder to hit: "a longsword of Additional Damage (10/4/8) (+0,+3)", "a bow (8)", "a breastplate [4,+4]".
- `F1`, `?` or `Ctrl+H` shows a help screen with the controls and every key (three pages: `Space` or a click turns on, `Esc` or the CLOSE scroll returns). `Esc` opens the options panel, which `F1` did in DOS.
- The keys are UltimaHacks':

| Keys | Action |
| --- | --- |
| `W` `S` / `X` | run, back / walk forward |
| `A` `D` | slide left, right |
| grey `←` `→` | turn |
| `Ctrl` + grey arrows | step forward, back, left, right |
| left `Shift` / `Ctrl` | fly up / down (while flying); a lone left `Shift` jumps when let go, with `Ctrl` a standing long jump |
| `1` `3` / grey `↑` `↓`, keypad `5` | look up, down, straight ahead |
| `Space` | bash (hold to charge); `.` `;` `P` thrust, slash, bash |
| `Q` / `E` | look at / use what is under the cursor (`E` as a right click) |
| `R` / `F` | the rune bag / statistics panel (again: back to the inventory) |
| `C` `V` `B` | close the open container, scroll the inventory down, up |
| `G` / `H` | game info / both flasks |
| `Z` | the map; there `S` `W` go a level up, down and `C` back to the player's |
| `Ctrl+Alt+letter` | put that rune on the shelf; `Ctrl+Alt+Backspace` clears the shelf, `Ctrl+Alt+Space` casts |

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

`-import SRC` finds the game anywhere under `SRC` (also inside a CD image, such as
the `game.gog` of the GOG installer), copies what it needs to
`-data` with a cover picture, and exits. This mode is for
[goplay](https://ruzzoli.de/goplay/), which runs the WebAssembly build
(`GOOS=js GOARCH=wasm go build ./cmd/uw1`) on the player's own files, with
`GOPLAY_DATA` naming the game's folder. When the MT-32 on that page has its
ROMs, goplay also sets `GOPLAY_MT32` and the music plays on it (munt
compiled to WebAssembly, offered by the page as `goplayMT32`).

## Debugging

`UW1GO_DEBUG=1` logs to stderr. `UW1GO_INPUT` plays a script of timed key
presses and clicks (and ignores the real keyboard and mouse);
`UW1GO_SHOT=DIR` is where its screenshots go. `UW1GO_WARP=x,y`,
`UW1GO_LIGHT` and `UW1GO_TALK=1` help get to a place or a conversation
quickly.
