package uw

import (
	"os"
	"strings"
)

// Port of GAMEWRAP.C: the save directory and its four slots, copying a slot to and from
// the working directory SAVE0, loading and saving one level's map, and the special cases
// some levels need on arrival or departure.

const homeDir = "SAVE0\\"

// realDScheck is set after a restore or a level change.
var realDScheck bool

// initSave makes SAVE0 and empties it. The free-space check of the original is moot.
func initSave() bool {
	if err := os.MkdirAll(dosPath(strings.TrimSuffix(homeDir, "\\")), 0o755); err != nil {
		return false
	}
	clearDir(homeDir)
	return true
}

// GetLevel loads level from SAVE0's LEV.ARK: the map and objects, with the player's
// inventory kept aside meanwhile, then its texture map, its automap and its creatures.
func GetLevel(level int) int {
	SavePlayerInv("")
	if GrSq >= 0 {
		GrSq = -1
	}
	var arc Arc
	if !openArc(&arc, "SAVE0\\lev.ark") {
		return 0
	}
	ok := b2i(MapLoad(&arc, level))
	RestorePlayerInv("")
	if ok > 0 {
		TxmLoad(&arc, level)
		ClearAutoMap()
		clearPaths()
		initLevelCreatureStuff()
		if ok == 1 {
			GetAutoMapLevel(&arc, level)
		}
	}
	closeArc(&arc)
	return ok
}

// SaveLevel writes level back into SAVE0's LEV.ARK, without the player's inventory.
func SaveLevel(level int) bool {
	SavePlayerInv("")
	FreePlayerInv(&ThePlayer.OL)
	if GrSq >= 0 {
		ObjRem(&Level.Tiles[GrSq].Objects, ThePlayer)
	}
	GrSq = -1
	ThePlayer.SetMajor(0)
	var arc Arc
	ok := openArc(&arc, "SAVE0\\lev.ark")
	if ok {
		ok = MapSave(&arc, level)
		ok = ok && TxmSave(&arc, level)
		ok = ok && SaveAutoMapLevel(&arc, level)
		ok = closeArc(&arc) && ok
	}
	RestorePlayerInv("")
	return ok
}

// getSaveDescs reads SAVE1\DESC to SAVE4\DESC; bit n-1 of found is set for each slot n.
func getSaveDescs() (descs [4]string, found int) {
	for i := 0; i < 4; i++ {
		b := readFile("SAVE" + string(rune('1'+i)) + "\\desc")
		if b != nil {
			s := string(b)
			if j := strings.IndexByte(s, '\n'); j >= 0 {
				s = s[:j+1]
			}
			if len(s) > 38 {
				s = s[:38]
			}
			descs[i] = s
			found |= 1 << i
		} else {
			descs[i] = "<not used yet>"
		}
	}
	return
}

// ShowSaveRest lists the four slots' descriptions in the message scroll.
func ShowSaveRest() {
	labels := [4]string{"I- ", "II- ", "III- ", "IV- "}
	scrollClear(true)
	descs, _ := getSaveDescs()
	scrollPrint("\\6    Save Game Descriptions")
	scrollEsc = false
	for i := 0; i < 4; i++ {
		scrollPrint("\n")
		scrollPrint(labels[i])
		scrollPrint(descs[i])
	}
	scrollEsc = true
	scrollPrint("\\0")
}

// DoSaveRest saves to or restores from slot 1..4 and prints the outcome.
func DoSaveRest(restore bool, slot int) {
	descs, found := getSaveDescs()
	var msg int
	if restore {
		if found&(1<<(slot-1)) == 0 {
			msg = 1
		} else if RestoreGame(slot) {
			msg = 2
			restoredGame()
		} else {
			msg = 3
		}
	} else if d := descs[slot-1]; !SaveGame(slot, &d) {
		msg = 4
	} else {
		msg = 5
	}
	gameSprint(msg + 0xA0)
}

// quickSlot is F5's and F7's slot, SAVE5, beside the four the options panel offers.
const quickSlot = 5

// quickSave, F5, saves to the quick slot without asking for a description.
func quickSave() {
	if GameInputMode != 0 {
		gameSprint(0xA0) // "You cannot select options partway through an action."
		return
	}
	setCreaturesToSavedGame()
	scrollClear(true)
	if saveToSlot(quickSlot, "Quick save") {
		scrollPrint("Quick saved.\n")
	} else {
		gameSprint(0xA4)
	}
}

// quickLoad, F7, restores the quick slot.
func quickLoad() {
	if GameInputMode != 0 {
		gameSprint(0xA0)
		return
	}
	if readFile(slotDir(quickSlot)+"desc") == nil {
		scrollPrint("There is no quick save yet.\n")
		return
	}
	if !RestoreGame(quickSlot) {
		gameSprint(0xA3)
		return
	}
	restoredGame()
	scrollPrint("Quick loaded.\n")
}

