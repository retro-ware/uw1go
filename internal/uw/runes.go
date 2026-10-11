package uw

import "github.com/memmaker/uw1go/internal/gr"

// Port of RUNES.C: the rune bag and casting from runes: adding a runestone to the bag,
// drawing the bag, clicking runes onto the shelf, clicking an active spell, and casting
// the spell the shelf spells out.

// Spell is a spell table entry: cls (class << 3), the three runes packed five bits each
// (0x18 none) and the minor.
type Spell struct {
	cls   byte
	runes int16
	sub   byte
}

func (s Spell) class() int { return int(s.cls&0xF8) >> 3 }

// spells is SPELLS.C's table of the 53 spells.
var spells = [NUM_SPELLS]Spell{
	{0x00, 0x2178, 0x83}, {0x10, 0x0512, 0x02}, {0x29, 0x3938, 0x01},
	{0x40, 0x2197, 0x01}, {0x18, 0x48F8, 0x02}, {0x08, 0x51F8, 0x01},
	{0x18, 0x0258, 0x01}, {0x08, 0x446F, 0x02}, {0x20, 0x202C, 0x02},
	{0x58, 0x5998, 0x01}, {0x38, 0x4058, 0x01}, {0x40, 0x2138, 0x03},
	{0x58, 0x466F, 0x00}, {0x18, 0x064B, 0x03}, {0x00, 0x4178, 0x85},
	{0x29, 0x38D8, 0x02}, {0x5A, 0x4938, 0x02}, {0x10, 0x2258, 0x43},
	{0x08, 0x5DF8, 0x44}, {0x20, 0x2198, 0x04}, {0x08, 0x1DF8, 0x03},
	{0x38, 0x3598, 0x04}, {0x18, 0x48B8, 0x46}, {0x5A, 0x0138, 0x03},
	{0x29, 0x3CB8, 0x03}, {0x38, 0x004C, 0x02}, {0x5A, 0x3AD7, 0x04},
	{0x18, 0x1A4F, 0x05}, {0x5A, 0x12F8, 0x05}, {0x58, 0x01B8, 0x06},
	{0x20, 0x550C, 0x0F}, {0x30, 0x55C6, 0x42}, {0x58, 0x562F, 0x0A},
	{0x38, 0x008F, 0x05}, {0x00, 0x550B, 0x86}, {0x5A, 0x39F7, 0x08},
	{0x08, 0x54EF, 0x05}, {0x38, 0x2191, 0x03}, {0x40, 0x2998, 0x04},
	{0x18, 0x564B, 0x44}, {0x30, 0x5416, 0x03}, {0x30, 0x3810, 0x81},
	{0x10, 0x22B2, 0x45}, {0x58, 0x55F7, 0x09}, {0x58, 0x39F6, 0x07},
	{0x30, 0x14F8, 0x44}, {0x58, 0x0278, 0x0B}, {0x58, 0x5542, 0x0C},
	{0x30, 0x6318, 0x05}, {0x28, 0x6318, 0x04}, {0x58, 0x6318, 0x0D},
	{0x50, 0x6318, 0x03}, {0x50, 0x6318, 0x09},
}

var (
	newShelf   bool // set by tryCast: the next rune click starts a new shelf
	spellDelay byte
	lstime     uint32
)

func hasRune(rune int) bool { return player.Runebag[rune>>3]>>(7-rune&7)&1 != 0 }

// addRune puts a runestone into the rune bag, freeing the object.
func addRune(obj *Object) bool {
	rune := obj.Item() - FIRST_RUNESTONE
	if rune < 0 || rune > NUM_RUNES {
		return false
	}
	ObjFree(obj)
	if rune>>3 < len(player.Runebag) {
		player.Runebag[rune>>3] |= 1 << (7 - rune&7)
	} else {
		player.Shelf[0] |= 1 << 7 // rune 0x18 lands in the byte after the bag
	}
	return true
}

// clearRunes empties the rune bag (and, as in the original, the shelf and the weight
// that follow it in the player record).
func clearRunes() {
	player.Runebag = [3]byte{}
	player.Shelf = [3]byte{}
	player.Weight = 0
}

// ShowRune draws rune's picture in its place in the rune bag panel, four to a row.
func ShowRune(rune int) {
	mouseHide()
	picToScreen(rune+FIRST_RUNESTONE, rune&3*0x12+0xF4, 0xBB-rune>>2*0xF)
	mouseShow()
}

