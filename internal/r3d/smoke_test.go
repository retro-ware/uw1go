package r3d

import (
	"os"
	"testing"
)

func TestSmoke(t *testing.T) {
	b, err := os.ReadFile("../../gamedata/UW/UW.EXE")
	if err != nil {
		t.Skip(err)
	}
	if err := Load(b); err != nil {
		t.Fatal(err)
	}
	CInit3d()
	CPlaceFB(0x34, 0x13, 0xAC, 0x70)
	s, c := CSinCos(0x4000)
	t.Logf("sincos(4000) = %d %d", s, c)
	r := CRender()
	t.Logf("render ticks %d", r)
}
