package uw

// Port of COMBAT.C (seg022_230E): melee and missile combat. A blow is a few globals filled
// in by the attacker's side (playerAttack for the player, critterAttack for the AI) and
// then doAttack: resolveAttack finds the target in front, frpCheck rolls to hit, and
// doDamage rolls the damage (handing it to damageItem) or doMiss plays the miss.
// missileThwack is the same damage path for a missile that struck.

var (
	// by swing (1-9, the ninths of the 3D view from the bottom left): 0 slash, 1 bash, 2 stab
	swingKind = [9]int{2, 2, 2, 0, 0, 0, 1, 1, 1}
	// by swing / 3: the scan code of the key that holds the swing
	swingKeys = [3]int{0x34, 0x27, 0x19}
	pQatt     = 0
	attackKey = -1
	// the mouse button holding the blow (1 left, 2 right), 0 when a key does
	attackButton = 0
	// by hit location: the height of the blood splash; [4] is set to -hitz before use
	hitzTab = [5]int{5, 3, 1, 7, 0}

	askill, wsize, towhere int
	otime                  uint32
	elapsed                int
	swingState             int
	playPow                int
	targx, targy           int
	hitobj                 int
	cmbModTH               [4]int8
	hitz                   int
	damage                 int
	hitloc                 int
	fireMode               bool
	hitangle               int
	criti                  bool
	fromwho                int
	currWeapon             *Object
	weaponData             *Weapon
	power                  int
)

// pickloc picks the hit location (0..3) from the defender's and the attack's heights.
func pickloc(dz, dtop, az, atop int) int {
	dmid := (dz + dtop) >> 1
	amid := (az + atop) >> 1
	if dz+1 > amid {
		return 2
	}
	if dtop-1 < amid {
		return 3
	}
	if amid < dmid {
		if rand()%2 != 0 {
			return 2
		}
	} else if rand()%3 == 0 {
		return 3
	}
	if rand()%3 != 0 {
		return 0
	}
	return 1
}

// collSquare sets targx, targy to the square of collision i.
func collSquare(c *MotionCalc, i int) {
	t := int(oCollisions[i].offset) & MAP_MASK
	targx = (int(c.x)>>3 + t) & MAP_MASK
	t = targx - int(c.x)>>3
	targy = (int(c.y)>>3 + (int(oCollisions[i].offset)-t)/MAP_SIZE) & MAP_MASK
}

// setHitobj picks the object in the blow's way nearest the attacker. Returns its
// oCollisions index, or -1.
func setHitobj(c *MotionCalc) int {
	found := -1
	best := 100000
	last := int(c.first) + int(c.count)
	att := ObjIntTMem(fromwho)
	ax := att.HomeX()<<3 + att.FineX()
	ay := att.HomeY()<<3 + att.FineY()
	for i := int(c.first); i < last; i++ {
		idx := oCollisions[i].link.Index()
		obj := ObjIntTMem(idx)
		if obj.Major() == MAJOR_TRAP || idx == fromwho {
			continue
		}
		if fromwho == 1 && IsMobElem(obj) && obj.Ally() != 0 && (last-1 != i || best != 100000) {
			continue
		}
		collSquare(c, i)
		dx := ax - (targx<<3 + obj.FineX())
		dy := ay - (targy<<3 + obj.FineY())
		if dist := dx*dx + dy*dy; dist < best {
			best = dist
			found = i
		}
	}
	if found >= 0 {
		collSquare(c, found)
	}
	return found
}

// findWallColl puts a spark where a blow met a wall.
func findWallColl(heading, dist int, c *MotionCalc) {
	curP = c
	curP.radius = 1
	curP.index = 0
	dist++
	fx, fy := curP.x<<4, curP.y<<4
	for {
		TerrainCheck(0)
		if (curP.hits0|curP.hits1)&0x300 != 0 {
			obj := CreateObj(ITEM_DAMAGE, false)
			if obj == nil {
				return
			}
			obj.SetFineX(int(curP.x) & 7)
			obj.SetFineY(int(curP.y) & 7)
			tx, ty := int(curP.x)>>3, int(curP.y)>>3
			obj.SetZ(int(curP.z) + 8)
			if fromwho == 1 {
				playEffect(7, int(curP.x), int(curP.y), 0)
			}
			if addAnimobj(ObjMemTPtr(obj), 2, 0, tx, ty) == -1 {
				ObjFree(obj)
			} else {
				ObjAddEnd(&MapGetAddr(tx, ty).Objects, obj)
			}
			return
		}
		moveAlong(heading, 0x10, &fx, &fy)
		curP.x, curP.y = fx>>4, fy>>4
		if dist--; dist <= 0 {
			break
		}
	}
}

