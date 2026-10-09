package uw

// IMATH.ASM and C3DENTRY.ASM's maths entries: the table sine and cosine, the square root
// and atan2, with the tables read from seg063 of UW.EXE. Angles are a full turn in 0x10000;
// sines are 1.15 fixed point.

var (
	sinTab  []int16 // seg063:04E0, 0x141 entries; the cosine table is the same from 0x40 on
	asinTab []int16 // seg063:0762
	acosTab []int16 // seg063:0864
)

func initMath() {
	rd := func(off, n int) []int16 {
		b := exeBytes(segSeg063, off, n*2)
		t := make([]int16, n)
		for i := range t {
			t[i] = int16(le16(b, i*2))
		}
		return t
	}
	sinTab = rd(0x4E0, 0x142)
	asinTab = rd(0x762, 0x81)
	acosTab = rd(0x864, 0x81)
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
