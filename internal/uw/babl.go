package uw

import "strconv"

// BABL.C: the conversation interpreter ("babl"). A conversation is a compiled script in
// DATA\CNV.ARK, one block per conversation slot (cnvID: the NPC's whoami, or 0x100 plus
// the creature class for whoami 0). load_script reads the block, its header and import
// table and the code; babl_run runs it on a small stack machine.
//
// Machine: code is an array of words; mem is nvars words of variables (imports, the
// conversation's private globals) followed by a stack of 0x800 words; sp and bp index the
// stack, pc the code, reg is the result register. Built-ins are called through funcs with
// top, the mem index of the top of the stack, where the argument count is: arg n is the
// address held at mem[top-n].
//
// The script heap of the original (bab_malloc and friends) is not needed: strings are Go
// strings, and bab_free never freed anything anyway.

// bablImport is one entry of the script's import table.
type bablImport struct {
	name              string
	count, index, typ int
	kind              int
}

var (
	bablImports []bablImport
	bablMem     []int16
	bablCode    []int16
	bablFuncs   []any
	bablSP      int
	bablBP      int
	bablPC      int
	bablReg     int16
	bablNvars   int
	stackBase   int
	hdrWord     int
	emptyString int
	currentFunc int
	bablDummy   int16
)

// bmem points at script memory addr; a stray address gets a scratch word instead of a
// crash.
func bmem(addr int) *int16 {
	if addr < 0 || addr >= len(bablMem) {
		bablDummy = 0
		return &bablDummy
	}
	return &bablMem[addr]
}

// stk is the stack slot i.
func stk(i int) *int16 { return bmem(stackBase + i) }

func getmem(addr int) int       { return int(*bmem(addr)) }
func babSetmem(addr, value int) { *bmem(addr) = int16(value) }
func convLocal(addr int) int    { return int(*stk(bablBP + addr)) }
func barg(top, n int) int       { return int(*bmem(top - n)) }  // the address of arg n
func bargv(top, n int) int      { return getmem(barg(top, n)) } // its value
func dynString(s string) int {
	b := []byte(s + "\x00")
	return makeString(&b, STRBLK_DYNAMIC)
}

// initBabl copies DATA\babglobs.dat (each conversation's slot and number of private
// globals) to SAVE0\bglobals.dat with every global zeroed: a new game. 0, or an error.
func initBabl() int {
	src := readFile("DATA\\babglobs.dat")
	if src == nil {
		return ERR_READ | 7
	}
	dest := dosOpen("SAVE0\\bglobals.dat", O_WRONLY|O_CREAT|O_TRUNC)
	if dest < 0 {
		return ERR_WRITE | 1
	}
	defer dosClose(dest)
	for o := 0; o+4 <= len(src); o += 4 {
		size := int(int16(le16(src, o+2)))
		if dosWrite(dest, src[o:o+4]) != 4 {
			return ERR_WRITE | 1
		}
		if size > 0 && dosWrite(dest, make([]byte, size<<1)) != size<<1 {
			return ERR_WRITE | 1
		}
	}
	return 0
}

// babGlobals finds conversation cnvID's record in SAVE0\bglobals.dat and reads (put
// false) or writes its private globals, at most count words.
func babGlobals(memory []int16, count int, put bool) {
	mode := O_RDONLY
	if put {
		mode = O_RDWR
	}
	h := dosOpen("SAVE0\\bglobals.dat", mode)
	if h < 0 {
		return
	}
	defer dosClose(h)
	var hdr [4]byte
	for {
		if dosRead(h, hdr[:]) < 4 {
			return
		}
		block := int(le16(hdr[:], 0))
		size := int(int16(le16(hdr[:], 2)))
		if block > int(cnvID) {
			return
		}
		if block != int(cnvID) {
			dosSeek(h, int64(uint16(size<<1)), 1)
			continue
		}
		if count < size {
			size = count
		}
		buf := make([]byte, size*2)
		if put {
			for i := 0; i < size; i++ {
				buf[i*2], buf[i*2+1] = byte(memory[i]), byte(uint16(memory[i])>>8)
			}
			dosWrite(h, buf)
			return
		}
		n := dosRead(h, buf)
		for i := 0; i < n/2; i++ {
			memory[i] = int16(le16(buf, i*2))
		}
		if n < size*2 {
			return
		}
	}
}

