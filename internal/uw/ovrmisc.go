package uw

// The small overlays: DEBUG.C (ovr106), OVR126.C (the credits), TRIGSAVE.C (ovr149), and
// the empty hooks of OVR090.C, OVR096.C (browse), OVR137.C and OVR152.C, which nothing
// calls. OVR139/144/146's class data handlers, which return 0, are getClassData's nil
// cases (objclass.go). OVR127.C raised the COM1 interrupt, a development hook with no
// handler here.

// initDebug binds Alt+F4, in every screen mode, to ovr127_0.
func initDebug() {
	inputAddkey(KEY_ALT|KEY_F4, 0, 0xFF, func(any) { ovr127_0() })
}

// ovr127_0 raised int 0Ch (COM1) in software; there is nothing on it.
func ovr127_0() {}

// showCredits plays cutscene 10, the credits, and asks for a full redraw.
func showCredits() {
	runcutscene(10)
	editchng(0x7FFE)
}

// trapSave writes the trigger table to an open save file.
func trapSave(fd int) { dosWrite(fd, Triggers[:]) }

// OVR096.C: the browse hooks.
func initBrowse()      {}
func freeBrowse()      {}
func placeBrowse() int { return 1 }
func browseShow()      {}
func browseHide()      {}
func browseRedisplay() {}
func browseChain()     {}
func ovr137_0()        {}
func ovr090_5()        { newscr(16) }
func ovr152_5()        { newscr(16) }
func ovr106_20()       {}
