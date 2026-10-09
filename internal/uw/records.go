package uw

import (
	"bytes"
	"encoding/binary"
)

// The player record (player.h's struct Player, 0xD2 bytes as PLAYER.DAT holds it), a
// creature type (critter.h's struct Creature, 48 bytes) and an object type's common
// properties (object.h's struct ComObj, 11 bytes). Plain fields are fields; each run of
// bitfields is kept as the bytes it packs into, with accessors.

type Player struct {
	Name              [0x1E]byte
	Strength          byte
	Dexterity         byte
	Intelligence      byte
	Skills            [20]byte
	Health            byte
	MaxHealth         byte
	PlayMana          byte
	MaxMana           byte
	Hunger            byte
	Fatigue           byte
	FoodHeal          byte
	B3C               byte
	Level             byte
	Spells            [3]uint16
	Runebag           [3]byte
	Shelf             [3]byte
	Weight            uint16
	MaxWeight         uint16
	Exp               uint32
	SkillPoints       byte
	SkillPointsEarned byte
	SavedX            int16
	SavedY            int16
	SavedZ            int16
	SavedFacing       int16
	SavedLevel        int16
	B5E               byte   // moonstone:4, tree:4
	W5F               uint16 // b5F_0, drawn, poison:4, active_spells:4, nrunes:2, armageddon, orb, key, cup
	W61               uint16 // incense:2, shrooms:2, drunk:6, talisman_ok, garamon, maze, b62_5:3
	Light             byte
	B64               byte // lefty, female, body:3, pclass:3
	Quests            int32
	QuestBytes        [4]byte
	Talismans         byte
	Dreams            uint16
	GameVars          [0x40]byte
	SavedMana         byte
	BB1               [3]byte
	Easy              byte
	BB5               byte   // sound:2, music:2, detail:4
	WB6               uint16 // fps:3, terrain:8, bB7_3:5
	MotionState       byte
	SwimCount         byte
	Crithit           byte
	Typehit           byte
	Crithittime       int32
	Hitx              byte
	Hity              byte
	Lore              [8]byte
	BCA               [4]byte
	GameClock         uint32
}

const playerSize = 0xD2

func init() {
	if binary.Size(Player{}) != playerSize || binary.Size(Creature{}) != 0x30 {
		panic("record sizes")
	}
}

func (p *Player) Bytes() []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, p)
	return b.Bytes()
}

func (p *Player) SetBytes(b []byte) {
	binary.Read(bytes.NewReader(b[:playerSize]), binary.LittleEndian, p)
}

func bits8(v byte, shift, width uint) int { return int(v>>shift) & (1<<width - 1) }
func setBits8(v *byte, shift, width uint, x int) {
	m := byte(1<<width-1) << shift
	*v = *v&^m | byte(x<<shift)&m
}
func bits16(v uint16, shift, width uint) int { return int(v>>shift) & (1<<width - 1) }
func setBits16(v *uint16, shift, width uint, x int) {
	m := uint16(1<<width-1) << shift
	*v = *v&^m | uint16(x<<shift)&m
}

