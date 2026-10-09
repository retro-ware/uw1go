package uw

import "unsafe"

// object.h, map.h and level.h's records: the link word, objects, tiles and the level block.

// Link is union Link: an object's index in the top ten bits (0 none, 1..0xFF mobile,
// 0x100..0x3FF static) and six other bits below it.
type Link uint16

func (l Link) Index() int        { return int(l >> 6) }
func (l Link) Low() int          { return int(l & 0x3F) }
func (l *Link) SetIndex(i int)   { *l = *l&0x3F | Link(i&0x3FF)<<6 }
func (l *Link) SetLow(v int)     { *l = *l&^0x3F | Link(v&0x3F) }
func mkLink(index, low int) Link { return Link(index&0x3FF)<<6 | Link(low&0x3F) }

// Object is struct Object: 27 bytes for a mobile object, the first 8 for a static one.
// QN is the quality (low six bits) and the next object (index); OL the owner (low six
// bits) and the contents or quantity (index).
type Object struct {
	ID           uint16
	Pos          uint16
	QN           Link
	OL           Link
	HP           byte
	Heading      byte
	B0A          byte
	GoalWord     uint16
	AttitudeWord uint16
	B0F          uint16
	B11          byte
	LastHit      byte
	B13          byte
	B14          byte
	B15          byte
	Home         uint16
	B18          byte
	B19          byte
	Whoami       byte
}

// decode and encode an object record (n is 27 or 8 bytes).
func (o *Object) decode(b []byte, mobile bool) {
	o.ID = le16(b, 0)
	o.Pos = le16(b, 2)
	o.QN = Link(le16(b, 4))
	o.OL = Link(le16(b, 6))
	if !mobile {
		return
	}
	o.HP = b[8]
	o.Heading = b[9]
	o.B0A = b[10]
	o.GoalWord = le16(b, 11)
	o.AttitudeWord = le16(b, 13)
	o.B0F = le16(b, 15)
	o.B11 = b[17]
	o.LastHit = b[18]
	o.B13 = b[19]
	o.B14 = b[20]
	o.B15 = b[21]
	o.Home = le16(b, 22)
	o.B18 = b[24]
	o.B19 = b[25]
	o.Whoami = b[26]
}

func (o *Object) encode(b []byte, mobile bool) {
	put16(b, 0, o.ID)
	put16(b, 2, o.Pos)
	put16(b, 4, uint16(o.QN))
	put16(b, 6, uint16(o.OL))
	if !mobile {
		return
	}
	b[8] = o.HP
	b[9] = o.Heading
	b[10] = o.B0A
	put16(b, 11, o.GoalWord)
	put16(b, 13, o.AttitudeWord)
	put16(b, 15, o.B0F)
	b[17] = o.B11
	b[18] = o.LastHit
	b[19] = o.B13
	b[20] = o.B14
	b[21] = o.B15
	put16(b, 22, o.Home)
	b[24] = o.B18
	b[25] = o.B19
	b[26] = o.Whoami
}

