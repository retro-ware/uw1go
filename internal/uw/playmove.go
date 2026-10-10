package uw

// Port of PLAYMOVE.C: the movement controls, the clock that drives player and object
// physics (checkPhysics, every frame), head bob and the other camera effects, footstep
// and swimming sounds, the noise the player makes, and placing the 3D camera (getEye).
//
// PlayerInput is the movement command (doPlayerInput has the list), ForwInpRate and
// TurnInpRate how fast to go forward and turn. playerMod[0..3] are offsets to the
// camera's height, heading, pitch and bank, used when doMod is set.

var (
	combEfflen  byte // effect MS_SHAKE
	tremEfflen  byte // effect MS_TREMOR
	tsteps      byte
	bobEffect   = [16]int8{1, 3, 4, 3, 1, -3, 0, 0, 1, 3, 4, 3, 1, -3, 0, 0}
	sliEffect   = [16]int8{0, 0, -1, -2, -3, -4, -5, -6, -6, -4, -3, -2, -1, 0, 0, 0}
	rorEffect   = [16]int8{-4, -3, -2, -1, 0, 1, 2, 3, 4, 3, 2, 1, 0, -1, -2, -3}
	PlayerInput int16
	PlayerTurn  int16 = 15
	MoveCamera  bool
	ForwInpRate int16
	TurnInpRate int16
	MoveSide    int16 // the slide keys held with PIN_FORWARD: -1 left, 1 right
	MoveBack    bool  // the back key held with PIN_FORWARD

	moveKeys = [9]byte{0x11, 0x1F, 0x1E, 0x20, 0x2C, 0x2E, 0x2D, 0x12, 0x10}
	// UltimaHacks' movementKeys, the key held for each of moveKeys' meanings (run,
	// walk, turn left, turn right, slide left, slide right, back, fly up, fly down): w, x, the
	// grey Left and Right (0x69 and 0x6A in keyOn: KEY puts E0 keys at 0x60 and up), a, d, s,
	// left Shift, left Ctrl
	modernKeys = [9]byte{0x11, 0x2D, 0x69, 0x6A, 0x1E, 0x20, 0x1F, 0x2A, 0x1D}

	lastTime   uint32
	frameInc   byte
	nextstep   uint32
	stepFoot   bool
	waterEff   = 0xFF
	watertime  uint32
	noiseCount byte

	doMod              bool
	playerMod          [4]int16
	PlayerPitch        int16
	PlayerBank         int16
	campos, camang     [3]int16
	vortRad, vortTimer int16
	vortTheta          int16
	vortX, vortY       byte
)

// parsePlayin sets the movement command of a key binding (the jumps).
func parsePlayin(command int) {
	doPlayerKeyboard()
	switch command {
	case PIN_RUN_JUMP, PIN_JUMP:
		if PN.Terrain&FOOT_AIR == 0 && player.MotionState != MS_SWIM {
			PlayerInput = int16(command)
		} else {
			PlayerInput = PIN_FORWARD
		}
	case PIN_NONE:
		TurnInpRate, ForwInpRate, MoveSide, MoveBack = 0, 0, 0, false
		PlayerInput = int16(command)
	}
}

// doPlayerKeyboard reads the movement keys held down (UltimaHacks' keys, modernKeys), with
// Shift or Ctrl held too (its fly keys) but not with both Alt and Ctrl (the rune keys).
// Running, walking, backing, sliding and turning add up into one PIN_FORWARD, which
// doPlayerInput turns into a direction, where the game took the last key alone: a slide
// stopped a run, so there was no diagonal.
func doPlayerKeyboard() {
	TurnInpRate, ForwInpRate, MoveSide, MoveBack = 0, 0, 0, false
	if altDown() && ctrlDown() {
		return
	}
	for i, k := range moveKeys {
		if keyOn[modernKeys[i]] == 0 {
			continue
		}
		switch k {
		case 0x11:
			ForwInpRate = 0x70
		case 0x1F:
			ForwInpRate = max(ForwInpRate, 0x32)
		case 0x1E:
			TurnInpRate -= 0x5A
		case 0x20:
			TurnInpRate += 0x5A
		case 0x2D:
			MoveBack = true
		case 0x2C:
			MoveSide--
		case 0x2E:
			MoveSide++
		}
		switch {
		case k != 0x12 && k != 0x10:
			PlayerInput = PIN_FORWARD
		case motionbits&(MB_LEVITATE|MB_FLY) == 0: // UltimaHacks' fly keys do nothing on the ground
		case k == 0x12:
			PlayerInput = PIN_UP
		default:
			PlayerInput = PIN_DOWN
		}
	}
	if ForwInpRate != 0 && MoveBack { // forward and back cancel, as in Quake
		ForwInpRate, MoveBack = 0, false
	}
}

