package uw

// Port of MAINLOOP.C: the main loop and the change dispatcher.

var (
	notdone       bool
	changed       int
	scrnum        int
	pmouseHandled int
	// editorDispatch and changeState are UWEDIT.C's: per screen, a handler per change bit,
	// and the bits kept set.
	editorDispatch [3][16]func()
	changeState    = [3]int{0x3800, 0x1000, 0}
)

func mainloop() {
	for notdone {
		if changed != 0 {
			doChanges()
		}
		inputDispatch(inplist)
	}
}

// doChanges runs editor_dispatch[scrnum][bit] for each change bit set.
func doChanges() {
	if inGame && scrnum == 0 {
		debugWarp()
		debugTalk()
	}
	if changed == 0 {
		return
	}
	for i, bit := 0, 1; i < 15; i, bit = i+1, bit<<1 {
		if changed&bit != 0 {
			changed &^= bit
			if f := editorDispatch[scrnum][i]; f != nil {
				f()
			}
		}
	}
	changed |= changeState[scrnum]
	if pmouseHandled > 0 {
		pmouseHandled--
		if pmouseHandled == 0 {
			mouseFreereign()
		}
	}
}

func editchng(bits int) { changed |= bits }
