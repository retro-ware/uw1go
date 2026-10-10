package uw

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/memmaker/uw1go/internal/host"
	"github.com/memmaker/uw1go/internal/snd"
)

// SOUND.C: the MIDI sound effects and instruments played on channels locked from the
// music driver, the music themes SOUND\UWnn.XMI / AWnn.XMI with their timbres from
// SOUND\UW.AD, the cutscenes' digitised speech SOUND\nn.VOC, and the sound settings.
//
// The AIL drivers are internal/snd: an emulated OPL2 with AIL's XMIDI driver on it.
// Cards 2 to 5 (AdLib, Sound Blaster, SB Pro, PAS FM) play through it, 4 and 5 in stereo;
// the MT-32 (6) is munt's emulation, given its ROMs in UW1GO_MT32_ROMS (default ./roms),
// and falls back to the AdLib themes and timbres without them; the PC speaker (1) is not
// emulated: as on DOS it has no music, and its effects stay silent.
// The driver files the original loads (SOUND\*.ADV) must still be there. The card numbers
// come from DATA\UW.CFG; UW1GO_SOUND and UW1GO_SPEECH override them (the shipped UW.CFG
// says no card). The game clock is the host's (initTimers), the 16 Hz effects timer runs
// off the host's ticks, and a .VOC is handed to the player whole, so the speech needs no
// EMS pages and updateSpeech has nothing to feed.

// music_repeats: whether a theme may start again by itself when it ends.
var musicRepeats = [14]byte{0, 0, 1, 1, 1, 1, 1, 1, 1, 0, 0, 0, 1, 1}

type soundEffect struct {
	patch, note, vol byte
	length           int
}

var (
	effects     [24]soundEffect
	numeffects  int
	fxMask      byte
	fxNote      [4]byte
	fxTimbre    [4]byte
	fxMidiChan  [4]int
	fxTicksLeft [4]int
	fxChannels  uint16
	fxAcc       int

	curmusic, newmusic int
	themeChanged       uint32
	xmiSequence        = -1
	timbreLib          []byte // the timbre library, SOUND\UW.AD or UW.MT
	mt32               bool   // the MT-32 is emulated (munt)

	mSettings, speechCfg  [3]int
	soundCard, speechCard int
	musicDriver           = -1
	sphdriver             = -1
	musicOn, fxOn         = true, true
	fxOK, musicOK         = true, true
	speechok              bool
	speechLeft            int
	musicDrivers          = [7]string{"", "pcspkr.adv", "adlib.adv", "sbfm.adv", "sbpfm.adv", "pasfm.adv", "mt32mpu.adv"}
	speechDrivers         = [4]string{"", "sbdig.adv", "sbpdig.adv", "pasdig.adv"}
	fxTimerRegistered     bool
)

// initTimers: the game clock is the host's 256 Hz clock.
func initTimers() {}

// initSounds sets up the music driver for soundCard and, when there is one, the speech
// driver; the effects (initFx) and the timbres. Card 1 gets effects only.
func initSounds() {
	if v, err := strconv.Atoi(os.Getenv("UW1GO_SOUND")); err == nil {
		soundCard = v
	}
	if v, err := strconv.Atoi(os.Getenv("UW1GO_SPEECH")); err == nil {
		speechCard = v
	}
	if soundCard < 0 || soundCard > 6 {
		soundCard = CARD_NONE
	}
	if speechCard < 0 || speechCard > 3 {
		speechCard = 0
	}
	if soundCard == CARD_NONE {
		if speechDrivers[speechCard] != "" {
			initSpeech()
		}
	} else {
		if soundCard == CARD_PCSPKR {
			musicOK = false
			musicOn = false
		}
		if !fileExists("SOUND\\" + musicDrivers[soundCard]) {
			goto fail
		}
		if soundCard == CARD_MT32 {
			dir := os.Getenv("UW1GO_MT32_ROMS")
			if dir == "" {
				dir = "roms"
			}
			if err := snd.StartupMT32(dir); err != nil {
				fmt.Println("MT-32:", err, "- using the AdLib emulation")
				snd.Startup(false)
			} else {
				mt32 = true
			}
		} else {
			snd.Startup(soundCard == CARD_SBPFM || soundCard == CARD_PASFM)
		}
		musicDriver = soundCard
		if !initFx() {
			goto fail
		}
		if speechDrivers[speechCard] != "" {
			initSpeech()
		}
		newmusic = 0
		if musicOK && !initTimbres() {
			goto fail
		}
		return
	fail:
		fmt.Println("Sound system initialization failed.")
	}
	fxOK = false
	musicOK = false
	musicOn = false
	fxOn = false
}