// loadScript loads conversation cnvID from the archive name. 1, or -1 when the block
// cannot be used; an empty block prints 'You get no response.' and also gives 1.
func loadScript(name string) int {
	var arc Arc
	var buf []byte
	if openArc(&arc, name) {
		buf = getArc(&arc, int(cnvID))
		closeArc(&arc)
		if len(buf) == 0 {
			scrollPrint(getString(STR_CONV | 1)) // "You get no response.\n"
			return 1
		}
	} else {
		pfatalCode(ERR_READ | 0xA)
	}
	if !readScriptHeader(buf) {
		return -1
	}
	bablMem = make([]int16, bablNvars+0x800)
	babGlobals(bablMem, bablNvars, false)
	stackBase = bablNvars
	emptyString = dynString("")
	babVarClear()
	babFun("compare", babCompare)
	babFun("random", babRandom)
	babFun("plural", babPlural)
	babFun("contains", babContains)
	babFun("append", babAppend)
	babFun("copy", babCopy)
	babFun("find", babFind)
	babFun("length", babLength)
	babFun("val", babVal)
	return 1
}

// readScriptHeader reads the header (a long nothing uses, the code size in words as a
// long, a word, the string block, nvars, the number of imports), the imports (a
// length-prefixed name, then index, count, kind and type words) and the code.
func readScriptHeader(b []byte) bool {
	p := 0
	w := func() int {
		if p+2 > len(b) {
			p += 2
			return 0
		}
		v := int(int16(le16(b, p)))
		p += 2
		return v
	}
	if len(b) < 16 {
		return false
	}
	p = 4
	codeSize := int(int32(le32(b, p)))
	p += 4
	hdrWord = w()
	CutsceneOrConversationStringBlock = w()
	bablNvars = w()
	nimports := w()
	bablImports = bablImports[:0]
	funcCount := 0
	for i := 0; i < nimports; i++ {
		n := w()
		if n < 0 || p+n > len(b) {
			return false
		}
		name := cstr(b[p : p+n])
		p += n
		imp := bablImport{name: name}
		imp.index = w()
		imp.count = w()
		imp.kind = w()
		imp.typ = w()
		bablImports = append(bablImports, imp)
		if imp.kind == IMPORT_FUNC {
			funcCount++
		}
	}
	bablFuncs = make([]any, funcCount)
	for i := range bablFuncs {
		bablFuncs[i] = unbound
	}
	if codeSize < 0 {
		codeSize = 0
	}
	bablCode = make([]int16, codeSize)
	for i := 0; i < codeSize && p+2 <= len(b); i++ {
		bablCode[i] = int16(le16(b, p))
		p += 2
	}
	return true
}

// unbound is what an import the game never bound calls.
func unbound() int { return 0 }

// babFun binds built-in fn to the import called name; a name the script does not import
// is ignored.
func babFun(name string, fn any) {
	for _, e := range bablImports {
		if e.count == 0 {
			break
		}
		if e.name == name {
			if e.index >= 0 && e.index < len(bablFuncs) {
				bablFuncs[e.index] = fn
			}
			return
		}
	}
}

// babVar copies values into the imported variable called name (at most its count).
func babVar(name string, values ...int) {
	for _, e := range bablImports {
		if e.count == 0 {
			break
		}
		if e.name == name {
			for i := 0; i < len(values) && i < e.count; i++ {
				babSetmem(e.index+i, values[i])
			}
			return
		}
	}
}

// babVarOut reads the imported variable name into *value; untouched when the script does
// not import it.
func babVarOut(name string, value *int16) {
	for _, e := range bablImports {
		if e.count == 0 {
			break
		}
		if e.name == name {
			*value = int16(getmem(e.index))
			return
		}
	}
}

// babVarClear clears every imported variable: 0x126 one word and 0x12B every element to
// 0, 0x128 one word and 0x12A every element to the empty string.
func babVarClear() {
	for _, e := range bablImports {
		if e.count == 0 {
			break
		}
		if e.kind == IMPORT_FUNC {
			continue
		}
		switch e.typ {
		case 0x126:
			babSetmem(e.index, 0)
		case 0x12B:
			for i := 0; i < e.count; i++ {
				babSetmem(e.index+i, 0)
			}
		case 0x128:
			babSetmem(e.index, emptyString)
		case 0x12A:
			for i := 0; i < e.count; i++ {
				babSetmem(e.index+i, emptyString)
			}
		}
	}
}

// The string built-ins.

