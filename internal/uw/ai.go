package uw

// Port of AI.C (seg007_1798): critter movement and AI. moveMobile (the per-frame update)
// walks the active mobile list, and each object whose time bin has come round gets
// critterAi (critters) or PATHFIND.C's moveMeJoe (missiles and other mobile objects).
// critterAi moves the critter through the physics engine, plays out attack, casting and
// dying sequences, and otherwise calls critterMv, which notices being hit, chooses a goal
// and runs it.
//
// Goals (the goal word's low nibble): 0 and 7 stand still, 1 go home, 2 wander, 3 keep
// close to the target, 4 guard, 5 attack, 6 flee, 8 mill about near home, 9 fight back
// when cornered, 10 talk to the player, 11 flutter at random, 12 hover at home.
//
// A critter's home word (HomeX, HomeY) is the tile it stands on now; its home in the
// AI's sense, myxhome and myyhome, is kept in its quality and owner fields.

var (
	lastcombattime uint32
	crithittime    int32
	hitx, hity     int
	hitpz          int
	curBin         int
	lastbin        int
	crithit        int
	typehit        = 0xFF
)

// atkCharge is the charge of a critter's blow, by attack frame.
var atkCharge = [16]int{50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 155, 170, 185, 205, 230, 255}

func nextFrame() { meptr.SetFrame((meptr.Frame() + 1) % 4) }

// setFacing sets the critter's heading of travel and facing to head (0-255).
func setFacing(head byte) {
	meptr.Heading = head
	meptr.SetHeading(int(head >> 5))
	meptr.SetFineHead(int(head))
}

func isTybalGuard() bool {
	return PlayerLevel == LEVEL_TYBAL && player.Orb() == 0 && mycst.Race == 0x13
}

func nerve() int { return int(mycst.B1C & 0xF) }

// critDrunkwalk is goal 2, wandering, and the fallback when no path is found.
func critDrunkwalk() {
	if meptr.B157() != 0 {
		freeMyPath()
	}
	tile := mapTileAt(myxpos, myypos)
	if control == 0 {
		meptr.SetRate(1)
		return
	}
	if meptr.Attitude() == ATT_HOSTILE && rand()%2 != 0 {
		critGuard()
		return
	}
	if mycst.Flier() != 0 {
		if myzpos > 0xE {
			meptr.SetPitch(rand()%3 + 0xE)
		} else if myzpos < tile.Height()+2 {
			meptr.SetPitch(rand()%3 + 0x10)
		} else {
			meptr.SetPitch(rand()%5 + 0xE)
		}
	}
	if meptr.Seq() == SEQ_STAND {
		r := rand() % 16
		if mycst.Lazy() > r && meptr.Frame() == 3 {
			meptr.SetSeq(SEQ_WALK)
		}
	} else {
		r := rand() % 16
		if mycst.Lazy() < r && meptr.Frame() == 3 {
			meptr.SetSeq(SEQ_STAND)
		} else {
			meptr.SetSeq(SEQ_WALK)
		}
	}
	if meptr.Seq() == SEQ_WALK {
		if failed != 0 && aligned == 0 {
			setFacing(byte((int(meptr.Heading) + (rand()%2*2-1)*0x40 + 0x100) % 0x100))
			meptr.SetSpeed(0)
			return
		}
		var head byte
		if rand()%0x40 < mycst.Lazy()+8 {
			head = byte((int(meptr.Heading) + rand()%0x40 + 0xE0) % 0x100)
		} else {
			head = meptr.Heading
		}
		if aligned == 0 {
			head = critAvoidPlayer(head, 10)
		}
		setFacing(head)
	} else if mycst.Lazy() > rand()%0x80 {
		setFacing(byte((int(meptr.Heading) + rand()%0x40 + 0xE0) % 0x100))
	}
	if meptr.Seq() == SEQ_STAND {
		meptr.SetB156(1)
		meptr.SetSpeed(0)
		meptr.SetRate(6)
		if rand()%2 != 0 {
			nextFrame()
		}
	} else {
		meptr.SetB156(0)
		meptr.SetSpeed(int(mycst.Speed))
		meptr.SetRate(4)
		nextFrame()
	}
	checkOutPlayer()
}

// critFollow is goal 3: within about 1.4 tiles of the target face it with the first
// attack animation; further than 8 tiles jump to its side; in between head for it.
func critFollow() {
	if tdistsqr <= 2 {
		if control == 0 {
			return
		}
		meptr.SetSeq(SEQ_ATTACK1)
		nextFrame()
		meptr.SetSpeed(0)
		meptr.SetHeading(int(deltatotheta(int8(tdx), int8(tdy))))
		meptr.SetRate(4)
	} else if tdistsqr > 0x40 {
		dist := cSqRt(uint32(tdistsqr))
		x := int(int8(int(txpos) + (int(int8(myxpos))-int(int8(txpos)))*4/dist))
		y := int(int8(int(typos) + (int(int8(myypos))-int(int8(typos)))*4/dist))
		from := mapTileAt(myxpos, myypos)
		to := mapTileAt(x, y)
		ObjRem(&from.Objects, meptr)
		ObjAdd(&to.Objects, meptr)
		meptr.SetHomeX(x)
		meptr.SetHomeY(y)
		meptr.SetFineX(4)
		meptr.SetFineY(4)
		meptr.SetZ(to.Height() << 3)
	} else {
		if control == 0 {
			return
		}
		critHeadForLoc(int(txpos), int(typos), int(tzpos))
	}
}