// The id word.
func (o *Object) Item() int          { return int(o.ID & ID_ITEM) }
func (o *Object) Major() int         { return int(o.ID&ID_MAJOR) >> 6 }
func (o *Object) Minor() int         { return int(o.ID&ID_MINOR) >> 4 }
func (o *Object) Class() int         { return int(o.ID&ID_CLASS) >> 4 }
func (o *Object) ClassID() int       { return int(o.ID & ID_CLASS) }
func (o *Object) InMajor() int       { return int(o.ID & ID_INMAJOR) }
func (o *Object) InClass() int       { return int(o.ID & ID_INCLASS) }
func (o *Object) Flags() int         { return int(o.ID&ID_FLAGS) >> 9 }
func (o *Object) DoorDir() int       { return int(o.ID&ID_DOORDIR) >> 13 }
func (o *Object) Invis() int         { return int(o.ID&ID_INVIS) >> 14 }
func (o *Object) IsQuant() int       { return int(o.ID&ID_ISQUANT) >> 15 }
func (o *Object) Enchanted() int     { return int(o.ID&ID_ENCHANT) >> 12 }
func itemClass(item int) int         { return (item & ID_CLASS) >> 4 }
func (o *Object) SetItem(v int)      { o.ID = o.ID&0xFE00 | uint16(v)&ID_ITEM }
func (o *Object) SetMajor(v int)     { o.ID = o.ID&0xFE3F | uint16(v&7)<<6 }
func (o *Object) SetMinor(v int)     { o.ID = o.ID&0xFFCF | uint16(v&3)<<4 }
func (o *Object) SetInClass(v int)   { o.ID = o.ID&0xFFF0 | uint16(v)&0xF }
func (o *Object) SetFlags(v int)     { o.ID = o.ID&0xE1FF | uint16(v&0xF)<<9 }
func (o *Object) SetFlag9(v int)     { o.ID = o.ID&0xFDFF | uint16(v&1)<<9 }
func (o *Object) SetFlag10(v int)    { o.ID = o.ID&0xFBFF | uint16(v&1)<<10 }
func (o *Object) SetFlag11(v int)    { o.ID = o.ID&0xF7FF | uint16(v&1)<<11 }
func (o *Object) SetEnchanted(v int) { o.ID = o.ID&0xEFFF | uint16(v&1)<<12 }
func (o *Object) SetDoorDir(v int)   { o.ID = o.ID&0xDFFF | uint16(v&1)<<13 }
func (o *Object) SetInvis(v int)     { o.ID = o.ID&0xBFFF | uint16(v&1)<<14 }
func (o *Object) SetIsQuant(v int)   { o.ID = o.ID&0x7FFF | uint16(v&1)<<15 }

// The position word.
func (o *Object) Z() int           { return int(o.Pos & POS_Z) }
func (o *Object) HeadingBits() int { return int(o.Pos&POS_HEADING) >> 7 }
func (o *Object) FineY() int       { return int(o.Pos&POS_YFINE) >> 10 }
func (o *Object) FineX() int       { return int(o.Pos&POS_XFINE) >> 13 }
func (o *Object) SetZ(v int)       { o.Pos = o.Pos&0xFF80 | uint16(v)&POS_Z }
func (o *Object) SetHeading(v int) { o.Pos = o.Pos&0xFC7F | uint16(v&7)<<7 }
func (o *Object) SetFineY(v int)   { o.Pos = o.Pos&0xE3FF | uint16(v&7)<<10 }
func (o *Object) SetFineX(v int)   { o.Pos = o.Pos&0x1FFF | uint16(v&7)<<13 }

// The quality and next word, and the owner and link word.
func (o *Object) Quality() int     { return o.QN.Low() }
func (o *Object) Next() int        { return o.QN.Index() }
func (o *Object) Owner() int       { return o.OL.Low() }
func (o *Object) LinkF() int       { return o.OL.Index() }
func (o *Object) SetQuality(v int) { o.QN.SetLow(v) }
func (o *Object) SetNext(v int)    { o.QN.SetIndex(v) }
func (o *Object) SetOwner(v int)   { o.OL.SetLow(v) }
func (o *Object) SetLinkF(v int)   { o.OL.SetIndex(v) }
func (o *Object) SetEnchant(v int) { o.OL.SetIndex(o.OL.Index()&0x1F0 | v&0xF | LINK_SPECIAL) }

// Byte 0x0A.
func (o *Object) Bin() int         { return int(o.B0A & 0xF) }
func (o *Object) Terrain() int     { return int(o.B0A&0x70) >> 4 }
func (o *Object) Loner() int       { return int(o.B0A&0x80) >> 7 }
func (o *Object) SetBin(v int)     { o.B0A = o.B0A&0xF0 | byte(v&0xF) }
func (o *Object) SetTerrain(v int) { o.B0A = o.B0A&0x8F | byte(v&7)<<4 }
func (o *Object) SetLoner(v int)   { o.B0A = o.B0A&0x7F | byte(v&1)<<7 }

// The goal word (0x0B).
func (o *Object) Goal() int      { return int(o.GoalWord & 0xF) }
func (o *Object) GTarg() int     { return int(o.GoalWord&0xFF0) >> 4 }
func (o *Object) Frame() int     { return int(o.GoalWord&0xF000) >> 12 }
func (o *Object) SetGoal(v int)  { o.GoalWord = o.GoalWord&0xFFF0 | uint16(v&0xF) }
func (o *Object) SetGTarg(v int) { o.GoalWord = o.GoalWord&0xF00F | uint16(v&0xFF)<<4 }
func (o *Object) SetFrame(v int) { o.GoalWord = o.GoalWord&0xFFF | uint16(v&0xF)<<12 }

