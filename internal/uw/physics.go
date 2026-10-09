package uw

// Port of PHYSICS.C: the player's physics. The terrain the player stands in and the
// movement state it gives (newFPS), the step-and-turn movement (simpleFizix), the
// per-tick setup from the input (setPlayerPhysParams), applying MOTION's result to the
// player object (physAffectPlayer), and placing the player on a tile (playerSetup).
//
// The movement state: 0 on foot, 1 swimming, 2 on lava, 3 unused (ice), and in the air
// 4 levitating, 5 flying, 6 slow falling. motionbits holds the motion spells (MB_*).

var (
	pTurn          int16    // the turn rate for the current motion state
	GrSq           int = -1 // the player's tile, as an index into the level's tiles
	pFPS           [3]int16 // forward, side and backward speeds
	lastTerr       int16
	MaxPlayerAccel int16 = 0x60
	BackFPS        int16 = 0xBC
	SideFPS        int16 = 0xEB
	RunFPS         int16 = 0x3AC
	plyMoType      int16
	motionbits     byte
	fizUpdate      = true
)

// waterSet starts swimming if terr has the water bit. It reports whether it did.
func waterSet(terr int) bool {
	if terr&FOOT_WATER != 0 {
		player.SwimCount = 0x60
		puntFightmode()
		return true
	}
	player.SwimCount = 0x10
	return false
}

// parsePlayerTerr turns the terrain byte under the player into the movement state, when
// it changes or with force, and in the air applies gravity or the levitation damping.
func parsePlayerTerr(terr int, force bool) {
	if int(lastTerr) != terr || force {
		swimming := false
		state := FPS_WALK
		lastTerr = int16(terr)
		switch {
		case terr&(FOOT_WATER|FOOT_SHORE) != 0:
			if motionbits&MB_WATER_WALK == 0 {
				swimming = waterSet(terr)
				state = FPS_SWIM
			}
		case terr&FOOT_LAVA != 0:
			state = FPS_LAVA
		case terr&FOOT_AIR != 0:
			switch {
			case motionbits&MB_LEVITATE != 0:
				state = FPS_LEVITATE
			case motionbits&MB_FLY != 0:
				state = FPS_FLY
			case motionbits&MB_SLOW_FALL != 0:
				state = FPS_SLOW_FALL
			}
		}
		newFPS(state)
		if !swimming {
			player.SwimCount = 0
		}
	}
	if terr&FOOT_AIR != 0 {
		if motionbits&(MB_LEVITATE|MB_FLY) != 0 {
			PN.Acc[2] = 0
			if abs16(PN.Vel[2]) > 10 {
				PN.Vel[2] = (PN.Vel[2] << 2) / 5
			} else {
				PN.Vel[2] = 0
			}
		} else {
			if PN.Acc[2] == 0 {
				PN.Acc[2] = -4
			}
			if motionbits&MB_SLOW_FALL != 0 && PN.Vel[2] <= -94 {
				PN.Vel[2] = -94
				if PN.Speed > 20 {
					PN.Speed /= 2
				} else {
					PN.Speed = 0
				}
			}
		}
	}
}

// etherealVoidSpecialEffects: a flash, a scaled hit to the player's hit points, a shake
// and a random compass frame.
func etherealVoidSpecialEffects() {
	fillFB(0xB5)
	hp := int(ThePlayer.HP)
	switch {
	case hp > 100:
		hp -= rand() % 6
	case hp > 50:
		hp -= rand() % 4
	case hp > 20 && rand()&3 < 1:
		hp -= rand() % 3
	case hp > 1 && rand()&7 == 0:
		hp--
	}
	ThePlayer.HP = byte(hp)
	if rand()%12 != 0 {
		setEffect(MS_TREMOR, rand()%30+15)
	}
	setScreenFrame(2, rand()&0xF)
}

// playerChangeSq moves the player object to the tile under PN when it changed.
func playerChangeSq() {
	if sq := int(PN.X>>8) + int(PN.Y>>8)<<6; sq != GrSq {
		if GrSq != -1 {
			ObjRem(&Level.Tiles[GrSq].Objects, ThePlayer)
		}
		GrSq = sq
		ObjAdd(&Level.Tiles[GrSq].Objects, ThePlayer)
		ThePlayer.SetHomeX(int(PN.X >> 8))
		ThePlayer.SetHomeY(int(PN.Y >> 8))
	}
	ThePlayer.SetFineX(int(PN.X>>5) & 7)
	ThePlayer.SetFineY(int(PN.Y>>5) & 7)
}

