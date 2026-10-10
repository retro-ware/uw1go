package uw

import (
	"strconv"

	"github.com/memmaker/uw1go/internal/gr"
)

// CONVERSE.C: conversations. TalkTo checks the NPC will talk and switches to screen 4,
// whose start function strtConverse draws the conversation screen, sets up the trade
// slots and calls Converse: load the script, bind the built-ins, hand over the variables,
// run it and take them back. Then newscr(1) returns to the game, whose screen exit
// freeConverse puts things back.
//
// Built-ins get top, the script memory index of the argument count; arg n is the address
// at mem[top-n] (barg), its value bargv.

var (
	rightPanelSaved int
	convoPics       [17][]byte
	convoMode       int
	convoWorkspace  bool
	convoContinue   bool
	convoLineOption [10]int
	bablChoice      int
	bablOpts        [20]string
	bablDisplay     [20]string
	talkingTo       *Object
	bablOptionIds   [10]int
	bablOptCount    int

	convoSayFlag bool
	babAskHandle int
	convoAskstr  []byte
	cnvID        uint16
)

const cnvFile = "DATA\\cnv.ark"

// TalkTo starts a conversation with thing, if it will talk.
func TalkTo(thing *Object) {
	if thing.Item() == ITEM_SHRINE {
		mantraAdvance()
		return
	}
	if thing.Item() == ITEM_TMAP_C {
		if byte(w64Types[thing.Owner()]&0xFF) == TERR_PRINCESS {
			gameSprint(0x110) // "There is no reaction from the princess."
		}
		return
	}
	if thing.Major() != MAJOR_CREATURE {
		scrollPrint(getString(STR_CONV)) // "You cannot talk to that!\n"
		return
	}
	talkingTo = thing
	who := int(talkingTo.Whoami)
	subclass := talkingTo.InMajor()
	g := talkingTo.Goal()
	if who != 0x16 && who != 0x8E && who != 0xE7 &&
		(((g == GOAL_ATTACK || g == GOAL_FLEE || g == GOAL_CORNERED) && talkingTo.GTarg() == 1 ||
			talkingTo.Attitude() == ATT_HOSTILE) && talkingTo.Ally() == 0 || who == 0xFF) &&
		g != GOAL_TALK {
		scrollPrint(getString(STR_CONV | 1)) // "You get no response.\n"
		return
	}
	if who == 0 {
		cnvID = uint16(subclass + 0x100)
	} else {
		cnvID = uint16(who)
	}
	if checkArc(cnvFile, int(cnvID)) <= 0 {
		scrollPrint(getString(STR_CONV | 1))
		return
	}
	newscr(4)
}

// moveConvpic notes where a portrait landed, past its 5-byte header.
func moveConvpic(img []byte, size, which int) bool {
	if which < len(convoPics) {
		if len(img) >= 5 {
			convoPics[which] = img[5:]
		} else {
			convoPics[which] = nil
		}
	}
	return size != 0
}

