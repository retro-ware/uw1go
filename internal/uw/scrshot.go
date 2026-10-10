package uw

import "github.com/memmaker/uw1go/internal/gr"

// SCRSHOT.C: saving the screen as a GIF file (Alt+Q). saveScreenshot picks the first
// unused name uwpicNNN.gif (NNN octal), writes a GIF87a header for a 320 by 200 picture
// with the palette scaled from 6 to 8 bits, an image descriptor, the LZW data and the ';'
// trailer. The encoder is the usual GIF LZW with a 5003-entry open-addressed hash table:
// codes start at bits + 1 bits, grow to 12, and a clear code restarts the table when it
// fills.

var (
	gifHeader = [13]byte{'G', 'I', 'F', '8', '7', 'a', 0x40, 0x01, 0xC8, 0, 0xF7, 0, 0}
	gifImage  = [10]byte{',', 0, 0, 0, 0, 0x40, 0x01, 0xC8, 0, 7}
)

const gifHashSize = 0x138B

// gifState is the encoder's state.
type gifState struct {
	bits, bitpos     int
	codes            [gifHashSize]uint16
	bytepos, clear   int
	pixels           [0x142]byte
	hash             [gifHashSize]uint16
	bytes            [0x104]byte
	codebits, x, y   int
	end, limit, next int
	suffix           [gifHashSize]byte
	fd               int
}

func saveScreenshot() {
	number := 0
	name := []byte("uwpic000.gif")
	for {
		fd := dosOpen(string(name), O_RDONLY)
		if fd == -1 {
			break
		}
		dosClose(fd)
		number++
		name[7] = byte(number&7) + '0'
		name[6] = byte(number>>3&7) + '0'
		name[5] = byte(number>>6&7) + '0'
	}
	fd := dosOpen(string(name), O_WRONLY|O_CREAT)
	if fd == -1 {
		return
	}
	g := &gifState{fd: fd}
	dosWrite(fd, gifHeader[:])
	pal := make([]byte, 0x300)
	for i := range pal {
		pal[i] = gr.Palette[i] << 2
	}
	dosWrite(fd, pal)
	dosWrite(fd, gifImage[:])
	g.encode(8)
	dosWrite(fd, []byte{';'})
	dosClose(fd)
}

// reset resets the LZW table for a root size of bits.
func (g *gifState) reset(bits int) {
	g.codebits = bits + 1
	g.clear = 1 << bits
	g.end = g.clear + 1
	g.next = g.clear + 2
	g.limit = 1 << g.codebits
	g.hash = [gifHashSize]uint16{}
}

// block writes a data sub-block: its length byte, then size bytes.
func (g *gifState) block(size int) {
	dosWrite(g.fd, []byte{byte(size)})
	dosWrite(g.fd, g.bytes[:byte(size)])
}

// put appends code, codebits wide, flushing a sub-block once 254 bytes are full.
func (g *gifState) put(code int) {
	g.bytepos = g.bitpos >> 3
	g.bits = g.bitpos & 7
	if g.bytepos >= 0xFE {
		g.block(g.bytepos)
		g.bytes[0] = g.bytes[g.bytepos]
		g.bitpos = g.bits
		g.bytepos = 0
	}
	if g.bits > 0 {
		v := uint32(code)<<g.bits | uint32(g.bytes[g.bytepos])
		g.bytes[g.bytepos] = byte(v)
		g.bytes[g.bytepos+1] = byte(v >> 8)
		g.bytes[g.bytepos+2] = byte(v >> 16)
	} else {
		g.bytes[g.bytepos] = byte(code)
		g.bytes[g.bytepos+1] = byte(code >> 8)
	}
	g.bitpos += g.codebits
}

// encode writes the LZW-coded image: the minimum code size, the sub-blocks and the empty
// block that ends them.
func (g *gifState) encode(bits int) {
	dosWrite(g.fd, []byte{byte(bits)})
	g.bitpos = 0
	g.reset(bits)
	g.put(g.clear)
	prefix := g.pixel()
	for {
		next := g.pixel()
		if next == -1 {
			break
		}
		slot := (prefix ^ next<<5) % gifHashSize
		step := 1
		for {
			if g.hash[slot] == 0 {
				g.put(prefix)
				step = g.next
				if g.next <= 0xFFF {
					g.codes[slot] = uint16(prefix)
					g.suffix[slot] = byte(next)
					g.hash[slot] = uint16(g.next)
					g.next++
				}
				if step == g.limit {
					if g.codebits < 12 {
						g.codebits++
						g.limit <<= 1
					} else {
						g.put(g.clear)
						g.reset(bits)
					}
				}
				prefix = next
				break
			}
			if int(g.codes[slot]) == prefix && int(g.suffix[slot]) == next {
				prefix = int(g.hash[slot])
				break
			}
			slot += step
			step += 2
			if slot >= gifHashSize {
				slot -= gifHashSize
			}
		}
	}
	g.put(prefix)
	g.put(g.end)
	if g.bitpos > 0 {
		g.block((g.bitpos + 7) / 8)
	}
	g.block(0)
}

// pixel is the next pixel, top row first, or -1 at the end.
func (g *gifState) pixel() int {
	if g.x == 0x140 && g.y == 0xC7 {
		return -1
	}
	if g.x == 0x140 {
		g.x = 0
		g.y++
	}
	if g.x == 0 {
		gr.Grab(g.pixels[1:], 0, 0xC7-g.y, 0x140, 1)
	}
	g.x++
	return int(g.pixels[g.x])
}
