package uw

// Port of MOTION.C: stepping a physics record through the tile grid, bouncing off walls,
// floors and objects. Everything that moves under physics comes through doPhysics: the
// player (PN with PT), critters (CN1, CN2, CN4 with CT1, CT2, CT4) and thrown or falling
// objects and missiles (CN3 with CT3).
//
// A move is a Bresenham walk along the larger of the x and y velocities (MP), one 1/8
// tile cell per step, with the minor axis and z carried as fractions. After each step
// checkPositions asks COLLIDE what the mover now overlaps (getPcoll's state bits) and
// reacts. The arithmetic keeps the original's 16-bit integer behaviour.

// Phys is a physics record.
type Phys struct {
	X, Y, Z int16 // x and y in 1/256 tiles, z in 1/8 of an object's z unit
	Vel     [3]int16
	Acc     [3]int16 // Acc[2] is gravity
	Time    int16
	Speed   int16
	Bounce  byte
	Flags   byte // 0x80 slide along walls, 0x40 never turn
	Mass    int16
	Light   byte
	HP      byte
	Resist  byte
	B1D     byte
	Heading int16
	Index   int16
	Radius  byte
	Height  byte
	B24     byte // the step height
	Terrain byte // FOOT_* bits
	Impact  uint16
}

// PhysHandler is a mover's handler: which collision bits to ignore, which to pass to
// special, and which stop it climbing onto objects.
type PhysHandler struct {
	ignore  uint16
	mask    uint16
	noclimb uint16
	w6      uint16
	special func(state *uint16) bool
}

// MotionParams is the stepping state.
type MotionParams struct {
	frac     [3]int16
	step     [3]int16
	major    int16
	minor    int16
	steps    int16
	rem      int16
	dt       int16
	done     int16
	hit      int8
	item     int16
	targz    int16
	f21      int16
	f23      int16
	zspeed   int16
	headings [8]uint16
}

var (
	Ppd                    MotionCalc
	bounceFlag             int8
	PN, CN1, CN2, CN3, CN4 Phys
	hitFlag                bool
	CP                     *Phys
	targCeil               bool
	terrType               byte
	PT, CT1, CT2, CT3, CT4 PhysHandler
	TP                     *PhysHandler

	resToTerr = [18]byte{0, 0, 1, 0, 2, 0, 0, 0, 3, 0, 0, 0, 0, 0, 0, 0, 4}

	MP = MotionParams{headings: [8]uint16{0, 0xE000, 0xC000, 0xA000, 0x8000, 0x6000, 0x4000, 0x2000}}
)

// mpPos is Ppd's x, y and z (MP.pos).
func mpPos(i int16) *int16 {
	switch i {
	case 0:
		return &Ppd.x
	case 1:
		return &Ppd.y
	}
	return &Ppd.z
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}

// doPhysics moves pp for pp.Time ticks under the handler tp. It gives up after 16 steps
// that collided and then stops the mover.
func doPhysics(pp *Phys, tp *PhysHandler) {
	trycnt := 0
	CP = pp
	TP = tp
	terrType = CP.Terrain
	bounceFlag = 0
	if int8(spaceToMotion(true, true)) == 0 {
		return
	}
	for MP.steps+1 > MP.done {
		if trycnt == 0x10 {
			CP.Speed, CP.Vel[2], CP.Acc[2] = 0, 0, 0
			return
		}
		trycnt++
		if gridMove(1) != 0 {
			checkPositions()
		}
	}
	backToSpace()
}

// spaceToPos loads Ppd and MP's cell position from CP.
func spaceToPos() {
	curP = &Ppd
	Ppd.heading = uint16(CP.Heading)
	Ppd.radius = CP.Radius
	Ppd.height = CP.Height
	Ppd.index = CP.Index
	Ppd.x = CP.X >> 5
	Ppd.y = CP.Y >> 5
	Ppd.z = CP.Z >> 3
	MP.frac[0] = (CP.X & 0x1F) << 8
	MP.frac[1] = (CP.Y & 0x1F) << 8
	MP.frac[2] = (CP.Z & 7) << 8
}

func collItem(i int) int { return ObjIntTMem(oCollisions[i].link.Index()).Item() }

