package uw

import (
	"strconv"
	"time"

	"github.com/memmaker/uw1go/internal/gr"
)

// BARTER.C: bartering in conversations: the four trade slots a side on the conversation
// screen, the built-ins setup_to_barter, do_offer, do_demand, do_decline, do_judgement,
// set_likes_dislikes and the npc_inv_* family, and how a trader values an item. Objects in
// the slots are out of both inventories; endBarter drops what is left by its owner. Side 0
// is the NPC, side 1 the player.

type barterXY struct{ x, y int }

var (
	playSlotXY = [NUM_TRADE_SLOTS]barterXY{{0x94, 0xBC}, {0xA9, 0xBC}, {0x94, 0xAA}, {0xA9, 0xAA}}
	playMarkXY = [NUM_TRADE_SLOTS]barterXY{{0x91, 0xB5}, {0xBB, 0xB5}, {0x91, 0xA3}, {0xBB, 0xA3}}
	npcSlotXY  = [NUM_TRADE_SLOTS]barterXY{{0x5B, 0xBC}, {0x70, 0xBC}, {0x5B, 0xAA}, {0x70, 0xAA}}
	npcMarkXY  = [NUM_TRADE_SLOTS]barterXY{{0x58, 0xB5}, {0x82, 0xB5}, {0x58, 0xA3}, {0x82, 0xA3}}

	barterNpcIDs   [NUM_TRADE_SLOTS]int
	barterPlyIDs   [NUM_TRADE_SLOTS]int
	barterResult   bool
	npcWit         int
	npcLikes       = -1 // script memory addresses of the like and dislike lists, -1 none
	npcDislikes    = -1
	npcSelect      [NUM_TRADE_SLOTS]int
	playSelect     [NUM_TRADE_SLOTS]int
	npcAssess      int
	patience       int
	greed          int
	lastOffer      int
	npcJudgement   [NUM_TRADE_SLOTS]int
	npcAppraisals  [NUM_TRADE_SLOTS]int
	npcUndersave   [NUM_TRADE_SLOTS]int
	playJudgement  [NUM_TRADE_SLOTS]int
	playAppraisals [NUM_TRADE_SLOTS]int
	playUndersave  [NUM_TRADE_SLOTS]int
)

func wallTime() uint { return uint(time.Now().Unix()) }

// setupToBarter fills the NPC's trade slots from its inventory (at most 0x28 objects
// looked at). Once all four are full it goes round again, an item replacing the slot's
// with chance 3 in 8, until it comes back to the first item it put back.
func setupToBarter() {
	count := 0
	skippedWeapon, allFilled := false, false
	if talkingTo.HasInv() == 0 {
		generateInventory(talkingTo)
	}
	npcLink := &talkingTo.OL
	obj := ObjPtrTMem(&talkingTo.OL)
	slot := 0
	var inSlot *Object
	for obj != nil && obj != inSlot && count < 0x28 {
		count++
		next := ObjPtrTMem(&obj.QN)
		if obj.Minor() == 0 && !skippedWeapon || ComObjData[obj.Item()].Value() == 0 ||
			allFilled && rand()&7 < 5 {
			if obj.Minor() == 0 {
				skippedWeapon = true
			}
			obj = next
			continue
		}
		ObjRem(npcLink, obj)
		if barterNpcIDs[slot] != 0 {
			if inSlot == nil {
				inSlot = ObjIntTMem(barterNpcIDs[slot])
			}
			ObjAdd(&talkingTo.OL, ObjIntTMem(barterNpcIDs[slot]))
		}
		barterNpcIDs[slot] = ObjMemTPtr(obj)
		drawTradeSlot(0, slot)
		slot++
		if slot > 3 {
			slot = 0
			allFilled = true
		}
		obj = next
	}
}