// critMill is goal 8, milling about near home.
func critMill() {
	if control == 0 {
		return
	}
	if meptr.Attitude() == ATT_HOSTILE && meptr.Goal() != GOAL_GUARD {
		critterSetGoal(GOAL_GUARD, 1)
		return
	}
	dx := int(int8(myxhome - myxpos))
	dy := int(int8(myyhome - myypos))
	if dx*dx+dy*dy > mycst.Range()*mycst.Range() {
		critHeadForLoc(myxhome, myyhome, mapTileAt(myxhome, myyhome).Height())
	} else {
		critDrunkwalk()
	}
}

// critGuard is goals 0, 4 and 7, and the hostile branch of wandering.
func critGuard() {
	if control == 0 {
		return
	}
	if meptr.Attitude() == ATT_HOSTILE {
		meptr.SetGTarg(1)
		setUpTarget()
		if meptr.B190() != 0 {
			critterSetGoal(GOAL_ATTACK, 1)
			return
		}
		if meptr.B191() != 0 {
			if rand()%16 > mycst.Alert() {
				meptr.SetB191(0)
			} else {
				lookForTarget(false)
			}
		}
		if rand()%16 < mycst.Alert() {
			var x, y byte
			switch targetFound(&x, &y) {
			case ATT_HOSTILE:
				meptr.SetB190(1)
				setLoc(int(x), int(y), int(tzpos))
				critterSetGoal(GOAL_ATTACK, 1)
				return
			case ATT_MELLOW:
				meptr.SetB191(1)
				if rand()%2 == 0 {
					critHeadForLoc(int(x), int(y), int(tzpos))
					return
				}
			}
		}
	}
	switch meptr.Goal() {
	case GOAL_WANDER:
		critDrunkwalk()
	case GOAL_STAND, GOAL_STAND_7:
		meptr.SetRate(6)
		meptr.SetSpeed(0)
		meptr.SetSeq(SEQ_STAND)
		if rand()%2 != 0 {
			nextFrame()
		}
	default:
		critMill()
	}
}

func tdist() uint16 { return uint16(int(tdx)*int(tdx) + int(tdy)*int(tdy)) }

func zClose() bool { return iabs(int(int8(myzpos))-int(tzpos)) < 4 }

// critOffense is goal 5, attacking the target.
func critOffense() {
	attacked := false
	how := 4
	if control == 0 {
		return
	}
	if isTybalGuard() {
		how = 1
	}
	dist := tdist()
	dx := int(int8(myxhome - myxpos))
	dy := int(int8(myyhome - myypos))
	homedist := uint16(dx*dx + dy*dy)
	if meptr.GTarg() == 1 {
		meptr.SetAttitude(ATT_HOSTILE)
	}
	if (dist < 0x64 || myxpos == int(txpos) && myypos == int(typos)) && (zClose() || mycst.Flier() != 0) {
		critAttack(dist)
	} else if mycst.Caster() > 0 {
		if !maybeCastDefensiveSpell() && mycst.B2D&1 != 0 {
			attacked = critMagikAttack()
		}
	} else if mycst.ArmsItem(0)>>4 == CLASS_MISSILE {
		attacked = critMissileAttack()
	}
	if attacked {
		if s := meptr.Seq(); s != SEQ_FIRE && s != SEQ_CAST && s != SEQ_ATTACK1 {
			meptr.SetSeq(SEQ_COMBAT)
			meptr.SetRate(4)
			nextFrame()
			meptr.SetSpeed(0)
		}
		return
	}
	if dist > 0x100 && meptr.OldGoal() == GOAL_GUARD && meptr.B195() == 0 &&
		uint16(mycst.Range()*mycst.Range()*4) < homedist {
		meptr.SetB190(0)
		meptr.SetB191(0)
		critterSetGoal(GOAL_GUARD, 0)
		return
	}
	if mycst.B2D&1 == 0 {
		how = 1
	}
	critOffenseFindTarget(txpos, typos, how)
}

