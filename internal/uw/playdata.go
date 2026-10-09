package uw

// Port of PLAYDATA.C: the player record's load and save, and everything worked out again
// from the player's equipment and active spells (FixPlayerEquips).

var (
	PlayerDat        Player
	player           = &PlayerDat
	playerdat        *Creature
	ThePlayer        *Object
	playerNameHandle int
	PlayerLevel      int
	PlayerFacing     int16
	PlayerHeading    int16

	defenceSlotIndex      = [6]int{3, 0, 1, 2, 2, 0}
	plyNotice             = [2]byte{0x0F, 0x0F}
	UsingPole             bool
	DragonSkinBoots       bool
	MazeSpell             bool
	shroomsEnabled        = -1
	shroomsRelated        = true
	damageProtectionFlags = [5]byte{0x40, 0x08, 0x10, 0x01, 0x02}
	spellClassValues      = [16]byte{0x0E, 0xFF, 0x0D, 0x04, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x0B, 0x80, 0x80, 0x80, 0x80}
)

// savePlayerData writes the record as PLAYER.DAT has it: a key byte, then the record
// xor-encoded.
func savePlayerData(fd int) {
	key := player.Name[0] ^ 0xAA
	player.Strength = playerdat.Attr[ATTR_STR]
	player.Dexterity = playerdat.Attr[ATTR_DEX]
	player.Intelligence = playerdat.Attr[ATTR_INT]
	player.Health = ThePlayer.HP
	player.MaxHealth = playerdat.Avghit
	player.SavedX = PN.X
	player.SavedY = PN.Y
	player.SavedZ = PN.Z
	player.SavedFacing = int16(PlayerFacing)
	player.SavedLevel = int16(PlayerLevel)
	player.SetSound(b2i(fxIsOn()))
	player.SetMusic(b2i(musicIsOn()))
	player.SetTerrain(int(PN.Terrain))
	dosWrite(fd, []byte{key})
	xorwrite(fd, key, player.Bytes())
}

func readPlayerData(fd int) {
	var k [1]byte
	dosRead(fd, k[:])
	buf := make([]byte, playerSize)
	xorread(fd, k[0], buf)
	player.SetBytes(buf)
	playerdat.Attr[ATTR_STR] = player.Strength
	playerdat.Attr[ATTR_DEX] = player.Dexterity
	playerdat.Attr[ATTR_INT] = player.Intelligence
	ThePlayer.HP = player.Health
	playerdat.Avghit = player.MaxHealth
	PN.X = player.SavedX
	PN.Y = player.SavedY
	PN.Z = player.SavedZ
	PlayerFacing = player.SavedFacing
	PlayerLevel = int(player.SavedLevel)
	PN.Terrain = byte(player.Terrain())
	turnFx(player.Sound() != 0)
	turnMusic(player.Music() != 0)
	setGraphicsLevel()
	newFPS(player.FPS())
}

func initSpells() {
	ComObjData[ITEM_ADVENTURER].SetResist(0)
	plyNotice[0] = 13 - player.Skills[SKILL_STEALTH]/3
	plyNotice[1] = 15 - player.Skills[SKILL_STEALTH]/5
	motionbits = 0
	cmbModTH = [4]int8{}
	Hasted, WizEye, Blessed, TimeStop, DragonSkinBoots, MazeSpell = false, false, false, false, false, false
	PickDist = 0x90
	if UsingPole {
		PickDist = 0x190
	}
	plyregen[0] = 0
}

// swapTmap loads the floor textures, first choosing texture tex (0xFF keeps it).
func swapTmap(tex int) {
	if tex != 0xFF {
		floorIDs[4] = int16(tex)
	}
	loadTrMem("DATA\\f32.tr", floorIDs[:], &floorNum, floorTex32[:])
	loadTrMem("DATA\\f16.tr", floorIDs[:], &floorNum, floorTex16[:])
}