// barterInit sets up the slots and the NPC's trading temper when a conversation starts;
// the seed is the NPC's object index, so the same NPC always gets the same temper.
func barterInit() {
	crit := &Creatures[talkingTo.InMajor()]
	gr.Transparency = true
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		playUndersave[s] = gr.Valloc(16, 16)
		npcUndersave[s] = gr.Valloc(16, 16)
	}
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		gr.SaveRect(playUndersave[s], playSlotXY[s].x, playSlotXY[s].y, 16, 16)
		gr.SaveRect(npcUndersave[s], npcSlotXY[s].x, npcSlotXY[s].y, 16, 16)
	}
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		barterPlyIDs[s], barterNpcIDs[s] = 0, 0
		playJudgement[s], npcJudgement[s] = -1, -1
		playAppraisals[s], npcAppraisals[s] = -1, -1
		playSelect[s], npcSelect[s] = 0, 0
		showSelection(1, s)
		showSelection(0, s)
	}
	barterResult = false
	srand(uint(ObjMemTPtr(talkingTo)))
	greed = rangePct(crit.Haggle()*6, -25, 25)
	patience = rangePct(crit.Patience(), -20, 100)
	npcAssess = rangePct((15-crit.Shrewd())*6, -25, 50)
	npcWit = rangePct(crit.Level(), -20, 20)
	lastOffer = 0
	charm := int(player.Skills[SKILL_CHARISMA])
	greed -= charm * 2
	patience += charm >> 1
	npcWit -= charm / 6
	if npcWit < 1 {
		npcWit = 1
	}
	npcLikes, npcDislikes = -1, -1
	srand(wallTime())
}

// endBarter gives back whatever is still in the slots.
func endBarter() {
	mouseHide()
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		if barterPlyIDs[s] > 0 {
			nearMobPutAt(ThePlayer, ObjIntTMem(barterPlyIDs[s]), 5, false)
			gr.RestoreRect(playUndersave[s])
		}
		if barterNpcIDs[s] > 0 {
			nearMobPutAt(talkingTo, ObjIntTMem(barterNpcIDs[s]), 5, false)
			gr.RestoreRect(npcUndersave[s])
		}
	}
	mouseShow()
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		gr.Vfree(playUndersave[s])
		gr.Vfree(npcUndersave[s])
		playUndersave[s], npcUndersave[s] = 0, 0
	}
}

// playBarter and npcBarter are the mouse handlers of the two slot areas.
func playBarter(int) {
	x, y := inplist.x+0x8B, inplist.y+0x98
	if s := slotHit(&playSlotXY, x, y); s >= 0 {
		useTradeSlot(1, s, &barterPlyIDs, &playSelect)
	}
}

func npcBarter(int) {
	x, y := inplist.x+0x52, inplist.y+0x98
	if s := slotHit(&npcSlotXY, x, y); s >= 0 {
		useTradeSlot(0, s, &barterNpcIDs, &npcSelect)
	}
}

// slotHit is which slot of a side is at x, y, or -1.
func slotHit(xy *[NUM_TRADE_SLOTS]barterXY, x, y int) int {
	for i := range xy {
		if xy[i].x <= x && xy[i].x+0x10 >= x && xy[i].y >= y && xy[i].y-0x10 <= y {
			return i
		}
	}
	return -1
}

// anySlotHit finds the slot of either side at x, y.
func anySlotHit(x, y int, side, slot *int, content **[NUM_TRADE_SLOTS]int, active **[NUM_TRADE_SLOTS]int) bool {
	if i := slotHit(&playSlotXY, x, y); i > -1 {
		*side, *slot, *content, *active = 1, i, &barterPlyIDs, &playSelect
		return true
	}
	if i := slotHit(&npcSlotXY, x, y); i > -1 {
		*side, *slot, *content, *active = 0, i, &barterNpcIDs, &npcSelect
		return true
	}
	return false
}

// convInvSpecial is a click on the player's slots during a conversation.
func convInvSpecial() {
	x, y := mouseGetxy()
	if s := slotHit(&playSlotXY, x, y); s > -1 {
		useTradeSlot(1, s, &barterPlyIDs, &playSelect)
	}
}