// critAttack is melee manoeuvring and starting blows.
func critAttack(dist uint16) bool {
	head := deltatotheta(int8(tdx), int8(tdy))
	meptr.SetHeading(int(head))
	meptr.SetFineHead(0)
	meptr.Heading = head << 5
	meptr.SetB156(0)
	switch {
	case dist < 0x31:
		if rand()%4 == 0 {
			head = byte((int(head) + (rand()%2*2-1)*2 + 8) % 8)
			meptr.SetSeq(SEQ_COMBAT)
			meptr.Heading = head << 5
			meptr.SetSpeed(int(mycst.Speed) * 2 / 3)
		} else {
			head = (head + 4) % 8
			meptr.SetSeq(SEQ_BACK_OFF)
			meptr.Heading = head << 5
			meptr.SetSpeed(2)
		}
	case dist > 0x51:
		meptr.SetSeq(SEQ_WALK)
		meptr.Heading = head << 5
		meptr.SetSpeed(2)
	case rand()%0x40 < int(mycst.Attr[ATTR_DEX]):
		head = byte(rand() % 8)
		meptr.SetSeq(SEQ_COMBAT)
		meptr.Heading = head << 5
		meptr.SetSpeed(1)
	default:
		meptr.SetSeq(SEQ_COMBAT)
		meptr.Heading = head << 5
		meptr.SetSpeed(0)
	}
	if mycst.Flier() != 0 {
		dz := int8(mytarget.Z() + 0xE - meptr.Z())
		if dz > 1 {
			meptr.SetPitch(0x12)
		} else if dz < -1 {
			meptr.SetPitch(0xE)
		} else {
			meptr.SetPitch(rand()%3 + 0xF)
		}
	}
	if dist <= 0x64 {
		if rand()%4 == 0 {
			r := rand() % 100
			i := 0
			for ; int(mycst.Attacks[i].Prob) <= r && i < 2; i++ {
				r -= int(mycst.Attacks[i].Prob)
			}
			meptr.SetSeq(i + 1)
			meptr.SetRate(4)
			meptr.SetFrame(0)
			return true
		} else if meptr.AtkFrame() < 0xF {
			meptr.SetAtkFrame(meptr.AtkFrame() + 1)
		}
	}
	meptr.SetRate(4)
	nextFrame()
	return true
}

// critOffenseFindTarget moves towards the target's square (x, y), standing off how tiles.
func critOffenseFindTarget(x, y byte, how int) {
	if (meptr.DestX() != int(x) || meptr.DestY() != int(y)) && rand()%8 == 0 {
		switch targetFound(&x, &y) {
		case 0:
			setLoc(int(x), int(y), int(tzpos))
		case 1:
			meptr.SetB190(0)
			meptr.SetB191(0)
			critterDiscardGoal()
			return
		case 2:
			if rand()%2 == 0 {
				meptr.SetB190(0)
				meptr.SetB191(1)
				critterDiscardGoal()
				return
			}
			setLoc(int(x), int(y), int(tzpos))
		}
	}
	if int(x) == myxpos && int(y) == myypos && zClose() {
		return
	}
	if how > 1 && how*how < int(tdistsqr) || uint32(how*how*8*8) < tdisttsqr || how <= 1 && !zClose() {
		critHeadForLoc(int(x), int(y), int(tzpos))
		if meptr.B186() != 0 {
			critterDiscardGoal()
			meptr.SetB191(0)
		}
	}
}

// maybeCastDefensiveSpell casts the creature's third spell with chance caster/256.
func maybeCastDefensiveSpell() bool {
	if mycst.Spells[2] != 0xFF && rand()%0x100 < mycst.Caster() && !antiMagic(myxpos, myypos) &&
		(PlayerLevel != LEVEL_TYBAL || player.Orb() != 0 || mycst.Race != 0x13) {
		meptr.SetSpeed(0)
		meptr.SetSeq(SEQ_CAST)
		meptr.SetCast(3)
		meptr.SetFrame(0)
		return true
	}
	return false
}

func canSeeTarget() bool {
	return lineOfSight(int(myxpost), int(myypost), meptr.Z()+ComObjData[meptr.Item()].Height(),
		int(txpost), int(typost), mytarget.Z()+ComObjData[mytarget.Item()].Height())
}

// critMagikAttack is a spell attack; true if one was possible.
func critMagikAttack() bool {
	if antiMagic(myxpos, myypos) || isTybalGuard() {
		return false
	}
	if tdistsqr < 0x40 && !antiMagic(myxpos, myypos) && canSeeTarget() && lookForTarget(true) {
		if mycst.Caster() > rand()%0x80 {
			meptr.SetSpeed(0)
			meptr.SetSeq(SEQ_CAST)
			if rand()%16 < 0xB {
				meptr.SetCast(1)
			} else {
				meptr.SetCast(2)
			}
			meptr.SetFrame(0)
		}
		return true
	}
	return false
}

// critMissileAttack is a missile attack; true if one was possible.
func critMissileAttack() bool {
	if tdistsqr < 0x10 && canSeeTarget() && lookForTarget(true) {
		if rand()%0xC0 <= int(mycst.Attr[ATTR_DEX]) {
			meptr.SetSpeed(0)
			meptr.SetSeq(SEQ_FIRE)
			meptr.SetFrame(0)
		}
		return true
	}
	return false
}

func standInCombat(head byte) {
	meptr.SetSpeed(0)
	setHtx(head)
	meptr.SetRate(4)
	meptr.SetSeq(SEQ_COMBAT)
	nextFrame()
}

// critDefense is goal 9, cornered.
func critDefense() {
	if control == 0 {
		return
	}
	dist := tdist()
	if dist < 0x90 || int(txpos) == myxpos && myypos == int(typos) {
		critAttack(dist)
		return
	}
	if tdistsqr > 4 {
		if maybeCastDefensiveSpell() {
			return
		}
		if mycst.Caster() > 0 {
			critMagikAttack()
		} else if mycst.ArmsItem(0)>>4 == CLASS_MISSILE {
			critMissileAttack()
		} else {
			critFlee()
		}
	} else {
		standInCombat(deltatotheta(int8(tdx), int8(tdy)))
	}
}

