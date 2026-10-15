package uw

import (
	"bytes"
	mrand "math/rand/v2"
	"testing"
)

func TestDiffPatch(t *testing.T) {
	r := mrand.New(mrand.NewPCG(1, 2))
	for n := 0; n < 200; n++ {
		a := make([]byte, syncLen)
		for i := range a {
			a[i] = byte(r.IntN(4))
		}
		b := bytes.Clone(a)
		for k := r.IntN(50); k > 0; k-- {
			b[r.IntN(len(b))] ^= byte(1 + r.IntN(255))
		}
		d := diffBytes(a, b)
		c := bytes.Clone(a)
		if !patchBytes(c, d, func(int) bool { return false }) || !bytes.Equal(c, b) {
			t.Fatalf("round %d: patched block differs", n)
		}
		kept := bytes.Clone(a)
		patchBytes(kept, d, func(i int) bool { return i < 0x4000 })
		if !bytes.Equal(kept[:0x4000], a[:0x4000]) || !bytes.Equal(kept[0x4000:], b[0x4000:]) {
			t.Fatalf("round %d: kept bytes changed", n)
		}
	}
	if d := diffBytes(make([]byte, syncLen), make([]byte, syncLen)); len(d) != 0 {
		t.Fatal("a diff of equal blocks")
	}
}
