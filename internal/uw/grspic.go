package uw

import "github.com/memmaker/uw1go/internal/gr"

// Port of GRSPIC.C: graphic resource lookup, decoding and drawing by icon number.

// grsUnpack returns a bitmap's 8-bit pixels: its data for an 8-bit bitmap, else the
// expansion of a 4-bit one through its auxiliary palette.
func grsUnpack(bm []byte) []byte {
	if len(bm) < 5 {
		return nil
	}
	if bm[0] != BM_8BIT {
		return cFrmtoRaw(bm, Palettes[bm[3]&0x1F][:], noShade)
	}
	return bm[5:]
}

// bmSize returns a bitmap's width and height.
func bmSize(bm []byte) (int, int) {
	if len(bm) < 3 {
		return 0, 0
	}
	return int(bm[1]), int(bm[2])
}

// showSlotIcon draws slot icon at x, y on the screen.
func showSlotIcon(icon, x, y int) {
	if icon >= firstVram {
		hd := slotVram(icon)
		w, h := hd.Size()
		hd.Show(x, y, w, h, 0, 0)
		return
	}
	bm := slotPic(icon)
	w, h := bmSize(bm)
	if pix := grsUnpack(bm); pix != nil {
		gr.Show(x, y, pix, h, w, 0, 0)
	}
}

// grsFbplot draws slot icon into the 3D view's frame buffer.
func grsFbplot(icon, x, y int) {
	bm := slotPic(icon)
	w, h := bmSize(bm)
	if pix := grsUnpack(bm); pix != nil {
		gr.FBShow(pix, x, y, w, h)
	}
}

// grsWhich1 maps an icon number to its slot.
func grsWhich1(icon int) int {
	switch {
	case icon >= ICON_VRAM:
		return icon - ICON_VRAM + firstVram
	case icon >= ICON_ART:
		return icon - ICON_ART + firstButton
	}
	if 2*icon < 0 || 2*icon >= len(objTab) {
		return 0
	}
	return int(objTab[2*icon] & 0x3FF)
}

// picToScreen draws an icon on the screen; icons 101Bh..101Eh always with Transparency.
// (The height and width arguments of the original are not used.)
func picToScreen(icon, x, y int) {
	special := icon >= ICON_BUTTONS+0x1B && icon <= ICON_BUTTONS+0x1E
	if special {
		gr.Transparency = true
	}
	showSlotIcon(grsWhich1(icon), x, y)
	if special {
		gr.Transparency = false
	}
}

// picSize returns an icon's width and height (seg009_2C6).
func picSize(icon int) (int, int) {
	s := grsWhich1(icon)
	if s >= firstVram {
		return slotVram(s).Size()
	}
	return bmSize(slotPic(s))
}

func picToFbuf(icon, x, y int) {
	grsFbplot(grsWhich1(icon), x, y)
}

// maskToScreen draws a video memory picture skipping clip of its rows (see GRSPIC.C on
// the swapped parameter names).
func maskToScreen(icon, x, y, width, height, clip int) {
	s := grsWhich1(icon)
	slotVram(s).Show(x, y, height, width+clip, 0, clip)
}
