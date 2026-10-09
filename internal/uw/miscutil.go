package uw

// MISCUTIL.C: directions, the "it is to the north" messages, start-up checks, whole-file
// reads and writes and the PLAYER.DAT cipher.

func flipBool(p *bool) { *p = !*p }

// solveCompass: which ninth of an a by b box (x, y) falls in, as a direction.
func solveCompass(a, b, x, y int) int {
	q := ((x * 3) / a) % 3
	r := ((y * 3) / b) % 3
	switch q {
	case 2:
		return 3 - r
	case 0:
		return r + 5
	case 1:
		if r == 1 {
			return 8
		}
		return 4 - r*2
	}
	return 8
}

// mpos is the direction of the vector (x, y), 0 to 7: 0 is +y, 2 +x, 4 -y, 6 -x.
func mpos(x, y int8) int {
	ax, ay := abs(int(x)), abs(int(y))
	if ax/2 > ay {
		if x > 0 {
			return 2
		}
		return 6
	} else if ay/2 > ax {
		if y > 0 {
			return 0
		}
		return 4
	} else if x < 0 {
		return b2i(y > 0)*2 + 5
	}
	return b2i(y < 0)*2 + 1
}

func postodir(x1, y1, x2, y2 int) int { return mpos(int8(x2)-int8(x1), int8(y2)-int8(y1)) }

// printPathTo prints s and then where something is (see MISCUTIL.C).
func printPathTo(s string, px, py, how, ox, oy, previous, radius int) {
	shown := false
	scrollPrint(s)
	if radius < 0 {
		gameSprint(0x24 - radius - 1)
		shown = true
	} else if abs(px-ox)+abs(py-oy) > radius {
		gameSprint(postodir(px, py, ox, oy) + 0x24)
		shown = true
	}
	if how != previous && how != 0 {
		if shown {
			scrollPrint(" and ")
		}
		gameSprint(how + 0x33 - previous)
	} else if !shown && how != 0 {
		scrollPrint("very near")
	}
	scrollPrint(".\n")
}

// errmsg prints "Underworld internal error" and the two strings.
func errmsg(a, b string) {
	scrollPrint("Underworld internal error\n ")
	scrollPrint(a)
	scrollPrint(" ")
	scrollPrint(b)
}

func doBeep(freq, ms int) int { return 0 }

// checkDirs: DATA, CRIT and CUTS must exist.
func checkDirs() {
	if !dirExist("data") || !dirExist("crit") || !dirExist("cuts") {
		firstPunt(ERR_READ | 1)
	}
}

// blttodrive writes buf as the whole file name.
func blttodrive(buf []byte, name string) bool {
	fd := dosOpen(name, O_WRONLY|O_CREAT|O_TRUNC)
	if fd < 0 {
		return false
	}
	ok := dosWrite(fd, buf) == len(buf)
	ok = dosClose(fd) == 0 && ok
	return ok
}

// bltfromdrive reads len(buf) bytes of file name into buf.
func bltfromdrive(name string, buf []byte) bool {
	fd := dosOpen(name, O_RDONLY)
	if fd < 0 {
		return false
	}
	ok := dosRead(fd, buf) == len(buf)
	ok = dosClose(fd) == 0 && ok
	return ok
}

// xorTable is the PLAYER.DAT cipher's key table: entry i is key + 3*(i+1).
func xorTable(key byte) [0x50]byte {
	var t [0x50]byte
	for i := range t {
		key += 3
		t[i] = key
	}
	return t
}

// xorread reads n bytes of fd into buf, deciphered.
func xorread(fd int, key byte, buf []byte) int {
	t := xorTable(key)
	total := 0
	for o := 0; o < len(buf); o += 0x50 {
		end := min(o+0x50, len(buf))
		n := dosRead(fd, buf[o:end])
		if n < 0 {
			break
		}
		for i := 0; i < n; i++ {
			buf[o+i] ^= t[i]
		}
		total += n
	}
	return total
}

// xorwrite writes buf to fd, enciphered.
func xorwrite(fd int, key byte, buf []byte) int {
	t := xorTable(key)
	total := 0
	enc := make([]byte, 0x50)
	for o := 0; o < len(buf); o += 0x50 {
		end := min(o+0x50, len(buf))
		for i := o; i < end; i++ {
			enc[i-o] = buf[i] ^ t[i-o]
		}
		total += dosWrite(fd, enc[:end-o])
	}
	return total
}
