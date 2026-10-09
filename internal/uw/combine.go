package uw

// Port of COMBINE.C: combining objects by DATA\CMB.DAT's rules, and rotworm stew.

// Combination is a CMB.DAT rule: two sources (bit 15: used up) and the output.
type Combination struct{ first, second, output uint16 }

const (
	NUM_COMBINATIONS = 10
	CMB_USED_UP      = 0x8000
)

var ObjectCombinations [NUM_COMBINATIONS]Combination

func initCombinables() {
	var b [NUM_COMBINATIONS * 6]byte
	bltfromdrive("DATA\\cmb.dat", b[:])
	for i := range ObjectCombinations {
		ObjectCombinations[i] = Combination{le16(b[:], i*6), le16(b[:], i*6+2), le16(b[:], i*6+4)}
	}
}

func singleObj(o *Object) bool {
	return !(o.IsQuant() != 0 && o.LinkF() > 1 || o.IsQuant() == 0 && o.LinkF() > 0)
}

// ObjsBeCombinable is the index of the rule that combines a and b, in either order, or -1.
func ObjsBeCombinable(a, b *Object) int {
	if !singleObj(a) || !singleObj(b) {
		return -1
	}
	ia, ib := a.Item(), b.Item()
	for i := range ObjectCombinations {
		e := &ObjectCombinations[i]
		first, second := int(e.first&ID_ITEM), int(e.second&ID_ITEM)
		if (e.first|e.second)&CMB_USED_UP != 0 &&
			(first == ia && second == ib || first == ib && second == ia) {
			return i
		}
	}
	return -1
}

// CombineObjs creates a new static object of rule combo's output item.
func CombineObjs(combo int) *Object {
	return CreateObj(int(ObjectCombinations[combo].output), false)
}

// RemoveAfterCombine is whether rule combo uses obj up.
func RemoveAfterCombine(obj *Object, combo int) bool {
	e := ObjectCombinations[combo].first
	if int(e&ID_ITEM) != obj.Item() {
		e = ObjectCombinations[combo].second
	}
	return e&CMB_USED_UP != 0
}

// makeStew makes rotworm stew in a bowl holding just its three ingredients.
func makeStew() bool {
	recipe := [3]int{ITEM_DEAD_ROTWORM, ITEM_MUSHROOM, ITEM_FLASK_OF_PORT}
	var found [3]int
	var where int
	bowl := FindObj(MAJOR_MISC, 0, ITEM_BOWL&ID_INCLASS, 4, &where)
	if bowl == nil {
		gameSprint(0x96)
		return false
	}
	for obj := ObjPtrTMem(&bowl.OL); obj != nil; obj = ObjPtrTMem(&obj.QN) {
		ok := false
		for i := 0; i < 3; i++ {
			if recipe[i] == obj.Item() {
				found[i]++
				ok = true
			}
		}
		if !ok {
			gameSprint(0x94)
			return false
		}
	}
	for i := 0; i < 3; i++ {
		if found[i] == 0 {
			gameSprint(0x94)
			return false
		}
	}
	if OpenBag != nil && ObjMemTPtr(bowl) == OpenBag.obj.Index() {
		CloseTheBag()
	}
	ObjFreeChain(&bowl.OL)
	bowl.SetItem(ITEM_ROTWORM_STEW)
	DisplayInventory()
	gameSprint(0x95)
	return true
}