// useTradeSlot is a click on a trade slot: pick up what is there, put down what is held,
// toggle the selection, or look at the item. The NPC's items can only be taken after a
// successful offer or demand, and nothing can be put in its slots.
func useTradeSlot(side, slot int, content *[NUM_TRADE_SLOTS]int, active *[NUM_TRADE_SLOTS]int) {
	var moved *Object
	hadCursor := CursorObjPtr != nil
	if CursorObjPtr == nil && content[slot] == 0 {
		return
	}
	if CursorObjPtr == nil && mouseDragged(true) {
		if side == 0 && !barterResult {
			return
		}
		found := ObjIntTMem(content[slot])
		if found.IsQuant() != 0 && found.LinkF()&LINK_SPECIAL == 0 && found.LinkF() != 1 {
			if moved = AskHowMany(found); moved == nil {
				return
			}
		}
		if !EncumCheck(found) {
			if moved != nil && moved != found {
				found.SetLinkF(found.LinkF() + moved.LinkF())
				ObjRem(&found.QN, moved)
				ObjFree(moved)
			}
			gameSprint(0xFC) // "That is too heavy to take.\n"
			return
		}
		if moved != nil && moved != found {
			ObjAdd(&found.QN, moved)
		}
		hadCursor = true
		pickUpFromSlot(slot, content, moved != nil && moved != found)
		drawTradeSlot(side, slot)
		active[slot] = 0
		playJudgement[slot] = -1
		playAppraisals[slot] = -1
		showSelection(side, slot)
		if CursorObjPtr == nil {
			return
		}
		if moved != nil {
			GameInputMode = 1
			return
		}
		mouseRelease(true)
		x, y := mouseGetxy()
		if !anySlotHit(x, y, &side, &slot, &content, &active) {
			GameInputMode = 1
			DoInventoryMouse(-1)
			return
		}
	}
	if CursorObjPtr != nil {
		mouseRelease(true)
		GameInputMode = 1
		if side == 0 {
			return
		}
		putInSlot(side, slot, content)
		drawTradeSlot(side, slot)
		active[slot] = 1
		showSelection(side, slot)
		playJudgement[slot] = -1
		playAppraisals[slot] = -1
	} else if inplist.cmd&1 != 0 {
		active[slot] = b2i(active[slot] == 0)
		showSelection(side, slot)
	} else {
		lore := 1
		obj := ObjIntTMem(content[slot])
		if side == 0 {
			if skillCheck(int(player.Skills[SKILL_LORE]), 20) > 0 {
				lore++
			}
		} else {
			lore += skillCheck(int(player.Skills[SKILL_LORE]), 15)
		}
		LookAt(obj, lore)
	}
	if hadCursor && CursorObjPtr == nil {
		unforceMouseCursor(3)
		GameInputMode = 0
	}
}

// drawTradeSlot draws what is in one trade slot, with a count for a stack.
func drawTradeSlot(side, slot int) {
	mouseHide()
	gr.Transparency = true
	ids, xy, under := &barterNpcIDs, &npcSlotXY, &npcUndersave
	if side != 0 {
		ids, xy, under = &barterPlyIDs, &playSlotXY, &playUndersave
	}
	index := ids[slot]
	gr.RestoreRect(under[slot])
	if index != 0 {
		obj := ObjIntTMem(index)
		picToScreen(obj.Item(), xy[slot].x, xy[slot].y)
		qty := 0
		if obj.IsQuant() != 0 && obj.LinkF()&LINK_SPECIAL == 0 {
			qty = obj.LinkF()
		}
		gr.Transparency = false
		if qty > 1 {
			grfxLoadFont("font4x5p.sys")
			gr.ForegroundColor = 0x60
			gr.StringToScreen(strconv.Itoa(qty), xy[slot].x+3, xy[slot].y-1)
			grfxLoadFont("font5x6p.sys")
		}
	}
	mouseShow()
}

// pickUpFromSlot picks up what is in a slot; with split the rest of a divided stack stays.
func pickUpFromSlot(slot int, content *[NUM_TRADE_SLOTS]int, split bool) {
	hadCursor := CursorObjPtr != nil
	CursorObjPtr = ObjIntTMem(content[slot])
	content[slot] = 0
	if CursorObjPtr != nil {
		if split {
			content[slot] = ObjMemTPtr(ObjPtrTMem(&CursorObjPtr.QN))
		}
		mouseHide()
		if hadCursor {
			unforceMouseCursor(0)
		}
		forceMouseCursor(CursorObjPtr.Item())
		mouseShow()
	}
}