// loadGlobalTimbre finds bank, patch in the timbre library's directory (6-byte entries:
// patch, bank, offset, ending at bank 0xFF) and returns the timbre with its length word.
func loadGlobalTimbre(bank, patch int) []byte {
	lib := timbreLib
	for i := 0; i+6 <= len(lib); i += 6 {
		if lib[i+1] == 0xFF {
			return nil
		}
		if int(lib[i+1]) == bank && int(lib[i]) == patch {
			off := int(binary.LittleEndian.Uint32(lib[i+2:]))
			if off+2 > len(lib) {
				return nil
			}
			size := int(binary.LittleEndian.Uint16(lib[off:]))
			if size < 2 || off+size > len(lib) {
				return nil
			}
			return lib[off : off+size]
		}
	}
	return nil
}

// initTimbres reads the timbre library SOUND\UW.AD, or UW.MT on the MT-32, where every
// effect's timbre (bank 1) is installed at once.
func initTimbres() bool {
	if mt32 {
		timbreLib = readFile("SOUND\\uw.mt")
	} else {
		timbreLib = readFile("SOUND\\uw.ad")
	}
	if timbreLib == nil {
		musicOn = false
		musicOK = false
		return false
	}
	if mt32 {
		for i := 0; i < numeffects; i++ {
			installTimbre(1, i)
		}
	}
	return true
}

func installTimbre(bank, patch int) bool {
	if t := loadGlobalTimbre(bank, patch); t != nil {
		snd.InstallTimbre(bank, patch, t)
		return true
	}
	return false
}

// loadNewMusic loads theme music as the sequence unless it is loaded already, installing
// the timbres it asks for; with start it starts it at volume 0x60. A failure turns the
// music off for the session.
func loadNewMusic(music int, start bool) bool {
	if !musicOK || !musicOn {
		return false
	}
	if music != curmusic {
		name := []byte("uw00.xmi")
		if !mt32 {
			name[0] = 'a'
		}
		name[2] = byte(music>>3) + '0'
		name[3] = byte(music&7) + '0'
		b := readFile("SOUND\\" + string(name))
		if b == nil {
			return loadMusicFail()
		}
		snd.StopSequence(xmiSequence)
		snd.ReleaseSequence(xmiSequence)
		if xmiSequence = snd.RegisterSequence(b); xmiSequence == -1 {
			return loadMusicFail()
		}
		for t := snd.TimbreRequest(xmiSequence); t != 0xFFFF; t = snd.TimbreRequest(xmiSequence) {
			if !installTimbre(t>>8, t&0xFF) {
				return loadMusicFail()
			}
		}
	}
	if start {
		snd.StartSequence(xmiSequence)
		snd.SetRelativeTempo(xmiSequence, 100, 0)
		snd.SetRelativeVolume(xmiSequence, 0x60, 0)
	}
	dprintf("music: theme %d start %v seq %d\n", music, start, xmiSequence)
	curmusic = music
	newmusic = 0
	return true
}

func loadMusicFail() bool {
	xmiSequence = -1
	musicOK = false
	return false
}

// playMusic starts the loaded theme again if it is not playing.
func playMusic() {
	if !musicOK || !musicOn || xmiSequence == -1 {
		return
	}
	if snd.SequenceStatus(xmiSequence) != snd.SeqPlaying {
		snd.StartSequence(xmiSequence)
	}
	snd.SetRelativeVolume(xmiSequence, 0x60, 0)
}

func getCurrentMusic() int { return curmusic }

func musicIsOn() bool { return musicOK && musicOn }

func fxIsOn() bool { return fxOK && fxOn }

// turnMusic: turning it on with no theme loaded picks a walking theme.
func turnMusic(on bool) {
	if !musicOK {
		return
	}
	if on && !musicOn {
		musicOn = true
		if xmiSequence == -1 {
			seg014_1DC5_15C5()
			loadNewMusic(newmusic, true)
		}
		snd.StartSequence(xmiSequence)
	} else if !on && musicOn && xmiSequence != -1 {
		musicOn = false
		snd.StopSequence(xmiSequence)
	}
}

