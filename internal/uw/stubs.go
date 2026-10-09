package uw

// Temporary stand-ins for routines of files not ported yet. Each goes when its file does.

func preloadCr(first bool) bool { return true }
func seg042_19B()               {}

// PLAYER.C / PLAYDATA.C
var (
	LeftPanel     int
	nextSpellTime uint32
)

func findGoodXAndY(o *Object, x, y int, loose bool) (int, int, bool) { return x, y, true }

// MAP.C, GAMEWRAP.C, OBJECTS.C, LIGHTING.C, COMBINE.C

func initLighting() {}

// PANELS.C, INVPANEL.C, VIEW3D.C

func place3dView(x, y, w, h int) {}
func renderFB()                  {}
func establishView()             {}
func init3d()                    {}
func fadeout3d(n int)            {}
func fadein3d(n int)             {}
func saveScreenshot()            {}

// COMBAT.C, SKILLS.C, PHYSICS.C, AI.C
func initAi() {}

// CONVERSE.C, BABL.C
func strtConverse() {}
func freeConverse() {}
func initBabl() int { return 0 }

// SOUND.C
func initSounds()                  {}
func freeSounds()                  {}
func initTimers()                  {}
func loadNewMusic(n int, now bool) {}
func readCfgData(b []byte)         {}

// CUTS.C, MAINMENU.C

// SOUND.C: speech and effects
func speechAvailable() bool               { return false }
func initVoc() bool                       { return false }
func playSpeech(n int)                    {}
func stopSpeech()                         {}
func updateSpeech()                       {}
func speechOver() bool                    { return true }
func freeSpeechStuff()                    {}
func loopMusicMaybe()                     {}
func playEffectHere(fx, pan, vol int) int { return 0xFF }

// TEXTMAPS.C

// SAVEGAME (PLAYDATA.C / GAMEWRAP.C), CHARGEN.C, SKILLS.C
func showCredits() {}