// setTargz chooses the height the mover will stop at in z, MP.targz, and what it would
// land on or hit, MP.hit and MP.item.
func setTargz(how bool) {
	targCeil = true
	processObjlist()
	MP.hit = -1
	MP.f23 = 0x7F
	found := int(Ppd.found)
	first := int(Ppd.first)
	count := int(Ppd.count)
	switch {
	case CP.Vel[2] > 0:
		if found > 0 && first+count < found && first >= 0 {
			MP.targz = int16(oCollisions[first+count].bottom) - int16(CP.Height)
			MP.hit = int8(first + count)
		} else {
			MP.targz = 0x80 - int16(CP.Height)
		}
		targCeil = false
		if int(Ppd.z)+int(CP.Radius) < int(Ppd.top) {
			MP.targz = int16(Ppd.top)
			targCeil = true
		}
	case CP.Vel[2] == 0:
		if int(Ppd.z)+int(CP.B24) >= int(Ppd.top) {
			MP.targz = int16(Ppd.top)
		} else {
			MP.targz = int16(Ppd.floor)
		}
		solid := count == 0 && first > 0 && first <= found
		for i := 0; i < found; i++ {
			if ComObjData[collItem(i)].Touch() == 0 {
				continue
			}
			c := &oCollisions[i]
			if first <= i {
				if int16(c.bottom)-1 < MP.f23 {
					MP.f23 = int16(c.bottom) - 1
				}
				if first+count > i && int16(c.top) > MP.targz && int(c.top) < 0x80-int(CP.Height) {
					MP.hit = int8(i)
					MP.targz = int16(c.top)
				}
			} else if solid && int16(c.top) >= MP.targz {
				MP.targz = int16(c.top)
				MP.hit = int8(i)
			}
		}
	default:
		MP.targz = int16(Ppd.top)
		if first > 0 && first <= found && int16(oCollisions[first-1].top) > MP.targz {
			MP.targz = int16(oCollisions[first-1].top)
			MP.hit = int8(first - 1)
		}
		targCeil = false
	}
	if MP.hit != -1 {
		MP.item = int16(ObjPtrTMem(&oCollisions[MP.hit].link).Item())
	}
}

// spaceToMotion sets up the step walk for the rest of CP's time. It returns 0 when
// nothing moves.
func spaceToMotion(pos, check bool) byte {
	s, c := cSinCos(int(uint16(CP.Heading)))
	CP.Vel[0] = int16(int32(s) * int32(CP.Speed) >> 15)
	CP.Vel[1] = int16(int32(c) * int32(CP.Speed) >> 15)
	CP.Vel[0] += CP.Acc[0] * CP.Time
	CP.Vel[1] += CP.Acc[1] * CP.Time
	CP.Vel[2] += CP.Acc[2] * CP.Time
	vel := &CP.Vel
	if vel[0]|vel[1]|vel[2] == 0 {
		return 0
	}
	if pos {
		spaceToPos()
	}
	if abs16(CP.Vel[0]) > abs16(CP.Vel[1]) {
		MP.major = 0
	} else {
		MP.major = 1
	}
	MP.minor = (MP.major + 1) % 2
	if vel[MP.major] > 0 {
		MP.step[MP.major] = 0x2000
	} else {
		MP.step[MP.major] = -0x2000
	}
	if vel[MP.major] != 0 {
		MP.step[MP.minor] = MP.step[MP.major] / 0x100 * vel[MP.minor] / vel[MP.major] << 8
		t := abs16(vel[MP.major] * CP.Time)
		MP.steps = int16(uint16(t) >> 13)
		if t < 0 {
			MP.steps = t >> 13
		}
		MP.rem = t & 0x1FFF
		MP.dt = abs16(0x2000 / vel[MP.major])
	} else {
		MP.step[MP.minor] = 1
		MP.step[MP.major] = 1
		MP.steps = 0
		MP.rem = 0
		MP.dt = CP.Time
	}
	MP.done = 0
	if (Ppd.index == 1 || vel[2] != 0) && check {
		TerrainCheck(CP.B24)
		ObjectCheck(false, false)
	}
	if vel[2] != 0 {
		if vel[2] > 0 {
			MP.step[2] = 0x800
		} else {
			MP.step[2] = -0x800
		}
		setTargz(check)
		if vel[MP.major] != 0 {
			a := int32(vel[2])
			a *= int32(MP.step[MP.major] / 0x2000)
			b := int32(vel[MP.major])
			b *= int32(MP.step[2] / 0x800)
			a *= 0x100
			a /= b
			if a > 0x7FFF || a < -0x8000 {
				MP.rem = 0
				MP.zspeed = abs16((vel[2] >> 1) * CP.Time >> 5)
			} else {
				MP.zspeed = int16(a)
			}
		} else {
			MP.zspeed = abs16((vel[2] >> 1) * CP.Time >> 5)
		}
	} else {
		MP.step[2] = 0
	}
	return 1
}