// critFlee is goal 6, fleeing.
func critFlee() {
	if control == 0 {
		return
	}
	head := deltatotheta(int8(tdx), int8(tdy))
	dz := int(int8(mytarget.Z() - meptr.Z()))
	if mycst.Flier() != 0 {
		if meptr.Z() > 0x6E {
			meptr.SetPitch(rand()%5 + 0xD)
		} else {
			meptr.SetPitch(rand()%5 + 0xF)
		}
	}
	if tdistsqr <= 3 && iabs(dz) < 0x10 {
		if rand()%0x100 < nerve()>>3 || failed != 0 && aligned == 0 {
			meptr.SetB194(1)
			critterSetGoal(GOAL_CORNERED, meptr.GTarg())
			return
		}
		meptr.Heading = (head + 4) % 8 << 5
		meptr.SetHeading(int(head))
		meptr.SetFineHead(0)
		meptr.SetSeq(SEQ_BACK_OFF)
		nextFrame()
		meptr.SetSpeed((int(mycst.Speed) + 1) / 2)
		return
	}
	var newh byte
	if failed != 0 && aligned == 0 {
		if tdistsqr < 9 {
			if meptr.Goal() == GOAL_CORNERED {
				standInCombat(head)
				return
			}
			meptr.SetB194(1)
			critterSetGoal(GOAL_CORNERED, meptr.GTarg())
			return
		}
		newh = byte(((int(meptr.Heading>>5)+(rand()%2*2-1)*2+8)%8)<<5 + rand()%0x20)
	} else {
		if maybeCastDefensiveSpell() {
			return
		}
		if rand()%0x40 < mycst.Lazy()+8 {
			newh = byte((int(meptr.Heading) + rand()%0x40 + 0xE0) % 0x100)
		} else {
			newh = meptr.Heading
		}
		if aligned == 0 {
			newh = critAvoidPlayer(newh, 0x18)
		}
	}
	setFacing(newh)
	if tdistsqr < 0x40 {
		meptr.SetSpeed(int(mycst.Run))
	} else {
		meptr.SetSpeed(int(mycst.Speed))
	}
	meptr.SetSeq(SEQ_WALK)
	nextFrame()
	meptr.SetRate(4)
}

// critAvoidPlayer steers a heading (0-255) away from the player when the player is
// within how fine units.
func critAvoidPlayer(heading byte, how int) byte {
	plyr := ObjIntTMem(1)
	px := plyr.HomeX()<<3 + plyr.FineX()
	py := plyr.HomeY()<<3 + plyr.FineY()
	dx := int(int16(px - int(myxpost)))
	dy := int(int16(py - int(myypost)))
	dist := uint16(dx*dx + dy*dy)
	if how*how > int(dist) {
		th := (deltatotheta(int8(dx), int8(dy)) + 4) % 8 << 5
		diff := th - heading
		switch {
		case diff < 0x40 || diff > 0xC0:
			return heading
		case diff < 0x60:
			return th + 0xE0
		case diff < 0x80:
			return heading + 0x20
		case diff > 0xA0:
			return th + 0x20
		default:
			return heading + 0xE0
		}
	}
	return heading
}

func standIdle() {
	meptr.SetRate(6)
	meptr.SetSpeed(0)
	meptr.SetSeq(SEQ_STAND)
	if rand()%2 != 0 {
		nextFrame()
	}
}

// critTalk is goal 10, wanting to talk.
func critTalk() {
	if control == 0 {
		return
	}
	meptr.SetGTarg(1)
	setUpTarget()
	dist := tdist()
	var x, y byte
	found := targetFound(&x, &y)
	if found != 1 && dist < 0x190 {
		head := deltatotheta(int8(tdx), int8(tdy))
		meptr.SetSpeed(0)
		meptr.SetSeq(SEQ_STAND)
		meptr.SetRate(6)
		if rand()%2 != 0 {
			nextFrame()
		}
		setHtx(head)
		if dist < 0x90 {
			rel := (ThePlayer.HeadingBits() + 8 - int(head)) & 7
			if rel >= 3 && rel <= 5 {
				pmouseHandled = 0
				mouseFreereign()
				TalkTo(meptr)
			}
		}
	} else {
		standIdle()
	}
}

// checkOutPlayer: a standing critter, or any critter while the player has a weapon
// drawn, faces the player and stands when the player is within 1.5 tiles.
func checkOutPlayer() {
	if meptr.Speed() <= 0 || player.Drawn() != 0 {
		meptr.SetGTarg(1)
		setUpTarget()
		if tdist() < 0x90 {
			head := deltatotheta(int8(tdx), int8(tdy))
			meptr.SetSpeed(0)
			meptr.SetSeq(SEQ_STAND)
			meptr.SetRate(6)
			if rand()%2 != 0 {
				nextFrame()
			}
			meptr.SetHeading(int(head))
			meptr.SetFineHead(0)
		}
	}
}

// critHover is goal 12: go home and stand there.
func critHover() {
	if control == 0 {
		return
	}
	dx := int8(myxhome - myxpos)
	dy := int8(myyhome - myypos)
	if meptr.Attitude() == ATT_HOSTILE && meptr.Goal() != GOAL_GUARD {
		critterSetGoal(GOAL_GUARD, 1)
	} else if dx != 0 || dy != 0 {
		critHeadForLoc(myxhome, myyhome, mapTileAt(myxhome, myyhome).Height())
	} else {
		standIdle()
	}
}