// The attitude word (0x0D).
func (o *Object) OldGoal() int      { return int(o.AttitudeWord & 0xF) }
func (o *Object) TargetZ() int      { return int(o.AttitudeWord&0xF0) >> 4 }
func (o *Object) Temp() int         { return int(o.AttitudeWord&0x100) >> 8 }
func (o *Object) NoHeal() int       { return int(o.AttitudeWord&0x200) >> 9 }
func (o *Object) Powerful() int     { return int(o.AttitudeWord&0x400) >> 10 }
func (o *Object) B0D11() int        { return int(o.AttitudeWord&0x800) >> 11 }
func (o *Object) HasInv() int       { return int(o.AttitudeWord&0x1000) >> 12 }
func (o *Object) TalkedTo() int     { return int(o.AttitudeWord&0x2000) >> 13 }
func (o *Object) Attitude() int     { return int(o.AttitudeWord&0xC000) >> 14 }
func (o *Object) SetOldGoal(v int)  { o.AttitudeWord = o.AttitudeWord&0xFFF0 | uint16(v&0xF) }
func (o *Object) SetTargetZ(v int)  { o.AttitudeWord = o.AttitudeWord&0xFF0F | uint16(v&0xF)<<4 }
func (o *Object) SetTemp(v int)     { o.AttitudeWord = o.AttitudeWord&0xFEFF | uint16(v&1)<<8 }
func (o *Object) SetNoHeal(v int)   { o.AttitudeWord = o.AttitudeWord&0xFDFF | uint16(v&1)<<9 }
func (o *Object) SetPowerful(v int) { o.AttitudeWord = o.AttitudeWord&0xFBFF | uint16(v&1)<<10 }
func (o *Object) SetB0D11(v int)    { o.AttitudeWord = o.AttitudeWord&0xF7FF | uint16(v&1)<<11 }
func (o *Object) SetHasInv(v int)   { o.AttitudeWord = o.AttitudeWord&0xEFFF | uint16(v&1)<<12 }
func (o *Object) SetTalkedTo(v int) { o.AttitudeWord = o.AttitudeWord&0xDFFF | uint16(v&1)<<13 }
func (o *Object) SetAttitude(v int) { o.AttitudeWord = o.AttitudeWord&0x3FFF | uint16(v&3)<<14 }

// The word at 0x0F.
func (o *Object) DestX() int        { return int(o.B0F & 0x3F) }
func (o *Object) DestY() int        { return int(o.B0F&0xFC0) >> 6 }
func (o *Object) AtkFrame() int     { return int(o.B0F&0xF000) >> 12 }
func (o *Object) SetAtkFrame(v int) { o.B0F = o.B0F&0xFFF | uint16(v&0xF)<<12 }
func (o *Object) SetDestX(v int)    { o.B0F = o.B0F&0xFFC0 | uint16(v&0x3F) }
func (o *Object) SetDestY(v int)    { o.B0F = o.B0F&0xF03F | uint16(v&0x3F)<<6 }

// Bytes 0x11, 0x13, 0x14, 0x15.
func (o *Object) Damage() int      { return int(o.B11) }
func (o *Object) SetDamage(v int)  { o.B11 = byte(v) }
func (o *Object) Speed() int       { return int(o.B13 & 0x7F) }
func (o *Object) SetSpeed(v int)   { o.B13 = o.B13&0x80 | byte(v&0x7F) }
func (o *Object) SetGravity(v int) { o.B13 = o.B13&0x7F | byte(v&1)<<7 }
func (o *Object) Rate() int        { return int(o.B14 & 7) }
func (o *Object) Pitch() int       { return int(o.B14&0xF8) >> 3 }
func (o *Object) SetRate(v int)    { o.B14 = o.B14&0xF8 | byte(v) }
func (o *Object) SetPitch(v int)   { o.B14 = o.B14&7 | byte(v&0x1F)<<3 }
func (o *Object) Seq() int         { return int(o.B15 & 0x3F) }
func (o *Object) B156() int        { return int(o.B15&0x40) >> 6 }
func (o *Object) B157() int        { return int(o.B15&0x80) >> 7 }
func (o *Object) SetSeq(v int)     { o.B15 = o.B15&0xC0 | byte(v&0x3F) }
func (o *Object) SetB156(v int)    { o.B15 = o.B15&0xBF | byte(v)<<6 }
func (o *Object) SetB157(v int)    { o.B15 = o.B15&0x7F | byte(v)<<7 }

