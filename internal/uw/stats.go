package uw

import (
	"strconv"
	"strings"

	"github.com/felix/uw1go/internal/gr"
)

// Port of STATS.C: the character panel's statistics page: the name, class and level
// header, the three attributes, hit points, mana, experience and a scrolling list of six
// skills, drawn with the 5x6 italic font right-aligned at x statRight.

const statRight = 0x138

var (
	skillTop byte
	ordinals = [4]string{"ST", "ND", "RD", "TH"}
	// spsave holds the saved backgrounds of the skill list and the hit point and mana figures.
	spsave [3]int
)

func statRightAligned(s string, y int) { gr.StringToScreen(s, statRight-gr.StringWidth(s), y) }

// spHdr draws the name (up to 15 characters, upper-cased, centred), class and level.
func spHdr() {
	name := cstr(player.Name[:])
	if len(name) > 0xF {
		name = name[:0xF]
	}
	name = strings.ToUpper(name)
	gr.StringToScreen(name, (0x48-gr.StringWidth(name))/2+0xF2, 0xB9)
	gr.StringToScreen(strings.ToUpper(getString((player.PClass()+0x17)|STR_CHARGEN)), 0xF2, 0xB2)
	n := 3
	if player.Level < 4 {
		n = int(player.Level) - 1
	}
	statRightAligned(strconv.Itoa(int(player.Level))+ordinals[n&3], 0xB2)
}

// spAtt draws attribute i (0 to 2), one row each from y 0xB1 down.
func spAtt(i int) {
	statRightAligned(strconv.Itoa(int(playerdat.Attr[i])), (2-i)*7+0x9D)
}

func spHp() {
	statRightAligned(strconv.Itoa(int(ThePlayer.HP))+"/"+strconv.Itoa(int(playerdat.Avghit)), 0x96)
}

func spMp() {
	statRightAligned(strconv.Itoa(int(player.PlayMana))+"/"+strconv.Itoa(int(player.MaxMana)), 0x8F)
}

// spXp: experience is shown divided by ten.
func spXp() {
	statRightAligned(strconv.Itoa(int(int32(player.Exp)/10)), 0x88)
}

// spSkill draws row i of the skill list: skill i + skillTop.
func spSkill(i int) {
	s := i + int(skillTop)
	gr.ShowBlock(spsave[0], 0xF0, 0x80-i*7+1, 0x4C, (i+1)*7, 0, i*7)
	gr.StringToScreen(strings.ToUpper(getString((s+0x1F)|STR_CHARGEN)), 0xF2, 0x80-i*7)
	statRightAligned(strconv.Itoa(int(player.Skills[s])), 0x80-i*7)
}

// RedispStat draws the statistics page, saving the backgrounds it redraws the first time.
func RedispStat() {
	if spsave[0] == 0 {
		if spsave[0] = gr.Valloc(0x4B, 0x2A); spsave[0] != 0 {
			gr.SaveRect(spsave[0], 0xF0, 0x81, 0x4B, 0x2B)
		}
		if spsave[1] = gr.Valloc(0x23, 0x15); spsave[1] != 0 {
			gr.SaveRect(spsave[1], 0x115, 0x96, 0x23, 0x15)
		}
	}
	gr.ForegroundColor, gr.BackgroundColor = 0xF1, 0xF1
	mouseHide()
	grfxLoadFont("font5x6i.sys")
	spHdr()
	for i := 0; i < 3; i++ {
		spAtt(i)
	}
	spHp()
	spMp()
	spXp()
	gr.ForegroundColor, gr.BackgroundColor = 0x68, 0x68
	for i := 0; i < 6; i++ {
		spSkill(i)
	}
	grfxLoadFont("font5x6p.sys")
	mouseShow()
}

// mousInStat: a click in the arrow strip at the bottom scrolls the skill list.
func mousInStat() {
	grfxLoadFont("font5x6i.sys")
	gr.ForegroundColor, gr.BackgroundColor = 0x68, 0x68
	if inplist.y < 8 {
		v := int16(skillTop)
		dir, limit := 1, 0xE
		if inplist.x < 0x25 {
			dir, limit = -1, 0
		}
		if mvcheck(&v, limit, 1, dir) {
			skillTop = byte(v)
			mouseHide()
			for i := 0; i < 6; i++ {
				spSkill(i)
			}
			mouseShow()
		}
	}
	grfxLoadFont("font5x6p.sys")
	mouseRelease(true)
}

// panelCheck (ovr145_4FB) redraws the statistics page when it is showing.
func panelCheck() {
	if RightPanel == PANEL_STATS {
		prettyPanelagain()
	}
}