// putInSlot puts what is held into a slot, or onto a stack there.
func putInSlot(side, slot int, content *[NUM_TRADE_SLOTS]int) {
	if content[slot] == 0 {
		content[slot] = ObjMemTPtr(CursorObjPtr)
		CursorObjPtr = nil
	} else if combineToSlot(CursorObjPtr, side, slot, content) {
		CursorObjPtr = nil
	}
}

// stackable is whether obj can join other's stack.
func stackable(obj, other *Object) bool {
	return obj.IsQuant() != 0 && other.IsQuant() != 0 &&
		obj.LinkF()&LINK_SPECIAL == 0 && other.LinkF()&LINK_SPECIAL == 0 &&
		obj.Item() == other.Item() && obj.LinkF()+other.LinkF() < STACK_LIMIT
}

// combineToSlot adds a held stack to the like stack in a slot, or else swaps them; true
// when the held object was merged and freed.
func combineToSlot(obj *Object, side, slot int, content *[NUM_TRADE_SLOTS]int) bool {
	found := ObjIntTMem(content[slot])
	if found.Major() == MAJOR_MISC && found.Minor() == MINOR_CONTAINER {
		return false
	}
	merged := false
	if stackable(obj, found) {
		found.SetLinkF(found.LinkF() + obj.LinkF())
		ObjFree(obj)
		merged = true
	} else {
		held := ObjMemTPtr(CursorObjPtr)
		pickUpFromSlot(slot, content, false)
		content[slot] = held
	}
	drawTradeSlot(side, slot)
	return merged
}

// showSelection draws the cross beside a slot: colour 0x60 when selected, 0xF1 when not.
func showSelection(side, slot int) {
	mark, color := &npcMarkXY[slot], 0xF1
	if side != 0 {
		mark = &playMarkXY[slot]
		if playSelect[slot] == 1 {
			color = 0x60
		}
	} else if npcSelect[slot] == 1 {
		color = 0x60
	}
	mouseHide()
	grPixel(mark.x, mark.y, color)
	grPixel(mark.x-1, mark.y, color)
	grPixel(mark.x+1, mark.y, color)
	grPixel(mark.x, mark.y-1, color)
	grPixel(mark.x, mark.y+1, color)
	mouseShow()
}

// returnTradeObjectsToNPC puts the NPC's slot items back: all, or only the unselected.
func returnTradeObjectsToNPC(onlyUnselected bool) {
	mouseHide()
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		if barterNpcIDs[s] > 0 && (!onlyUnselected || npcSelect[s] == 0) {
			ObjAdd(&talkingTo.OL, ObjIntTMem(barterNpcIDs[s]))
			gr.RestoreRect(npcUndersave[s])
			npcSelect[s] = 0
			barterNpcIDs[s] = 0
			showSelection(0, s)
		}
	}
	mouseShow()
}

// tradeObjects hands the NPC every selected item of the player's it doesn't dislike.
func tradeObjects() {
	mouseHide()
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		if barterPlyIDs[s] > 0 && playSelect[s] != 0 && doesNpcLike(barterPlyIDs[s]) != -1 {
			npcInvAddCoins(ObjIntTMem(barterPlyIDs[s]))
			gr.RestoreRect(playUndersave[s])
			playSelect[s] = 0
			barterPlyIDs[s] = 0
			showSelection(1, s)
		}
	}
	mouseShow()
}

// npcInvAddCoins gives the NPC an object, adding coins to a pile it already has.
func npcInvAddCoins(obj *Object) {
	if obj.Item() == ITEM_GOLD_COIN {
		for other := ObjPtrTMem(&talkingTo.OL); other != nil; other = ObjPtrTMem(&other.QN) {
			if stackable(obj, other) {
				other.SetLinkF(other.LinkF() + obj.LinkF())
				ObjFree(obj)
				return
			}
		}
	}
	ObjAdd(&talkingTo.OL, obj)
}

