package uw

import "strconv"

// Port of LOOK.C (ovr122): the description printed when the player looks at an object -
// 'You see a <quality> <name> of <spell> with N full charges belonging to <race>.' - and
// what books, scrolls, keys, bones, doors, critters, gravestones and plaques add. lore says
// how much the player knows: 2 shows that an object is magical, 3 (identified) names the
// enchantment and charges.

// LookAt prints the look description of obj.
func LookAt(obj *Object, lore int) {
	if obj == nil {
		return
	}
	com := &ComObjData[obj.Item()]
	if com.Lookable() != 0 {
		if lore == 3 && talismanDesc(obj, com) {
			return
		}
		text := "You see "
		var first byte
		name := ""
		if doMods(obj, lore, &name) {
			first = name[0]
		}
		if obj.Major() == MAJOR_CREATURE {
			critterLook(obj, text)
			return
		}
		q := 0
		if obj.Quality() != 0 {
			if com.QualClass() != 3 {
				q = obj.Quality()>>4 + 1
			} else {
				q = 5
			}
		}
		qual := getString((com.QualType()*6 + q) | STR_OBJLOOK)
		if qual != "" {
			first = qual[0]
		}
		plural := false
		if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 && obj.LinkF() > 1 {
			plural = true
			first = 'x'
			text += strconv.Itoa(obj.LinkF()) + " "
		} else if first != 0 && com.QualType() != 13 {
			text += article(first)
		}
		if qual != "" {
			text += qual + " "
		}
		text += name
		n, _ := getName(obj, first == 0, plural)
		text += n
		doOf(obj, lore, &text)
		if com.CanOwn() != 0 && obj.Owner() > 0 && obj.Owner()&0x1F <= 0x1B {
			text += " belonging to"
			text += getString((obj.Owner()&0x1F + 0x172) | STR_GAME)
		}
		scrollPrint(text + ".\n")
	} else if obj.Class() == CLASS_DECAL {
		rectLook(obj, lore)
	}
	specialLook(obj, lore)
}

// article is "an " before a vowel, else "a ".
func article(c byte) string {
	switch c {
	case 'a', 'e', 'i', 'o', 'u':
		return "an "
	}
	return "a "
}

// doMods adds the word before the name: 'magical ' for an enchanted object at lore 2
// (returning true), 'cursed ' for a cursed one at lore 3.
func doMods(obj *Object, lore int, s *string) bool {
	var major, effect int
	var flag bool
	if decodeObjSpell(obj, &major, &effect, &flag) {
		if lore == 2 {
			*s += "magical "
			return true
		} else if lore == 3 && !flag && major == 9 {
			*s += "cursed "
		}
	}
	return false
}

// doOf adds ' of <enchantment>' and the charges for an identified object.
func doOf(obj *Object, lore int, s *string) bool {
	var major, effect int
	var flag bool
	alwaysDecode = true
	found := decodeObjSpell(obj, &major, &effect, &flag)
	alwaysDecode = false
	if !found || lore != 3 {
		return false
	}
	switch {
	case major == 12:
		major = 0x1C0
		if obj.Minor() > 1 {
			effect += 0x10
		}
		effect += major
	case major == 9:
		return false
	case flag && major <= 0:
		effect += 0x100
	default:
		effect += major << 4
	}
	name := getString(effect | STR_SPELLS)
	if name == "" {
		name = "UNNAMED"
	}
	*s += " of " + name
	if obj.IsQuant() == 0 {
		charges := -1
		link := &obj.OL
		if spell := ObjInList(&link, false, MAJOR_SPEC, 2, 0); spell != nil && spell.ID&ID_FLAG11 != 0 {
			charges = spell.Quality()
		}
		if charges >= 0 {
			*s += " with "
			if charges > 0 {
				*s += strconv.Itoa(charges)
			} else {
				*s += "no"
			}
			*s += " full charge"
			if charges != 1 {
				*s += "s"
			}
		}
	}
	return true
}

