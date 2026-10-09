package uw

// More temporary stand-ins (see stubs.go).

// GRIDDB.C, VIEW3D.C
var (
	UsPtr   *Object
	curelem *Object
)

func setGraphicsLevel()   {}
func setCyb(on bool)      {}
func randomLight(on bool) {}
func camerasFade()        {}
func fillFB(c byte)       {}
func cFillFB(c byte)      {}

// LIGHTING.C
func setLight(n int) {}

// PANELS.C
var ()

func seg041_35D7_E9(n int) {}

// COMBAT.C, SPELLS.C, DAMAGE.C, MAGIC
var (
	cmbModTH [4]int8
)

func playerAttack(n int)                                              {}
func clearFightState()                                                {}
func damageItem(obj, who *Object, x, y, damage, typ int) bool         { return false }
func restoreHp(obj *Object, n int)                                    {}
func restoreMana(obj *Object, n int)                                  {}
func getHpBack(obj *Object, n int)                                    {}
func backfire(obj *Object, n int)                                     {}
func mdetect(a, b int)                                                {}
func decodeObjSpell(obj *Object, major, effect *int, flag *bool) bool { return false }
func removeSpell(obj *Object)                                         {}
func placeNew(obj *Object, item int) *Object                          { return nil }
func clearAllLoretries()                                              {}

// SOUND.C
func setNewMusic(n int) {}
func changeMusicMaybe() {}
func turnMusic(on bool) {}
func turnFx(on bool)    {}
func musicIsOn() bool   { return false }
func fxIsOn() bool      { return false }

// CRITTERS, AI
func DoWanderingMonsters(n int) {}
func NightCleanCritPages()      {}
func seg014_1DC5_15C5()         {}
func seg014_1DC5_C7C()          {}

// EFFECT.C, TRIGGER.C, OBJPHYS.C, WORLDEV.C
var (
	animcount        int
	MapObjX, MapObjY int
)

func remAnimFromList(index int)                        {}
func addAnimobj(index, length, a, x, y int) int        { return 1 }
func AnimLoad(a *Arc, level int) bool                  { return true }
func AnimSave(a *Arc, level int) bool                  { return true }
func trapObjDel(head *Link, obj *Object)               {}
func UseTrigger(who, start, trig *Object, typ int) int { return 0 }
func SetOffTrap(who, obj, trap *Object, x, y int)      {}
func deleteTrap(head *Link, trap *Object)              {}
func DoClosingDoors(n int)                             {}
func doTeleport(obj *Object, x, y, level int)          {}

// OPTIONS.C, CONVERSE.C, BARTER.C
func doOptionShortcut(k int) {}
func doEscapeKey(n int)      {}
func convPlayMenu(n int)     {}
func npcBarter(n int)        {}
func playBarter(n int)       {}

// PATHFIND.C, AI.C (critter.h)
type PathSq struct{ x, y, unused, flag byte }

var (
	meptr            *Object
	mycst            *Creature
	myxpos, myypos   int
	pathlen          int
	pathsq           [64]PathSq
	XP, YP           int
	crithit, typehit int
	crithittime      int32
	hitx, hity       int
)

type areaFn func(x, y int, target *Object, tile *Tile, src int) bool

func critterSetGoal(goal, gtarg int)                         {}
func setCritterVars(obj *Object)                             {}
func floodPath(x, y, h, dx, dy, dz, rng int) bool            { return false }
func setLoc(x, y, z int)                                     {}
func clearPaths()                                            {}
func lineOfSight(x1, y1, z1, x2, y2, z2 int) bool            { return false }
func gronkArea(obj *Object, n int, fn areaFn, a, b, rng int) {}
func processArea(n, m int, fn areaFn, a, x, y, w, h int)     {}
func UseTrap(trap *Object, x, y int)                         {}
func ReturnObject(obj *Object, how int) bool                 { return false }
func convInvSpecial()                                        {}
func UseObj(who, obj *Object, how int) bool                  { return false }
func UseFood(who, obj *Object, how int) int                  { return 0 }

// INTERACT.C dependencies: SOUND, OPTIONS, SPELLS, TRIGGER, LOOK, CONVERSE, GRIDDB.
var (
	PickUp                 int = 0xBF
	colorToObj, colorToMap [0xBF]uint16
)

func playMusic()                                {}
func stopMusic()                                {}
func getCurrentMusic() int                      { return 0 }
func runOptionsPanel(n int)                     {}
func optionsMouseClick(x, y int)                {}
func do3dGrab()                                 {}
func pickPixel(x, y int) int                    { return 0 }
func checkTrap(who, obj *Object, how, x, y int) {}
func TalkTo(obj *Object)                        {}
func LookAt(obj *Object, how int)               {}
func BlastFunction()                            {}

// SPELLS.C
var mspellMused int

func doSpell(cls, sub int, caster, target *Object) bool { return false }

// AI.C: the 16-step time-bin clock
var curBin int

// SOUND.C
func playEffect(fx int, x, y, vol int) bool                { return false }
func playEffectOnMobile(fx int, obj *Object, vol int) bool { return false }

// EFFECT.C
func putEffect(who *Object, cls, length, frame, z, x, y int) bool { return false }
func fireballEffect(src *Object, x, y int)                        {}
func mtsDoanim(obj *Object, x, y int, who int) bool               { return false }

// ANIMOBJ: points an animation object's record at its new copy
func changeAnimPtr(dst, src *Object) {}

// COMBAT.C, DAMAGE.C
func missileThwack(attacker int, missile, def *Object, x, y, dmg, typ int) {}
func checkRes(obj *Object, damage, typ int) bool                           { return false }

// VIEW3D.C: the camera the view is drawn from
type camera struct{ x, y, z, heading, pitch, bank int16 }

var cPlayer camera

// ANIMOBJ, AI.C
var DoAnimO = true

func updateAnimobj(frames int) {}
func moveMobile(frames int)    {}

// SOUND.C
func killEffect(n int) {}