func setDrugged(on bool) {
	if on {
		if shroomsEnabled < 0 {
			if shroomsRelated {
				shroomsEnabled = 0
				shroomsRelated = false
			} else {
				shroomsEnabled = rand() % 3
			}
			switch shroomsEnabled {
			case 0:
				setCyb(true)
			case 1:
				grfxQuikpal(rand() & 7)
			case 2:
				randomLight(true)
			}
		}
	} else if shroomsEnabled >= 0 {
		switch shroomsEnabled {
		case 0:
			setCyb(false)
		case 1:
			grfxQuikpal(PAL_GAME)
		case 2:
			randomLight(false)
		}
		shroomsEnabled = -1
	}
}

func playerAffectedBy(major, minor int, bonuses *uint16, slot int) bool {
	switch major {
	case SPELLC_ARMOUR:
		if int(*bonuses>>4) < minor {
			*bonuses = *bonuses&0xF + uint16(minor<<4)
		}
	case SPELLC_PROTECT:
		if minor == 1 {
			for i := range cmbModTH {
				cmbModTH[i] += 3
			}
		} else if minor <= 4 {
			*bonuses |= 1 << (minor - 1)
		} else if minor <= 9 {
			c := &ComObjData[ITEM_ADVENTURER]
			c.SetResist(c.Resist() | int(damageProtectionFlags[minor-5]))
		}
	case SPELLC_MOTION:
		motionbits |= 1 << (minor - 1)
	case SPELLC_LIGHT:
		if int(player.Light&0xF0)>>4 < minor {
			player.Light = byte(minor << 4)
		}
	case SPELLC_XT:
		switch minor {
		case 0:
			TimeStop = true
		case 1:
			WizEye = true
		case 2:
			Hasted = true
		case 3:
			PickDist = 0
		case 14:
			plyregen[0] |= 1
		case 15:
			plyregen[0] |= 2
		}
	case SPELLC_BACKFIRE:
		backfire(ThePlayer, minor)
	case SPELLC_SPECIAL:
		if minor == 4 {
			MazeSpell = true
		}
	case 12:
		if slot < 0 {
			break
		}
		slots := [2]int{-1, -1}
		i := 0
		if slot > 4 {
			slots[0], slots[1] = 0, 1
		} else {
			slots[0] = defenceSlotIndex[slot]
		}
		for ; i < 2 && slots[i] != -1; i++ {
			toughness := 0
			if minor&8 != 0 {
				toughness = minor&7 + 1
			} else {
				cmbModTH[slots[i]] = int8(minor&7) + cmbModTH[slots[i]] + 1
			}
			playerdat.Armour[slots[0]] += byte(toughness)
		}
	}
	return false
}

func parseAspells(out []byte) {
	for i := range out[:3] {
		out[i] = 0x15
	}
	for i := 0; i < player.ActiveSpells(); i++ {
		out[i] = spellClassValues[player.Spells[i]&0xF]
		out[i] += byte(player.Spells[i]&0xF0) >> 4
	}
}

func setMaze(on bool) {
	if PlayerLevel == LEVEL_TYBAL {
		tex := -1
		if on && floorIDs[4] != 0x0C {
			tex = 0x0C
		}
		if !on && floorIDs[4] == 0x0C {
			tex = 0x0E
		}
		if tex >= 0 {
			swapTmap(tex)
		}
	}
	player.SetMaze(b2i(on))
}

func parseSpells(bonuses uint16) {
	for i := 0; i < 4; i, bonuses = i+1, bonuses>>1 {
		if bonuses&1 != 0 {
			switch i {
			case 1:
				plyNotice[0] -= min(plyNotice[0], 0x10)
			case 2:
				plyNotice[1] -= min(plyNotice[1], 5)
			case 3:
				plyNotice[1] -= min(plyNotice[1], 0x10)
			}
		}
	}
	for i := 0; i < 4; i++ {
		playerdat.Armour[i] += byte(bonuses>>4) & 0xF
	}
	setMaze(MazeSpell)
	var spells [3]byte
	parseAspells(spells[:])
	activeSpells(spells[:])
}