// resolveAttack probes where the attacker's blow goes. True with hitobj and hitloc set
// when it meets an object.
func resolveAttack() bool {
	var calc MotionCalc
	curP = &calc
	curP.index = int16(fromwho)
	curP.radius = byte(wsize + 1)
	curP.height = byte((wsize*2 + 1) * 4)
	att := ObjIntTMem(fromwho)
	h := ComObjData[att.Item()].Height()
	z := att.Z() + h*(towhere/3)/3
	if fromwho == 1 {
		z += int(PlayerPitch) / 0x200
	}
	curP.z = int16(z)
	hitz = int(int8(z + h/6))
	curP.x = int16(att.HomeX()<<3 + att.FineX())
	curP.y = int16(att.HomeY()<<3 + att.FineY())
	heading := att.HeadingBits()<<5 + att.FineHead()
	moveAlong(heading, wsize+3, &curP.x, &curP.y)
	ObjectCheck(false, true)
	if curP.found != 0 {
		processObjlist()
		if curP.count == 0 {
			return false
		}
		i := setHitobj(curP)
		if i < 0 {
			return false
		}
		hitloc = pickloc(int(oCollisions[i].bottom), int(oCollisions[i].top), int(curP.z), int(curP.height)+int(curP.z))
		hitobj = oCollisions[i].link.Index()
		return true
	}
	TerrainCheck(0)
	if (curP.hits0|curP.hits1)&0x300 != 0 {
		curP.x = int16(att.HomeX()<<3 + att.FineX())
		curP.y = int16(att.HomeY()<<3 + att.FineY())
		findWallColl(heading, wsize+3, curP)
	}
	return false
}

// frpCheck is the to-hit roll: 0 a hit, 1 or 2 a miss.
func frpCheck(attacker, defender int) int {
	def := ObjIntTMem(defender)
	if def.Major() != MAJOR_CREATURE {
		if attacker == 1 && def.Class() == CLASS_DOOR && rand()*12/0x8000 < int(def.ID&7)<<1 {
			DamageInventory(INV_WEAPON_HAND-player.Lefty(), rollem(2, 4), 4, 0, true)
		}
		return 0
	}
	cr := &Creatures[def.InMajor()]
	if hitobj == 1 {
		askill -= int(cmbModTH[hitloc&3])
	}
	result := skillCheck(askill+hitangle, int(cr.Defence))
	criti = false
	if result == 2 {
		criti = true
		damage = damage * ((rand()&0x1F + 0x30) >> 5)
		if defender == 1 {
			fillFB(0xB8)
			slot := (hitloc + 1) & 3
			if slot == 3 {
				slot += b2int(rand()%5 == 0)
			} else if slot != 0 && slot <= 2 {
				slot = player.Lefty() + INV_HAND
			}
			DamageInventory(slot, rollem(2, 4), 4, 1, true)
		}
		return 0
	}
	if result == -1 && attacker == 1 && Creatures[ObjIntTMem(hitobj).InMajor()].Passive() == 0 {
		DamageInventory(INV_WEAPON_HAND-player.Lefty(), rollem(2, 3), 4, 0, true)
	}
	return 1 - result
}