func turnFx(on bool) {
	if !fxOK {
		return
	}
	fxOn = on
	if !fxOn {
		seg014_1DC5_C7C()
	}
}

func stopMusic() {
	if !musicOK || !musicOn {
		return
	}
	snd.StopSequence(xmiSequence)
}

// playEffect plays effect fx as if from x, y (eighths of a tile), panned by the
// direction to it in the player's frame and faded with distance: full within a tile,
// nothing beyond six. Returns the effect's slot or 0xFF.
func playEffect(fx int, x, y, vol int) int {
	if !fxOK || !fxOn {
		return 0xFF
	}
	e := &effects[fx&0xFF]
	px := ThePlayer.HomeX()<<3 + ThePlayer.FineX()
	py := ThePlayer.HomeY()<<3 + ThePlayer.FineY()
	ldx := int32(x - px)
	ldy := int32(y - py)
	dist := cSqRt(uint32(ldx*ldx + ldy*ldy))
	var v, pan int
	vol = int(int8(vol))
	if dist == 0 {
		v = int(e.vol) + vol
		pan = 0x40
	} else {
		d := int32(dist)
		var nx, ny int
		switch {
		case ldy == d:
			ny = 0x7F
		case -ldy == d:
			ny = 0x80
		default:
			ny = int(int16(ldy << 7 / d))
		}
		switch {
		case ldx == d:
			nx = 0x7F
		case -ldx == d:
			nx = 0x80
		default:
			nx = int(ldx << 7 / d)
		}
		angle := (ThePlayer.HeadingBits()<<5 + ThePlayer.FineHead()) << 8
		angle = (0x4000 - angle) & 0xFFFF
		a, b := cFstSinCos(angle)
		ai, bi := int(a>>8), int(b>>8)
		side := (ny*bi - nx*ai) >> 8
		pan = 0x40 - side
		if pan > 0x7F {
			pan = 0x7F
		} else if pan < 0 {
			pan = 0
		}
		v = int(e.vol) + vol
		if dist > 0x30 {
			return 0xFF
		}
		if dist >= 8 {
			v = int(int16(v * (0x30 - dist) / 0x28))
		}
	}
	if v > 0x7F {
		v = 0x7F
	} else if v < 0 {
		v = 0
	}
	return fxPlay(fx, int(e.patch), int(e.note), v, pan, e.length)
}

// playEffectHere plays effect fx at a given pan, with no position.
func playEffectHere(fx, pan, vol int) int {
	if !fxOK || !fxOn {
		return 0xFF
	}
	e := &effects[fx&0xFF]
	v := int(e.vol) + int(int8(vol))
	if v > 0x7F {
		v = 0x7F
	} else if v < 0 {
		v = 0
	}
	return fxPlay(fx, int(e.patch), int(e.note), v, pan&0xFF, e.length)
}

func playEffectOnMobile(fx int, obj *Object, vol int) int {
	if !fxOK || !fxOn {
		return 0xFF
	}
	return playEffect(fx, obj.HomeX()<<3+obj.FineX(), obj.HomeY()<<3+obj.FineY(), vol)
}

// silenceFx ends effect slot i's note: all notes off on the PC speaker, else its channel
// released.
func silenceFx(i int) {
	if soundCard == CARD_PCSPKR {
		snd.SendVoice(fxMidiChan[i]+0xAF, 0x7B, 0)
	} else {
		snd.ReleaseChannel(fxMidiChan[i])
	}
	fxChannels &^= 1 << uint(fxMidiChan[i])
}

// killEffect silences effect slot n if it is playing.
func killEffect(n int) {
	n &= 0xFF
	if n < 4 && fxMask&(1<<uint(n)) != 0 {
		fxMask -= 1 << uint(n)
		silenceFx(n)
	}
}

// seg014_1DC5_C7C silences every effect (UW2's kill_all_effects).
func seg014_1DC5_C7C() {
	for i := 0; i < 4; i++ {
		if fxMask&(1<<uint(i)) != 0 {
			fxMask -= 1 << uint(i)
			silenceFx(i)
		}
	}
}