// recalcVecs charges the steps taken against CP.Time and sets up the walk again.
func recalcVecs(how bool) {
	CP.Time -= MP.dt * MP.done
	if CP.Time <= 0 || spaceToMotion(false, how) == 0 {
		MP.done = MP.steps + 1
	}
}

// stopMe stops the mover dead.
func stopMe() {
	CP.Acc = [3]int16{}
	CP.Vel = [3]int16{}
	CP.Speed = 0
	MP.done = MP.steps + 1
}

// backToSpace writes the cell position back into the physics record.
func backToSpace() {
	CP.X = Ppd.x<<5 + MP.frac[0]>>8
	CP.Y = Ppd.y<<5 + MP.frac[1]>>8
	CP.Z = Ppd.z<<3 + MP.frac[2]>>8
	if Ppd.hits0&0x2000 != 0 && abs16(Ppd.z-int16(Ppd.floor)) <= int16(CP.Radius) &&
		Ppd.index == 1 && CP.Vel[2] == 0 {
		CP.Z = int16(GetSlopeHgt(int(CP.X), int(CP.Y)))
	}
	CP.Heading = int16(Ppd.heading)
}

func carry(i int16) bool {
	if MP.frac[i]&-0x2000 != 0 {
		if MP.frac[i] > 0 {
			*mpPos(i)++
		} else {
			*mpPos(i)--
		}
		MP.frac[i] &= 0x1FFF
		return true
	}
	return false
}

// flatMove takes one step in x and y, forward (dir 1) or back (dir -1).
func flatMove(crossed bool, dir int16) byte {
	back := int16(0)
	if dir == -1 {
		back = 1
	}
	if MP.steps+back > MP.done {
		if dir == 1 {
			MP.frac[MP.minor] += MP.step[MP.minor]
		} else {
			MP.frac[MP.minor] -= MP.step[MP.minor]
		}
		if MP.step[MP.major]*dir > 0 {
			*mpPos(MP.major)++
		} else {
			*mpPos(MP.major)--
		}
		carry(MP.minor)
		MP.done += dir
		return 1
	}
	d := (MP.step[MP.minor] >> 5) * (MP.rem >> 8)
	if dir == 1 {
		MP.frac[MP.minor] += d
	} else {
		MP.frac[MP.minor] -= d
	}
	if MP.step[MP.major]*dir > 0 {
		MP.frac[MP.major] += MP.rem
	} else {
		MP.frac[MP.major] -= MP.rem
	}
	if carry(0) {
		crossed = true
	}
	if carry(1) {
		crossed = true
	}
	MP.done += dir
	return byte(b2i(crossed))
}

// rehead bounces CP off a wall whose normal is heading. It returns false when the mover
// cannot be redirected.
func rehead(heading uint16) bool {
	if CP.Flags&0x40 != 0 {
		return false
	}
	if CP.Vel[2] != 0 {
		d := CP.Vel[2] / 16
		d *= int16(CP.Bounce) + 1
		CP.Vel[2] = d
	}
	diff := int16(heading - Ppd.heading)
	if diff > 0x4000 || diff < -0x4000 {
		heading += 0x8000
		diff += -0x8000
	}
	ad := abs16(diff)
	if CP.Flags&0x80 != 0 {
		if ad > 0x3000 && ad < 0x5000 {
			CP.Impact += uint16(CP.Speed)
			return false
		}
		if Ppd.heading != heading {
			Ppd.heading = heading
		} else {
			gy := int16(1)
			gx := int16(0)
			if heading&0x4000 != 0 {
				gx = 1
			}
			if rand()%2 != 0 {
				gx, gy = 1-gx, 1-gy
			}
			MP.frac[0] = gx * 0x1F00
			MP.frac[1] = gy * 0x1F00
		}
	} else if ad > 0x3000 && ad < 0x5000 {
		Ppd.heading = heading + uint16(diff)
	} else {
		t := diff / 15
		Ppd.heading = heading + uint16(t*int16(CP.Bounce))
	}
	if CP.Flags&0x80 == 0 {
		CP.Impact += uint16(CP.Speed * (15 - int16(CP.Bounce)) / 15)
		CP.Speed = CP.Speed * int16(CP.Bounce) / 15
	}
	CP.Heading = int16(Ppd.heading)
	return true
}

