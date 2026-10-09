package uw

import (
	"bytes"
	"encoding/binary"
)

// Port of OBJCLASS.C with the class loaders of HACK.C, MISC.C, CREATURE.C, ANIMOBJ.C and
// TRIGGER.C's trap_init: DATA\OBJECTS.DAT's tables and DATA\COMOBJ.DAT.

type MissileInfo struct {
	Damage byte
	Type   byte
	Ammo   int8
}

type Weapon struct {
	Damage     [3]byte
	MinCharge  byte
	Speed      byte
	MaxCharge  byte
	Skill      byte
	Durability byte
}

type Armour struct {
	Protection byte
	Durability byte
	B2         byte
	Category   byte
}

type Container struct {
	Capacity byte
	Mask     int16
}

type Light struct {
	Duration byte
	Pad      byte
}

type AnimClass struct {
	Flags uint16
	Start int8
	Count byte
}

var (
	ActiveObj  *Object
	ComObjData [512]ComObj

	Missile    [16]MissileInfo
	Weapons    [16]Weapon
	Armor      [32]Armour
	Containers [16]Container
	Lights     [16]Light
	Food       [16]byte
	Creatures  [NUM_CREATURES]Creature
	animclassd [16]AnimClass
	Triggers   [16]byte

	crType, crClass int
	cst             *Creature
)

func readInto(r *bytes.Reader, v any) { binary.Read(r, binary.LittleEndian, v) }

// initObjects reads OBJECTS.DAT and COMOBJ.DAT; 0 or ERR_READ with 5 or 6.
func initObjects() int {
	b := readFile("DATA\\objects.dat")
	if b == nil {
		return ERR_READ | 0x5
	}
	r := bytes.NewReader(b[2:])
	readInto(r, &Weapons)
	readInto(r, &Missile)
	readInto(r, &Armor)
	readInto(r, &Creatures)
	readInto(r, &Containers)
	readInto(r, &Lights)
	readInto(r, &Food)
	readInto(r, &Triggers)
	readInto(r, &animclassd)
	b = readFile("DATA\\comobj.dat")
	if b == nil {
		return ERR_READ | 0x6
	}
	for i := range ComObjData {
		if 2+i*11+11 <= len(b) {
			copy(ComObjData[i][:], b[2+i*11:])
		}
	}
	return 0
}

func encodeRec(v any) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, v)
	return b.Bytes()
}

// getClassData returns ActiveObj's class record as bytes, or nil for a class without one.
func getClassData() []byte {
	sub := ActiveObj.InClass()
	switch ActiveObj.Major() {
	case 0:
		switch ActiveObj.Minor() {
		case 1:
			return encodeRec(&Missile[sub])
		case 0:
			return encodeRec(&Weapons[sub])
		case 3:
			return encodeRec(&Armor[sub+16])
		case 2:
			return encodeRec(&Armor[sub])
		}
		return nil
	case 1:
		return encodeRec(creatureClassData())
	case 2:
		switch ActiveObj.Minor() {
		case 0:
			return encodeRec(&Containers[sub])
		case 1:
			return encodeRec(&Lights[sub])
		case 2:
			return nil
		}
		return Food[sub : sub+1]
	case 6:
		if ActiveObj.Minor() == 2 {
			return Triggers[sub : sub+1]
		}
		return nil
	case 7:
		return encodeRec(&animclassd[sub])
	}
	return nil
}

func creatureClassData() *Creature {
	crClass = ActiveObj.Minor()
	crType = ActiveObj.InClass()
	return &Creatures[crClass*16+crType]
}

// creatureObjInit gives ActiveObj a new critter's starting state.
func creatureObjInit() bool {
	a := ActiveObj
	x, y := 0x20, 0x20
	a.SetHomeX(x)
	a.SetHomeY(y)
	a.SetQuality(x)
	a.SetOwner(y)
	cst = &Creatures[a.InMajor()]
	a.HP = byte(int(cst.Avghit) * (rand()%0x18 + 0x10) / 0x20)
	a.Heading = byte(a.HeadingBits() << 5)
	a.SetGoal(GOAL_MILL)
	a.SetGTarg(0)
	a.SetOldGoal(GOAL_STAND)
	a.SetDestX(0)
	a.SetDestY(0)
	a.SetTargetZ(0)
	a.SetNoHeal(0)
	a.SetPowerful(0)
	a.SetB0D11(0)
	a.SetTemp(0)
	a.SetB185(0)
	a.SetAtkFrame(0)
	a.SetBin(0)
	a.SetRate(4)
	a.SetSeq(SEQ_STAND)
	a.SetFrame(0)
	a.SetPitch(0x10)
	a.SetGravity(0)
	a.SetSpeed(0)
	a.B11 = 0
	a.LastHit = 0
	a.SetB157(0)
	a.SetB187(0)
	a.SetB186(0)
	a.SetPath(0)
	a.SetB156(0)
	a.Whoami = 0
	a.SetB190(0)
	a.SetB191(0)
	a.SetB194(0)
	a.SetB195(0)
	a.SetAlly(0)
	a.SetFed(0)
	a.SetHasInv(0)
	a.SetTalkedTo(0)
	a.SetAttitude(ATT_MELLOW)
	a.SetLoner(0)
	a.SetCast(0)
	return true
}