// The home word (0x16).
func (o *Object) Path() int      { return int(o.Home & 0xF) }
func (o *Object) HomeY() int     { return int(o.Home&HOME_Y) >> 4 }
func (o *Object) HomeX() int     { return int(o.Home&HOME_X) >> 10 }
func (o *Object) SetPath(v int)  { o.Home = o.Home&0xFFF0 | uint16(v)&0xF }
func (o *Object) SetHomeY(v int) { o.Home = o.Home&0xFC0F | uint16(v&0x3F)<<4 }
func (o *Object) SetHomeX(v int) { o.Home = o.Home&0x3FF | uint16(v&0x3F)<<10 }

// Byte 0x18.
func (o *Object) FineHead() int     { return int(o.B18 & 0x1F) }
func (o *Object) B185() int         { return int(o.B18&0x20) >> 5 }
func (o *Object) B186() int         { return int(o.B18&0x40) >> 6 }
func (o *Object) B187() int         { return int(o.B18&0x80) >> 7 }
func (o *Object) SetFineHead(v int) { o.B18 = o.B18&0xE0 | byte(v&0x1F) }
func (o *Object) SetB185(v int)     { o.B18 = o.B18&0xDF | byte(v)<<5 }
func (o *Object) SetB186(v int)     { o.B18 = o.B18&0xBF | byte(v&1)<<6 }
func (o *Object) SetB187(v int)     { o.B18 = o.B18&0x7F | byte(v&1)<<7 }

// Byte 0x19.
func (o *Object) B190() int     { return int(o.B19 & 1) }
func (o *Object) B191() int     { return int(o.B19&2) >> 1 }
func (o *Object) Cast() int     { return int(o.B19&0xC) >> 2 }
func (o *Object) B194() int     { return int(o.B19&0x10) >> 4 }
func (o *Object) B195() int     { return int(o.B19&0x20) >> 5 }
func (o *Object) Ally() int     { return int(o.B19&0x40) >> 6 }
func (o *Object) Fed() int      { return int(o.B19&0x80) >> 7 }
func (o *Object) SetB190(v int) { o.B19 = o.B19&0xFE | byte(v&1) }
func (o *Object) SetB191(v int) { o.B19 = o.B19&0xFD | byte(v&1)<<1 }
func (o *Object) SetCast(v int) { o.B19 = o.B19&0xF3 | byte(v&3)<<2 }
func (o *Object) SetB194(v int) { o.B19 = o.B19&0xEF | byte(v)<<4 }
func (o *Object) SetB195(v int) { o.B19 = o.B19&0xDF | byte(v)<<5 }
func (o *Object) SetAlly(v int) { o.B19 = o.B19&0xBF | byte(v&1)<<6 }
func (o *Object) SetFed(v int)  { o.B19 = o.B19&0x7F | byte(v&1)<<7 }

// Tile is struct Tile: word 0 holds type (0-3), height (4-7), light (8-9), floor (10-13)
// and door (14-15); Objects is the head of the tile's object list, whose low six bits are
// the wall texture.
type Tile struct {
	W0      uint16
	Objects Link
}

func (t *Tile) Type() int       { return int(t.W0 & 0xF) }
func (t *Tile) Height() int     { return int(t.W0>>4) & 0xF }
func (t *Tile) Light() int      { return int(t.W0>>8) & 3 }
func (t *Tile) Floor() int      { return int(t.W0>>10) & 0xF }
func (t *Tile) Door() int       { return int(t.W0>>14) & 3 }
func (t *Tile) Wall() int       { return t.Objects.Low() }
func (t *Tile) SetType(v int)   { t.W0 = t.W0&^0xF | uint16(v&0xF) }
func (t *Tile) SetHeight(v int) { t.W0 = t.W0&^0xF0 | uint16(v&0xF)<<4 }
func (t *Tile) SetLight(v int)  { t.W0 = t.W0&^0x300 | uint16(v&3)<<8 }
func (t *Tile) SetFloor(v int)  { t.W0 = t.W0&^0x3C00 | uint16(v&0xF)<<10 }
func (t *Tile) SetDoor(v int)   { t.W0 = t.W0&^0xC000 | uint16(v&3)<<14 }
func (t *Tile) SetWall(v int)   { t.Objects.SetLow(v) }