// do2dbounce steps back from a wall and turns the mover with rehead.
func do2dbounce(how bool) {
	if bounceFlag > 0 {
		gridMove(-1)
	} else {
		var h int
		if how {
			ComputeHeading()
			h = int(Ppd.slope)
			if h == 9 {
				h = int(MP.major) * 2
			}
		} else {
			h = int(MP.major) * 2
		}
		gridMove(-1)
		if rehead(MP.headings[h&7]) {
			recalcVecs(true)
			bounceFlag = 2
			return
		}
	}
	MP.done = MP.steps + 1
}

// bounceThatGuy throws a mover that landed on something it cannot rest on up and sideways.
func bounceThatGuy() {
	CP.Vel[2] = 0xEB
	CP.Acc[2] = -4
	if CP.Speed < 0xEB {
		CP.Speed = 0xEB
	}
	CP.Terrain = FOOT_AIR
	CP.Heading -= 0x3000
	CP.Heading += int16(rand() % 0x6000)
}

// doZbounce: the mover reached MP.targz: it lands, hits an object, or bounces.
func doZbounce() {
	mass := ComObjData[ObjIntTMem(int(Ppd.index)).Item()].Mass()
	if MP.zspeed > 4 {
		dz := abs16(Ppd.z-MP.targz) * MP.dt
		dz = (dz << 4) / (MP.zspeed / 4)
		CP.Time -= dz
	} else {
		CP.Time = 0
	}
	Ppd.z = MP.targz
	MP.frac[2] = 0
	if MP.hit == -1 && Ppd.hits0&1 != 0 && int(Ppd.floor)+int(Ppd.radius) >= int(Ppd.z) && CP.Vel[2] < 0 {
		stopMe()
		CP.Terrain = FOOT_WATER
		playEffect(5, int(CP.X>>5), int(CP.Y>>5), (mass-600)/50)
		return
	}
	vol := int(abs16(CP.Vel[2]))/10 + (mass-600)/50 - 40
	playEffect(0xF, int(CP.X>>5), int(CP.Y>>5), vol)
	r := doObjhit(int(MP.hit), int(Ppd.index))
	if r&0x18 != 0 {
		if r&0x10 != 0 {
			stopMe()
		} else {
			MP.done = MP.steps + 1
		}
		return
	}
	if r&4 != 0 {
		up := CP.Vel[2] <= 0
		CP.Vel[2] = -(CP.Vel[2] / 15)
		CP.Impact = uint16(abs16(CP.Vel[2] * (15 - int16(CP.Bounce))))
		CP.Vel[2] *= int16(CP.Bounce)
		if CP.Bounce != 0 {
			CP.Speed -= (15 - int16(CP.Bounce)) * CP.Speed / 30
		} else {
			CP.Speed = 0
		}
		if up && CP.Vel[2] < 0x8D {
			CP.Vel[2] = 0
			CP.Acc[2] = 0
			creature := ObjIntTMem(int(CP.Index)).Major() == MAJOR_CREATURE
			switch {
			case MP.hit != -1:
				if ComObjData[collItem(int(MP.hit))].Solid() != 0 || creature {
					CP.Terrain = FOOT_FLOOR
				} else {
					bounceThatGuy()
				}
			case int(Ppd.floor)+int(Ppd.radius) >= int(Ppd.z):
				CP.Terrain = 1 << (Ppd.hits0 & 3)
			case !creature:
				bounceThatGuy()
			case Ppd.hits1&0x10 != 0:
				CP.Terrain = FOOT_WATER
			case Ppd.hits1&0x20 != 0:
				CP.Terrain = FOOT_LAVA
			default:
				CP.Terrain = FOOT_FLOOR
			}
		}
		recalcVecs(false)
	} else {
		if MP.hit != -1 {
			Ppd.found--
			oCollisions[MP.hit] = oCollisions[Ppd.found]
		}
		recalcVecs(false)
	}
}