// physicsRound charges a round of time and moves everything.
func physicsRound(delta uint32, easy bool) {
	player.GameClock += delta
	lastTime = GameTime()
	frames := int(frameInc)
	if Hasted {
		frames >>= 1
		frameInc &= 1
	} else {
		frameInc = 0
	}
	movePhysics(int(delta), frames, easy)
}

// playerSimpleMove makes a step-key move, runs one 0x40-unit round of physics with it,
// and waits until 0x18 ticks have passed since the key.
func playerSimpleMove(dir int) {
	start := GameTime()
	if simpleFizix(dir) {
		lastTime = GameTime()
		frameInc += 4
		PN.Speed = 0
		if DoAnimO {
			updateAnimobj(1)
		}
		physicsRound(0x40, true)
		editchng(10)
	}
	for GameTime()-start < 0x18 {
	}
	mouseClearQ()
}

// checkPhysics runs the physics for the time since the last call (at most 0x40).
func checkPhysics() {
	delta := GameTime() - lastTime
	var frames byte
	if delta > 0x40 {
		delta = 0x40
		frameInc += 4
		frames = 1
	} else {
		frameInc += byte(GameTime()>>4 - lastTime>>4)
		frames = byte(GameTime()>>6 - lastTime>>6)
	}
	if delta != 0 {
		if frames != 0 && DoAnimO {
			updateAnimobj(int(frames))
		}
		physicsRound(delta, false)
	}
}

// movePhysics is one round: the player, the mobile objects, the screen effects, and the
// player's noise and footsteps.
func movePhysics(incr, frames int, easy bool) {
	doMod = false
	playerMod[0] = 0
	tsteps += byte(incr)
	if PlayerInput == PIN_NONE {
		doPlayerKeyboard()
	}
	if (PlayerInput != PIN_NONE || PN.Speed != 0 || PN.Vel[2] != 0 || PN.Acc[2] != 0 ||
		PN.Acc[1] != 0 || PN.Acc[0] != 0 || fizUpdate) && !easy {
		movePlayer(incr)
	}
	if MoveCrits && !TimeStop && frames != 0 {
		moveMobile(frames)
	}
	if player.MotionState != 0 {
		parseEffect()
	}
	setSound(easy)
	makeNoise(easy)
}

// finishPlayer runs the player's physics until the player is at rest.
func finishPlayer() {
	PlayerInput = PIN_NONE
	for PN.Speed != 0 || PN.Vel[2] != 0 || PN.Acc[2] != 0 || PN.Acc[1] != 0 || PN.Acc[0] != 0 || fizUpdate {
		movePlayer(0x40)
	}
}