// LevelBlock is struct LevelBlock, a level's block in LEV.ARK (0x7C08 bytes). The objects
// are kept as one array of 1024, the first 256 mobile; the free lists and the active list
// are kept as the record has them.
type LevelBlock struct {
	Tiles      [MAP_SIZE * MAP_SIZE]Tile
	Objs       [NUM_OBJECTS]Object
	MobFree    [0xFE]uint16
	StaticFree [NUM_STATIC]uint16
	Active     [0x104]byte
	NActive    uint16
	NMobFree   uint16
	NStatFree  uint16
	Magic      uint16
}

func (l *LevelBlock) decode(b []byte) {
	for i := range l.Tiles {
		l.Tiles[i].W0 = le16(b, i*4)
		l.Tiles[i].Objects = Link(le16(b, i*4+2))
	}
	for i := 0; i < NUM_MOBILE; i++ {
		l.Objs[i].decode(b[0x4000+i*MOBILE_SIZE:], true)
	}
	for i := 0; i < NUM_STATIC; i++ {
		l.Objs[NUM_MOBILE+i] = Object{}
		l.Objs[NUM_MOBILE+i].decode(b[0x5B00+i*STATIC_SIZE:], false)
	}
	for i := range l.MobFree {
		l.MobFree[i] = le16(b, 0x7300+i*2)
	}
	for i := range l.StaticFree {
		l.StaticFree[i] = le16(b, 0x74FC+i*2)
	}
	copy(l.Active[:], b[0x7AFC:0x7C00])
	l.NActive = le16(b, 0x7C00)
	l.NMobFree = le16(b, 0x7C02)
	l.NStatFree = le16(b, 0x7C04)
	l.Magic = le16(b, 0x7C06)
}

func (l *LevelBlock) encode() []byte {
	b := make([]byte, LEVEL_SIZE)
	for i := range l.Tiles {
		put16(b, i*4, l.Tiles[i].W0)
		put16(b, i*4+2, uint16(l.Tiles[i].Objects))
	}
	for i := 0; i < NUM_MOBILE; i++ {
		l.Objs[i].encode(b[0x4000+i*MOBILE_SIZE:], true)
	}
	for i := 0; i < NUM_STATIC; i++ {
		l.Objs[NUM_MOBILE+i].encode(b[0x5B00+i*STATIC_SIZE:], false)
	}
	for i := range l.MobFree {
		put16(b, 0x7300+i*2, l.MobFree[i])
	}
	for i := range l.StaticFree {
		put16(b, 0x74FC+i*2, l.StaticFree[i])
	}
	copy(b[0x7AFC:0x7C00], l.Active[:])
	put16(b, 0x7C00, l.NActive)
	put16(b, 0x7C02, l.NMobFree)
	put16(b, 0x7C04, l.NStatFree)
	put16(b, 0x7C06, l.Magic)
	return b
}

// Level is mapdata: the level block of the current level.
var Level = new(LevelBlock)

// objIndexOf returns obj's index in the level's object store (Obj_MemTPtr), or -1 for an
// object that is not in it.
func objIndexOf(obj *Object) int {
	if obj == nil {
		return 0
	}
	base := uintptr(unsafe.Pointer(&Level.Objs[0]))
	p := uintptr(unsafe.Pointer(obj))
	if p < base || p >= base+uintptr(NUM_OBJECTS)*unsafe.Sizeof(Object{}) {
		return -1
	}
	return int((p - base) / unsafe.Sizeof(Object{}))
}

// tileIndexOf returns a tile's index in the map, x + y*64.
func tileIndexOf(t *Tile) int {
	base := uintptr(unsafe.Pointer(&Level.Tiles[0]))
	return int((uintptr(unsafe.Pointer(t)) - base) / unsafe.Sizeof(Tile{}))
}