// doDamage rolls and applies a hit's damage of type typ, with the feedback.
func doDamage(typ int) {
	def := ObjIntTMem(hitobj)
	item := def.Item()
	if damage < 2 {
		damage = 2
	}
	quot, rem := damage/6, damage%6
	damage = 0
	if quot != 0 {
		damage = rollem(quot, 6)
	}
	if rem != 0 {
		damage += rollem(1, rem)
	}
	ddone := damage*power>>7 + hitangle
	if hitobj == 1 {
		playEffectHere(3, 0x40, ddone<<2)
	} else {
		playEffect(4, targx<<3+def.FineX(), targy<<3+def.FineY(), ddone<<2)
	}
	cr := &Creatures[item&ID_INMAJOR]
	if item>>6 == MAJOR_CREATURE {
		armour := int(cr.Armour[hitloc%4])
		if armour == 0xFF {
			hitloc &= 4
			armour = int(cr.Armour[0])
		}
		if hitobj != 1 && def.Powerful() != 0 {
			armour = armour * 5 / 3
		}
		if armour > ddone {
			ddone = 0
		} else {
			ddone -= armour
		}
	}
	level := ddone / 4
	if level >= 4 {
		level = 3
	}
	if hitobj == 1 && player.Easy != 0 {
		ddone >>= 1
	}
	var result bool
	if isPeerAvatar(hitobj) { // another player: suffered there, and only from creatures
		if fromwho > 1 && fromwho < NUM_MOBILE {
			hurtPeer(ddone, typ)
		}
	} else {
		result = damageItem(def, ObjIntTMem(fromwho), targx, targy, ddone, typ)
	}
	if ddone == 0 || fromwho == -1 {
		return
	}
	if hitloc >= 4 {
		hitzTab[4] = -hitz
		hitloc = 4
	}
	if UsPtr != nil && ObjMemTPtr(UsPtr) == hitobj {
		setEffect(0x20, level*5)
		return
	}
	if item>>6 == MAJOR_CREATURE {
		if fromwho == 1 {
			hp := int(def.HP)
			if result {
				hp = 0
			}
			showEnemyHealth(hp, int(cr.Avghit))
		}
		if cr.Blood() != 0 {
			putEffect(def, 0, 1, level, hitzTab[hitloc], targx, targy)
			if criti && fromwho == 1 {
				putEffect(def, 0, 1, level, hitzTab[hitloc]+(rand()&1)*5-2, targx, targy)
			}
			return
		}
		result = false
	}
	if result {
		def = nil
	}
	if (item&ID_CLASS)>>4 == CLASS_DOOR || item == ITEM_MOVING_DOOR {
		// def is nil when the blow destroyed the door: DOS reads the vector table
		d := def
		if d == nil {
			d = &Level.Objs[0]
		}
		if d.Z() > hitz {
			hitz = d.Z() + 2
		}
	}
	putEffect(def, 0xB, 1, level, -hitz, targx, targy)
}

// doMiss plays the sound of a miss (hit 0: at nothing).
func doMiss(hit int) bool {
	if hit == 0 {
		playEffectOnMobile(10, ObjIntTMem(fromwho), 0)
		return false
	}
	// the attacker's item id goes into fromwho itself, as in the original
	fromwho = ObjIntTMem(fromwho).Item()
	weapon := 1
	if fromwho != 1 && fromwho < NUM_MOBILE {
		weapon = Creatures[fromwho&ID_INMAJOR].WeaponKind()
	}
	victim := 0
	if hitobj == 1 {
		if armour := AskInventory((hitloc + 1) & 3); armour != nil {
			switch armour.Item() {
			case ITEM_LEATHER_VEST, ITEM_LEATHER_LEGGINGS, ITEM_LEATHER_GLOVES, ITEM_LEATHER_BOOTS, ITEM_LEATHER_CAP:
			default:
				victim = 1
			}
		}
	} else if hitobj < NUM_MOBILE {
		victim = Creatures[fromwho&ID_INMAJOR].ArmourKind()
	}
	fx := 8
	if weapon == 1 && victim == 1 {
		fx = 7
	}
	playEffectOnMobile(fx, ObjIntTMem(hitobj), 0)
	return false
}

// computeHitangle: how far round from the defender's front the attacker stands (0..4).
func computeHitangle() {
	hitangle = (ObjIntTMem(hitobj).HeadingBits() + 0xC - ObjIntTMem(fromwho).HeadingBits()) & 7
	if hitangle > 4 {
		hitangle = 8 - hitangle
	}
}

// doAttack makes one melee blow from fromwho. True for a hit.
func doAttack() bool {
	if !resolveAttack() {
		return doMiss(0)
	}
	if fromwho != 1 && hitobj != 1 && !isPeerAvatar(hitobj) && IsMobElem(ObjIntTMem(hitobj)) &&
		ObjIntTMem(hitobj).Ally()^ObjIntTMem(fromwho).Ally() == 0 {
		return false
	}
	computeHitangle()
	if result := frpCheck(fromwho, hitobj); result != 0 {
		damageItem(ObjIntTMem(hitobj), ObjIntTMem(fromwho), targx, targy, 0, DMG_PHYSICAL)
		return doMiss(result)
	}
	doDamage(DMG_PHYSICAL)
	return true
}