// bookLook reads a book or scroll.
func bookLook(obj *Object, print int) {
	if print < 1 {
		return
	}
	if obj.Item() == ITEM_MAP {
		gameSprint(0x97) // 'Enscribed upon the scroll is your map.'
		return
	}
	if obj.ID&ID_ENCHANT != 0 && obj.Major() != MAJOR_RECT {
		return
	}
	if obj.ID&ID_FLAG10 != 0 {
		runcutscene(obj.LinkF()&0x1FF + 0x100)
		return
	}
	if obj.LinkF()&0x1FF > 0xFF {
		scrollPrint(getString(obj.LinkF()&0x1FF | STR_BOOKS))
		return
	}
	text := "You read the "
	if n, ok := getName(obj, false, false); ok {
		text += n
	} else {
		text += "UNNAMED"
	}
	scrollPrint(text + "...\n")
	scrollPrint(getString(obj.LinkF()&0x1FF | STR_BOOKS))
	scrollPrint("\n")
}

// rectLook looks at a decal: the texture behind it, a bridge, a gravestone or a plaque.
func rectLook(obj *Object, look int) {
	base := 0x160
	switch obj.InClass() {
	case 14, 15:
		if look >= 0 {
			lookNothing(2, obj.Owner()+1)
		}
		if look > 0 && byte(w64Types[obj.Owner()]&0xFF) == TERR_WINDOW {
			playerLookShaft()
		}
	case 4:
		if obj.Flags() >= 2 {
			lookNothing(2, obj.Flags()+0x2F)
		} else {
			gameSprint(0xAB) // 'You see a bridge.'
		}
	case 5, 6:
		if obj.InClass() == 6 {
			base += 0x10
		}
		index := obj.Owner()
		if obj.IsQuant() != 0 {
			index = obj.LinkF() & 0x1FF
		}
		var c byte
		if obj.InClass() == 5 {
			b := readFile("DATA\\grave.dat")
			if b == nil || index >= len(b) {
				break
			}
			c = b[index]
		}
		grave := getString(index | STR_WRITING)
		// get_string gives a pointer for any string in range, empty or not
		haveGrave := stringExists(index | STR_WRITING)
		if haveGrave && c != 0 {
			scrollClear(true)
		}
		if obj.InClass() == 6 || c == 0 {
			scrollPrint(getString((base + obj.Flags()) | STR_WRITING))
		}
		if haveGrave {
			scrollPrint(fixNameString(grave, true, false))
			scrollPrint("\n")
		}
		if c != 0 {
			playerLookGrave(int(c))
		}
	}
}

// bonesLook says whose bones these are.
func bonesLook(obj *Object, print int) {
	owner := obj.Owner()
	if print == 0 || owner == 0 || owner == 0x28 || (owner >= 0x3C && owner != 0x3F) {
		return
	}
	id := 0x16
	if obj.Item() == ITEM_PILE_OF_BONES_C6 || obj.LinkF() > 1 {
		id++
	}
	gameSprint(id)
	if owner == 0x3F {
		scrollPrint("an adventurer.\n")
		return
	}
	var tmp Object
	tmp.SetItem(owner + FIRST_CREATURE)
	tmp.Whoami = 0
	n, _ := getName(&tmp, true, false)
	scrollPrint(n)
	scrollPrint(".\n")
}

// keyLook prints a key's description, if it has one.
func keyLook(obj *Object, print int) {
	if print != 0 {
		scrollPrint(getString((obj.Owner() + 0x64) | STR_OBJLOOK))
	}
}