func babRandom(top int) int { return rand()*bargv(top, 1)/0x8000 + 1 }

// babCompare: 1 when two strings are equal after substitution, ignoring case.
func babCompare(top int) int {
	a := convertString(getString(bargv(top, 1)))
	b := convertString(getString(bargv(top, 2)))
	return b2i(strLower(a) == strLower(b))
}

func babPlural(top int) int {
	if bargv(top, 3) > 1 {
		return bargv(top, 1)
	}
	return bargv(top, 2)
}

// lowerMade lowercases a made string in place, as the original's in-place lowercasing
// does to a dynamic string.
func lowerMade(id int) {
	block := int(uint16(id)) >> 9
	for i := 0; i < stringBlocks; i++ {
		if strBlocks[i].block == block {
			if p := strBlocks[i].strings[id&0x1FF]; p != nil {
				for j, c := range *p {
					if c >= 'A' && c <= 'Z' {
						(*p)[j] = c + 0x20
					}
				}
			}
			return
		}
	}
}

func isPunct(c byte) bool {
	return c > 0x20 && c < 0x7F && !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z')
}
func isSpace(c byte) bool { return c == ' ' || c >= 9 && c <= 13 }

// babContains: does arg1's string hold arg2's as a whole word? The sources are lowercased
// in place, but the substituted copies searched, so case only counts when there was an @.
func babContains(top int) int {
	id1, id2 := bargv(top, 1), bargv(top, 2)
	s1, s2 := getString(id1), getString(id2)
	c1, c2 := convertString(s1), convertString(s2)
	lowerMade(id2)
	lowerMade(id1)
	if indexByte(s1, '@') < 0 {
		c1 = strLower(c1)
	}
	if indexByte(s2, '@') < 0 {
		c2 = strLower(c2)
	}
	if c2 == "" {
		return b2i(true)
	}
	for p := 0; p <= len(c1)-len(c2); {
		i := indexStr(c1[p:], c2)
		if i < 0 {
			break
		}
		p += i
		l := len(c2)
		if p == 0 || isSpace(c1[p-1]) || isPunct(c1[p-1]) {
			if p+l == len(c1) || isSpace(c1[p+l]) || isPunct(c1[p+l]) {
				return 1
			}
		}
		p += l
	}
	return 0
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func babAppend(top int) int {
	return dynString(getString(bargv(top, 2)) + getString(bargv(top, 1)))
}

func babCopy(top int) int { return dynString(getString(bargv(top, 1))) }

// babFind: the 1-based position of arg1 in the arg2 words at arg3's address, or 0.
func babFind(top int) int {
	value, count, start := bargv(top, 1), bargv(top, 2), barg(top, 3)
	for i := 0; i < count; i++ {
		if getmem(start+i) == value {
			return i + 1
		}
	}
	return 0
}

func babLength(top int) int { return len(getString(bargv(top, 1))) }
func babVal(top int) int    { return strAtoi(getString(bargv(top, 1))) }

// convertString replaces each @-variable in a conversation string with its value: @, the
// kind (G global, P parameter, S stack slot, C constant), for all but C the type (I an
// integer, else a string id), the number, and optionally an index variable. "@@" is @.
func convertString(text string) string {
	if indexByte(text, '@') < 0 {
		return text
	}
	out := make([]byte, 0, len(text)*2)
	s := 0
	for s < len(text) {
		if text[s] == '@' {
			s++
			if s < len(text) && text[s] != '@' {
				var value int
				kind, typ := atVar(text, &s)
				_ = kind
				value = typ
				if atLastType != 'I' {
					if str := getString(value); str != "" {
						out = append(out, convertString(str)...)
					}
				} else {
					out = append(out, strconv.Itoa(value)...)
				}
				continue
			}
		}
		if s < len(text) {
			out = append(out, text[s])
			s++
		}
	}
	return string(out)
}

// atLastType is the type letter of the variable atVar last read.
var atLastType byte

// atVar reads one @-variable from text at *s (after the @) and returns its kind and
// value, leaving its type in atLastType. An index part is read by atIndex.
func atVar(text string, s *int) (byte, int) {
	at := func() byte {
		if *s < len(text) {
			c := text[*s]
			*s++
			return c
		}
		return 0
	}
	kind := at()
	typ := byte('I')
	if kind != 'C' {
		typ = at()
	}
	number := strAtoi(text[*s:min(*s+19, len(text))])
	for *s < len(text) && (text[*s] >= '0' && text[*s] <= '9' || text[*s] == '-') {
		*s++
	}
	extra := 0
	if *s < len(text) {
		if c := text[*s]; c == 'G' || c == 'S' || c == 'P' || c == 'C' {
			extra = atIndex(text, s) - 1
		}
	}
	atLastType = typ
	return kind, atValue(kind, number, extra)
}

func atValue(kind byte, number, extra int) int {
	switch kind {
	case 'G':
		return getmem(number + extra)
	case 'P':
		return getmem(convLocal(number) + extra)
	case 'S':
		return convLocal(number + extra)
	}
	return number
}

// atIndex reads the index part of an @-variable, itself a variable (an integer one, else
// 0), and advances *s past it.
func atIndex(text string, s *int) int {
	at := func() byte {
		if *s < len(text) {
			c := text[*s]
			*s++
			return c
		}
		return 0
	}
	kind := at()
	typ := byte('I')
	if kind != 'C' {
		typ = at()
	}
	if typ != 'I' {
		return 0
	}
	parsed := strAtoi(text[*s:min(*s+19, len(text))])
	for *s < len(text) && (text[*s] >= '0' && text[*s] <= '9' || text[*s] == '-') {
		*s++
	}
	extra := 0
	if *s < len(text) {
		if c := text[*s]; c == 'G' || c == 'S' || c == 'P' || c == 'C' {
			extra = atIndex(text, s) - 1
		}
	}
	return atValue(kind, parsed, extra)
}

// code reads code word i (0 past the end, which stops the machine as NOP... not quite:
// a stray pc reads 0x7FFF, an unknown opcode, which ends the script).
func codeAt(i int) int {
	if i < 0 || i >= len(bablCode) {
		return 0x7FFF
	}
	return int(bablCode[i])
}

// babRun runs the loaded script from word 0, which must be OP_START (else -1). It stops at
// EXIT, at a RET with nothing on the stack, or at an unknown opcode, then saves the
// globals and gives 1.
func babRun() int {
	bablSP, bablBP, bablPC = 0, 0, 0
	if codeAt(0) != OP_START {
		return -1
	}
	binop := func(f func(a, b int16) int16) {
		r := f(*stk(bablSP - 1), *stk(bablSP))
		bablSP--
		*stk(bablSP) = r
	}
	cmp := func(c bool) int16 {
		if c {
			return 1
		}
		return 0
	}
	for running := true; running; {
		switch codeAt(bablPC) {
		case OP_NOP, OP_START:
			bablPC++
		case OP_ADD:
			binop(func(a, b int16) int16 { return a + b })
			bablPC++
		case OP_MUL:
			binop(func(a, b int16) int16 { return a * b })
			bablPC++
		case OP_SUB:
			binop(func(a, b int16) int16 { return a - b })
			bablPC++
		case OP_DIV:
			binop(func(a, b int16) int16 {
				if b == 0 {
					return 0x7FFF
				}
				return int16(int(a) / int(b))
			})
			bablPC++
		case OP_MOD:
			binop(func(a, b int16) int16 {
				if b == 0 {
					return 0x7FFF
				}
				return int16(int(a) % int(b))
			})
			bablPC++
		case OP_NEG:
			*stk(bablSP) = -*stk(bablSP)
			bablPC++
		case OP_JMP:
			bablPC = codeAt(bablPC + 1)
		case OP_BRA:
			bablPC = codeAt(bablPC+1) + bablPC + 1
		case OP_OR:
			binop(func(a, b int16) int16 { return cmp(a != 0 || b != 0) })
			bablPC++
		case OP_AND:
			binop(func(a, b int16) int16 { return cmp(a != 0 && b != 0) })
			bablPC++
		case OP_NOT:
			*stk(bablSP) = cmp(*stk(bablSP) == 0)
			bablPC++
		case OP_TSTGT:
			binop(func(a, b int16) int16 { return cmp(a > b) })
			bablPC++
		case OP_TSTGE:
			binop(func(a, b int16) int16 { return cmp(a >= b) })
			bablPC++
		case OP_TSTLT:
			binop(func(a, b int16) int16 { return cmp(a < b) })
			bablPC++
		case OP_TSTLE:
			binop(func(a, b int16) int16 { return cmp(a <= b) })
			bablPC++
		case OP_TSTEQ:
			binop(func(a, b int16) int16 { return cmp(a == b) })
			bablPC++
		case OP_TSTNE:
			binop(func(a, b int16) int16 { return cmp(a != b) })
			bablPC++
		case OP_BEQ:
			v := *stk(bablSP)
			bablSP--
			if v == 0 {
				bablPC = codeAt(bablPC+1) + bablPC + 1
			} else {
				bablPC += 2
			}
		case OP_BNE:
			v := *stk(bablSP)
			bablSP--
			if v != 0 {
				bablPC = codeAt(bablPC+1) + bablPC + 1
			} else {
				bablPC += 2
			}
		case OP_CALL:
			bablSP++
			*stk(bablSP) = int16(bablPC + 2)
			bablPC = codeAt(bablPC + 1)
		case OP_RET:
			if bablSP > 0 {
				bablPC = int(*stk(bablSP))
				bablSP--
			} else {
				running = false
			}
		case OP_POP:
			bablSP--
			bablPC++
		case OP_FETCHM:
			*stk(bablSP) = *bmem(int(*stk(bablSP)))
			bablPC++
		case OP_OFFSET:
			binop(func(a, b int16) int16 { return b + a - 1 })
			bablPC++
		case OP_SWAP:
			a, b := stk(bablSP), stk(bablSP-1)
			*a, *b = *b, *a
			bablPC++
		case OP_PUSHBP:
			bablSP++
			*stk(bablSP) = int16(bablBP)
			bablPC++
		case OP_POPBP:
			bablBP = int(*stk(bablSP))
			bablSP--
			bablPC++
		case OP_SPTOBP:
			bablBP = bablSP
			bablPC++
		case OP_BPTOSP:
			bablSP = bablBP
			bablPC++
		case OP_ADDSP:
			bablSP += int(*stk(bablSP)) - 1
			bablPC++
		case OP_PUSHI:
			bablSP++
			*stk(bablSP) = int16(codeAt(bablPC + 1))
			bablPC += 2
		case OP_STO:
			*bmem(int(*stk(bablSP - 1))) = *stk(bablSP)
			bablSP -= 2
			bablPC++
		case OP_PUSHI_EFF:
			bablSP++
			*stk(bablSP) = int16(stackBase + codeAt(bablPC+1) + bablBP)
			bablPC += 2
		case OP_CALLI:
			bablCalli()
		case OP_SAVE_REG:
			bablReg = *stk(bablSP)
			bablPC++
		case OP_PUSH_REG:
			bablSP++
			*stk(bablSP) = bablReg
			bablPC++
		case OP_STRCMP:
			r1 := convertString(getString(int(*stk(bablSP))))
			r2 := convertString(getString(int(*stk(bablSP - 1))))
			bablSP--
			*stk(bablSP) = cmp(r1 == r2)
			bablPC++
		case OP_EXIT:
			running = false
		case OP_SAY:
			bablSayOp("say")
			bablPC++
		case OP_RESPOND:
			bablSayOp("respond")
			bablPC++
		default:
			running = false
		}
	}
	clearDynamics(STRBLK_DYNAMIC)
	babGlobals(bablMem, bablNvars, true)
	return 1
}

// bablCalli calls built-in n with the top of the stack; the result replaces the argument
// count and goes into reg. A built-in with no result leaves 0 (the original leaves AX).
func bablCalli() {
	n := codeAt(bablPC + 1)
	currentFunc = n
	top := stackBase + bablSP
	result := 0
	if n >= 0 && n < len(bablFuncs) {
		switch fn := bablFuncs[n].(type) {
		case func(int) int:
			result = fn(top)
		case func(int):
			fn(top)
		case func() int:
			result = fn()
		case func():
			fn()
		case func(string):
			fn(convertString(getString(bargv(top, 1))))
		}
	}
	*stk(bablSP) = int16(result)
	bablReg = *stk(bablSP)
	bablPC += 2
}

// bablSayOp pops a string id, substitutes it and passes it to the built-in imported as
// name ("say" or "respond").
func bablSayOp(name string) {
	text := convertString(getString(int(*stk(bablSP))))
	bablSP--
	for _, e := range bablImports {
		if e.count == 0 {
			break
		}
		if e.name == name {
			if e.index >= 0 && e.index < len(bablFuncs) {
				if fn, ok := bablFuncs[e.index].(func(string)); ok {
					fn(text)
				}
			}
			return
		}
	}
}