func npcInvAdd(obj *Object) { npcInvAddCoins(obj) }

func nothingThere(content, selected *[NUM_TRADE_SLOTS]int) bool {
	for i := 0; i < NUM_TRADE_SLOTS; i++ {
		if selected[i] > 0 && content[i] > 0 {
			return false
		}
	}
	return true
}

// doOffer: the player offers the selected items for the NPC's. arg5..arg1 are the
// strings for accepted, not good enough, worse than before, out of patience, nothing
// offered. 1 when the trade is made.
func doOffer(top int) int {
	yesStr, noStr, worseStr := bargv(top, 5), bargv(top, 4), bargv(top, 3)
	tiredStr, noneStr := bargv(top, 2), bargv(top, 1)
	if patience < 0 {
		npcSay(getString(tiredStr))
		return 0
	}
	if nothingThere(&barterPlyIDs, &playSelect) || nothingThere(&barterNpcIDs, &npcSelect) {
		npcSay(getString(noneStr))
		barterResult = false
		return 0
	}
	playerValue := totalOffering(true, &barterPlyIDs, &playSelect, &playAppraisals, npcAssess)
	npcValue := totalOffering(false, &barterNpcIDs, &npcSelect, &npcAppraisals, npcAssess)
	eval := 100
	if npcValue > 0 {
		eval = int(int16((playerValue - npcValue) * 100 / npcValue))
	}
	if eval >= greed {
		npcSay(getString(yesStr))
		returnTradeObjectsToNPC(true)
		tradeObjects()
		barterResult = true
		return 1
	}
	switch {
	case lastOffer == 0:
		if eval*2 < greed {
			npcSay(getString(noStr))
			patience--
		}
	case eval < lastOffer:
		npcSay(getString(worseStr))
		patience -= 2
	case (greed-lastOffer)*3/2 > greed-eval:
		npcSay(getString(noStr))
		patience--
	}
	lastOffer = eval
	return 0
}

// doDemand: the player demands the NPC's selected items for nothing. arg2 and arg1 are
// the strings for given up and refused. 1 when the NPC gives in.
func doDemand(top int) int {
	insistStr, wontStr := bargv(top, 2), bargv(top, 1)
	crit := &Creatures[talkingTo.InMajor()]
	health := 1
	if playerdat.Avghit > 0 {
		health = 2 - (int(player.MaxHealth)-int(ThePlayer.HP))*2/int(playerdat.Avghit)
	}
	playerScore := int(player.Level) + player.Drawn() + health + int(player.Skills[SKILL_CHARISMA])/6
	demanded := totalOffering(false, &barterNpcIDs, &npcSelect, &npcAppraisals, npcAssess)
	health = 1
	if crit.Avghit > 0 {
		health = 2 - (int(crit.Avghit)-int(talkingTo.HP))*2/int(crit.Avghit)
	}
	var attitude int16
	babVarOut("npc_attitude", &attitude)
	mood := 0
	if talkingTo.Ally() != 0 {
		mood = -1
	} else if attitude < 2 {
		mood = 1
	}
	npcScore := crit.Level() + mood + health + demanded/10
	if playerScore > npcScore || talkingTo.Ally() != 0 {
		npcSay(getString(insistStr))
		returnTradeObjectsToNPC(true)
		barterResult = true
		if attitude > 0 {
			attitude--
			babVar("npc_attitude", int(attitude))
		}
		return 1
	}
	npcSay(getString(wontStr))
	returnTradeObjectsToNPC(false)
	changeCritterGoal(talkingTo, GOAL_ATTACK, 1)
	return 0
}

func doDecline() { returnTradeObjectsToNPC(false) }

// doJudgement is the player's own appraisal of the offer, spoken as the player's line.
func doJudgement() {
	skill := int(player.Skills[SKILL_APPRAISE])
	accuracy := 50 - skill*45/30
	playerValue := totalOffering(false, &barterPlyIDs, &playSelect, &playJudgement, accuracy)
	npcValue := totalOffering(false, &barterNpcIDs, &npcSelect, &npcJudgement, accuracy)
	evaluation := 100
	if npcValue > 0 {
		evaluation = int(int16((playerValue - npcValue) * 100 / npcValue))
	}
	certainty := min(skill/6, 4)
	result := 8
	for i, lim := range []int{50, 35, 25, 10, -10, -25, -35, -50} {
		if evaluation > lim {
			result = i
			break
		}
	}
	playSay(getString((certainty+3)|STR_CONV) + getString(2|STR_CONV) + getString((result+8)|STR_CONV))
}