// strtConverse is the conversation screen's start function.
func strtConverse() {
	updateSprites()
	convoWorkspace = true
	setNewMusic(MUSIC_MAPS)
	changeMusicMaybe()
	mouseHide()
	gr.GrSoftPageFlip()
	gr.SetTheWindow(0, 0xC7, 0x13F, 0)
	gr.CopyVisibleToHidden()
	if !gronkGr("converse", 0, -1, moveConvpic) {
		freeConverse()
		return
	}
	gr.SetTheColor(0xF1)
	gr.Rectangle(0x2A, 0xC7, 0xC2, 0x99)
	gr.Show(0x2B, 0xC7, convoPics[0], 9, 0x5E, 0, 0)
	gr.Show(0x8B, 0xC7, convoPics[0], 9, 0x5E, 0, 0)
	gr.Show(0x52, 0xBE, convoPics[1], 0x26, 0x37, 0, 0)
	gr.Show(0x8B, 0xBE, convoPics[1], 0x26, 0x37, 0, 0)
	gr.Show(0x2B, 0xBE, convoPics[2], 0x26, 0x26, 0, 0)
	gr.Show(0xC3, 0xBE, convoPics[2], 0x26, 0x26, 0, 0)
	gr.Show(0x2A, 0x97, convoPics[3], 0xA, 0xC0, 0, 0)
	gr.Show(0x2A, 0x49, convoPics[4], 0xA, 0xC0, 0, 0)
	gr.Show(0xEC, 0xC0, convoPics[5], 0x72, 0x54, 0, 0)
	gr.SetTheColor(0xF1)
	gr.UHLine(0x34, 0x98, 0xDC)
	rightPanelSaved = RightPanel
	RightPanel = 0
	invRefresh = false
	BeginInventory()
	RedispInv()
	invRefresh = true
	doPlayScroll()
	scrollClear(false)
	doNpcScroll()
	scrollClear(false)
	grfxLoadFont("font5x6p.sys")
	gr.ForegroundColor = 0x65
	gr.BackgroundColor = 0x65
	if !gronkGr("heads", player.Female()*5+player.Body(), 1, moveConvpic) {
		freeConverse()
		return
	}
	gr.Transparency = true
	gr.Show(0xC5, 0xBC, convoPics[0], 0x22, 0x22, 0, 0)
	gr.Transparency = false
	gr.StringToScreen(getString(playerNameHandle), 0x90, 0xC5)
	who := int(talkingTo.Whoami)
	if who <= 0 || !gronkGr("charhead", who-1, 1, moveConvpic) {
		gronkGr("genhead", talkingTo.InMajor(), 1, moveConvpic)
	}
	gr.Transparency = true
	gr.Show(0x2D, 0xBC, convoPics[0], 0x22, 0x22, 0, 0)
	gr.Transparency = false
	if name, ok := getName(talkingTo, false, false); ok {
		gr.StringToScreen(name, 0x30, 0xC5)
	}
	gr.GrPageFlip()
	gr.GrSoftPageFlip()
	barterInit()
	doMainScroll()
	mouseShow()
	pmouseHandled = 0
	mouseFreereign()
	Converse(int(talkingTo.Whoami), talkingTo.InMajor())
	newscr(1)
}

// freeConverse is the conversation screen's exit function.
func freeConverse() {
	if convoWorkspace {
		convoWorkspace = false
		endBarter()
	}
	seg014_1DC5_15C5()
	RightPanel = rightPanelSaved
	doMainScroll()
}

func doEscapeKey(int) {}

// Converse loads and runs the conversation cnvID names. At the end the last line stays a
// while when the player spoke last.
func Converse(who, subclass int) {
	if loadScript(cnvFile) < 0 {
		scrollPrint(getString(STR_CONV | 1))
		return
	}
	for _, b := range []struct {
		name string
		fn   any
	}{
		{"babl_menu", convChoice},
		{"babl_fmenu", convFmenu},
		{"say", npcSay},
		{"respond", playRespond},
		{"get_quest", getQuest},
		{"set_quest", setQuest},
		{"sex", sexFn},
		{"babl_ask", getInputText},
		{"print", convPrint},
		{"show_inv", convCheckInv},
		{"give_to_npc", convGiveInv},
		{"find_inv", convFindInv},
		{"take_from_npc", convTakeInv},
		{"take_id_from_npc", convTakeInvID},
		{"identify_inv", convInvName},
		{"do_offer", doOffer},
		{"do_demand", doDemand},
		{"do_decline", doDecline},
		{"do_judgement", doJudgement},
		{"end_barter", endBarter},
		{"setup_to_barter", setupToBarter},
		{"pause", convPause},
		{"set_likes_dislikes", npcLikesDislikes},
		{"do_inv_create", convInvCreate},
		{"do_inv_delete", convInvDelete},
		{"check_inv_quality", checkInvQuality},
		{"set_inv_quality", setInvQuality},
		{"count_inv", countInv},
		{"gronk_door", gronkDoor},
		{"set_attitude", setAttitude},
		{"set_race_attitude", setRaceAttitude},
		{"take_from_npc_inv", takeFromNpcInv},
		{"add_to_npc_inv", addToNpcInv},
		{"place_object", placeObject},
		{"remove_talker", removeTalker},
		{"x_skills", xSkills},
		{"x_traps", xTraps},
		{"x_obj_stuff", xObjStuff},
		{"x_obj_pos", xObjPos},
		{"find_barter", findBarter},
		{"find_barter_total", findBarterTotal},
		{"give_ptr_npc", givePtrNpc},
	} {
		babFun(b.name, b.fn)
	}
	setupConverseData(talkingTo)
	convoAskstr = make([]byte, 0xA0)
	if talkingTo.HasInv() == 0 {
		generateInventory(talkingTo)
	}
	babRun()
	wait := 0x1F4
	if updateConverseData(talkingTo) || !convoSayFlag {
		wait = 0
	}
	scrollWait(wait, false)
}