// fullMove takes one step with vertical motion.
func fullMove(crossed bool, dir int16) byte {
	back := int16(0)
	if dir == -1 {
		back = 1
	}
	var z int32
	switch {
	case MP.steps+back > MP.done:
		if dir*MP.step[2] > 0 {
			z = int32(MP.zspeed << 5)
		} else {
			z = -int32(MP.zspeed << 5)
		}
	case MP.rem != 0:
		z = int32(MP.rem)
		z *= int32(dir * (MP.step[2] / 0x800) * MP.zspeed)
		z /= 0x100
	default:
		z = 0x40
		z *= int32(dir * (MP.step[2] / 0x800) * MP.zspeed)
	}
	z += int32(MP.frac[2])
	var dz int16
	if z >= 0 {
		dz = int16(z / 0x800)
	} else {
		dz = -int16(-z/0x800 + 1)
	}
	MP.frac[2] = int16(z & 0x7FF)
	switch {
	case dir == -1:
		Ppd.z += dz
	case dz > 0:
		CP.Terrain = FOOT_AIR
		if Ppd.z+dz <= MP.targz {
			Ppd.z += dz
		} else {
			doZbounce()
			return 0
		}
	case dz < 0:
		CP.Terrain = FOOT_AIR
		if Ppd.z+dz >= MP.targz {
			Ppd.z += dz
		} else {
			doZbounce()
			return 0
		}
	}
	return flatMove(crossed, dir)
}

// gridMove takes one step forward (dir 1) or back (dir -1).
func gridMove(dir int16) byte {
	check := false
	if dir == -1 {
		ObjectCheck(false, false)
		setTargz(false)
		CP.Terrain = terrType
		check = hitFlag
	} else {
		bounceFlag--
	}
	var r byte
	if CP.Vel[2] != 0 {
		r = fullMove(false, dir)
	} else {
		r = flatMove(false, dir)
	}
	if check {
		state := getPcoll()
		terrType = CP.Terrain
		CP.Terrain = setResterr(uint16(state))
	}
	return r
}

// setResterr turns a collision state word into the terrain byte.
func setResterr(bits uint16) byte {
	if bits&0x1000 != 0 {
		return FOOT_AIR
	}
	if bits&4 != 0 {
		if Ppd.index == 1 && bits&3 == TERRAIN_WATER && bits&0xF8 != bits&0x90 {
			return FOOT_SHORE
		}
		return 1 << (bits & 3)
	}
	if bits&0x88 == 0 {
		if bits&0x10 != 0 {
			return FOOT_WATER
		}
		if bits&0x20 != 0 {
			return FOOT_LAVA
		}
		return FOOT_ICE
	}
	return FOOT_FLOOR
}