// movePlayer is one step of the player's physics, then the head bob, the crouch of a
// jump and the sway of a sideways move.
func movePlayer(incr int) {
	PN.Radius = byte(ComObjData[ThePlayer.Item()].Radius())
	PN.Height = byte(ComObjData[ThePlayer.Item()].Height())
	playerMod[1], playerMod[2], playerMod[3] = 0, 0, 0
	setPlayerPhysParams(incr)
	doPhysics(&PN, &PT)
	physAffectPlayer()
	editchng(10)
	if PN.Terrain&FOOT_AIR == 0 {
		sliding := PlayerInput == PIN_FORWARD && MoveSide != 0 && ForwInpRate == 0 && !MoveBack
		if PN.Speed > pFPS[0]>>2 && PlayerInput == PIN_FORWARD && !sliding && !MoveBack {
			bob := int8(PN.Speed*4/(pFPS[0]>>1) - 1)
			if bob < 2 {
				bob = 2
			}
			doMod = true
			playerMod[0] = int16(bobEffect[tsteps>>4]) * int16(bob)
		}
		if PlayerInput == PIN_JUMP {
			playerMod[0] = -0x20
			playerMod[2] = -0x100
			doMod = true
			PlayerInput = PIN_NONE
		}
		if PlayerInput == PIN_LEFT || PlayerInput == PIN_RIGHT || sliding {
			doMod = true
			playerMod[0] = int16(sliEffect[tsteps>>4]) * 2
		}
	}
	PlayerInput = PIN_NONE
}

func footstep() {
	vol := int(int8(PN.Speed>>5 - 0x10))
	if stepFoot {
		playEffectHere(1, 0x48, vol)
	} else {
		playEffectHere(2, 0x38, vol)
	}
	stepFoot = !stepFoot
}

// makeNoise plays the water loop while swimming and the footsteps.
func makeNoise(easy bool) {
	if player.MotionState&MS_SWIM != 0 {
		if waterEff != 0xFF && watertime+0x1800 <= GameTime() {
			killEffect(waterEff)
			waterEff = 0xFF
		}
		if waterEff == 0xFF {
			watertime = GameTime()
			waterEff = playEffectHere(0, 0x40, 0)
		}
		return
	}
	if waterEff != 0xFF {
		killEffect(waterEff)
		waterEff = 0xFF
	}
	if player.MotionState&MS_FLOAT != 0 || PN.Terrain&FOOT_AIR != 0 {
		return
	}
	if easy {
		footstep()
		nextstep = GameTime() + 100
	} else if PN.Speed != 0 && GameTime() > nextstep {
		footstep()
		delay := uint32(6000/(int(PN.Speed)>>2+1) + 0x40)
		if delay > 200 {
			delay = 200
		}
		nextstep = GameTime() + delay
	}
}

// setSound sets the player's noise and visibility for the critters.
func setSound(easy bool) {
	n := int8(plyNotice[0])
	switch {
	case easy:
		n += 4
	case PN.Speed == 0:
		n = 0
	default:
		n = int8(int(PN.Speed)*10/int(pFPS[0])) + n - 5
	}
	if player.MotionState != 0 {
		n += 4
	}
	if n < 0 {
		n = 0
	} else if n > 15 {
		n = 15
	}
	if playerdat.Noise() > int(n) {
		if noiseCount == 0 {
			playerdat.SetNoise(playerdat.Noise() - 1)
		}
	} else {
		playerdat.SetNoise(int(n))
	}
	noiseCount = (noiseCount + 1) % 8
	playerdat.SetVisibility(int(plyNotice[1]))
}