// restoredGame redraws and resets after a restore.
func restoredGame() {
	loadWeapcm()
	displayScr()
	realDScheck = true
	initScrgr()
	newFPS(-1)
	fizUpdate = true
	editchng(0x7FFE)
}

func slotDir(slot int) string { return "SAVE" + string(rune('0'+slot)) + "\\" }

// RestoreGame restores slot 1..4 into SAVE0 and loads its level.
func RestoreGame(slot int) bool {
	gameSprint(0xA6)
	if clearDir(homeDir) {
		gameSprint(0xAA)
		if copyDir(slotDir(slot), homeDir) {
			gameSprint(0xAA)
			resetGame()
			if RestorePlayerInv(homeDir) {
				gameSprint(0xAA)
				if GetLevel(PlayerLevel) != 0 {
					doLevelHacks(PlayerLevel, 3)
					gameSprint(0xAA)
					setCreaturesFromSavedGame()
					return true
				}
			}
		}
	}
	scrollPrint("\n")
	return false
}

// SaveGame asks for a description and saves to slot 1..4.
func SaveGame(slot int, desc *string) bool {
	setCreaturesToSavedGame()
	scrollClear(true)
	gameSprint(0xA8)
	key, s := wdialog("", desc, true, 0x1E)
	if key == 0x1B {
		scrollClear(true)
		return false
	}
	*desc = s
	scrollPrint("\n")
	return saveToSlot(slot, s)
}

// saveToSlot saves the game to slot with the description s.
func saveToSlot(slot int, s string) bool {
	dir := slotDir(slot)
	gameSprint(0xA7)
	if os.MkdirAll(dosPath(strings.TrimSuffix(dir, "\\")), 0o755) != nil || !clearDir(dir) ||
		!blttodrive([]byte(s), "SAVE0\\desc") {
		scrollClear(true)
		return false
	}
	gameSprint(0xAA)
	if SavePlayerInv(homeDir) {
		gameSprint(0xAA)
		if SaveLevel(PlayerLevel) {
			gameSprint(0xAA)
			if copyDir(homeDir, dir) {
				scrollPrint("\\0")
				scrollClear(true)
				return true
			}
		}
	}
	scrollClear(true)
	return false
}

// clearDir deletes every file in dir (a path ending in a backslash).
func clearDir(dir string) bool {
	p := dosPath(strings.TrimSuffix(dir, "\\"))
	ents, err := os.ReadDir(p)
	if err != nil {
		return true
	}
	for _, e := range ents {
		if e.Type().IsRegular() {
			if os.Remove(p+"/"+e.Name()) != nil {
				return false
			}
		}
	}
	return true
}

// copyFile copies the file src to dst.
func copyFile(src, dst string) bool {
	b, err := os.ReadFile(dosPath(src))
	if err != nil {
		return false
	}
	return os.WriteFile(dosPath(dst), b, 0o644) == nil
}

// copyDir copies every file of src into dst.
func copyDir(src, dst string) bool {
	p := dosPath(strings.TrimSuffix(src, "\\"))
	ents, err := os.ReadDir(p)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if e.Type().IsRegular() && !copyFile(src+e.Name(), dst+e.Name()) {
			return false
		}
	}
	return true
}

// ChangeLevel moves the game from level from to level to.
func ChangeLevel(from, to int) bool {
	clearFightState()
	if GameInputMode == 2 && CursorObjPtr != nil {
		GameInputMode = 0
		CursorObjPtr = nil
		unforceMouseCursor(3)
	}
	doLevelHacks(from, 1)
	ok := SaveLevel(from)
	if ok {
		if ok = GetLevel(to) != 0; ok {
			doLevelHacks(to, 0)
		} else {
			return false
		}
	}
	if ok {
		realDScheck = true
	}
	return ok
}

// doLevelHacks: mode 0 arriving, 1 leaving, 3 after a restore.
func doLevelHacks(level, mode int) {
	if player.Armageddon() != 0 && mode == 0 {
		clearobj()
		return
	}
	if mode == 0 {
		initLevelCreatureStuff()
	} else if mode == 1 {
		updateAllCrittersWhilstPlayerSnoozes()
	}
	switch level {
	case LEVEL_TYBAL:
		if player.Orb() == 0 {
			if mode == 0 {
				player.SavedMana = player.MaxMana
				player.MaxMana = 0
				player.PlayMana = 0
				setMaze(player.Maze() != 0)
			} else if mode == 1 {
				player.MaxMana = player.SavedMana
				player.PlayMana = player.SavedMana >> 2
			}
		}
	case LEVEL_VOID:
		if mode == 0 {
			player.SavedMana = byte(b2i(automapEnabled))
			automapEnabled = false
		} else if mode == 1 {
			automapEnabled = player.SavedMana != 0
		} else if mode == 3 {
			automapEnabled = false
		}
	}
}
