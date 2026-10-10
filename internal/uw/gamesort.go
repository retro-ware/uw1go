package uw

// Port of GAMESORT.C: sorting the objects of each tile of the 3D view for drawing. do_objsort
// orders a tile's objects farthest first and hands each to DRAWOBJ.C's doObj with its view
// position (objxloc, objyloc, objzloc) and distance shade (locsqmod) set. Animated objects
// reaching over their tile's edge are held back (refugees, holdmid, holdtmp) and drawn
// with the neighbouring tile that is drawn later.

var (
	holdmid, holdtmp [9]uint16
	locsqmod         byte
	sortlist         [61]int8
	sortdata         [61][4]int8
	mptrmod          int
	sdXmod, sdYmod   int
	dirval           int8
	// a row of 9 per column, from column -1 (the original reaches refugees[-1] and [33]),
	// flat, so that a full row runs on into the next as in the original
	refugees                  [36 * 9]uint16
	objxloc, objyloc, objzloc int
	objptrs                   [61]uint16
)

func refugeesAt(col int) []uint16 { return refugees[(col+1)*9:] }

var trans_pos_x = [64]int8{
	0, 0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 0,
	0, 0, 0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7,
	7, 0, 6, 0, 5, 0, 4, 0, 3, 0, 2, 0, 1, 0, 0, 0,
	0, 7, 0, 6, 0, 5, 0, 4, 0, 3, 0, 2, 0, 1, 0, 0,
}

// sortSetup is told by subprocess which part of the row comes next.
func sortSetup(mode int) {
	switch mode {
	case -10:
		refugees = [36 * 9]uint16{}
		holdmid = [9]uint16{}
		holdtmp = [9]uint16{}
	case 2:
		holdmid = [9]uint16{}
		holdtmp = [9]uint16{}
	case 1:
		holdmid = holdtmp
		holdtmp[0] = 0
	case 0:
		if holdmid[0] != 0 {
			if holdmid[0]+holdtmp[0] >= 9 {
				holdmid[0] = 9 - holdtmp[0] - 1
			}
			copy(holdtmp[holdtmp[0]+1:], holdmid[1:1+holdmid[0]])
			holdtmp[0] += holdmid[0]
		}
	}
	dirval = int8(mode)
}

func sortObj(first, count int) {
	for last := count - 1; last >= first; last-- {
		for i := first; i <= last; i++ {
			if sortdata[sortlist[i]][0] < sortdata[sortlist[i+1]][0] {
				sortlist[i], sortlist[i+1] = sortlist[i+1], sortlist[i]
			}
		}
	}
}

func buildSort(count int) {
	for i := 0; i < count; i++ {
		sortlist[i] = int8(i)
	}
}

func doPartition(reverse bool, point *int, skip, count, height, fromData int) {
	var ends, steps [2]int
	if reverse {
		ends[0], steps[0] = count-1, -1
		ends[1], steps[1] = 0, 1
	} else {
		ends[0], steps[0] = 0, 1
		ends[1], steps[1] = count-1, -1
	}
	for i := 0; i < count; i++ {
		if i == skip {
			continue
		}
		var value int
		if fromData == 0 {
			value = ObjIntTMem(int(objptrs[i])).Z()
		} else {
			value = int(sortdata[i][fromData])
		}
		side := b2int(value > height)
		if ends[side] >= 0 && ends[side] < len(sortlist) {
			sortlist[ends[side]] = int8(i)
		}
		ends[side] += steps[side]
	}
	if ends[0] != ends[1] {
		return
	}
	*point = ends[0]
	sortlist[*point] = int8(skip)
}

func zPart(index int, point *int, count int) {
	z := ObjIntTMem(int(objptrs[index])).Z()
	doPartition(ThePlayer.Z() < z, point, index, count, z, 0)
}

func doorPart(index int, point *int, count int) {
	var reverse bool
	var pos, axis int
	if (ObjIntTMem(int(objptrs[index])).HeadingBits()+quad*2)&3 == 0 {
		axis = 2
		pos = int(sortdata[index][2])
		reverse = true
	} else {
		axis = 1
		pos = int(sortdata[index][1])
		switch dirval {
		case 2:
			reverse = false
		case 1:
			reverse = true
		default:
			reverse = int(cPlayer.x>>5) < pos
		}
	}
	doPartition(reverse, point, index, count, pos, axis)
}

func setSds(data *[4]int8, o *Object) {
	data[1] = trans_pos_x[quad*16+o.FineX()*2] + trans_pos_x[((quad+1)&3)*16+o.FineY()*2]
	data[2] = trans_pos_x[quad*16+o.FineX()*2+1] + trans_pos_x[((quad+1)&3)*16+o.FineY()*2+1]
	data[3] = int8(o.Z())
}

func setOsum(data *[4]int8) {
	switch dirval {
	case 0:
		data[0] = data[2] << 1
	case 1:
		data[0] = data[1] + data[2] + 1
	case 2:
		data[0] = 8 - data[1] + data[2]
	}
}

