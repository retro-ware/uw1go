package uw

// ARC.C: the .ark archives (LEV.ARK, CNV.ARK). A word, the number of blocks n, then n
// longs, the blocks' file offsets (0 for an absent block); a block runs to the next
// block's offset or the end of the file. Writing a block that is new appends it; one of
// the same length is overwritten in place; one whose length changed is moved to the end,
// the archive copied into _arc.tmp without the old copy and renamed over the archive.

// Arc is an open archive (file.h's struct Arc).
type Arc struct {
	fd     int
	tmpfd  int
	count  int
	offtab []uint32
	dirty  bool
}

// arcstr: the archive's and the temporary file's names.
var arcName, arcTmpName string

// openArc opens the archive name into arc, with its temporary file, and reads its offset
// table; true if all went well.
func openArc(arc *Arc, name string) bool {
	path := ""
	if i := lastIndexByte(name, '\\'); i >= 0 {
		path = name[:i+1]
	}
	path += "_arc.tmp"
	fd := dosOpen(name, O_RDWR)
	if fd == -1 {
		return false
	}
	ok := true
	var cnt [2]byte
	ok = ok && dosRead(fd, cnt[:]) == 2
	count := int(le16(cnt[:], 0))
	tab := make([]byte, count*4)
	ok = ok && dosRead(fd, tab) == len(tab)
	tmpfd := dosOpen(path, O_WRONLY|O_CREAT|O_TRUNC)
	ok = ok && tmpfd != -1
	arc.fd = fd
	arc.tmpfd = tmpfd
	arc.count = count
	arc.offtab = make([]uint32, count)
	for i := range arc.offtab {
		arc.offtab[i] = le32(tab, i*4)
	}
	arc.dirty = false
	arcName = name
	arcTmpName = path
	return ok
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// closeArc writes the offset table back if it changed, closes the archive and deletes the
// temporary file.
func closeArc(arc *Arc) bool {
	ok := true
	if arc.dirty {
		ok = ok && dosSeek(arc.fd, 2, 0) == 2
		tab := make([]byte, arc.count*4)
		for i, o := range arc.offtab {
			put32(tab, i*4, o)
		}
		ok = ok && dosWrite(arc.fd, tab) == len(tab)
	}
	ok = dosClose(arc.fd) == 0 && ok
	dosClose(arc.tmpfd)
	dosUnlink(arcTmpName)
	return ok
}

// putArc writes buf as block blk.
func putArc(arc *Arc, blk int, buf []byte) bool {
	if arc.count < blk || blk >= len(arc.offtab) {
		return false
	}
	off := arc.offtab[blk]
	length := uint32(len(buf))
	if off == 0 {
		pos := dosSeek(arc.fd, 0, 2)
		ok := dosWrite(arc.fd, buf) == len(buf)
		arc.dirty = true
		arc.offtab[blk] = uint32(pos)
		return ok
	}
	room := uint32(dosSeek(arc.fd, 0, 2)) - off
	for i := 0; i < arc.count; i++ {
		diff := arc.offtab[i] - off
		if arc.offtab[i] > off && diff < room {
			room = diff
		}
	}
	if length == room {
		dosSeek(arc.fd, int64(off), 0)
		return dosWrite(arc.fd, buf) == len(buf)
	}
	arc.dirty = true
	dosSeek(arc.fd, 0, 0)
	dosSeek(arc.tmpfd, 0, 0)
	var pos uint32
	chunk := make([]byte, 0x2000)
	for pos < off {
		n := off - pos
		if n > 0x2000 {
			n = 0x2000
		}
		r := dosRead(arc.fd, chunk[:n])
		if r <= 0 {
			break
		}
		pos += uint32(dosWrite(arc.tmpfd, chunk[:r]))
	}
	dosSeek(arc.fd, int64(room), 1)
	for {
		n := dosRead(arc.fd, chunk)
		if n <= 0 {
			break
		}
		pos += uint32(dosWrite(arc.tmpfd, chunk[:n]))
	}
	dosWrite(arc.tmpfd, buf)
	for i := 0; i < arc.count; i++ {
		if arc.offtab[i] > 0 && arc.offtab[i] > off {
			arc.offtab[i] -= room
		}
	}
	arc.offtab[blk] = pos
	dosClose(arc.fd)
	dosClose(arc.tmpfd)
	dosUnlink(arcName)
	dosRename(arcTmpName, arcName)
	arc.fd = dosOpen(arcName, O_RDWR)
	arc.tmpfd = dosOpen(arcTmpName, O_WRONLY|O_CREAT|O_TRUNC)
	return true
}

// getArc reads block blk; nil if it is out of range or absent.
func getArc(arc *Arc, blk int) []byte {
	if arc.count < blk || blk >= len(arc.offtab) {
		return nil
	}
	off := arc.offtab[blk]
	if off == 0 {
		return nil
	}
	size := uint32(dosSeek(arc.fd, 0, 2)) - off
	for i := 0; i < arc.count; i++ {
		var d uint32
		if arc.offtab[i] > off {
			d = arc.offtab[i] - off
		}
		if d > 0 && d < size {
			size = d
		}
	}
	dosSeek(arc.fd, int64(off), 0)
	buf := make([]byte, size)
	n := dosRead(arc.fd, buf)
	if n < 0 {
		return nil
	}
	return buf[:n]
}

// countArc is the number of blocks in the archive at name; -1 if it cannot be opened.
func countArc(name string) int {
	b := readFile(name)
	if len(b) < 2 {
		return -1
	}
	return int(le16(b, 0))
}

// checkArc says whether the archive at name holds block blk; -1 if it cannot be read.
func checkArc(name string, blk int) int {
	b := readFile(name)
	if b == nil || len(b) < blk*4+6 {
		return -1
	}
	return b2i(le32(b, blk*4+2) != 0)
}