// simpleFizix is the step-and-turn movement: turn -1 or 1 turns 45 degrees; 0 steps
// half a tile forward but not into a different terrain or off a ledge; 2 takes the same
// step without those checks; -2 steps a quarter tile back. It reports whether the
// player moved or turned.
func simpleFizix(turn int) bool {
	if PN.Acc[2] != 0 || PN.Speed >= MaxPlayerAccel {
		return false
	}
	backwards, flying := false, false
	var dist, heading int
	switch turn {
	case 2, 0:
		backwards = turn == 2
		dist = 0x80
		heading = int(PlayerFacing) / 256
	case -2:
		dist = 0x40
		heading = int(uint16(PlayerFacing+-0x8000) >> 8)
	default:
		if PlayerFacing&0x1FFF != 0 {
			PlayerFacing = PlayerFacing&-0x2000 + int16(b2i(turn > 0))<<13
		} else {
			PlayerFacing += int16(turn << 13)
		}
		ThePlayer.SetHeading(int(PlayerFacing >> 13))
		ThePlayer.SetFineHead(int(PlayerFacing >> 8))
		return true
	}
	if motionbits&(MB_LEVITATE|MB_FLY) != 0 {
		flying = true
	}
	x, y := PN.X, PN.Y
	moveAlong(heading, dist, &x, &y)
	if !canPlace(ITEM_ADVENTURER, 1, int(x/32), int(y/32), ThePlayer.Z(), backwards || flying, 8) {
		return false
	}
	if !backwards && nvokTerr != FOOT_FLOOR && nvokTerr != lastTerr && (nvokTerr != FOOT_AIR || !flying) {
		return false
	}
	PN.X, PN.Y = x, y
	playerChangeSq()
	if !(backwards || flying) || PN.Z>>3-8 <= nvokHgt {
		ThePlayer.SetZ(int(nvokHgt))
		PN.Z = nvokHgt << 3
	} else if PN.Acc[2] == 0 && !flying {
		PN.Acc[2] = -4
	}
	parsePlayerTerr(int(nvokTerr), false)
	ThePlayer.SetFrame(int(GameTime()&0xFF) >> 6)
	oldP := curP
	var calc MotionCalc
	curP = &calc
	curP.index = 1
	curP.radius = byte(ComObjData[ITEM_ADVENTURER].Radius())
	curP.height = byte(ComObjData[ITEM_ADVENTURER].Height())
	curP.x = x / 32
	curP.y = y / 32
	curP.z = int16(ThePlayer.Z())
	ObjectCheck(false, false)
	processObjlist()
	for i := int(curP.first); i < int(curP.first)+int(curP.count); i++ {
		obj := ObjPtrTMem(&oCollisions[i].link)
		if obj.Item() == ITEM_MOVE_TRIGGER {
			UseTrigger(ThePlayer, nil, obj, 0)
		}
	}
	curP = oldP
	return true
}

// setPlayerPhysParams sets PN up for a tick of rate time units from the input.
func setPlayerPhysParams(rate int) {
	speed := int16(0)
	if PN.Acc[2] == 0 {
		doPlayerInput(int(PlayerInput), rate, &speed)
		if PN.Acc[2] == 0 {
			d := speed - PN.Speed
			if abs16(d) > MaxPlayerAccel {
				if d > 0 {
					d = MaxPlayerAccel
				} else {
					d = -MaxPlayerAccel
				}
			}
			PN.Speed += d
			if PN.Speed > pFPS[0] {
				PN.Speed = pFPS[0]
			} else if PN.Speed < 0 {
				PN.Speed = 0
			}
		}
	}
	if PN.Acc[2] != 0 {
		PlayerFacing += int16(rate * int(PlayerTurn) * int(TurnInpRate/4) / 4)
	}
	PN.Time = int16(rate)
	PN.Bounce = 5
	PN.Flags = 0
	if PN.Acc[0]|PN.Acc[1]|PN.Acc[2] == 0 {
		PN.Flags = 0x80
	}
	if PN.Speed == 0 {
		PlayerHeading = PlayerFacing
	}
	PN.Heading = PlayerHeading
	PT.ignore = 0
	if motionbits&(MB_LEVITATE|MB_FLY) != 0 {
		PT.ignore = 0x1000
		PN.Flags = 0x80
	}
	PN.Impact = 0
}