func (p *Player) Moonstone() int        { return bits8(p.B5E, 0, 4) }
func (p *Player) SetMoonstone(v int)    { setBits8(&p.B5E, 0, 4, v) }
func (p *Player) Tree() int             { return bits8(p.B5E, 4, 4) }
func (p *Player) SetTree(v int)         { setBits8(&p.B5E, 4, 4, v) }
func (p *Player) Drawn() int            { return bits16(p.W5F, 1, 1) }
func (p *Player) SetDrawn(v int)        { setBits16(&p.W5F, 1, 1, v) }
func (p *Player) Poison() int           { return bits16(p.W5F, 2, 4) }
func (p *Player) SetPoison(v int)       { setBits16(&p.W5F, 2, 4, v) }
func (p *Player) ActiveSpells() int     { return bits16(p.W5F, 6, 4) }
func (p *Player) SetActiveSpells(v int) { setBits16(&p.W5F, 6, 4, v) }
func (p *Player) NRunes() int           { return bits16(p.W5F, 10, 2) }
func (p *Player) SetNRunes(v int)       { setBits16(&p.W5F, 10, 2, v) }
func (p *Player) Armageddon() int       { return bits16(p.W5F, 12, 1) }
func (p *Player) SetArmageddon(v int)   { setBits16(&p.W5F, 12, 1, v) }
func (p *Player) Orb() int              { return bits16(p.W5F, 13, 1) }
func (p *Player) SetOrb(v int)          { setBits16(&p.W5F, 13, 1, v) }
func (p *Player) Key() int              { return bits16(p.W5F, 14, 1) }
func (p *Player) SetKey(v int)          { setBits16(&p.W5F, 14, 1, v) }
func (p *Player) Cup() int              { return bits16(p.W5F, 15, 1) }
func (p *Player) SetCup(v int)          { setBits16(&p.W5F, 15, 1, v) }
func (p *Player) Incense() int          { return bits16(p.W61, 0, 2) }
func (p *Player) SetIncense(v int)      { setBits16(&p.W61, 0, 2, v) }
func (p *Player) Shrooms() int          { return bits16(p.W61, 2, 2) }
func (p *Player) SetShrooms(v int)      { setBits16(&p.W61, 2, 2, v) }
func (p *Player) Drunk() int            { return bits16(p.W61, 4, 6) }
func (p *Player) SetDrunk(v int)        { setBits16(&p.W61, 4, 6, v) }
func (p *Player) TalismanOK() int       { return bits16(p.W61, 10, 1) }
func (p *Player) SetTalismanOK(v int)   { setBits16(&p.W61, 10, 1, v) }
func (p *Player) Garamon() int          { return bits16(p.W61, 11, 1) }
func (p *Player) SetGaramon(v int)      { setBits16(&p.W61, 11, 1, v) }
func (p *Player) Maze() int             { return bits16(p.W61, 12, 1) }
func (p *Player) SetMaze(v int)         { setBits16(&p.W61, 12, 1, v) }
func (p *Player) Lefty() int            { return bits8(p.B64, 0, 1) }
func (p *Player) SetLefty(v int)        { setBits8(&p.B64, 0, 1, v) }
func (p *Player) Female() int           { return bits8(p.B64, 1, 1) }
func (p *Player) SetFemale(v int)       { setBits8(&p.B64, 1, 1, v) }
func (p *Player) Body() int             { return bits8(p.B64, 2, 3) }
func (p *Player) SetBody(v int)         { setBits8(&p.B64, 2, 3, v) }
func (p *Player) PClass() int           { return bits8(p.B64, 5, 3) }
func (p *Player) SetPClass(v int)       { setBits8(&p.B64, 5, 3, v) }
func (p *Player) Sound() int            { return bits8(p.BB5, 0, 2) }
func (p *Player) SetSound(v int)        { setBits8(&p.BB5, 0, 2, v) }
func (p *Player) Music() int            { return bits8(p.BB5, 2, 2) }
func (p *Player) SetMusic(v int)        { setBits8(&p.BB5, 2, 2, v) }
func (p *Player) Detail() int           { return bits8(p.BB5, 4, 4) }
func (p *Player) SetDetail(v int)       { setBits8(&p.BB5, 4, 4, v) }
func (p *Player) FPS() int              { return bits16(p.WB6, 0, 3) }
func (p *Player) SetFPS(v int)          { setBits16(&p.WB6, 0, 3, v) }
func (p *Player) Terrain() int          { return bits16(p.WB6, 3, 8) }
func (p *Player) SetTerrain(v int)      { setBits16(&p.WB6, 3, 8, v) }

// Creature is a creature type, OBJECTS.DAT's critter table.
type Creature struct {
	Armour  [4]byte
	Avghit  byte
	Attr    [3]byte
	B08     byte // death:3, blood:2, corpse:3
	Race    byte
	B0A     byte // passive, bA_1, remains:3, bA_5, swims, flier
	Speed   byte
	Run     byte
	W0D     uint16 // level:4, shrewd:4, haggle:4, patience:4
	B0F     byte
	B10     byte // sound:4, armour_kind:2, weapon_kind:2
	Equip   int8
	Defence int8
	Attacks [3]struct {
		Chance int8
		Damage byte
		Prob   byte
	}
	B1C    byte // b1C_0:4, range:4
	B1D    byte // noise:4, visibility:4
	B1E    byte // hearing:4, sight:4
	B1F    byte // lazy:4, alert:4
	Arms   [2]byte
	Other  [2]uint16
	W26    uint16 // treasure_prob:4, treasure_rate:4, food_prob:4, food_item:4
	Exp    int16
	Spells [3]byte
	B2D    byte // b2D_0, caster:7
	Locks  byte
	B2F    byte
}