// converseEventLoop waits for a menu choice, keeping music and animation going; when
// menusActive has been cleared it reprints the options.
func converseEventLoop() {
	for convoContinue {
		loopMusicMaybe()
		if changed != 0 {
			doChanges()
		}
		if !menusActive {
			scrollWait(0x1F4, false)
			doPlayScroll()
			scrollClear(true)
			for i := 1; i < bablOptCount; i++ {
				scrollPrint(string(rune('0'+i)) + ". " + bablDisplay[i] + "\n")
			}
			doMainScroll()
			menusActive = true
		}
		inputDispatch(inplist)
	}
}

// showMenu prints the collected options numbered in the player's scroll and waits.
func showMenu() {
	doPlayScroll()
	scrollClear(true)
	for i := range convoLineOption {
		convoLineOption[i] = -1
	}
	used := 0
	for i := 1; i < bablOptCount; i++ {
		lines := scrollPrint(string(rune('0'+i)) + ". " + bablDisplay[i] + "\n")
		for row := used; row <= lines; row++ {
			if row >= 0 && row < len(convoLineOption) {
				convoLineOption[row] = i
			}
		}
		used = lines + 1
	}
	doMainScroll()
	convoContinue = true
	menusActive = true
	converseEventLoop()
}

func addMenuOption(id int) {
	if bablOptCount >= len(bablOptionIds) {
		return
	}
	bablOpts[bablOptCount] = getString(id)
	bablDisplay[bablOptCount] = convertString(bablOpts[bablOptCount])
	bablOptionIds[bablOptCount] = id
	bablOptCount++
}

// convChoice is babl_menu(arg1 array): a 0-terminated array of string ids as a numbered
// menu; returns the number chosen.
func convChoice(top int) int {
	convoMode = 1
	bablOptCount = 1
	base := barg(top, 1)
	for id := getmem(base); id != 0; id = getmem(base + bablOptCount - 1) {
		n := bablOptCount
		addMenuOption(id)
		if bablOptCount == n {
			break
		}
	}
	showMenu()
	return bablChoice
}

// convFmenu is babl_fmenu(arg1 strings, arg2 flags): only the strings whose flag is set,
// returning the chosen string's id.
func convFmenu(top int) int {
	convoMode = 1
	bablOptCount = 1
	base, vbase := barg(top, 1), barg(top, 2)
	for i := 1; ; i++ {
		id := getmem(base + i - 1)
		if id == 0 {
			break
		}
		if getmem(vbase+i-1) != 0 {
			addMenuOption(id)
		}
	}
	showMenu()
	return bablOptionIds[bablChoice]
}

// convPlayMenu is the menu's input handler: option is the key's number, or 0 for a click,
// whose row in the scroll picks the option.
func convPlayMenu(option int) {
	if convoMode == 0 {
		return
	}
	selected := option
	if option == 0 {
		y := inplist.y + 1
		row := (0x1E - y) / gr.CurFont.Height
		if row < 0 || row >= len(convoLineOption) {
			return
		}
		selected = convoLineOption[row]
	}
	if selected <= 0 || selected >= bablOptCount {
		return
	}
	convoMode = 0
	convoContinue = false
	mouseRelease(false)
	doPlayScroll()
	scrollClear(true)
	doMainScroll()
	menusActive = false
	doNpcScroll()
	playSay(bablDisplay[selected])
	doMainScroll()
	bablChoice = selected
}