// playerSetup puts the player in the middle of tile x, y at the floor's height and
// resets the physics.
func playerSetup(x, y int) {
	if GrSq >= 0 {
		ObjRem(&Level.Tiles[GrSq].Objects, ThePlayer)
	}
	PT.special = playerSqhandler
	PT.mask = 0x1100
	PT.ignore = 0
	PN.Speed = 0
	PN.Acc = [3]int16{}
	PN.Vel = [3]int16{}
	PN.X = int16(x<<8 + 0x80)
	PN.Y = int16(y<<8 + 0x80)
	PN.B24 = 8
	PN.Index = 1
	GrSq = x + y<<6
	t := &Level.Tiles[GrSq]
	PN.Z = hgtVal[t.Height()]
	if tileWalls[t.Type()]&0x20 != 0 {
		PN.Z += 0x20
	}
	ThePlayer.SetZ(int(PN.Z >> 3))
	ThePlayer.SetHomeX(x)
	ThePlayer.SetHomeY(y)
	ThePlayer.SetFineX(3)
	ThePlayer.SetFineY(3)
	ThePlayer.SetSeq(0x2C)
	ThePlayer.SetNext(0)
	ThePlayer.SetQuality(0)
	var calc MotionCalc
	curP = &calc
	curP.index = int16(ObjMemTPtr(ThePlayer))
	curP.radius = byte(ComObjData[ITEM_ADVENTURER].Radius())
	curP.height = byte(ComObjData[ITEM_ADVENTURER].Height())
	curP.x = int16(x<<3 + 3)
	curP.y = int16(y<<3 + 3)
	curP.z = PN.Z >> 3
	TerrainCheck(PN.B24)
	PN.Terrain = setResterr(uint16(curP.hits0 | curP.hits1))
	parsePlayerTerr(int(PN.Terrain), false)
	playerMod[0] = 0
	parseEffect()
	fizUpdate = true
	ObjAdd(&Level.Tiles[GrSq].Objects, ThePlayer)
}

// physAffectPlayer copies PN back to the player object after doPhysics, turns the
// facing while sliding along a wall, and turns the impact into falling damage.
func physAffectPlayer() {
	playerChangeSq()
	ThePlayer.SetZ(int(PN.Z >> 3))
	ThePlayer.SetFrame(int(GameTime()&0xFF) >> 6)
	if PN.Impact != 0 && PN.Heading == PlayerHeading {
		PN.Speed = 0
	}
	if PN.Heading != PlayerHeading {
		PlayerHeading = PN.Heading
		h := PN.Heading - plyMoType<<14
		if PN.Flags&0x80 != 0 {
			switch {
			case abs16(PlayerFacing-h) < 0x400:
				PlayerFacing = h
			case uint16(PlayerFacing-h) < 0x7FFF:
				PlayerFacing -= 0x400
			default:
				PlayerFacing += 0x400
			}
		}
	}
	ThePlayer.SetHeading(int(PlayerFacing >> 13))
	ThePlayer.SetFineHead(int(PlayerFacing >> 8))
	if PN.Impact != 0 {
		if PN.Bounce > 0 {
			dmg := int(PN.Impact >> 8)
			if PN.Vel[2] != 0 {
				dmg <<= 1
			}
			acro := int(player.Skills[SKILL_ACROBAT])
			if skillCheck(acro, dmg<<1) > 0 {
				dmg = dmg * (30 - acro) / 30
			}
			if dmg > 3 {
				damageItem(ThePlayer, nil, 0, 0, dmg, 0)
			}
			if dmg > 1 || PN.Terrain&FOOT_AIR != 0 {
				playEffectHere(0xF, 0x40, int(int8(dmg<<2-60)))
			}
		}
		PN.Impact = 0
	}
	parsePlayerTerr(int(PN.Terrain), false)
	fizUpdate = false
}

// playerSqhandler stops a player walking slowly off a ledge at the edge.
func playerSqhandler(w *uint16) bool {
	if *w&0x1000 != 0 && PN.Vel[2] == 0 && int(PN.Speed)*10 < int(pFPS[0])*3 {
		PN.Vel[0], PN.Vel[1] = 0, 0
		return true
	}
	return false
}