// targetFound says whether the current critter perceives its target: 0 seen or heard
// close, 2 heard faintly, 1 not perceived. Sets *x and *y to the target's tile.
func targetFound(x, y *byte) int {
	*x = txpos
	*y = typos
	dx := int(int8(int(txpos) - myxpos))
	dy := int(int8(int(typos) - myypos))
	dist := dx*dx + dy*dy
	tc := &Creatures[mytarget.InMajor()]
	h := mycst.Hearing() * tc.Noise() / 16
	heard := h * h
	if heard/4 > dist {
		return 0
	}
	s := mycst.Sight() * tc.Visibility() / 16
	if dist <= s*s {
		th := int(deltatotheta(int8(dx), int8(dy)))
		rel := (th - meptr.HeadingBits() + 8) % 8
		if (rel == 0 || rel == 1 || rel == 7) && canSeeTarget() {
			meptr.SetB190(1)
			return 0
		}
	}
	if heard*4 > dist {
		return 2
	}
	meptr.SetB190(0)
	return 1
}

// lookForTarget turns towards the target: smoothly with how (true once facing it), or
// by one eighth (true only if it already faced it).
func lookForTarget(how bool) bool {
	dx := int8(int(txpost) - int(myxpost))
	dy := int8(int(typost) - int(myypost))
	th := int(deltatotheta(dx, dy))
	rel := (th - meptr.HeadingBits() + 8) % 8
	if how {
		return turnRealFine(dx, dy)
	}
	if rel == 0 {
		return true
	}
	if rel <= 4 {
		meptr.SetHeading(meptr.HeadingBits() + 1)
	} else {
		meptr.SetHeading(meptr.HeadingBits() - 1)
	}
	return false
}

// constrainMovement limits how far a critter turns in one AI step.
func constrainMovement() {
	oldh := meptr.Heading
	newf := byte(meptr.HeadingBits()<<5 + meptr.FineHead())
	diff := newf - myoldfacing
	if diff >= 0x20 && diff <= 0xE0 {
		if diff < 0x80 {
			newf = myoldfacing + 0x20
		} else {
			newf = myoldfacing + 0xE0
		}
	}
	meptr.SetHeading(int(newf >> 5))
	meptr.SetFineHead(int(newf))
	if aligned != 0 {
		meptr.Heading = myoldheading
	} else if myoldspeed > 1 && meptr.Speed() > 1 {
		diff = oldh - myoldheading
		switch {
		case diff < 0x20 || diff > 0xE0:
			meptr.Heading = oldh
		case diff < 0x40:
			meptr.Heading = myoldheading + 0x20
		case diff > 0xC0:
			meptr.Heading = myoldheading + 0xE0
		default:
			meptr.SetSpeed(0)
			meptr.Heading = myoldheading
		}
	}
}

// turnRealFine turns the critter's facing towards (dx, dy) by at most 45 degrees; true
// once it faces that way.
func turnRealFine(dx, dy int8) bool {
	cur := byte(meptr.HeadingBits()<<5 + meptr.FineHead())
	d2 := int32(int16(int(tdx)*int(tdx) + int(tdy)*int(tdy)))
	dist := int32(uint16(cSqRt(uint32(d2))))
	ly, lx := int32(dx), int32(dy)
	if dist == 0 {
		return true
	}
	var sy, sx int16
	switch {
	case lx == dist:
		sy = 0x7FFF
	case -lx == dist:
		sy = -0x8000
	default:
		sy = int16((lx << 15) / dist)
	}
	switch {
	case ly == dist:
		sx = 0x7FFF
	case -ly == dist:
		sx = -0x8000
	default:
		sx = int16((ly << 15) / dist)
	}
	ang := uint16(cAtan2(int(sy), int(sx)))
	want := byte(0x40 - int(ang>>8))
	diff := want - cur
	var newf byte
	done := false
	switch {
	case diff < 0x20 || diff > 0xE0:
		newf = want
		done = true
	case diff < 0x80:
		newf = cur + 0x20
	default:
		newf = cur + 0xE0
	}
	meptr.SetHeading(int(newf >> 5))
	meptr.SetFineHead(int(newf))
	return done
}

// computeTrzOrTryRather is the vertical aim for a critter's spell or missile.
func computeTrzOrTryRather(speed int, flag bool) int8 {
	setUpTarget()
	dz := mytarget.Z() - meptr.Z()
	dist := cSqRt(tdisttsqr)
	if dist == 0 {
		if dz > 0 {
			return 0xF
		}
		return -0xF
	}
	trz := (dz << 2) / dist
	if trz > 0xF {
		trz = 0xF
	}
	if trz < -0xF {
		trz = -0xF
	}
	if !flag || speed == 0 {
		return int8(trz)
	}
	return int8(trz + dist*3/speed)
}

// loadMyVars loads the critter's position, home, old heading and speed and height.
func loadMyVars() {
	myxpos = meptr.HomeX()
	myypos = meptr.HomeY()
	myzpos = meptr.Z() >> 3
	myxpost = int16(myxpos<<3 + meptr.FineX())
	myypost = int16(myypos<<3 + meptr.FineY())
	myxhome = meptr.Quality()
	myyhome = meptr.Owner()
	myoldheading = meptr.Heading
	myoldfacing = byte(meptr.HeadingBits()<<5 + meptr.FineHead())
	myoldspeed = byte(meptr.Speed())
	myheight = byte(ComObjData[meptr.Item()].Height())
}

