package uw

import (
	"encoding/binary"
	"os"
	"testing"
)

// The computed tables are UW.EXE's (seg063), checked against a copy when there is one.
func TestMathTablesMatchTheOriginal(t *testing.T) {
	b, err := os.ReadFile("../../gamedata/UW/UW.EXE")
	if err != nil {
		t.Skip(err)
	}
	seg063 := 800*16 + 22052*16
	for _, c := range []struct {
		name  string
		at    int
		table []int16
	}{
		{"sine", seg063 + 1248, sinTab[:321]}, // the original's last entry is padding
		{"arcsine", seg063 + 1890, asinTab},
		{"arccosine", seg063 + 2148, acosTab},
	} {
		for i, v := range c.table {
			if want := int16(binary.LittleEndian.Uint16(b[c.at+2*i:])); v != want {
				t.Fatalf("%s[%d] = %d, the original has %d", c.name, i, v, want)
			}
		}
	}
}

// The keyboard layout is UW.EXE's (seg063's tables, for the 101/102-key keyboard).
func TestKeyLayoutMatchesTheOriginal(t *testing.T) {
	b, err := os.ReadFile("../../gamedata/UW/UW.EXE")
	if err != nil {
		t.Skip(err)
	}
	seg063 := 800*16 + 22052*16
	for i, c := range ascTab {
		if want := b[seg063+16+i]; c != want {
			t.Errorf("ascTab[%d] = %d, the original has %d", i, c, want)
		}
	}
	for i, k := range extendedKeys {
		if want := b[seg063+648+i]; k != want {
			t.Errorf("extendedKeys[%d] = %d, the original has %d", i, k, want)
		}
	}
	mods := seg063 + 665 + 2 // the 101/102-key column
	for m := range modKeyA {
		a := int(binary.LittleEndian.Uint16(b[mods+16*m:]))
		bb := int(binary.LittleEndian.Uint16(b[mods+16*m+8:]))
		if modKeyA[m] != a || modKeyB[m] != bb {
			t.Errorf("modifier %d: keys %d, %d; the original has %d, %d", m, modKeyA[m], modKeyB[m], a, bb)
		}
	}
}
