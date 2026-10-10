package r3d

import (
	"os"
	"testing"
)

func TestFB(t *testing.T) {
	b, err := os.ReadFile("../../gamedata/UW/UW.EXE")
	if err != nil {
		t.Skip(err)
	}
	Load(b)
	t.Logf("before: fbseg %04X rows %04X %04X %04X", R16(Seg048+0x95A), R16(Seg048+0x95E), R16(Seg048+0x960), R16(Seg048+0x962))
	CInit3d()
	CPlaceFB(0x34, 0x13, 0xAC, 0x70)
	t.Logf("after: fbseg %04X rows %04X %04X %04X", R16(Seg048+0x95A), R16(Seg048+0x95E), R16(Seg048+0x960), R16(Seg048+0x962))
	t.Logf("dbase %04X:%04X entry %04X:%04X dbbase %04X:%04X", R16(Seg051+0x2938+2), R16(Seg051+0x2938), R16(Seg051+0x73F2+2), R16(Seg051+0x73F2), R16(Seg051+0x161C+2), R16(Seg051+0x161C))
	t.Logf("dblen@AFF4 %04X bmhgtoff %04X bmsegoff %04X", R16(Seg051+0xAFF4), R16(Seg051+0xB000), R16(Seg051+0xB002))
	t.Logf("cam ptr? seg063:09C8 %04X  lightabs? pixxfer %04X", R16(Seg063+0x9C8), R16(Seg048+0xB12))
}