// fxTimer is the 16 Hz effects timer: it counts down each effect's ticks and ends it
// when they run out.
func fxTimer() {
	if fxMask == 0 {
		return
	}
	for i, bit := 0, byte(1); i < 4; i, bit = i+1, bit<<1 {
		if fxMask&bit != 0 && fxTicksLeft[i] != -1 {
			if fxTicksLeft[i]--; fxTicksLeft[i] == 0 {
				fxMask &^= bit
				if soundCard == CARD_PCSPKR {
					snd.SendVoice(fxMidiChan[i]+0xAF, 0x7B, 0)
				} else {
					snd.SendVoice(fxMidiChan[i]+0x7F, int(fxNote[i]), 0)
					snd.ReleaseChannel(fxMidiChan[i])
					fxChannels &^= 1 << uint(fxMidiChan[i])
				}
			}
		}
	}
}

// readFxData reads SOUND\SOUNDS.DAT: a count byte, then 5 bytes an effect: patch, note,
// volume, length (high byte first).
func readFxData() bool {
	b := readFile("SOUND\\sounds.dat")
	if b == nil || len(b) < 1 {
		return false
	}
	numeffects = int(b[0])
	for i := 0; i < numeffects && i < len(effects) && 1+i*5+5 <= len(b); i++ {
		r := b[1+i*5:]
		effects[i] = soundEffect{r[0], r[1], r[2], int(r[3])<<8 + int(r[4])}
	}
	return true
}

// initFx starts the 16 Hz effects timer and reads SOUNDS.DAT.
func initFx() bool {
	fxMask = 0
	if !fxTimerRegistered {
		fxTimerRegistered = true
		host.AddTimer(func() {
			fxAcc += 16
			for fxAcc >= host.TPS {
				fxAcc -= host.TPS
				fxTimer()
			}
		})
	}
	return readFxData()
}

// fxPlay plays an effect as a MIDI note on the first free effect slot, on a channel
// locked from the music: timbre bank 1, the patch, controllers reset, full volume and
// expression, the pan, then the note. length is in 1/256 seconds, -1 to hold. On the PC
// speaker only six effects sound, at fixed notes and lengths on channel 2. Returns the
// slot or 0xFF.
func fxPlay(fx, patch, note, vel, pan, length int) int {
	i := 0
	bit := byte(1)
	for ; fxMask&bit != 0 && i < 4; i, bit = i+1, bit<<1 {
	}
	if i == 4 {
		return 0xFF
	}
	var ch int
	if soundCard == CARD_PCSPKR {
		switch fx {
		case 4, 0x10:
			note, length = 0x48, 0x10
		case 7, 8, 0x15:
			note, length = 0x42, 8
		case 3, 0x16:
			note, length = 0x38, 4
		default:
			return 0xFF
		}
		fxMask |= bit
		fxMidiChan[i] = 2
		fxTicksLeft[i] = length
		ch = 2
	} else {
		if !snd.TimbreStatus(1, patch) && !installTimbre(1, patch) {
			return 0xFF
		}
		if ch = snd.LockChannel(); ch == 0 {
			return 0xFF
		}
		fxChannels |= 1 << uint(ch)
		fxMask |= bit
		fxMidiChan[i] = ch
		if length == -1 {
			fxTicksLeft[i] = -1
		} else {
			fxTicksLeft[i] = length / 16
		}
		fxNote[i] = byte(note)
		fxTimbre[i] = byte(patch)
		snd.SendVoice(ch+0xAF, 0x72, 1)
		snd.SendVoice(ch+0xBF, patch, 0)
		snd.SendVoice(ch+0xAF, 0x79, 0)
		snd.SendVoice(ch+0xAF, 7, 0x7F)
		snd.SendVoice(ch+0xAF, 0x0B, 0x7F)
		snd.SendVoice(ch+0xAF, 0x0A, pan)
	}
	dprintf("fx: %d slot %d chan %d note %#x vel %d pan %d\n", fx, i, ch, note, vel, pan)
	snd.SendVoice(ch+0x8F, note, vel)
	return i
}