// clearObjsort draws the objects held for this column of a tile not drawn.
func clearObjsort() {
	if refugeesAt(loopx)[0] > 0 {
		doObjsort(nil)
	}
	if holdtmp[0] > 0 {
		doObjsort(nil)
	}
}

// doObjsort sorts and draws one tile's objects (link nil for only the held ones).
func doObjsort(link *Link) {
	visited, partition, n, pivot := 0, 0, 0, -1
	ref := refugeesAt(loopx)
	for i := 1; i <= int(ref[0]) && n < 9; i++ {
		word := ref[i]
		objptrs[n] = word & 0x3FF
		next := ObjIntTMem(int(word & 0x3FF))
		data := &sortdata[n]
		setSds(data, next)
		if word&0x1000 != 0 {
			if word&0x2000 != 0 {
				data[1] += -8
			} else {
				data[1] += 8
			}
		}
		if word&0x4000 != 0 {
			data[2] += 8
		}
		setOsum(data)
		if word&0x8000 != 0 {
			data[0]--
		}
		n++
	}
	if holdtmp[0] != 0 {
		copy(ref, holdtmp[:])
	} else {
		ref[0] = 0
	}
	holdtmp[0] = 0

	var data *[4]int8
	next := ObjPtrTMem(link)
	for next != nil && visited < 60 {
		candidate, held := false, false
		item := next.Item()
		com := &ComObjData[item]
		if item == ITEM_BRIDGE || item>>4 == CLASS_DOOR || item == ITEM_MOVING_DOOR {
			partition = n + item<<6
		} else if com.Animated() != 0 {
			word := uint16(0)
			held = true
			data = &sortdata[n]
			setSds(data, next)
			radius := com.Radius()
			if int(data[2])-radius < 0 {
				word |= 0x4000
			}
			if dirval == 1 {
				radius = -radius
			}
			if dirval != 0 && (int(data[1])+radius)&^7 != 0 {
				word |= 0x1000
				if dirval == 2 {
					word |= 0x2000
				}
			}
			if word != 0 {
				candidate = true
				word |= uint16(link.Index())
				if item&ID_MAJOR == FIRST_ANIMOBJ {
					word |= 0x8000
				}
				var dest []uint16
				switch {
				case word&0x5000 == 0x5000:
					dest = holdtmp[:]
				case word&0x4000 != 0:
					dest = refugeesAt(loopx)[:]
				case word&0x2000 != 0:
					dest = refugeesAt(loopx + 1)[:]
				default:
					dest = refugeesAt(loopx - 1)[:]
				}
				if dest[0] >= 9 {
					candidate = false
				} else {
					dest[0]++
					if int(dest[0]) < len(dest) {
						dest[dest[0]] = word
					}
				}
			}
		}
		if !candidate {
			if !held {
				data = &sortdata[n]
				setSds(data, next)
			}
			setOsum(data)
			if item&ID_MAJOR == FIRST_ANIMOBJ {
				data[0]--
			} else if item&0x1FE == ITEM_TMAP_C {
				data[0] += 0x20
			}
			objptrs[n] = uint16(link.Index())
			if n < 60 {
				n++
			}
		}
		link = &next.QN
		next = ObjPtrTMem(link)
		visited++
	}
	if partition != 0 && n > 1 {
		if partition>>6 == ITEM_BRIDGE {
			zPart(partition&0x3F, &pivot, n)
		} else {
			doorPart(partition&0x3F, &pivot, n)
		}
		if pivot > 1 {
			sortObj(0, pivot-1)
		}
		if n-2 > pivot {
			sortObj(pivot+1, n-1)
		}
	} else {
		buildSort(n)
		if n > 1 {
			sortObj(0, n-1)
		}
	}
	for i := 0; i < n; i++ {
		w := int(sortlist[i])
		next := ObjIntTMem(int(objptrs[w]))
		if next == nil {
			continue
		}
		objxloc = int(int16((loopx-16)<<8 + int(sortdata[w][1])<<5 + 16))
		objzloc = int(int16(loopy<<8 + int(sortdata[w][2])<<5 + 16))
		if next.Major() == MAJOR_CREATURE || !IsMobElem(next) {
			objyloc = next.Z() << 3
		} else {
			objyloc = int(int16(next.B0F))
		}
		if PickUp != 0 {
			mptrmod = int(sortdata[w][2]) / 8 * sdYmod
			mptrmod += ((int(sortdata[w][1])+64)/8 - 8) * sdXmod
		} else {
			temp := (objxloc - int(cPlayer.x&0xFF)) >> 5
			distance := temp * temp
			temp = (objzloc - int(cPlayer.y&0xFF)) >> 5
			distance += temp * temp
			temp = (objyloc - int(cPlayer.z)) >> 5
			distance += temp * temp
			distance = int(int16(distance))
			if distance > 0 {
				temp = cSqRt(uint32(distance))
			} else {
				temp = 0
			}
			distance = int(int16(temp*int(smoothDiv))) >> 6
			distance += int(smoothLowpass)
			if distance < 0 {
				distance = 0
			}
			locsqmod = byte(distance + int(uint8(smoothBase)))
			if locsqmod > 14 {
				locsqmod = 14
			}
		}
		doObj(next)
	}
}