// totalOffering is the value of a side's selected items, each valued once and cached.
func totalOffering(useLikes bool, items, selected, values *[NUM_TRADE_SLOTS]int, accuracy int) int {
	total := 0
	for i := 0; i < NUM_TRADE_SLOTS; i++ {
		if selected[i] > 0 && items[i] > 0 {
			if values[i] == -1 {
				values[i] = assessValue(useLikes, items[i], accuracy)
			}
			total += values[i]
		}
	}
	return int(int16(total))
}

// assessValue is an object's value, scaled by quantity and quality and blurred by up to
// accuracy %, the same each time for the same object, in 16 bits. With useLikes a
// disliked item is worth 0 and a liked one half as much again.
func assessValue(useLikes bool, item, accuracy int) int {
	obj := ObjIntTMem(item)
	if obj == nil {
		return 0
	}
	value := ComObjData[obj.Item()].Value()
	if useLikes {
		d := doesNpcLike(item)
		if d == -1 {
			return 0
		}
		if d != 0 {
			value = int(int16(value*3)) >> 1
		}
	}
	value = int(int16(value * objQty(obj)))
	if value > 0 {
		if q := obj.Quality(); q > 0 {
			value = int(int16(value*q)) >> 6
			if value == 0 {
				value = 1
			}
		} else {
			value = 0
		}
	}
	srand(uint(item))
	value = rangePct(value, -accuracy, accuracy)
	srand(wallTime())
	return value
}

// rangePct is base plus a random min..max % of it.
func rangePct(base, lo, hi int) int {
	return int(int16(base + int(int16(base*(lo+rand()*(hi-lo)/0x8000)))/100))
}

// playerBarterItems gives the item types and indexes of the player's selected items.
func playerBarterItems(items, indices []int) int {
	count := 0
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		if playSelect[s] != 0 {
			obj := ObjIntTMem(barterPlyIDs[s])
			indices[count] = barterPlyIDs[s]
			if obj != nil {
				items[count] = obj.Item()
			} else {
				items[count] = Level.Objs[0].Item() // FARNULLTRAP
			}
			count++
		}
	}
	return count
}

// playerBarterGive gives an object from the slots to the NPC.
func playerBarterGive(index int) {
	npcInvAdd(ObjIntTMem(index))
	mouseHide()
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		if barterPlyIDs[s] == index {
			gr.RestoreRect(playUndersave[s])
			barterPlyIDs[s] = 0
			playSelect[s] = 0
			showSelection(1, s)
		}
	}
	mouseShow()
}

// npcBarterFind finds an item (from 1000 a major and minor) in the NPC's or the player's
// inventory, containers too; its index or 0.
func npcBarterFind(item int, fromPlayer int) int {
	var head *Link
	if fromPlayer != 0 {
		head = &ThePlayer.OL
	} else {
		if talkingTo.HasInv() == 0 {
			generateInventory(talkingTo)
		}
		head = &talkingTo.OL
	}
	var major, minor, cls int
	if item > BARTER_CLASS-1 {
		major, minor, cls = (item-BARTER_CLASS)>>2, (item-BARTER_CLASS)&3, -1
	} else {
		major, minor, cls = item>>6, (item&ID_MINOR)>>4, item&0xF
	}
	return ObjMemTPtr(ObjInList(&head, true, major, minor, cls))
}

