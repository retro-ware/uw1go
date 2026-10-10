# uw1go

Ultima Underworld: The Stygian Abyss (1992), ported to Go and
[Ebiten](https://ebitengine.org). It runs on the original game files and
reads all of its data from them, UW.EXE's tables included. No game data is
in this repository: you need your own copy of the DOS game (the GOG version
works).

The port follows the decompiled source of
[underworld-exhumed](https://github.com/memmaker/underworld-exhumed)
function by function: one Go file per C file in `internal/uw`. The 3D renderer
(`internal/render`) is rewritten natively: the game draws each square's faces,
sprites and models through it directly, it runs the 3D models from UW.EXE as the
original's interpreter does, and it draws with floating point geometry and clipping.
It is not a faithful replica, though: it always plays as a modernized game, with the
changes below.

## What is modern

Most of it is John Glassmyer's
[UltimaHacks](https://github.com/JohnGlassmyer/UltimaHacks) (MIT) for
UW.EXE, as underworld-exhumed has it.

- The game starts at the main menu, without the title sequence or the introduction (the menu still offers it), and the menu's cursor wraps around.
- Textures are mapped perspective-correct (the idea from SiENcE's uwpatch), so walls no longer bend and floors no longer swim.
- You can look three times as far up and down, and the squares beside and behind you are drawn too. Creatures and objects keep their size when the view pitches.
- The 3D view fills the screen down to the message scroll, with no frame. The gargoyle sits in a small bar at the top centre and his eyes show the weapon's charge (dark grey, slight yellow, bright yellow, orange, glowing red); under him, for a few seconds after each blow you land, a bar shows the creature's health, narrowing as it falls and green, then orange, then red; the compass is at the bottom centre and the flasks at the bottom right. The icons, the power gem, the dragons and the frame are gone.
- Mouse look is on from the start; `V` turns it off and on. Leaving it shows the inventory panel over the view; turning it back on hides it. While it is on, the mouse turns the view and the cursor stays in the middle as a crosshair. A take goes straight into the pack (onto a matching stack first), as the crosshair cannot drag; with the pack full it stays carried. `Alt+V` cycles through the axis inversions. `UW1GO_LOOK_SPEED` sets its speed in percent (10 to 400, default 100).
- Movement is Quake-like: the keys held add up, so `W` with `A` or `D` goes diagonally (no faster than straight), sliding and backing up go most of running speed instead of a quarter, and speeding up and stopping take a tenth of a second at any frame rate. In a browser, a click takes the pointer back after `Esc` lets it go.
- Sliding along a wall no longer turns the view, and the right-hand panel slides twice as fast.
- The mouse acts and the keys walk. A left click in the view talks to a creature, takes an item in reach, and looks at anything else. A right click uses: it opens doors and chests, pulls levers, presses buttons. While something is carried, a left click in the view drops or throws it. On the inventory panel a left click picks an item up and the next puts it down (a drag works too), and a right click uses it: it opens a bag, eats food. Talk, get, look and use come from the click and `Q` `E`, with no modes; the sword is `Tab` and the options are `Esc`.
- `Tab` draws the weapon or puts it away. While it is drawn the left button slashes, the right thrusts and `Space` bashes; hold one to charge the blow and let go to strike; a quick click strikes at once, at the weakest.
- Looking at gear shows its stats after Angband: a weapon's damage by attack type, (slash/bash/thrust); the damage of a shot, (n), on ammunition and on the launcher that fires it, before the missile skill scales it; armour's protection at its quality, [n]. An identified enchantment adds (+accuracy,+damage) to a weapon and [n,+toughness] to armour, or (+n) for Protection, which makes the wearer harder to hit: "a longsword of Additional Damage (10/4/8) (+0,+3)", "a bow (8)", "a breastplate [4,+4]".
- `F1`, `?` or `Ctrl+H` shows a help screen with the controls and every key (three pages: `Space` or a click turns on, `Esc` or the CLOSE scroll returns). `Esc` opens the options panel, which `F1` did in DOS.
- The keys are UltimaHacks':

| Keys | Action |
| --- | --- |
| `W` `A` `S` `D` | walking (forward, slide left, back, slide right); the mouse turns; tap the left `Shift` while moving to run until you stop |
| `Space` | jump; held while standing still the knees bend, and let go from the full crouch (0.3 s) it is a standing long jump; with the weapon drawn bash (hold to charge); while flying, fly up |
| `C` | crouch while held: half as tall and half the walking speed, under anything that low; let go where there is no room and you stay down until there is; while flying, fly down |
| `Q` / `E` | look at / use what is under the cursor, in the view or on the inventory panel (`E` as a right click) |
| `I` / `R` / `K` | leave mouse look and show the inventory / rune bag / statistics panel (the same key again: back to mouse look) |
| `V` | mouse look on, off |
| `B` `N` | scroll the inventory up, down |
| `F` | cast the spell on the shelf |
| `Tab` | draw, sheathe the weapon |
| `G` / `H` | game info / both flasks |
| `M` | the map; there `S` `W` go a level up, down and `C` back to the player's |
| `F5` / `F7` | quick save / quick load |
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