func pickPhysics() {
	switch {
	case mycst.Flier() != 0:
		pnAct, tpAct = &CN2, &CT2
	case mycst.Swims() != 0:
		pnAct, tpAct = &CN4, &CT4
	default:
		pnAct, tpAct = &CN1, &CT1
	}
}

// setCritterVars loads the per-critter globals for obj and chooses its physics record
// and handler.
func setCritterVars(obj *Object) {
	meptr = obj
	myid = byte(ObjMemTPtr(meptr))
	mycst = &Creatures[meptr.InMajor()]
	loadMyVars()
	pickPhysics()
}

// critterAi is one AI step for meptr; false if the critter removed itself.
func critterAi() bool {
	myid = byte(ObjMemTPtr(meptr))
	mycst = &Creatures[meptr.InMajor()]
	myxpos = meptr.HomeX()
	myypos = meptr.HomeY()
	plyr := ObjIntTMem(1)
	px := int(int8(plyr.HomeX()))
	py := int(int8(plyr.HomeY()))
	ex, ey := int(int8(myxpos))-lastXeye, int(int8(myypos))-lastYeye
	eyedist := ex*ex + ey*ey
	qx, qy := int(int8(myxpos))-px, int(int8(myypos))-py
	pdist := qx*qx + qy*qy
	if eyedist > 0x64 && pdist > 0x64 && meptr.Goal() != GOAL_FOLLOW {
		meptr.SetBin((meptr.Bin() + 8) % 16)
		return true
	}
	pickPhysics()
	flameproof := ComObjData[meptr.Item()].Resist()&8 != 0
	if flameproof {
		tpAct.mask &^= 0x20
		tpAct.w6 &^= 0x20
		tpAct.ignore |= 0x20
	}
	failed, control, hitwall, aligned, didhitobj, hitadoor, dontchangedz = 0, 1, 0, 0, 0, 0, 0
	if meptr.Seq() != SEQ_WALK && meptr.Seq() != SEQ_STAND && meptr.B157() != 0 {
		freeMyPath()
	}
	if meptr.B156() == 0 || meptr.Speed() != 0 || meptr.Pitch() != 0x10 {
		getPhysData(meptr, pnAct)
		oldh := meptr.Heading
		critTerr = int16(getTerrain(meptr))
		doCritPhys(pnAct, tpAct)
		XP = meptr.HomeX()
		YP = meptr.HomeY()
		setPhysData(meptr, pnAct)
		if meptr.Heading != oldh {
			aligned = 1
		}
	}
	if flameproof {
		tpAct.mask |= 0x20
		tpAct.w6 |= 0x20
		tpAct.ignore &^= 0x20
	}
	loadMyVars()
	switch seq := meptr.Seq(); {
	case meptr.Goal() == GOAL_FLUTTER || meptr.Goal() == GOAL_FOLLOW:
		critterMv()
	case seq == SEQ_DYING:
		if meptr.Frame() == 3 {
			deathCheck(meptr, true)
			XP = meptr.HomeX()
			YP = meptr.HomeY()
			ObjRem(&mapTileAt(XP, YP).Objects, meptr)
			generateInventory(meptr)
			buildCorpse(meptr, mycst.Corpse(), mycst.Remains())
			dropSomeObjects(meptr)
			ObjFree(meptr)
			return false
		}
		meptr.SetFrame(meptr.Frame() + 1)
	case seq >= SEQ_ATTACK1 && seq <= SEQ_ATTACK3:
		if meptr.Frame() == 0 && meptr.GTarg() == 1 {
			if m := getCurrentMusic(); m < MUSIC_FOE_HURT || m > MUSIC_DANGER {
				setNewMusic(MUSIC_COMBAT)
			}
			lastcombattime = GameTime()
		}
		if meptr.Frame() == 4 {
			critterAttack(meptr, rand()%9, atkCharge[meptr.AtkFrame()], seq-1, int(mycst.B0F))
			meptr.SetSeq(SEQ_COMBAT)
			meptr.SetFrame(0)
			meptr.SetAtkFrame(0)
		} else {
			meptr.SetFrame(meptr.Frame() + 1)
		}
	case seq == SEQ_CAST && meptr.Cast() != 0:
		if meptr.Frame() == 4 {
			missileTry = int(computeTrzOrTryRather(0x1E, false))
			cast(int(mycst.Spells[meptr.Cast()-1]), meptr, nil)
			meptr.SetSeq(SEQ_COMBAT)
			meptr.SetFrame(0)
			meptr.SetCast(0)
		} else {
			meptr.SetFrame(meptr.Frame() + 1)
		}
	case seq == SEQ_FIRE:
		if meptr.Frame() == 4 {
			typ := mycst.ArmsItem(0) & ID_INCLASS
			missileTry = int(computeTrzOrTryRather(int(Missile[typ].Type), true))
			critterFire(meptr, typ, int(Missile[typ].Type))
			meptr.SetSeq(SEQ_COMBAT)
			meptr.SetFrame(0)
		} else {
			meptr.SetFrame(meptr.Frame() + 1)
		}
	default:
		critterMv()
	}
	meptr.SetBin((meptr.Bin() + meptr.Rate()) % 16)
	return true
}