// npcSay is "say": the NPC's line in the NPC's scroll.
func npcSay(s string) {
	doNpcScroll()
	scrollPrint("\\P" + s + "\n")
	doMainScroll()
	convoSayFlag = false
}

// playRespond is "respond": a line in the player's scroll.
func playRespond(s string) {
	doPlayScroll()
	scrollPrint(s + "\n")
	doMainScroll()
	convoSayFlag = true
}

// playSay is the player's chosen line, in the conversation scroll in colour 1.
func playSay(s string) {
	doNpcScroll()
	scrollPrint("\\1" + s + "\\0\n")
	doMainScroll()
	convoSayFlag = true
}

// convPrint is print(arg1 string): narration, in colour 2.
func convPrint(top int) {
	doNpcScroll()
	scrollPrint("\\2" + convertString(getString(bargv(top, 1))) + "\\0\n")
	doMainScroll()
}

// convPause is pause(arg1).
func convPause(top int) int {
	mouseRelease(false)
	scrollWait(bargv(top, 1)*0x1F4, false)
	return 1
}

// getInputText is babl_ask: up to 0x32 typed characters, as a dynamic string, the same id
// for every call in a conversation.
func getInputText() int {
	_, response := wdialog("", nil, true, 0x32)
	scrollPrint("\n")
	convoAskstr = append(convoAskstr[:0], response...)
	convoAskstr = append(convoAskstr, 0)
	if babAskHandle == 0 {
		babAskHandle = makeString(&convoAskstr, STRBLK_DYNAMIC)
	} else if !madeStringExists(babAskHandle) {
		replaceString(&convoAskstr, babAskHandle)
	}
	return babAskHandle
}

// madeStringExists is get_string giving a pointer for a made string.
func madeStringExists(id int) bool {
	block := int(uint16(id)) >> 9
	for i := 0; i < stringBlocks; i++ {
		if strBlocks[i].block == block {
			return strBlocks[i].strings[id&0x1FF] != nil
		}
	}
	return stringExists(id)
}

func objQty(obj *Object) int {
	if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
		return obj.LinkF()
	}
	return 1
}

// convCheckInv is show_inv(arg1 indices, arg2 item ids): the player's selected trade items.
func convCheckInv(top int) int {
	var ids, indices [NUM_TRADE_SLOTS]int
	count := playerBarterItems(ids[:], indices[:])
	for i := 0; i < NUM_TRADE_SLOTS; i++ {
		if count > i {
			babSetmem(barg(top, 2)+i, ids[i])
			babSetmem(barg(top, 1)+i, indices[i])
		} else {
			babSetmem(barg(top, 1)+i, 0)
			babSetmem(barg(top, 2)+i, 0)
		}
	}
	return count
}

// findBarter is find_barter(arg1 item): the player's selected trade item of that type
// (from 1000, of major and minor), or 0.
func findBarter(top int) int {
	var ids, indices [NUM_TRADE_SLOTS]int
	wanted := bargv(top, 1)
	count := playerBarterItems(ids[:], indices[:])
	for i := 0; i < count; i++ {
		if wanted < BARTER_CLASS {
			if ids[i] == wanted {
				return indices[i]
			}
		} else if ids[i]>>6 == (wanted-BARTER_CLASS)>>2 && (ids[i]&ID_MINOR)>>4 == (wanted-BARTER_CLASS)&3 {
			return indices[i]
		}
	}
	return 0
}

// findBarterTotal is find_barter_total(arg4 item, arg3 count out, arg2 indices out, arg1
// total out); a class search is never made.
func findBarterTotal(top int) int {
	var ids, indices [NUM_TRADE_SLOTS]int
	var matching [5]int
	wanted := bargv(top, 4)
	matches, total := 0, 0
	count := playerBarterItems(ids[:], indices[:])
	if wanted < BARTER_CLASS {
		for i := 0; i < count; i++ {
			if ids[i] == wanted {
				matching[matches] = indices[i]
				total += objQty(ObjIntTMem(indices[i]))
				matches++
			}
		}
	}
	babSetmem(barg(top, 3), matches)
	babSetmem(barg(top, 1), total)
	for i := 0; i < matches; i++ {
		babSetmem(barg(top, 2)+i, matching[i])
	}
	return b2i(total > 0)
}

