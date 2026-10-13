package uw

import (
	"fmt"
	"testing"
)

func TestZZModelUse(t *testing.T) {
	GameDir = "../../gamedata/UW"
	if loadStringsPak() != 0 {
		t.Skip()
	}
	ark := readFile("DATA\\LEV.ARK")
	for idx, m := range rect_sub {
		if m < 0 {
			continue
		}
		item := 0x150 + idx
		count := 0
		for lev := 0; lev < 9; lev++ {
			o := int(le32(ark, 2+lev*4))
			objs := o + 64*64*4
			for i := 1; i < 1024; i++ {
				at := objs + i*8
				if i >= 256 {
					at = objs + 256*27 + (i-256)*8
				} else {
					at = objs + i*27
				}
				if int(le16(ark, at))&0x1FF == item {
					count++
				}
			}
		}
		fmt.Printf("model %2d item %d %-24q in levels: %d\n", m, item, readString(4, item), count)
	}
}