// RedispRune draws every rune the bag holds.
func RedispRune() {
	mouseHide()
	for i := 0; i < NUM_RUNES; i++ {
		if hasRune(i) {
			gr.Transparency = true
			ShowRune(i)
			gr.Transparency = false
		}
	}
	mouseShow()
}

func clearShelf() {
	player.Shelf = [3]byte{RUNE_NONE, RUNE_NONE, RUNE_NONE}
	player.SetNRunes(0)
	setRunes(player.Shelf[:])
}

// mousInRune: a click in the rune bag panel: the bottom strip clears the shelf; a rune
// is looked at on a right click or added to the shelf on a left click.
func mousInRune() {
	if GameInputMode != 0 {
		return
	}
	if inplist.y < 0x12 {
		clearShelf()
	} else {
		rune := 0x14 - (inplist.y-0x12)/0xF<<2 + (inplist.x-3)/0x12
		if rune >= 0 && rune < NUM_RUNES && hasRune(rune) {
			if inplist.cmd&2 != 0 {
				var obj Object
				obj.SetItem(rune + FIRST_RUNESTONE)
				LookAt(&obj, 0)
			} else {
				if newShelf {
					clearShelf()
				}
				newShelf = false
				n := player.NRunes()
				if n == 3 {
					player.Shelf[0] = player.Shelf[1]
					player.Shelf[1] = player.Shelf[2]
					n--
				}
				player.Shelf[n] = byte(rune)
				player.SetNRunes(n + 1)
				setRunes(player.Shelf[:])
			}
		}
	}
	mouseRelease(true)
}

func notASpell() { scrollPrint("Not a spell\n") }

// tryClear: a click on an active spell icon: a right click tells how stable the spell
// is, a left click dispels it.
func tryClear(int) {
	if GameInputMode > 0 && GameInputMode < 4 {
		return
	}
	idx := 2 - inplist.x>>4
	if player.ActiveSpells() > idx {
		if inplist.cmd&2 != 0 {
			var active [4]byte
			parseAspells(active[:])
			scrollPrint(getString((int(active[idx]) + 0x180) | STR_SPELLS))
			stab := int(player.Spells[idx] >> 8)
			switch {
			case stab <= 2:
				stab = 0
			case stab <= 10:
				stab = 1
			default:
				stab = 2
			}
			gameSprint(stab + 0x89)
		} else if dispelSpell(&idx) {
			FixPlayerEquips()
		}
		mouseRelease(true)
	}
}

// tryCast casts the spell on the shelf.
func tryCast(how int) {
	if GameInputMode != 0 || panelShown {
		return
	}
	if inplist.cmd&2 != 0 && how == 0 {
		mouseRelease(true)
		return
	}
	newShelf = true
	if player.GameClock < lstime+uint32(spellDelay) {
		playEffectHere(0x15, 0x40, 0)
		return
	}
	mouseRelease(true)
	runes := int16(int(player.Shelf[0])<<10 + int(player.Shelf[1])<<5 + int(player.Shelf[2]))
	for i := 0; i < NUM_RUNE_SPELLS; i++ {
		if spells[i].runes == runes {
			playerCast(i)
			return
		}
	}
	notASpell()
}

// failSpell plays the fizzle and prints why a cast failed.
func failSpell(why int) bool {
	playEffectHere(0x16, 0x40, 0)
	gameSprint(why + 0xD2)
	return false
}

// playerCast casts spell idx after checking circle, mana and skill.
func playerCast(idx int) bool {
	level := idx/6 + 1
	cls := spells[idx].class()
	if (int(player.Level)+1)/2 < level {
		return failSpell(0)
	}
	if level*3 > int(player.PlayMana) {
		return failSpell(1)
	}
	sub := skillCheck(int(player.Skills[SKILL_CASTING])+5, level*2)
	if sub == 0 {
		return failSpell(2)
	}
	if sub == -1 {
		gameSprint(0xD6)
		cls = SPELLC_BACKFIRE
		sub = level / 2
	} else {
		sub = int(spells[idx].sub)
	}
	spellDelay = byte((level*2-int(player.Level))*4 + 0x40)
	lstime = player.GameClock
	mspellMused = level * 3
	if cls != SPELLC_MISSILE {
		player.PlayMana -= byte(mspellMused)
		mspellMused = 0
	}
	if doSpell(cls, sub, ThePlayer, ThePlayer) {
		playEffectHere(0x10, 0x40, 0)
		return true
	}
	mspellMused = 0
	return failSpell(3)
}
