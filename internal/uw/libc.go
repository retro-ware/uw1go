package uw

import "strings"

// What the game takes from Borland's C library.

var randSeed uint32 = 1

// rand is Borland C's: a linear congruential generator, 15 bits out.
func rand() int {
	randSeed = randSeed*0x015A4E35 + 1
	return int(randSeed>>16) & 0x7FFF
}

func srand(seed uint) { randSeed = uint32(seed) }

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// toUpper is strupr.
func toUpper(s string) string { return strings.ToUpper(s) }