// parseEffect sets the camera offsets from the movement state: the swimmer's dip and
// roll, lava burns, the float of levitation, and the two timed shakes.
func parseEffect() {
	amp, phase := int8(1), int8(1)
	doMod = true
	editchng(2)
	playerMod[1], playerMod[2], playerMod[3] = 0, 0, 0
	ms := player.MotionState
	if ms&0x11 != 0 {
		playerMod[0] = -int16(player.SwimCount)
		if player.SwimCount > 0x50 {
			amp = int8(PN.Speed*4/(pFPS[0]>>1) - 3)
			if amp < 1 {
				amp = 1
			}
			phase = int8(tsteps >> 4)
			if PN.Speed == 0 {
				playerMod[3] = int16(rand()&0x1FF - 0x100)
			} else {
				playerMod[3] = int16(amp) * int16(rorEffect[phase]) << 6
			}
			playerMod[0] += int16(rorEffect[(phase+2)&0xF]) * 2 * int16(amp)
			playerMod[1] = int16(amp) * int16(rand()&0x7F-0x40)
			playerMod[2] = int16(amp) * int16(rand()&0x7F-0x40)
		}
	}
	if ms&MS_LAVA != 0 && !DragonSkinBoots && rand()%5 == 0 {
		damageItem(ThePlayer, nil, 0, 0, 1, DMG_FIRE)
	}
	if player.MotionState&MS_FLOAT != 0 {
		playerMod[0] = abs16(0x10-int16(tsteps>>3)) * 3
	}
	if player.MotionState&(MS_SHAKE|MS_TREMOR) != 0 {
		if player.MotionState&MS_TREMOR != 0 {
			if tremEfflen == 0 {
				player.MotionState ^= MS_TREMOR
				editchng(2)
			}
			tremEfflen--
			amp = int8(tremEfflen / 10)
			if amp > 8 {
				amp = 8
			}
		}
		if player.MotionState&MS_SHAKE != 0 {
			if combEfflen == 0 {
				player.MotionState ^= MS_SHAKE
				editchng(2)
			}
			combEfflen--
			phase = int8(combEfflen / 8)
			if phase > 3 {
				phase = 3
			}
		}
		amp += phase
		playerMod[1] += int16(amp) * int16(rand()&0xFF-0x80)
		playerMod[2] += int16(amp) * int16(rand()&0x7F-0x40)
		playerMod[3] += int16(amp) * int16(rand()&0x1FF-0x100)
	}
}

// setEffect starts a timed shake, MS_SHAKE or MS_TREMOR, for amount rounds.
func setEffect(which byte, amount int) {
	switch which {
	case MS_SHAKE:
		combEfflen = byte(amount)
	case MS_TREMOR:
		tremEfflen = byte(amount)
	default:
		return
	}
	player.MotionState |= which
}

// getEye places the 3D camera by UsPtr: the player's eye with the effect offsets, the
// free camera, another mobile object, behind the player, or the moongate vortex.
func getEye() {
	c := &cPlayer
	switch {
	case UsPtr == ThePlayer:
		c.x, c.y, c.z = PN.X, PN.Y, PN.Z+0xA4
		c.heading, c.pitch, c.bank = PlayerFacing, PlayerPitch, PlayerBank
		if doMod {
			c.z += playerMod[0]
			if c.z > 1000 {
				c.z = 1000
			}
			c.heading += playerMod[1]
			c.pitch += playerMod[2]
			c.bank += playerMod[3]
		}
	case UsPtr != nil:
		c.x = int16(UsPtr.HomeX()<<8 + UsPtr.FineX()<<5)
		c.y = int16(UsPtr.HomeY()<<8 + UsPtr.FineY()<<5)
		c.z = int16(UsPtr.Z()<<3 + 0xB0)
		c.heading = int16(UsPtr.HeadingBits()<<13 + UsPtr.FineHead()<<8)
	case usPseudo == 0:
		c.x, c.y, c.z = campos[0], campos[1], campos[2]
		c.heading, c.pitch, c.bank = camang[0], camang[1], camang[2]
	case usPseudo == -1:
		x, y := cSinCos(int(uint16(PlayerFacing)))
		c.x = PN.X - x>>7
		c.y = PN.Y - y>>7
		c.z = PN.Z + 0x148
		c.heading, c.pitch, c.bank = PlayerFacing, PlayerPitch, PlayerBank
	case usPseudo == -2:
		x, y := cSinCos(int(uint16(vortTheta)))
		x /= 0x100
		y /= 0x100
		x = x * (0x40 - vortTimer) / 0x40
		y = y * (0x40 - vortTimer) / 0x40
		c.x = int16(vortX)<<8 + x*vortRad/2 + 0x80
		c.y = int16(vortY)<<8 + y*vortRad/2 + 0x80
		c.z = PN.Z + 0xA4 - vortTimer*2
		c.heading = vortTheta + 0x7FFF
		c.pitch = 0
		c.bank = vortTimer << 11
	}
}