func armorVal(obj *Object) int {
	if obj.Major() == MAJOR_HACK && obj.Minor() < MINOR_ARMOR {
		return 0
	}
	armour := int(Armor[obj.Item()-FIRST_ARMOR].Protection)
	protection := int(uint16(obj.Quality()*armour)) >> 6
	return protection + 1
}

// FixPlayerEquips works out the player's derived state from scratch.
func FixPlayerEquips() {
	var bonuses uint16
	for slot := 0; slot < 4; slot++ {
		playerdat.Armour[slot] = 0
	}
	for slot := 0; slot <= 4; slot++ {
		if item := AskInventory(slot); item != nil {
			playerdat.Armour[defenceSlotIndex[slot]] += byte(armorVal(item))
		}
	}
	item := AskInventory(player.Lefty() + 7)
	if item != nil && item.Major() == MAJOR_HACK && item.Minor() == MINOR_ARMOR2 &&
		item.InClass() >= 11 && item.InClass() <= 15 {
		armour := armorVal(item)
		playerdat.Armour[0] += byte(armour)
		playerdat.Armour[1] += byte(armour)
	}
	playerdat.Defence = int8(player.Skills[SKILL_DEFENSE])
	ActiveObj = AskInventory(8 - player.Lefty())
	armour := 2
	if ActiveObj != nil && ActiveObj.Major() == MAJOR_HACK && ActiveObj.Minor() < MINOR_ARMOR {
		if ActiveObj.Minor() == MINOR_WEAPON {
			armour = int(Weapons[ActiveObj.InClass()].Skill)
			if armour < 3 {
				armour = 3
			} else if armour > 5 {
				armour = 5
			}
			loadWeapon(int(int8(byte(armour) + 0xFD)))
		} else if ActiveObj.InClass() <= 7 {
			loadWeapon(3)
		} else {
			loadWeapon(-1)
		}
	} else {
		loadWeapon(3)
	}
	playerdat.Defence += int8(player.Skills[armour] >> 1)
	initSpells()
	bestSlot, brightness := 0, 0
	for slot := 0; slot <= 4; slot++ {
		if slot == 4 {
			ActiveObj = CursorObjPtr
		} else {
			ActiveObj = AskInventory(ValidLightSlots[slot])
		}
		if ActiveObj != nil && ActiveObj.Class() == CLASS_LIGHT && ActiveObj.InClass() >= 4 && ActiveObj.InClass() < 8 {
			data := getClassData()
			if int(data[1]) > brightness {
				brightness = int(data[1])
				bestSlot = slot
			}
		}
	}
	player.Light = byte(brightness<<4 + bestSlot)
	for slot := 0; slot < player.ActiveSpells(); slot++ {
		s := player.Spells[slot]
		playerAffectedBy(int(s&0xF), int(s&0xF0)>>4, &bonuses, -1)
	}
	for slot := 0; slot <= 10; slot++ {
		ActiveObj = AskInventory(slot)
		if ActiveObj != nil && ObjWorn(ActiveObj.Item(), slot) {
			var major, effect int
			var flag bool
			if decodeObjSpell(ActiveObj, &major, &effect, &flag) && !flag {
				if playerAffectedBy(major, effect, &bonuses, slot) {
					removeSpell(ActiveObj)
				}
			} else if ActiveObj.Item() == ITEM_DRAGON_SKIN_BOOTS {
				DragonSkinBoots = true
			}
		}
	}
	parseSpells(bonuses)
	if WizEye {
		setLight(6)
	} else {
		setLight(int(player.Light&0xF0) >> 4)
	}
	setDrugged(player.Shrooms() > 0)
	fizixUpdate()
	newFPS(-1)
}