// critterMv decides and runs the current critter's goal.
func critterMv() {
	targeted := false
	didmove = 0
	meptr.SetB185(0)
	meptr.SetB156(0)
	if meptr.Goal() != GOAL_FLUTTER {
		critterReact(&targeted)
	}
	switch meptr.Goal() {
	case GOAL_STAND, GOAL_STAND_7, GOAL_GUARD:
		critGuard()
	case GOAL_GO_HOME:
		critHeadForLoc(myxhome, myyhome, mapTileAt(myxhome, myyhome).Height())
	case GOAL_WANDER:
		critDrunkwalk()
	case GOAL_FOLLOW:
		if !targeted && !setUpTarget() {
			critterDiscardGoal()
		} else {
			critFollow()
		}
	case GOAL_ATTACK:
		if !targeted && !setUpTarget() {
			critterDiscardGoal()
		} else {
			critOffense()
		}
	case GOAL_FLEE:
		if !targeted && !setUpTarget() {
			critterDiscardGoal()
		} else {
			critFlee()
		}
	case GOAL_MILL:
		critMill()
	case GOAL_CORNERED:
		if !targeted && !setUpTarget() {
			critterDiscardGoal()
		} else {
			critDefense()
		}
	case GOAL_TALK:
		critTalk()
	case GOAL_FLUTTER:
		meptr.SetRate(4)
		meptr.SetSpeed(rand() % 2)
		meptr.Heading = byte(rand() % 0x100)
		meptr.SetPitch(rand()%3 + 0xF)
		nextFrame()
		meptr.SetB156(1)
	case GOAL_HOVER:
		critHover()
	default:
		meptr.SetRate(7)
	}
	constrainMovement()
}

// critterReact is critter_mv's first part: footsteps, joining a fight, and answering
// being hit.
func critterReact(targeted *bool) {
	if meptr.Seq() == SEQ_WALK && meptr.Frame()&1 == 1 {
		x, y := int(myxpost), int(myypost)
		switch mycst.Sound() {
		case 1:
			if meptr.Frame() == 1 {
				playEffect(1, x, y, 0)
			} else if meptr.Frame() == 3 {
				playEffect(2, x, y, 0)
			}
		case 3:
			playEffect(5, x, y, 0)
		case 2:
			playEffect(0x17, x, y, 0)
		case 5:
			playEffect(0xD, x, y, 0)
		case 4:
			playEffect(0xE, x, y, 0)
		}
	}
	if mycst.B0A&2 != 0 {
		return
	}
	ally := meptr.Ally() != 0
	if (!ally && crithit != int(myid) && int(mycst.Race) == typehit && meptr.Loner() == 0 || ally) &&
		uint32(crithittime)+0x200 > player.GameClock &&
		iabs(myxpos-hitx)+iabs(myypos-hity) < mycst.Hearing() {
		meptr.SetAttitude(ATT_HOSTILE)
		meptr.SetB190(1)
		if meptr.Goal() != GOAL_CORNERED && meptr.Goal() != GOAL_FLEE {
			if ally {
				critterSetGoal(GOAL_ATTACK, crithit)
			} else {
				critterSetGoal(GOAL_ATTACK, 1)
			}
			setLoc(hitx, hity, hitpz)
		}
	}
	lh := int(meptr.LastHit)
	if lh > 0 && (lh == 1 && !ally || ally || ObjIntTMem(lh).Ally() != 0) {
		if lh != meptr.GTarg() {
			meptr.SetGTarg(lh)
		}
		if !setUpTarget() {
			return
		}
		*targeted = true
		if lh == 1 {
			meptr.SetAttitude(ATT_HOSTILE)
			setLoc(ThePlayer.HomeX(), ThePlayer.HomeY(), ThePlayer.Z()>>3)
			meptr.SetB190(1)
		}
		switch {
		case tdistsqr > 2 && (mycst.B2D&1 == 0 || antiMagic(myxpos, myypos)):
			meptr.SetB195(1)
			critterSetGoal(GOAL_ATTACK, lh)
		case meptr.B195() != 0:
			critterSetGoal(GOAL_ATTACK, lh)
		case meptr.B194() == 0 && shouldIFlee(int(mycst.Avghit), int(meptr.HP), nerve(), meptr.Damage()):
			critterSetGoal(GOAL_FLEE, lh)
		case meptr.B194() != 0:
			meptr.SetB194(1)
			critterSetGoal(GOAL_CORNERED, lh)
		default:
			critterSetGoal(GOAL_ATTACK, lh)
		}
		meptr.LastHit = 0
		meptr.B11 = 0
	}
}

// setUpTarget loads the target globals from the critter's goal target; false if the
// target is dead.
func setUpTarget() bool {
	mytarget = ObjIntTMem(meptr.GTarg())
	if mytarget.HP == 0 {
		return false
	}
	txpos = byte(mytarget.HomeX())
	typos = byte(mytarget.HomeY())
	tzpos = int8(mytarget.Z() >> 3)
	txpost = uint16(int(txpos)<<3 + mytarget.FineX())
	typost = uint16(int(typos)<<3 + mytarget.FineY())
	tdx = int16(txpost) - myxpost
	tdy = int16(typost) - myypost
	ax, ay := int(txpos)-myxpos, int(typos)-myypos
	tdistsqr = uint16(ax*ax + ay*ay)
	bx, by := int(txpost-uint16(myxpost)), int(typost-uint16(myypost))
	tdisttsqr = uint32(uint16(bx*bx + by*by))
	return true
}

