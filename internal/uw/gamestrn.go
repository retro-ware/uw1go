package uw

// GAMESTRN.C: the game's strings: DATA\STRINGS.PAK, Huffman-decoded on demand, plus up to
// two blocks of strings made at run time (block 0x7C, a conversation's dynamic strings, and
// 0x7D, the player's name). A string id is block << 9 | index; block 0 means the current
// conversation's or cutscene's block.

type stringBlock struct {
	block   int
	strings [512]*[]byte
	count   int
}

var (
	strBlocks    [2]stringBlock
	stringBlocks int

	stringsPak     []byte
	strNodes       int // StringsPak_NoOfNodes
	strBlockTable  map[int]int
	stringBitsUsed = 8
	stringBits     byte

	// CutsceneOrConversationStringBlock: the block id 0 stands for.
	CutsceneOrConversationStringBlock int
)

// initStrings empties the made blocks and loads the Huffman tree; a failure is fatal.
func initStrings() bool {
	for i := range strBlocks {
		strBlocks[i] = stringBlock{block: -1}
	}
	if e := loadStringsPak(); e != 0 {
		firstPunt(e)
	}
	return true
}

func loadStringsPak() int {
	b := readFile("DATA\\strings.pak")
	if b == nil || len(b) < 2 {
		return ERR_READ | 2
	}
	stringsPak = b
	strNodes = int(le16(b, 0))
	strBlockTable = map[int]int{}
	o := 2 + strNodes*4
	n := int(le16(b, o))
	o += 2
	for i := 0; i < n; i++ {
		strBlockTable[int(int16(le16(b, o)))] = int(le32(b, o+2))
		o += 6
	}
	return 0
}

func freeStrings() {}

// getString returns the string with this id: from a made block, or decoded from the file.
// Strings past a block's end come back empty.
func getString(id int) string {
	block := int(uint16(id)) >> 9
	str := id & 0x1FF
	for i := 0; i < stringBlocks; i++ {
		if strBlocks[i].block == block {
			p := strBlocks[i].strings[str]
			if p == nil {
				return ""
			}
			return cstr(*p)
		}
	}
	if block == 0 {
		return readString(CutsceneOrConversationStringBlock, str)
	}
	return readString(block, str)
}

// makeString adds s (the buffer itself, not a copy) as the next string of a made block
// and returns its id; 0 if both slots hold other blocks.
func makeString(s *[]byte, block int) int {
	found := -1
	for i := 0; i < stringBlocks; i++ {
		if strBlocks[i].block == block {
			found = i
			break
		}
	}
	if found < 0 {
		if stringBlocks >= 2 {
			return 0
		}
		found = stringBlocks
		strBlocks[found] = stringBlock{block: block}
		stringBlocks++
	}
	index := strBlocks[found].count
	strBlocks[found].strings[index&0x1FF] = s
	strBlocks[found].count++
	return index | block<<9
}

// replaceString points an existing made string at new text.
func replaceString(s *[]byte, id int) int {
	block := int(uint16(id)) >> 9
	for i := 0; i < stringBlocks; i++ {
		if strBlocks[i].block == block {
			strBlocks[i].strings[id&0x1FF] = s
			return id
		}
	}
	return 0
}

// clearDynamics empties a made block, keeping its slot.
func clearDynamics(block int) {
	for i := 0; i < stringBlocks; i++ {
		if strBlocks[i].block == block {
			strBlocks[i] = stringBlock{block: block}
			return
		}
	}
}

// getName gives an object's name: a creature's personal name (block 7, whoami + 0x10),
// or its item name shaped by fixNameString. ok is false when the name is missing.
func getName(obj *Object, article, plural bool) (string, bool) {
	if obj.Major() == MAJOR_CREATURE {
		who := obj.Whoami
		if who > 0 && who < 0xF0 {
			name := getString((int(who) + 0x10) | STR_CONV)
			if name == "" {
				return "", false
			}
			return name, true
		}
	}
	name := getString(obj.Item() | STR_OBJNAMES)
	if name == "" {
		return "", false
	}
	return fixNameString(name, article, plural), true
}

// fixNameString shapes an item name of the form "article_singular&plural": the plural
// after the '&' (or an added "s"), or the singular; then the article dropped, or its '_'
// made a space.
func fixNameString(s string, article, plural bool) string {
	amp := indexByte(s, '&')
	if plural {
		if amp >= 0 {
			s = s[amp+1:]
		} else {
			s += "s"
		}
	} else if amp >= 0 {
		s = s[:amp]
	}
	if u := indexByte(s, '_'); u >= 0 {
		if !article {
			s = s[u+1:]
		} else {
			s = s[:u] + " " + s[u+1:]
		}
	}
	return s
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// gameSprint prints game string id (block 1) to the message scroll.
func gameSprint(id int) { scrollPrint(getString(id | STR_GAME)) }

// gameStrings3 prints up to three game strings run together; a negative one is left out.
func gameStrings3(first, second, third int) {
	text := getString(first | STR_GAME)
	if second >= 0 {
		text += getString(second | STR_GAME)
	}
	if third >= 0 {
		text += getString(third | STR_GAME)
	}
	scrollPrint(text)
}

// readString decodes string `str` of block `block`, high bit first, 1 going right, until
// a '|' or 0xFF character or 512 characters.
func readString(block, str int) string {
	off, ok := strBlockTable[block]
	if !ok || off+2 > len(stringsPak) {
		return ""
	}
	b := stringsPak
	count := int(int16(le16(b, off)))
	if count <= str {
		return ""
	}
	rel := int(le16(b, off+2+str*2))
	pos := off + 2 + count*2 + rel
	bitsUsed := 8
	var bits byte
	nextBit := func() bool {
		if bitsUsed == 8 {
			if pos < len(b) {
				bits = b[pos]
			} else {
				bits = 0xFF
			}
			pos++
			bitsUsed = 0
		}
		bit := bits&0x80 != 0
		bits <<= 1
		bitsUsed++
		return bit
	}
	out := make([]byte, 0, 64)
	for len(out) < 512 {
		node := strNodes - 1
		for b[2+node*4+2] != 0xFF {
			if nextBit() {
				node = int(b[2+node*4+3])
			} else {
				node = int(b[2+node*4+2])
			}
		}
		c := b[2+node*4]
		if c == 0xFF || c == '|' {
			break
		}
		out = append(out, c)
	}
	return string(out)
}

// strUpper upper-cases letters (seg039_3452_814).
func strUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 0x20
		}
	}
	return string(b)
}

// strLower lower-cases letters (seg039_3495_85A).
func strLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 0x20
		}
	}
	return string(b)
}

// strAtoi is the conversations' atoi: leading white space skipped, the sign looked for at
// s[0] only.
func strAtoi(s string) int {
	i, n := 0, 0
	for i < len(s) && (s[i] == ' ' || (s[i] >= 9 && s[i] <= 13)) {
		i++
	}
	if len(s) > 0 && s[0] == '-' {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int(s[i]-'0')
		i++
	}
	if len(s) > 0 && s[0] == '-' {
		return -n
	}
	return n
}
