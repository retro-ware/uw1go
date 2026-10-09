package uw

// The image decoders of EXPAND.ASM (exp_8str, exp_4str, exp_4run, exp_5run) behind
// cFrmtoRaw: a .GR/.CR bitmap in its stored form to one byte per pixel, translated through
// its auxiliary palette and, optionally, one row of the distance-shading table lightabs.
// As in the original the pixels go to one shared buffer (cmpbuf1), which the next decode
// overwrites.

// lightabs is the shading table: 16 rows of 256 colours (LIGHT.DAT).
var lightabs [16 * 256]byte

var (
	cmpbuf    [0x10000]byte
	unpackBuf [0x10000]byte
	uncmpPal  [32]byte
)

// noShade is cFrmtoRaw's DH = 0FFh: no lightabs row.
const noShade = 0xFF

// cFrmtoRaw decodes bitmap bm (its header at bm[0]: format, width, height, ...) with the
// 16- or 32-colour auxiliary palette pal and lightabs row shade (noShade for none). The
// result is valid until the next decode.
func cFrmtoRaw(bm []byte, pal []byte, shade int) []byte {
	if len(bm) < 5 {
		return nil
	}
	switch bm[0] {
	case 4:
		return exp8str(bm, shade)
	case 0xA:
		return exp4str(bm, pal, shade)
	case 8:
		return exp4run(bm, pal, shade)
	case 6:
		return exp5run(bm, pal, shade)
	}
	return nil
}

// setUncmpPal is seg004_63: the aux palette's rows of 16, shaded through lightabs.
func setUncmpPal(pal []byte, rows int, shade int) {
	for i := 0; i < rows*16; i++ {
		var v byte
		if i < len(pal) {
			v = pal[i]
		}
		if shade != noShade {
			v = lightabs[(shade&0xF)<<8|int(v)]
		}
		uncmpPal[i] = v
	}
}

func exp8str(bm []byte, shade int) []byte {
	n := int(le16(bm, 3))
	src := bm[5:]
	n = min(n, len(src))
	for i := 0; i < n; i++ {
		v := src[i]
		if shade != noShade {
			v = lightabs[(shade&0xF)<<8|int(v)]
		}
		cmpbuf[i] = v
	}
	return cmpbuf[:n]
}

func exp4str(bm []byte, pal []byte, shade int) []byte {
	setUncmpPal(pal, 1, shade)
	n := int(le16(bm, 4))
	src := bm[6:]
	n = min(n, len(src), len(cmpbuf)/2)
	for i := 0; i < n; i++ {
		cmpbuf[2*i] = uncmpPal[src[i]>>4]
		cmpbuf[2*i+1] = uncmpPal[src[i]&0xF]
	}
	return cmpbuf[:2*n]
}

func exp4run(bm []byte, pal []byte, shade int) []byte {
	setUncmpPal(pal, 1, shade)
	n := int(le16(bm, 4))
	src := bm[6:]
	for i := 0; i < (n+1)/2; i++ {
		var b byte
		if i < len(src) {
			b = src[i]
		}
		unpackBuf[2*i] = b >> 4
		unpackBuf[2*i+1] = b & 0xF
	}
	return runDecode(unpackBuf[:n])
}

func exp5run(bm []byte, pal []byte, shade int) []byte {
	setUncmpPal(pal, 2, shade)
	n := int(le16(bm, 4))
	src := bm[6:]
	// five bytes to eight 5-bit words, most significant bit first
	o := 0
	for g := 0; g < (n+7)/8; g++ {
		var acc uint64
		for k := 0; k < 5; k++ {
			acc <<= 8
			if i := g*5 + k; i < len(src) {
				acc |= uint64(src[i])
			}
		}
		for k := 7; k >= 0; k-- {
			if o < len(unpackBuf) {
				unpackBuf[o] = byte(acc>>(5*k)) & 0x1F
			}
			o++
		}
	}
	return runDecode(unpackBuf[:min(n, len(unpackBuf))])
}

// rleDecoder is seg004_18D/seg004_194: alternate repeat and run records over words stored
// one to a byte. patched stands for the self-modified ret at L01A6 used while decoding a
// group of repeat records (count 2).
type rleDecoder struct {
	in      []byte
	si      int
	out     []byte
	di      int
	patched bool
}

func (d *rleDecoder) next() int {
	if d.si >= len(d.in) {
		d.si++
		return 0
	}
	v := d.in[d.si]
	d.si++
	return int(v)
}

// count reads an extended count after a zero word: w<<4|w' when the next word is nonzero,
// else the four words after it as a 16-bit number.
func (d *rleDecoder) count() int {
	x := d.next()
	if x != 0 {
		return x<<4 | d.next()
	}
	a := d.next()
	a = a<<4 | d.next()
	a = a<<4 | d.next()
	a = a<<4 | d.next()
	return a & 0xFFFF
}

func (d *rleDecoder) fill(n int, v byte) {
	for ; n > 0; n-- {
		if d.di < len(d.out) {
			d.out[d.di] = v
		}
		d.di++
	}
}

func (d *rleDecoder) records() {
	for d.si < len(d.in) {
		switch al := d.next(); {
		case al > 2:
			d.fill(al, uncmpPal[d.next()&0x1F])
		case al == 0:
			n := d.count()
			d.fill(n, uncmpPal[d.next()&0x1F])
		case al == 2:
			d.patched = true
			n := d.next()
			if n == 0 {
				n = d.count()
			}
			for ; n > 0; n-- {
				d.records()
			}
			d.patched = false
		}
		// L01A6: the run record
		if d.patched {
			return
		}
		n := d.next()
		if n == 0 {
			n = d.count()
		}
		for ; n > 0; n-- {
			if d.di < len(d.out) {
				d.out[d.di] = uncmpPal[d.next()&0x1F]
			} else {
				d.next()
			}
			d.di++
		}
	}
}

func runDecode(words []byte) []byte {
	d := rleDecoder{in: words, out: cmpbuf[:]}
	d.records()
	return cmpbuf[:min(d.di, len(cmpbuf))]
}
