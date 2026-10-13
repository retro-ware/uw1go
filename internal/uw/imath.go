package uw

import "math"

// IMATH.ASM and C3DENTRY.ASM's maths entries: the table sine and cosine, the square root
// and atan2. Angles are a full turn in 65536; sines are 1.15 fixed point. The tables are
// the original's (UW.EXE's seg063), which these formulas give exactly.

var (
	sinTab  = sineTable()         // a 256th of a turn apart, and on: cosines are 64 entries in
	asinTab = arcTable(math.Asin) // of 0, 1/128 ... 1
	acosTab = arcTable(math.Acos)
)

// sineTable is the sine of each 256th of a turn, rounded, over a turn and a quarter (so
// that cosines can be read 64 entries on) and two more for the interpolation.
func sineTable() []int16 {
	t := make([]int16, 256+64+2)
	for i := range t {
		v := math.Round(math.Sin(2*math.Pi*float64(i)/256) * 32768)
		t[i] = int16(min(max(v, -32767), 32767))
	}
	return t
}

// arcTable is f of 0, 1/128 ... 1 as an angle (a full turn in 65536), truncated.
func arcTable(f func(float64) float64) []int16 {
	t := make([]int16, 129)
	for i := range t {
		t[i] = int16(f(float64(i)/128) * 65536 / (2 * math.Pi))
	}
	return t
}

// interp is the tables' linear step: base + ((next - base) * frac) >> 8, in 16 bits.
func interp(base, next int16, frac int) int16 {
	p := int32(int16(next-base)) * int32(frac)
	return int16(p>>8) + base
}

// cSinCos returns the sine and cosine of angle, interpolated.
func cSinCos(angle int) (int16, int16) {
	a := uint16(angle)
	i := int(a >> 8)
	f := int(a & 0xFF)
	s := interp(sinTab[i], sinTab[i+1], f)
	c := interp(sinTab[i+0x40], sinTab[i+0x41], f)
	return s, c
}

// cFstSinCos returns the table sine and cosine of the angle's high byte.
func cFstSinCos(angle int) (int16, int16) {
	i := int(uint16(angle) >> 8)
	return sinTab[i], sinTab[i+0x40]
}

// cSqRt is the integer square root of a 32-bit value, five rounds of Newton's method.
func cSqRt(v uint32) int {
	hi := uint16(v >> 16)
	lo := uint16(v)
	div := func(d uint32) uint32 {
		if d == 0 {
			return 0xFFFF
		}
		q := v / d
		if q > 0xFFFF {
			q = 0xFFFF
		}
		return q
	}
	switch {
	case hi>>8 != 0:
		var di uint32 = 0x4000
		if hi >= 0x4000 {
			di = 0xFFFF
		}
		for i := 0; i < 5; i++ {
			di = (di + div(di)) >> 1 & 0xFFFF
		}
		return int(di)
	case hi&0xFF != 0:
		var di uint32 = 0x400
		for i := 0; i < 5; i++ {
			di = (di + div(di)) >> 1 & 0xFFFF
		}
		return int(di)
	case lo>>8 != 0:
		var cx uint32 = 0x40
		for i := 0; i < 5; i++ {
			cx = (cx + div(cx)) >> 1 & 0xFFFF
		}
		return int(cx)
	case lo&0xFF != 0:
		var cl uint32 = 4
		for i := 0; i < 5; i++ {
			q := uint32(lo) / cl
			if q > 0xFF {
				q = 0xFF
			}
			cl = (cl + q&0xFF) >> 1 & 0xFF
		}
		return int(cl)
	}
	return 0
}

// asin is IMATH's _B78: the arcsine of a 1.15 value, interpolated, with its sign.
func asin(ax int16) int16 {
	neg := ax < 0
	a := ax
	if neg {
		a = -a
	}
	i := int(uint16(a) >> 8)
	f := int(uint16(a) & 0xFF)
	c := interp(asinTab[i], asinTab[i+1], f)
	if neg {
		c = -c
	}
	return c
}

// acos is IMATH's _BA2: the arccosine of a 1.15 value; a negative one gives the other half.
func acos(bx int16) int16 {
	neg := bx < 0
	a := bx
	if neg {
		a = -a
	}
	i := int(uint16(a) >> 8)
	f := int(uint16(a) & 0xFF)
	c := interp(acosTab[i], acosTab[i+1], f)
	if neg {
		c = -c
		c = int16(uint16(c) + 0x8000)
	}
	return c
}

// cAtan2 is the angle of the unit vector with sine ax and cosine bx.
func cAtan2(ax, bx int) int {
	s, c := int16(ax), int16(bx)
	if s <= 0x5A82 && s >= -0x5A82 {
		r := asin(s)
		if c < 0 {
			r = -int16(uint16(r) - 0x8000)
		}
		return int(r)
	}
	r := acos(c)
	if s < 0 {
		r = -r
	}
	return int(r)
}