// critterLook describes and prints a critter: 'a <attitude> <kind> named <Name>'.
func critterLook(obj *Object, s string) {
	desc := getString(obj.Item() | STR_OBJNAMES)
	who := obj.Whoami
	attitude := ""
	if who < 0xF0 || who == 0xFF {
		attitude = getString((obj.Attitude() + 0x60) | STR_OBJLOOK)
	}
	name := ""
	if who > 0 {
		name = getString((int(who) + 0x10) | STR_CONV)
	}
	upper := name != "" && name[0] >= 'A' && name[0] <= 'Z'
	if desc != "" && attitude != "" {
		s += article(attitude[0]) + attitude + " "
	}
	if desc != "" && (name == "" || upper) {
		s += fixNameString(desc, attitude == "", false)
	}
	if name != "" {
		if desc != "" && upper {
			s += " named "
		}
		s += fixNameString(name, desc == "", false)
	}
	s += ".\n"
	scrollPrint(s)
}

// specialLook prints the second line of a look for books, scrolls, keys, bones and doors.
func specialLook(obj *Object, print int) {
	minor := obj.Minor()
	switch obj.Major() {
	case MAJOR_SPEC:
		if minor == 3 {
			bookLook(obj, print)
		} else if minor == 0 {
			keyLook(obj, print)
		}
	case MAJOR_STUFF:
		if minor == 0 && obj.Item() >= ITEM_SKULL_C2 && obj.Item() <= ITEM_PILE_OF_BONES_C6 {
			bonesLook(obj, print)
		}
	case MAJOR_RECT:
		if minor == 0 && obj.InClass() < 8 && obj.Owner()&1 != 0 {
			gameSprint(0x83) // 'The door is spiked.'
		}
	}
}

// talismanDesc describes a talisman of virtue at lore 3 by its own text.
func talismanDesc(obj *Object, com *ComObj) bool {
	if com.Fate() != 10 {
		return false
	}
	which := 0
	switch obj.Item() {
	case ITEM_BOOK_136:
		which = 0
	case ITEM_TAPER:
		which = 1
	case ITEM_LIT_TAPER:
		which = 2
	case ITEM_BOTTLE_OF_WINE:
		which = 3
	case ITEM_STANDARD:
		which = 4
	case ITEM_SHINY_SHIELD:
		which = 5
	case ITEM_SHINY_CUP:
		which = 6
	case ITEM_SHINY_SWORD:
		which = 7
	case ITEM_IRON_RING:
		which = 8
	}
	gameSprint(0x104)
	gameSprint(which + 0x105)
	return true
}

// From USEITEMS.C.

// alwaysDecode makes decodeObjSpell recognise every spell object.
var alwaysDecode bool

// decodeObjSpell finds the spell obj carries: a spell object (MAJOR_SPEC minor 2) in its
// contents, or an enchanted is_quant object's own link. With ID_FLAG11 (a charged spell)
// major is the link's bits 6-8 plus 12 (-1 for 0) and effect bits 0-5; otherwise major is
// bits 4-8 and effect bits 0-3. A spell object of quality 0 goes unrecognised four times in
// ten unless alwaysDecode is set.
func decodeObjSpell(obj *Object, major, effect *int, flag *bool) bool {
	var spell *Object
	if obj.Major() == MAJOR_TRAP {
		return false
	}
	if obj.IsQuant() == 0 && obj.LinkF() > 0 {
		link := &obj.OL
		spell = ObjInList(&link, false, MAJOR_SPEC, 2, 0)
		if spell != nil && spell.Quality() == 0 && !alwaysDecode && rand()*10/0x8000 < 4 {
			return false
		}
	} else if obj.IsQuant() != 0 && obj.ID&ID_ENCHANT != 0 && obj.Major() != MAJOR_RECT {
		spell = obj
	}
	if spell == nil {
		return false
	}
	l := spell.LinkF() & 0x1FF
	*flag = spell.ID&ID_FLAG11 != 0
	if *flag {
		if *major = l >> 6; *major == 0 {
			*major = -1
		} else {
			*major += 12
		}
		*effect = l & 0x3F
	} else {
		*major = l >> 4
		*effect = l & 0xF
	}
	return true
}