// convGiveInv is give_to_npc(arg2 count, arg1 indices): give them all if every one is a
// selected trade item.
func convGiveInv(top int) int {
	var ids, indices [NUM_TRADE_SLOTS]int
	count := bargv(top, 2)
	have := playerBarterItems(ids[:], indices[:])
	if have < count {
		return 0
	}
	slotOf := [NUM_TRADE_SLOTS]int{-1, -1, -1, -1}
	itemOf := [NUM_TRADE_SLOTS]int{-1, -1, -1, -1}
	for i := 0; i < count; i++ {
		for j := 0; j < have; j++ {
			if getmem(barg(top, 1)+i) == indices[j] && itemOf[j] == -1 {
				slotOf[i] = j
				itemOf[j] = i
				break
			}
		}
		if slotOf[i] == -1 {
			return 0
		}
	}
	for j := 0; j < count; j++ {
		playerBarterGive(getmem(barg(top, 1) + j))
	}
	return 1
}

// givePtrNpc is give_ptr_npc(arg2 object, arg1 qty).
func givePtrNpc(top int) int {
	var ids, indices [NUM_TRADE_SLOTS]int
	index := bargv(top, 2)
	playerBarterItems(ids[:], indices[:])
	for i := 0; i < NUM_TRADE_SLOTS; i++ {
		if ids[i] != 0 && indices[i] == index {
			playerBarterGive(index)
			return 1
		}
	}
	qty := bargv(top, 1)
	obj := ObjIntTMem(index)
	if obj == nil {
		return 0
	}
	if qty < 0 || obj.IsQuant() == 0 || obj.LinkF()&LINK_SPECIAL != 0 {
		qty = -1
	}
	if !invRemoveObject(obj, qty) {
		return 0
	}
	npcInvAdd(obj)
	return 1
}

func convInvDelete(top int) int { return npcInvDelete(bargv(top, 1)) }
func convFindInv(top int) int   { return npcBarterFind(bargv(top, 2), bargv(top, 1)) }
func convTakeInv(top int) int   { return npcBarterGive(bargv(top, 1)) }
func convTakeInvID(top int) int { return npcBarterGiveID(bargv(top, 1)) }
func convInvCreate(top int) int { return npcInvCreate(bargv(top, 1)) }

// convInvName is identify_inv(arg4 object, arg3 with article, arg2 name out, arg1
// identified): the object's name as a dynamic string in arg2; returns its value to the NPC.
func convInvName(top int) int {
	index := bargv(top, 4)
	describe := bargv(top, 3) != 0
	ident := bargv(top, 1)
	value := assessValue(true, index, npcAssess)
	obj := ObjIntTMem(index)
	if obj == nil {
		return value
	}
	qty := objQty(obj)
	name := ""
	mods := ""
	var article byte
	if doMods(obj, ident, &mods) && mods != "" {
		article = mods[0]
	}
	if article != 0 && describe && qty == 1 {
		switch article {
		case 'a', 'e', 'i', 'o', 'u':
			name = "an "
		default:
			name = "a "
		}
		describe = false
	} else if qty > 1 {
		name += strconv.Itoa(qty) + " "
		describe = false
	}
	name += mods
	n, _ := getName(obj, describe, qty > 1)
	name += n
	doOf(obj, ident, &name)
	babSetmem(barg(top, 2), dynString(name))
	return value
}

func countInv(top int) int {
	obj := ObjIntTMem(bargv(top, 1))
	if obj == nil {
		return 1
	}
	return objQty(obj)
}

// checkInvQuality gives the quality (the original leaves it in AX).
func checkInvQuality(top int) int {
	obj := ObjIntTMem(bargv(top, 1))
	if obj == nil {
		return 0
	}
	return obj.Quality()
}

func setInvQuality(top int) int {
	if obj := ObjIntTMem(bargv(top, 2)); obj != nil {
		obj.SetQuality(bargv(top, 1) & 0x3F)
	}
	return 1
}