// getPcoll is what the mover overlaps at its new cell, as the collision state bits.
func getPcoll() uint16 {
	ok := false
	climb := TP.noclimb&0x80 == 0
	hitFlag = false
	TerrainCheck(CP.B24)
	ObjectCheck(false, false)
	setTargz(false)
	state := uint16(Ppd.hits0 | Ppd.hits1)
	step := state&TP.noclimb == 0
	if Ppd.found > 0 {
		for i := int(Ppd.first); int(Ppd.first)+int(Ppd.count) > i; i++ {
			r := doObjhit(i, int(Ppd.index))
			if r&4 != 0 {
				state |= 0x400
			}
			if r&0x18 != 0 {
				if r&0x10 != 0 {
					state |= 0x4000
				} else {
					state |= 0x8000
				}
				return state
			}
		}
	}
	if Ppd.z != MP.targz {
		if MP.hit == -1 && step {
			ok = abs16(Ppd.z-MP.targz) <= int16(CP.B24)
			ok = ok || CP.Vel[2] == 0 && Ppd.hits1&0x800 == 0 && Ppd.hits0&4 != 0
		} else if climb {
			ok = ComObjData[MP.item].Solid() == 1
			if ok {
				ok = abs16(Ppd.z-int16(oCollisions[MP.hit].top)) <= int16(CP.B24)
				if ok {
					state |= 0x80
				}
			}
		}
		ok = ok && targCeil
		if ok && MP.targz >= MP.f23 {
			state &^= 0x100
		} else if !ok && int16(Ppd.top) > Ppd.z {
			state |= 0x100
		}
		if ok && int(MP.targz)+int(CP.Height) > 0x7F {
			ok = false
			hitFlag = true
			state |= 0x200
		} else if ok && MP.hit == -1 && MP.targz+int16(CP.Height) > MP.f23 {
			ok = false
			hitFlag = true
			state |= 0x400
		}
		if !(ok && state&0x400 != 0) && ok && MP.hit != -1 &&
			(ComObjData[MP.item].Solid() == 0 || !climb) {
			ok = false
		}
		if ok {
			hitFlag = true
			Ppd.z = MP.targz
			if abs16(MP.targz-int16(Ppd.floor)) <= int16(CP.Radius) {
				state |= 4
			} else {
				state &^= 4
			}
		}
	} else if MP.hit != -1 && ComObjData[MP.item].Solid() == 1 && climb {
		state |= 0x80
		state &^= 4
	}
	if hitFlag && ok {
		processObjlist()
		state &^= 0x400
		for i := 0; i < int(Ppd.count); i++ {
			if doObjhit(i, int(Ppd.index))&4 != 0 {
				state |= 0x400
			}
		}
	}
	if state&0x80 != 0 && climb {
		low := 0
		if MP.hit >= 0 {
			low = oCollisions[MP.hit].link.Low()
		}
		if low&0x10 != 0 || Ppd.hits0&4 != 0 {
			state &^= 0x800
		}
	}
	if Ppd.hits1&0x100 != 0 && step && CP.Vel[2] == 0 && int(Ppd.z)+int(CP.B24) >= int(Ppd.top) {
		state &^= 0x100
		Ppd.z = int16(Ppd.top)
		if Ppd.z == int16(Ppd.floor) {
			state |= 4
		} else {
			state &^= 4
		}
	}
	if state&0xFC == 0 {
		state |= 0x1000
	}
	if state&0x80 == 0 && int(Ppd.z)-int(CP.Radius) > int(Ppd.top) {
		state |= 0x1000
	}
	return state
}

// checkPositions reacts to the collision state after each step.
func checkPositions() {
	bounce2d := false
	state := getPcoll()
	terrType = CP.Terrain
	CP.Terrain = setResterr(state)
	if state&0xC000 != 0 {
		gridMove(-1)
		if state&0x4000 != 0 {
			stopMe()
		} else {
			MP.done = MP.steps + 1
		}
		return
	}
	state &^= TP.ignore
	if state != 0 {
		if state&TP.mask != 0 && TP.special != nil && TP.special(&state) {
			gridMove(-1)
			MP.done = MP.steps + 1
			return
		}
		if state&0x700 != 0 {
			do2dbounce(state&0x400 == 0)
			bounce2d = true
		}
		if state&0x1000 != 0 && CP.Acc[2] == 0 {
			CP.Acc[2] = -4
			recalcVecs(bounce2d)
		}
	}
}

// IsaDoor is the first object in the mover's way that is a closed door, with its tile.
func IsaDoor(x, y *int) *Object {
	for i := 0; i < int(Ppd.count); i++ {
		c := &oCollisions[i+int(Ppd.first)]
		obj := ObjPtrTMem(&c.link)
		item := obj.Item()
		t := int(c.offset) & MAP_MASK
		*x = (int(Ppd.x>>3) + t) & MAP_MASK
		t = *x - int(Ppd.x>>3)
		*y = (int(Ppd.y>>3) + (int(c.offset)-t)/MAP_SIZE) & MAP_MASK
		if item>>4 == CLASS_DOOR && item&ID_INCLASS < 8 {
			return obj
		}
	}
	return nil
}

// CollObject is the first object in the mover's way, or nil.
func CollObject() *Object {
	if Ppd.count != 0 {
		return ObjPtrTMem(&oCollisions[Ppd.first].link)
	}
	return nil
}