func (c *Creature) Death() int            { return bits8(c.B08, 0, 3) }
func (c *Creature) Blood() int            { return bits8(c.B08, 3, 2) }
func (c *Creature) Corpse() int           { return bits8(c.B08, 5, 3) }
func (c *Creature) Passive() int          { return bits8(c.B0A, 0, 1) }
func (c *Creature) Remains() int          { return bits8(c.B0A, 2, 3) }
func (c *Creature) Swims() int            { return bits8(c.B0A, 6, 1) }
func (c *Creature) Flier() int            { return bits8(c.B0A, 7, 1) }
func (c *Creature) Level() int            { return bits16(c.W0D, 0, 4) }
func (c *Creature) Shrewd() int           { return bits16(c.W0D, 4, 4) }
func (c *Creature) Haggle() int           { return bits16(c.W0D, 8, 4) }
func (c *Creature) Patience() int         { return bits16(c.W0D, 12, 4) }
func (c *Creature) Sound() int            { return bits8(c.B10, 0, 4) }
func (c *Creature) ArmourKind() int       { return bits8(c.B10, 4, 2) }
func (c *Creature) WeaponKind() int       { return bits8(c.B10, 6, 2) }
func (c *Creature) Range() int            { return bits8(c.B1C, 4, 4) }
func (c *Creature) Noise() int            { return bits8(c.B1D, 0, 4) }
func (c *Creature) Visibility() int       { return bits8(c.B1D, 4, 4) }
func (c *Creature) SetNoise(v int)        { setBits8(&c.B1D, 0, 4, v) }
func (c *Creature) SetVisibility(v int)   { setBits8(&c.B1D, 4, 4, v) }
func (c *Creature) Hearing() int          { return bits8(c.B1E, 0, 4) }
func (c *Creature) Sight() int            { return bits8(c.B1E, 4, 4) }
func (c *Creature) Lazy() int             { return bits8(c.B1F, 0, 4) }
func (c *Creature) Alert() int            { return bits8(c.B1F, 4, 4) }
func (c *Creature) ArmsPresent(i int) int { return bits8(c.Arms[i], 0, 1) }
func (c *Creature) ArmsItem(i int) int    { return bits8(c.Arms[i], 1, 7) }
func (c *Creature) OtherProb(i int) int   { return bits16(c.Other[i], 0, 4) }
func (c *Creature) OtherItem(i int) int   { return bits16(c.Other[i], 4, 12) }
func (c *Creature) TreasureProb() int     { return bits16(c.W26, 0, 4) }
func (c *Creature) TreasureRate() int     { return bits16(c.W26, 4, 4) }
func (c *Creature) FoodProb() int         { return bits16(c.W26, 8, 4) }
func (c *Creature) FoodItem() int         { return bits16(c.W26, 12, 4) }
func (c *Creature) Caster() int           { return bits8(c.B2D, 1, 7) }

// ComObj is an object type's common properties, COMOBJ.DAT's 11-byte record.
type ComObj [11]byte

func (c *ComObj) w(off int) uint16 { return uint16(c[off]) | uint16(c[off+1])<<8 }
func (c *ComObj) Height() int      { return int(c[0]) }
func (c *ComObj) Radius() int      { return bits8(c[1], 0, 3) }
func (c *ComObj) Animated() int    { return bits8(c[1], 3, 1) }
func (c *ComObj) Mass() int        { return bits16(c.w(1), 4, 12) }
func (c *ComObj) B30() int         { return bits8(c[3], 0, 1) }
func (c *ComObj) Solid() int       { return bits8(c[3], 1, 1) }
func (c *ComObj) C32() int         { return bits8(c[3], 2, 1) }
func (c *ComObj) NoHit() int       { return bits8(c[3], 3, 1) }
func (c *ComObj) B34() int         { return bits8(c[3], 4, 1) }
func (c *ComObj) Pickup() int      { return bits8(c[3], 5, 1) }
func (c *ComObj) Stack() int       { return bits8(c[3], 6, 2) }
func (c *ComObj) Value() int       { return int(c.w(4)) }
func (c *ComObj) Touch() int       { return bits8(c[6], 0, 1) }
func (c *ComObj) Usable() int      { return bits8(c[6], 1, 1) }
func (c *ComObj) QualClass() int   { return bits8(c[6], 2, 2) }
func (c *ComObj) Light() int       { return bits8(c[6], 4, 1) }
func (c *ComObj) Bounce() int      { return bits16(c.w(6), 5, 4) }
func (c *ComObj) Fate() int        { return bits8(c[7], 1, 4) }
func (c *ComObj) Pickable() int    { return bits8(c[7], 5, 1) }
func (c *ComObj) B76() int         { return bits8(c[7], 6, 1) }
func (c *ComObj) CanOwn() int      { return bits8(c[7], 7, 1) }
func (c *ComObj) Resist() int      { return int(c[8]) }
func (c *ComObj) SetResist(v int)  { c[8] = byte(v) }
func (c *ComObj) Render() int      { return bits8(c[9], 0, 2) }
func (c *ComObj) Tenacity() int    { return bits8(c[9], 2, 4) }
func (c *ComObj) QualType() int    { return bits8(c[10], 0, 4) }
func (c *ComObj) Lookable() int    { return bits8(c[10], 4, 1) }
