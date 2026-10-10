package uw

// The actions of UltimaHacks' keys and rune keys (underworld-exhumed's port/ui/keys.c: UltimaHacks'
// keyAndMouseBindings in this port's names). A key that stands for a click sets what that
// click sets (the position in inplist, as the region's handler reads it) and calls the
// handler, so the game decides everything as it does for the click. Bound in initPlayer and
// AutoMap.

// keySpace: Space bashes with the weapon drawn, as the mouse buttons slash and thrust, and
// is charged while held; without a weapon it jumps (spaceJump).
func keySpace() {
	if !armed() {
		spaceJump()
		return
	}
	if pQatt == 0 {
		attackButton = 0
	}
	playerAttack(swingBash)
	attackKey = 0x39 // Space's scan code, held to charge, as UltimaHacks
}

// keyFlasks: h, as a click on the left flask, then on the right.
func keyFlasks() {
	inplist.x, inplist.y = 0, 11
	flaskInfo()
	inplist.x = flX[1] - flX[0]
	flaskInfo()
}

// keyBag: n, b scroll the inventory down (1), up.
func keyBag(how int) {
	if how == 1 {
		ScrollItemsDown()
	} else {
		ScrollItemsUp()
	}
}

// keyInteract: q looks (0), e uses (1) what is at the pointer (after UltimaHacks'
// interactAtCursor): only in the 3D view, and no use while carrying something or aiming.
func keyInteract(use int) {
	if GameInputMode != 0 && use != 0 {
		return
	}
	x, y := mouseGetxy()
	if x < m3dx || x > m3dx+m3dw || y > m3dy || y < m3dy-m3dh {
		return
	}
	inplist.x = x - m3dx // relative to the view, as pick3d reads a click
	inplist.y = y - (m3dy - m3dh)
	newPlObj = pick3d(2)
	switch {
	case newPlObj == nil: // nothing but terrain: a look describes it
		if use == 0 {
			lookNothing(2, pTxtId)
		}
	case use == 0:
		player3dlook()
	default:
		player3duse()
	}
}

// keyRune: Ctrl+Alt+letter, Backspace, Space (UltimaHacks' runeKey). A rune the player has
// goes on the shelf as a click on it in the rune bag would; one it lacks is refused.
func keyRune(ch int) {
	if ch == ' ' {
		tryCast(1)
		return
	}
	rune := -1
	if ch != KEY_BACKSPACE {
		if ch < 'a' || ch > 'y' || ch == 'x' {
			return
		}
		rune = ch - 'a'
		if ch == 'y' { // no X rune: Y is Ylem, 23
			rune = 23
		}
		if !hasRune(rune) {
			// UltimaHacks plays effect 45, which UW1 (24 effects) does not have
			return
		}
	}
	if rune < 0 {
		playEffectHere(4, 64, 0)      // as UltimaHacks: 4 clearing, 3 a rune taken
		inplist.x, inplist.y = 40, 10 // y < 0x12 clears the shelf
	} else {
		playEffectHere(3, 64, 0)
		col, row := rune&3, 5-rune>>2 // mousInRune's slot, inverted: four to a row, 20 to 23 at the top
		inplist.x = 3 + col*0x12 + 9
		inplist.y = 0x12 + row*0xF + 7
	}
	inplist.cmd &^= 2 // the left button: add, not look
	mode := GameInputMode
	GameInputMode = 0
	mousInRune()
	GameInputMode = mode
}

// mapKey: on the map, s goes up a level (1), w down (-1), c to the player's (2).
func mapKey(how int) {
	switch {
	case how == 1 && mapLevel < 0x63:
		ChangeAutoMapLevel(mapLevel + 1)
	case how == -1 && mapLevel > 1:
		ChangeAutoMapLevel(mapLevel - 1)
	case how == 2 && mapLevel != PlayerLevel:
		ChangeAutoMapLevel(PlayerLevel)
	}
}