// playInstrument lets the player play an instrument (which: 0 or 1, patches 0x39 and
// 0x48 of bank 0): keys 1 to 9 and 0 play ten notes up from middle C, Alt an octave
// higher, Ctrl an octave lower; a note stops after a quarter second or at the next key;
// Escape ends. The last 16 notes are kept: on level 3 near 24, 45 the right tune on
// instrument 1 brings up the cup of wonder.
func playInstrument(which int) {
	inst := [2]int{0x39, 0x48}
	notes := [10]int{0x3C, 0x3E, 0x40, 0x41, 0x43, 0x45, 0x47, 0x48, 0x4A, 0x4C}
	var hist [16]byte
	hpos := 0
	t := int64(-1)
	last := 0xFF
	ok := true
	ch := 0
	gameSprint(0xFA)
	patch := inst[which&1]
	if !snd.TimbreStatus(0, patch) && !installTimbre(0, patch) {
		ok = false
	}
	if ok && musicDriver == -1 {
		ok = false
	}
	if ok {
		if soundCard == CARD_PCSPKR {
			ch = 2
		} else {
			ch = snd.LockChannel()
			fxChannels |= 1 << uint(ch)
			snd.SendVoice(ch+0xAF, 0x72, 0)
			snd.SendVoice(ch+0xBF, patch, 0)
			snd.SendVoice(ch+0xAF, 0x79, 0)
			snd.SendVoice(ch+0xAF, 7, 0x7F)
			snd.SendVoice(ch+0xAF, 0x0B, 0x7F)
			snd.SendVoice(ch+0xAF, 0x0A, 0x40)
		}
	}
	for {
		key := mouseGetInput()
		if key == KEY_ESC {
			break
		}
		if key > 0 {
			k := key & 0xFCFF
			if k >= '0' && k <= '9' {
				n := k - '0'
				if n == 0 {
					n = 10
				}
				n = notes[n-1]
				if key&0x200 != 0 {
					n += 12
				}
				if key&0x100 != 0 {
					n -= 12
				}
				if t > 0 && last != 0xFF && ok {
					snd.SendVoice(ch+0x7F, last, 0)
				}
				hist[hpos] = byte(n)
				hpos = (hpos + 1) & 0x0F
				if ok {
					snd.SendVoice(ch+0x8F, n, 0x7F)
				}
				last = n
				t = int64(GameTime())
			}
		}
		if t > 0 && int64(GameTime())-t > 0x40 {
			t = -1
			if ok {
				snd.SendVoice(ch+0x7F, last, 0)
			}
			last = 0xFF
		}
	}
	snd.ReleaseChannel(ch)
	if which == 1 && PlayerLevel == 3 && abs(ThePlayer.HomeX()-0x18) <= 2 &&
		abs(ThePlayer.HomeY()-0x2D) <= 2 && playCupTune(hist[:]) {
		return
	}
	gameSprint(0xFB)
}

// playCupTune: if the cup of wonder is not yet found and the first nine notes played are
// its tune, puts the cup in the player's hand and marks it found.
func playCupTune(played []byte) bool {
	tune := [9]byte{0x40, 0x43, 0x41, 0x3E, 0x40, 0x47, 0x48, 0x47, 0x43}
	if player.Cup() != 0 {
		return false
	}
	for i := range tune {
		if tune[i] != played[i] {
			return false
		}
	}
	if placeNew(nil, 0xAE) != nil {
		gameSprint(0x88)
		player.SetCup(1)
		return true
	}
	return false
}

func freeSounds() { snd.Shutdown() }

// setNewMusic asks for theme m; changeMusicMaybe starts it.
func setNewMusic(m int) { newmusic = m }

// seg014_1DC5_15C5 picks a walking theme at random (UW2's set_random_walking_music).
func seg014_1DC5_15C5() { newmusic = rand()%3 + MUSIC_WALK_FIRST }

// loopMusicMaybe restarts the theme when it has ended; theme 1 is followed by theme 4.
func loopMusicMaybe() {
	m := curmusic
	if musicOver() {
		if curmusic == MUSIC_THEME {
			m = 4
		}
		loadNewMusic(m, true)
	}
}

func walkingMusic(m int) bool { return m >= MUSIC_WALK_FIRST && m <= MUSIC_WALK_LAST }
func combatMusic(m int) bool  { return m >= MUSIC_FOE_HURT && m <= MUSIC_DANGER }