// npcHandOver moves obj out of the NPC's inventory to the player's hand, a free slot, or
// the NPC's feet: 1, 2 when dropped, 3 when even that fails.
func npcHandOver(obj *Object) int {
	ObjRem(&talkingTo.OL, obj)
	if EncumCheck(obj) {
		CursorObjPtr = obj
		mouseHide()
		forceMouseCursor(obj.Item())
		GameInputMode = 1
		mouseShow()
		return 1
	}
	for s := 0; s < NUM_TRADE_SLOTS; s++ {
		if barterPlyIDs[s] == 0 {
			barterPlyIDs[s] = ObjMemTPtr(obj)
			playSelect[s] = 0
			playJudgement[s], npcAppraisals[s] = -1, -1
			showSelection(0, s)
			drawTradeSlot(1, s)
			return 1
		}
	}
	if nearMobPutAt(talkingTo, obj, 5, false) {
		return 2
	}
	return 3
}

// npcBarterGive is take_from_npc: the NPC hands over an item of a type (from 1000, of a
// class); 0 when there is none or the player's hand is full.
func npcBarterGive(item int) int {
	if CursorObjPtr != nil {
		return 0
	}
	if talkingTo.HasInv() == 0 {
		generateInventory(talkingTo)
	}
	for obj := ObjPtrTMem(&talkingTo.OL); obj != nil; obj = ObjPtrTMem(&obj.QN) {
		if item > BARTER_CLASS-1 {
			if obj.Class() != item-BARTER_CLASS {
				continue
			}
		} else if obj.Item() != item {
			continue
		}
		return npcHandOver(obj)
	}
	return 0
}

// npcBarterGiveID is the same for one object by its index.
func npcBarterGiveID(index int) int {
	if CursorObjPtr != nil {
		return 0
	}
	for obj := ObjPtrTMem(&talkingTo.OL); obj != nil; obj = ObjPtrTMem(&obj.QN) {
		if ObjMemTPtr(obj) == index {
			return npcHandOver(obj)
		}
	}
	return 0
}

// npcInvCreate creates an item (quality 63) in the NPC's inventory, adding it to a like
// stack there; its index (0 when merged).
func npcInvCreate(item int) int {
	obj := CreateObj(item, false)
	if obj == nil {
		return 0
	}
	obj.SetQuality(0x3F)
	for other := ObjPtrTMem(&talkingTo.OL); other != nil; other = ObjPtrTMem(&other.QN) {
		if stackable(obj, other) {
			other.SetLinkF(other.LinkF() + obj.LinkF())
			ObjFree(obj)
			return 0
		}
	}
	ObjAdd(&talkingTo.OL, obj)
	return ObjMemTPtr(obj)
}

// npcInvDelete removes and frees the first object of item in the NPC's inventory.
func npcInvDelete(item int) int {
	for obj := ObjPtrTMem(&talkingTo.OL); obj != nil; obj = ObjPtrTMem(&obj.QN) {
		if obj.Item() == item {
			ObjRem(&talkingTo.OL, obj)
			ObjFree(obj)
			return 1
		}
	}
	return 0
}

// npcLikesDislikes is set_likes_dislikes(arg2 likes, arg1 dislikes): two script arrays
// ending in -1.
func npcLikesDislikes(top int) int {
	npcLikes = barg(top, 2)
	npcDislikes = barg(top, 1)
	return 1
}

// doesNpcLike is 1 when the NPC likes an object, -1 when it dislikes it or it is
// worthless, else 0. Entries of 1000 and up name a class.
func doesNpcLike(index int) int {
	obj := ObjIntTMem(index)
	if obj == nil || ComObjData[obj.Item()].Value() == 0 {
		return -1
	}
	id := obj.Item()
	cls := id>>4 + BARTER_CLASS
	liked := 0
	if npcLikes >= 0 {
		for i := 0; i < 0x800 && getmem(npcLikes+i) > -1; i++ {
			v := getmem(npcLikes + i)
			if v < BARTER_CLASS {
				if v == id {
					return 1
				}
			} else if v == cls {
				liked = 1
			}
		}
	}
	if npcDislikes >= 0 {
		for i := 0; i < 0x800 && getmem(npcDislikes+i) > -1; i++ {
			v := getmem(npcDislikes + i)
			if v < BARTER_CLASS {
				if v == id {
					return -1
				}
			} else if v == cls {
				liked = -1
			}
		}
	}
	return liked
}