// checkAmmo finds the ammunition of missile weapon class weapon in the inventory.
// Returns its slot, or -1 after saying there is none.
func checkAmmo(weapon int) int {
	var found int
	ammo := int(Missile[weapon].Ammo)
	if FindObj(MAJOR_HACK, 1, ammo, 4, &found) == nil {
		var fake Object
		fake.SetItem(ammo + FIRST_MISSILE)
		scrollPrint("Sorry, you have no ")
		n, ok := getName(&fake, false, true)
		if !ok {
			n = "UNNAMED"
		}
		scrollPrint(n)
		scrollPrint(".\n")
		return -1
	}
	return found
}

// getPlayerWeapon finds the weapon in the player's weapon hand: 0 a missile weapon with
// ammunition, -1 one without, 1 melee (weap's data in wd, the fist when bare handed).
func getPlayerWeapon(wd **Weapon, weap **Object) int {
	*wd = nil
	*weap = AskInventory(INV_WEAPON_HAND - player.Lefty())
	if *weap != nil {
		item := (*weap).Item()
		if item>>4 == CLASS_MISSILE {
			if a := Missile[item&ID_INCLASS].Ammo; a >= 0 && a < 0x10 {
				if checkAmmo(item&ID_INCLASS) >= 0 {
					return 0
				}
				mouseRelease(true)
				return -1
			}
		} else if item>>4 == CLASS_WEAPON {
			*wd = &Weapons[item&ID_INCLASS]
			wsize = ComObjData[item].Radius()
		}
	}
	if *wd == nil {
		*wd = &Weapons[ITEM_FIST]
		wsize = ComObjData[ITEM_FIST].Radius()
	}
	return 1
}

// doPlayerWeapon sets up the player's blow: attack skill and damage.
func doPlayerWeapon(wd *Weapon, weap *Object, swing int) {
	skill := int(wd.Skill)
	if skill >= SKILL_MISSILE || skill < SKILL_BAREHAND {
		skill = SKILL_BAREHAND
	}
	askill = int(player.Skills[SKILL_ATTACK]>>1) + int(player.Skills[skill])
	askill += int(player.Dexterity) / 7
	if player.Easy != 0 {
		askill += 7
	}
	str := int(Creatures[ThePlayer.InMajor()].Attr[ATTR_STR])
	if skill == SKILL_BAREHAND {
		damage = int(player.Skills[SKILL_BAREHAND])*2/5 + str/6 + 4
	} else {
		damage = int(wd.Damage[swingKind[swing-1]]) + str/9
	}
	fromwho = 1
	towhere = swing
	if weap != nil {
		var major, effect int
		var flag bool
		decodeObjSpell(weap, &major, &effect, &flag)
		if !flag && major == 0xC {
			if effect&8 != 0 {
				damage += effect&7 + 1
			} else {
				askill += effect&7 + 1
			}
		}
	}
}

// missileFinish ends a bow or sling shot.
func missileFinish() {
	pQatt = -10
	attackKey = -1
	setScreenFrame(3, 0)
	GameInputMode -= 4
	unforceMouseCursor(3)
	setScreenFrame(8, 4)
	fireMode = false
}

// finAttack puts the weapon picture and power bar back.
func finAttack() {
	if player.Drawn() != 0 {
		setScreenFrame(8, 4)
	} else {
		setScreenFrame(8, 6)
	}
	setScreenFrame(3, 0)
}

// clearFightState cancels any attack in progress.
func clearFightState() {
	if fireMode && swingState == 0 {
		GameInputMode -= 4
		unforceMouseCursor(3)
	}
	finAttack()
	pQatt = 0
	attackKey = -1
	fromwho = -1
}