// shouldIFlee says whether a hurt critter flees.
func shouldIFlee(maxhp, hp, nerve, damage int) bool {
	if maxhp*3>>2 < hp {
		return false
	}
	if maxhp>>3 > hp {
		return false
	}
	if maxhp>>1 < damage {
		return true
	}
	if maxhp == 0 {
		return false
	}
	r := (hp<<4)/maxhp + rand()%4
	return 0xF-nerve >= r
}

// acceptableDanger is how much danger a path may cross for this critter.
func acceptableDanger() int {
	if meptr.Attitude() != ATT_HOSTILE || mycst.Avghit == 0 || meptr.DoorDir() != 0 {
		return 0
	}
	if PlayerLevel == 6 && meptr.Whoami == 0x16 {
		return 0
	}
	return int(byte(int(meptr.HP)<<2/int(mycst.Avghit) + nerve()/4))
}

// critterSetGoal gives the current critter a goal and goal target.
func critterSetGoal(goal, gtarg int) {
	if meptr.Goal() == GOAL_GUARD {
		meptr.SetOldGoal(meptr.Goal())
	}
	meptr.SetGoal(goal)
	meptr.SetGTarg(gtarg)
}

// critterDiscardGoal drops the current goal: back to the remembered one, or wander.
func critterDiscardGoal() {
	if meptr.OldGoal() != 0 {
		meptr.SetGoal(meptr.OldGoal())
		meptr.SetGTarg(1)
		meptr.SetOldGoal(GOAL_STAND)
	} else {
		meptr.SetGoal(GOAL_WANDER)
		meptr.SetGTarg(0)
	}
}

// goIntoDyingSequence starts obj's dying sequence unless a scripted death refuses.
func goIntoDyingSequence(obj *Object) bool {
	if obj.Whoami == 0 || deathCheck(obj, false) {
		obj.SetSeq(SEQ_DYING)
		obj.SetFrame(0)
		obj.SetRate(4)
		obj.HP = 0
		return true
	}
	return false
}

// critDie kills obj; true if it started dying here.
func critDie(obj *Object) bool {
	if obj.Seq() != SEQ_DYING {
		if !goIntoDyingSequence(obj) {
			return false
		}
		if mycst != nil && mycst.Death() == 1 {
			playEffect(6, int(myxpost), int(myypost), 0)
		}
		return true
	}
	return false
}

// damageCritter damages a critter (or the player); true if it died of it.
func damageCritter(obj *Object, damage int, from *Object) bool {
	dmg := byte(damage)
	cr := &Creatures[obj.Minor()<<4+obj.InClass()]
	obj.SetDamage(obj.Damage() + int(dmg))
	who := 0
	if from != nil {
		if from.Major() != MAJOR_CREATURE {
			who = int(from.LastHit)
		} else if m := ObjMemTPtr(from); m < NUM_MOBILE {
			who = m
		}
	}
	if who != 0 {
		obj.LastHit = byte(who)
	}
	if who == 1 && obj.Loner() == 0 {
		typehit = int(cr.Race)
		crithit = ObjMemTPtr(obj)
		hitx = obj.HomeX()
		hity = obj.HomeY()
		hitpz = obj.Z() >> 3
		crithittime = int32(player.GameClock)
	}
	if obj.HP <= dmg {
		obj.HP = 0
		if critDie(obj) {
			if who == 1 {
				playerKilledA(obj)
			}
			return true
		}
	} else {
		obj.HP -= dmg
		if obj == ThePlayer {
			panelCheckHpmp()
		}
	}
	if who == 1 {
		if int(obj.HP)<<6/(int(cr.Avghit)+1) < 0x10 {
			setNewMusic(MUSIC_FOE_HURT)
		} else {
			setNewMusic(MUSIC_COMBAT)
		}
		lastcombattime = GameTime()
	} else if obj == ThePlayer && who != 0 {
		if int(ThePlayer.HP)<<6/(int(playerdat.Avghit)+1) < 0x10 {
			setNewMusic(MUSIC_DANGER)
		} else {
			setNewMusic(MUSIC_COMBAT)
		}
		lastcombattime = GameTime()
	}
	return false
}

// timetodo says whether an object in time bin bin is due.
func timetodo(bin int) bool {
	return curBin > bin && bin+4 >= curBin || curBin+16 > bin && lastbin <= bin && lastbin > curBin
}

// moveMobile advances the bin clock by delta frames and updates every active mobile
// object until it is no longer due.
func moveMobile(delta int) {
	if delta&0xFF > 8 {
		delta = 8
	}
	curBin = (lastbin + delta) & 0xF
	meptr = nil
	for p := 0; p < nActive; p++ {
		meptr = ObjIntTMem(int(Level.Active[p]))
		for timetodo(meptr.Bin()) {
			var ok bool
			if meptr.Major() == MAJOR_CREATURE {
				ok = critterAi()
			} else {
				ok = moveMeJoe()
			}
			if !ok {
				p--
				break
			}
		}
	}
	if meptr != nil {
		editchng(2)
	}
	lastbin = curBin
}