// changeMusicMaybe keeps the right theme playing, from the main loop. Themes 9 and 11
// play to their end. Ten seconds after the last combat a combat theme gives way to theme
// 8 if the weapon is drawn, else to a walking theme. A requested theme starts at once,
// except that one combat theme replaces another at most every eight seconds. When a theme
// ends with none requested, a walking theme is picked if it does not repeat (or is a
// walking theme) in the 3D view, and theme 8 if the weapon is drawn; else it repeats.
func changeMusicMaybe() {
	if !musicOK || !musicOn {
		return
	}
	if (curmusic == MUSIC_VICTORY || curmusic == 0x0B) && !musicOver() {
		return
	}
	if combatMusic(curmusic) && GameTime() > lastcombattime+0xA00 {
		if player.Drawn() != 0 {
			newmusic = MUSIC_ARMED
		} else {
			newmusic = rand()%3 + MUSIC_WALK_FIRST
		}
	}
	if newmusic != 0 && newmusic != curmusic {
		if combatMusic(curmusic) && combatMusic(newmusic) {
			if GameTime() > themeChanged+0x800 {
				loadNewMusic(newmusic, true)
				themeChanged = GameTime()
			} else {
				newmusic = curmusic
			}
		} else {
			loadNewMusic(newmusic, true)
		}
		if combatMusic(newmusic) {
			themeChanged = GameTime()
		}
	} else if musicOver() {
		if (musicRepeats[curmusic%14] == 0 || walkingMusic(curmusic)) && scrmode == 1 || newmusic == 0 {
			newmusic = rand()%3 + MUSIC_WALK_FIRST
		}
		if player.Drawn() != 0 {
			newmusic = MUSIC_ARMED
		}
		loadNewMusic(newmusic, true)
		themeChanged = 0
	}
}

// musicOver is whether the theme has ended, or none is loaded.
func musicOver() bool {
	if xmiSequence == -1 {
		return true
	}
	return snd.SequenceStatus(xmiSequence) != snd.SeqPlaying
}

// initSpeech sets up the digital driver for speechCard.
func initSpeech() bool {
	if !fileExists("SOUND\\" + speechDrivers[speechCard]) {
		speechok = false
		return false
	}
	snd.StartOutput()
	sphdriver = speechCard
	speechok = true
	return true
}

func speechAvailable() bool { return speechok }

// initVoc gets the speech ready. The .VOC is held whole, so there are no pages to find.
func initVoc() bool { return speechok }

// playSpeech plays SOUND\nn.VOC, at most 96 KB of it as the original's six EMS pages
// hold. A failure turns the speech off.
func playSpeech(n int) {
	if !speechok {
		return
	}
	name := strconv.Itoa(n)
	if n < 10 {
		name = "0" + name
	}
	b := readFile("SOUND\\" + name + ".voc")
	if len(b) > 6*0x4000 {
		b = b[:6*0x4000]
	}
	dprintf("speech: %s.voc, %d bytes\n", name, len(b))
	if b == nil || !snd.PlayVOC(b) {
		speechok = false
		return
	}
	speechLeft = len(b)
}

// updateSpeech: the player has the whole .VOC, so there is nothing to refill.
func updateSpeech() {}

func speechOver() bool { return !speechok || !snd.SpeechPlaying() }

func stopSpeech() {
	if !speechok {
		return
	}
	snd.StopSpeech()
}

func freeSpeechStuff() { speechLeft = 0 }

// readCfgData reads UW.CFG's two sound lines (seg014_1DC5_1D0D): the music card, then
// the speech card, each with its IRQ, port (hex) and DMA.
func readCfgData(b []byte) {
	lines := strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n")
	parse := func(i int, card *int, s *[3]int) {
		if i >= len(lines) {
			return
		}
		f := strings.Fields(lines[i])
		for j := 0; j < 4 && j < len(f); j++ {
			base := 10
			if j == 2 {
				base = 16
			}
			v, err := strconv.ParseInt(f[j], base, 32)
			if err != nil {
				break
			}
			if j == 0 {
				*card = int(v)
			} else {
				s[j-1] = int(v)
			}
		}
	}
	parse(0, &soundCard, &mSettings)
	parse(1, &speechCard, &speechCfg)
}