// playerAttack runs the player's attack each frame; swing is the ninth of the 3D view
// clicked (1..9, 0 none).
func playerAttack(swing int) {
	held := false
	if attackKey > 0 {
		held = keyOn[attackKey] != 0
	}
	held = held || mouseGetbut()&attackButton != 0
	if pQatt > 0 {
		return
	}
	if pQatt < 0 {
		if weapFrame < 0 && pQatt <= -10 {
			finAttack()
			pQatt = 0
			attackKey = -1
			return
		}
		if weapFrame > 6 {
			return
		}
		wd := weaponData
		if weapFrame < 3 { // the wind-up: let go during it, the blow still comes, uncharged
			return
		}
		if weapFrame == 3 {
			if fireMode {
				if held || swingState < 0 {
					if swingState >= 0 {
						return
					}
					setScreenFrame(3, 9)
					swingState = 0
					GameInputMode += 4
					forceMouseCursor(ICON_CURSORS + 9)
					return
				}
				if mousIn3dP() {
					playerFire(currWeapon.InClass())
				}
				missileFinish()
				return
			}
			if held {
				playerdat.SetNoise(10)
				if elapsed < 0 {
					otime = GameTime()
					elapsed = 0
					return
				}
				elapsed += int(GameTime() - otime)
				otime = GameTime()
				for elapsed > 0x10 {
					playPow += int(wd.Speed)
					if playPow > 100 {
						playPow = 100
					}
					setScreenFrame(3, playPow/12+1)
					elapsed -= 0x10
				}
				return
			}
			if pQatt > -5 {
				setScreenFrame(8, -pQatt-1)
				pQatt = -5
			}
			return
		}
		if weapFrame != 6 || pQatt <= -10 {
			return
		}
		setScreenFrame(3, 0)
		charge := int(int16(int(wd.MaxCharge) - int(wd.MinCharge)))
		charge = int(int16(charge * playPow / 100))
		playPow = (int(wd.MinCharge) + charge) & 0xFF
		playerdat.SetNoise(15)
		power = playPow
		doPlayerWeapon(wd, currWeapon, swingState)
		doAttack()
		pQatt = -10
		return
	}
	if player.Drawn() == 0 || swing == 0 || weapFrame != -1 {
		return
	}
	swingState = swing
	tx := getPlayerWeapon(&weaponData, &currWeapon)
	if tx < 0 {
		return
	}
	fireMode = tx == 0
	if fireMode {
		swingState = -1
	}
	pQatt = -1 - swingKind[swing-1]
	// swing_keys[-1] reads swing_kind[8]
	if i := swing/3 - 1; i >= 0 {
		attackKey = swingKeys[i]
	} else {
		attackKey = swingKind[8]
	}
	setScreenFrame(8, -pQatt-1)
	setScreenFrame(3, 1)
	playPow = 0
	elapsed = -1
}

// missileThwack: a missile (or anything thrown) has hit def.
func missileThwack(attacker int, missile, def *Object, x, y, dmg, typ int) {
	targx, targy = x, y
	criti = false
	hitangle = 0
	hitz = int(int8(missile.Z() + ComObjData[missile.Item()].Height()/2))
	power = 0x80
	fromwho = attacker
	hitobj = ObjMemTPtr(def)
	hitloc = pickloc(def.Z(), def.Z()+ComObjData[def.Item()].Height(),
		missile.Z(), missile.Z()+ComObjData[missile.Item()].Height()) + 4
	damage = dmg
	if def == ThePlayer {
		playEffectHere(3, 0, 0)
	} else if IsMobElem(def) {
		playEffectOnMobile(4, def, 0)
	}
	doDamage(typ)
}

// critterAttack makes a critter's blow with its attack type typ. True for a hit.
func critterAttack(npc *Object, swing, charge, typ, poison int) bool {
	wsize = 2
	fromwho = ObjMemTPtr(npc)
	towhere = swing
	power = charge & 0xFF
	cr := &Creatures[npc.InMajor()]
	damage = int(cr.Attacks[typ].Damage) + int(cr.Attr[ATTR_STR])/5
	askill = int(cr.Attacks[typ].Chance) + int(cr.Equip>>1)
	if npc.Powerful() != 0 {
		askill += rand()%6 + 7
		damage += rand()%12 + 4
	}
	hurtPoison = poison
	result := doAttack()
	hurtPoison = 0
	if result && hitobj == 1 && player.Poison() < poison {
		if checkRes(ThePlayer, 1, DMG_POISON) != 0 {
			player.SetPoison(poison)
		}
	}
	return result
}

// cancelQueuedBlow (seg022_230E_16F5) cancels a player blow still queued.
func cancelQueuedBlow() {
	if pQatt > 0 {
		pQatt = 0
		attackKey = -1
		setScreenFrame(3, 0)
		setScreenFrame(8, 4)
	}
}

// playerKilledA gives the experience for killing npc.
func playerKilledA(npc *Object) {
	if npc.Major() != MAJOR_CREATURE {
		return
	}
	setScreenFrame(4, 2)
	setNewMusic(MUSIC_VICTORY)
	exp := int(Creatures[npc.InMajor()].Exp)
	exp = exp*4 + rollem(2, exp)
	if npc.Powerful() != 0 {
		exp = exp * (rand()%24 + 24) / 16
	}
	playerGetExp(exp)
}