// doPlayerInput turns the movement command into a heading and a wanted speed.
func doPlayerInput(input, rate int, speed *int16) {
	h := PlayerFacing
	switch input {
	case PIN_FORWARD:
		PlayerFacing += int16(rate * int(PlayerTurn) * int(TurnInpRate/4) / 4)
		h = PlayerFacing
		*speed = int16(int(pFPS[0]) * int(ForwInpRate>>2) / 32)
		plyMoType = 0
	case PIN_RIGHT:
		h += 0x4000
		*speed = pFPS[1]
		plyMoType = 1
	case PIN_LEFT:
		h -= 0x4000
		*speed = pFPS[1]
		plyMoType = -1
	case PIN_BACK:
		h += -0x8000
		*speed = pFPS[2]
		plyMoType = -2
	case PIN_RUN_JUMP, PIN_JUMP:
		if input == PIN_RUN_JUMP {
			if PN.Vel[2] != 0 || PN.Acc[2] != 0 || PN.Speed != 0 {
				break
			}
			h = PlayerFacing
			PlayerHeading = h
			*speed = pFPS[0] / 2
			PN.Speed = *speed
			plyMoType = 0
		}
		PN.Vel[2] = 0x263
		if PN.Z > 0x280 {
			PN.Vel[2] = PN.Vel[2] * 5 / 6
			if PN.Z > 0x2C0 {
				PN.Vel[2] = (PN.Vel[2] << 1) / 3
			}
		}
		if motionbits&MB_LEAP != 0 {
			PN.Acc[2] = -2
		} else {
			PN.Acc[2] = -4
		}
		return
	case PIN_UP:
		plyMoType = 0
		PN.Vel[2] = 0x8D
		PN.Acc[2] = 0
	case PIN_DOWN:
		plyMoType = 0
		PN.Vel[2] = -0x8D
		PN.Acc[2] = 0
	case PIN_NONE:
		*speed = 0
		return
	}
	PlayerHeading = h
}

// physBounceUp lifts the player a little unless in the air.
func physBounceUp(obj *Object) {
	if obj == ThePlayer {
		if PN.Terrain&FOOT_AIR == 0 {
			PN.Vel[2] = 0x8D
		}
		PN.Acc[2] = 0
	}
}

// fizixUpdate recomputes the movement state after the motion spells change.
func fizixUpdate() {
	parsePlayerTerr(int(PN.Terrain), true)
	fizUpdate = true
}

// bouncePlayer throws the player up by intensity and halves the horizontal speed.
func bouncePlayer(intensity int) {
	if PN.Acc[2] != -4 {
		PN.Acc[2] = -2
	}
	PN.Vel[2] = int16(intensity * 0x2F / 4)
	PN.Vel[0] /= 2
	PN.Vel[1] /= 2
}

// QuakeTrap: bit 0 of typ shakes the screen, bit 1 throws the player up.
func QuakeTrap(typ, intensity int) {
	if typ&1 != 0 {
		setEffect(MS_TREMOR, 0x1E)
	}
	if typ&2 != 0 {
		bouncePlayer(intensity)
	}
}

var (
	fpsRatios = [7]int{10, 3, 5, 10, 1, 7, 2}
	fpsTrans  = [7]byte{0, MS_SWIM, MS_LAVA, MS_ICE, MS_FLOAT, MS_FLOAT, 0}
)

// newFPS sets the movement state (-1 keeps the player's) and from it the speeds.
func newFPS(state int) {
	if state == -1 {
		state = player.FPS()
	} else {
		player.MotionState = player.MotionState&0xE0 + fpsTrans[state]
		player.SetFPS(state)
	}
	r := fpsRatios[state]
	pFPS[0] = int16(int(RunFPS) * r / 10)
	pFPS[1] = int16(int(SideFPS) * r / 10)
	pFPS[2] = int16(int(BackFPS) * r / 10)
	if state < 4 {
		pTurn = int16(int(PlayerTurn) * r / 10)
	} else {
		pTurn = PlayerTurn
	}
	if player.MaxWeight != 0 && player.Weight*2 > player.MaxWeight {
		MaxPlayerAccel = int16(0x60 - int(player.Weight)*0x60/(int(player.MaxWeight)*2))
	} else {
		MaxPlayerAccel = 0x60
	}
}
