package uw

// Port of DRAWOBJ.C: drawing one object into the 3D view's render database: a sprite
// (do_uwobj), a critter frame (do_uwcrit), a 3D model (doRect) or a door (doDoor), as the
// object type's render field in COMOBJ.DAT says. In a pick frame each object first gets its
// pick colour, and colorToObj and colorToMap record it for pick_3d.

var rect_cols = [30][4]byte{
	{0x01, 0xEC, 0x00, 0x00},
	{0x21, 0xEB, 0x00, 0x00},
	{0x11, 0xEC, 0x00, 0x3E},
	{0x01, 0xE4, 0x00, 0x00},
	{0x02, 0xB6, 0xB0, 0x00},
	{0x02, 0x64, 0x6C, 0x00},
	{0x02, 0x64, 0x6C, 0x00},
	{0x02, 0x64, 0x6C, 0x00},
	{0x42, 0xE8, 0xB8, 0x00},
	{0x01, 0xE4, 0x00, 0x00},
	{0x19, 0xE4, 0x00, 0x60},
	{0x03, 0xA3, 0xA4, 0xA6},
	{0x01, 0x68, 0x00, 0x00},
	{0x01, 0x68, 0x00, 0x00},
	{0x11, 0xEC, 0x00, 0x00},
	{0x21, 0xEC, 0x00, 0x00},
	{0x51, 0xB0, 0x00, 0xE4},
	{0x51, 0xB0, 0x00, 0xEC},
	{0x11, 0xB0, 0x00, 0xF4},
	{0x11, 0x6A, 0x00, 0x3C},
	{0x51, 0xB0, 0x00, 0x00},
	{0x11, 0xB0, 0x00, 0x00},
	{0x21, 0xB0, 0x00, 0x00},
	{0x83, 0x00, 0x02, 0x04},
	{0x02, 0xE4, 0x68, 0x00},
	{0x02, 0xE6, 0x68, 0x00},
	{0x01, 0xE4, 0x00, 0x00},
	{0x02, 0xE4, 0x6A, 0x00},
	{0x03, 0xE6, 0x6A, 0x71},
	{0x03, 0xE2, 0x62, 0xC4},
}

var rect_sub = [32]int{
	0x03, 0x08, 0x08, 0x07, 0x07, 0x06, 0x05, 0x0B,
	0x18, 0x09, 0x17, 0x1B, 0x1C, 0x19, 0x1A, 0x04,
	0x0A, 0x10, 0x11, -1, 0x02, 0x13, 0x12, -1,
	-1, -1, -1, -1, -1, -1, -1, -1,
}

var dirtab = [32]byte{
	0, 0, 0, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3, 4, 4,
	4, 4, 4, 5, 5, 5, 6, 6, 6, 6, 6, 7, 7, 7, 0, 0,
}

var rect_flagcol = [2]int{0xE4, 0x66}

// defres writes the object's point.
func defres() {
	dbw(OP_DEFRES)
	dbw(objxloc)
	dbw(objzloc)
	dbw(objyloc)
	dbw(0x7F8)
}

func movec(v, val int) {
	dbw(OP_MOVEC)
	dbw(v)
	dbw(val)
}

// doObj draws one object.
func doObj(o *Object) {
	if o == ThePlayer || o.Invis() == 1 {
		return
	}
	if PickUp != 0 {
		colorToMap[PickUp-1] = uint16(tmptr + mptrmod)
		colorToObj[PickUp-1] = uint16(ObjMemTPtr(o))
		dbw(OP_SETBMCOL)
		dbw(int(PickUp))
		if PickUp++; PickUp >= PICK_WALL {
			PickUp = 1
		}
	}
	if IsMobElem(o) && o.Major() != MAJOR_CREATURE {
		fx := int(o.GoalWord & 0xFF)
		fy := int(o.AttitudeWord & 0xFF)
		var x, y int
		switch quad {
		case 0:
			y, x = fy, fx
		case 1:
			y, x = fx, 0xFF-fy
		case 2:
			x, y = 0xFF-fx, 0xFF-fy
		case 3:
			y, x = 0xFF-fx, fy
		}
		objxloc = int(int16(objxloc&0xFF00 + x))
		objzloc = int(int16(objzloc&0xFF00 + y))
	}
	typ := ComObjData[o.Item()].Render()
	var item int
	if o.Major() == MAJOR_ANIMOBJ {
		AnimObjInPipe = 1
		item = o.Owner()
		if typ == 0 {
			if item > 0 {
				item += FIRST_ANIMOBJ
			} else {
				item = o.Item()
			}
		}
	} else {
		item = o.Item()
	}
	switch typ {
	case 2:
		item &= 0x3F
		if item>>4 != 0 {
			item -= 0x10
			model := rect_sub[item]
			if model < 0 || item >= 0x20 {
				return
			}
			doRect(model, o, -1, -1)
			return
		}
		doDoor(item, o)
	case 1:
		defres()
		frame := int(o.B15 & 0x3F)
		dir := int(dirtab[int(uint16(o.HeadingBits()<<2+0x20-int((uint16(cPlayer.heading)+headmod[quad])>>11)))%0x20])
		if frame >= 0x20 {
			frame = (frame-0x20)<<3 + dir + 0x20
		} else if frame != 0xC && (dir+5)&7 >= 3 {
			frame = dir + 0x20
		}
		dbw(OP_UWCRIT)
		dbw(item & 0x3F)
		dbw(int(locsqmod) * lighton)
		dbw(frame)
		dbw(o.Frame())
		dbw(0x7F8)
	case 0:
		if item&0x1E0 == 0xE0 && item&0x18 != 0 {
			item = ITEM_RUNESTONE
		}
		defres()
		dbw(OP_UWOBJ)
		dbw(item)
		dbw(int(locsqmod) * lighton)
		dbw(0x7F8)
	case 3:
		if o.Minor() == 3 {
			plain := tmapson|int(PickUp) == 0
			if plain {
				movec(Clk(8), 0)
			}
			doRect(0x14, o, -1, firstTmobj+item&0xF)
			if !plain {
				return
			}
			movec(Clk(8), 1)
		} else {
			tCacheOK = 0xE0
			txtwal(nil, sqmod, 4, byte(o.Owner()))
			dbw(OP_SET_TMCTXT)
			dbw(cTmBm)
			tm := byte(w64Types[o.Owner()] & 0xFF)
			var flat bool
			if tm == 3 || tm == 4 {
				curautocode = 3
				flat = true
			} else {
				flat = tm == 8 || tm == 0xB
			}
			flat = flat && tmapson|int(PickUp) == 0
			if flat {
				movec(Clk(8), 0)
			}
			doRect(0x16, o, -1, o.Owner())
			if flat {
				movec(Clk(8), 1)
			}
		}
	}
}

// doRect writes a call of 3D model `model` for object o. heading -1 turns it by the
// object's own heading; tex -1 takes the texture from rect_cols and the object's flags.
func doRect(model int, o *Object, heading int, tex int) {
	hilite := -1
	flags := int(rect_cols[model][0])
	movec(Clk(0xA), int(locsqmod)*lighton)
	switch {
	case flags&0x20 != 0:
		movec(Clk(0), int(texFirstPixel(tex+0x3A)))
		movec(Clk(0xA), int(sqmod)*lighton)
	case flags&0x80 != 0:
		AnimObjInPipe = 1
		val := int(GameTime()&0x1FF) >> 6
		if val&4 != 0 {
			val = 3 - val&3
		}
		for i := 0; i < flags&7; i++ {
			movec(Clk(i), o.LinkF()&0x1FF+val+int(rect_cols[model][i+1]))
		}
	default:
		for i := 0; i < flags&7; i++ {
			movec(Clk(i), int(rect_cols[model][i+1]))
		}
	}
	if flags&0x10 != 0 {
		first := int(rect_cols[model][3])
		n := first>>5 + 1
		if tex < 0 {
			if model == 2 {
				if o.Flags() >= n {
					tex = o.Flags() - n
					txtflr(nil, sqmod, byte(tex))
					dbw(OP_SET_TMCTXT)
					dbw(cTmBm)
					tex = -1
				} else {
					curautocode = 2
					tex = first&0x1F + firstTmobj + 0x10 + o.Flags()%n
					movec(Clk(0xB), rect_flagcol[o.Flags()&1])
					dbw(OP_SET_TMCTXT)
					dbw(6)
				}
			} else {
				tex = first&0x1F + firstTmobj + 0x10 + o.Flags()%n
			}
		}
		if tex >= 0 {
			dbw(OP_SETTMOBJ)
			dbw(tex)
			dbw(int(locsqmod) * lighton)
		}
		tCacheOK = 0xE0
	}
	if flags&8 != 0 {
		i := 0x400 - objyloc
		dbw(OP_DEFDELTA)
		dbw(0)
		dbw(0)
		dbw(i)
		dbw(0x800)
		movec(bmhgtoff()+0x30, i*2-1)
	}
	org := func(x, y, z int) {
		dbw(OP_ORG)
		for _, v := range [3]int{x, y, z} {
			dbw(v & 0xFFFF)
			dbw(v >> 16)
		}
	}
	org(objxloc, objyloc, objzloc)
	var head int
	if heading < 0 {
		head = (o.HeadingBits() + 8 - quad*2) % 8 << 13
	} else {
		head = (heading + 0x10 - quad*4) & 0xF << 12
	}
	head = int(int16(head))
	if flags&0x40 != 0 && flags&0x10 == 0 {
		j := 0
		if objIndexOf(o) < NUM_MOBILE {
			j = (o.Pitch() - 0x10) * 0x266
			head = int(int16((o.HeadingBits()<<5 + o.FineHead() + 0x100 - quad*64) % 0x100 << 8))
		}
		movec(Clk(5), j)
	}
	if (model == 0x10 || model == 0x11) && tmapson|int(PickUp) == 0 {
		hilite = 0
	}
	if hilite != -1 {
		movec(Clk(8), hilite)
	}
	if head != 0 {
		dbw(OP_IHCALL)
		dbw(head)
	} else {
		dbw(OP_SFCAL)
	}
	Ref(model+0x60, 1)
	if hilite != -1 {
		movec(Clk(8), (hilite+1)&1)
	}
	org(0, 0, 0)
}

// doDoor draws a door: its frame (model 1, in the tile's wall texture) and its leaf.
func doDoor(item int, o *Object) {
	z := -1
	first, step := 0, 1
	vis := 0
	var h, dir, oldz int
	curautocode = 1
	if item&7 == 6 {
		dir = 0
		z = objyloc
		oldz = objyloc - (o.Flags()&7)*0x30
		vis = 1
		h = 0x400 - oldz - 0xD0
		first, step = 1, -1
		movec(Clk(5), o.Flags()&7)
		dbw(OP_DEFDELTA)
		dbw(0)
		dbw(0)
		dbw(0xD0 - (o.Flags()&7)*0x30)
		dbw(0x400)
	} else {
		if o.Flags()&7 != 0 || o.Major() == MAJOR_ANIMOBJ {
			objyloc -= 0xC0
		}
		dir = (o.Flags() & 7) * (o.DoorDir()*2 - 1)
		h = 0x400 - objyloc - 0xD0
		movec(Clk(5), dir<<12)
	}
	_ = dir
	dbw(OP_DEFDELTA)
	dbw(0)
	dbw(0)
	dbw(h)
	dbw(0x800)
	if vis == 0 {
		v := int8((o.HeadingBits() - ThePlayer.HeadingBits() + 8) & 7)
		if v > 4 {
			v -= 8
		}
		vis = b2int(v >= -1 && v <= 2)
	}
	if o.Flags()&7 != 0 && z == -1 {
		q := (o.HeadingBits() - quad*2) & 7
		ddx := objxloc - int(cPlayer.x)
		dy := objzloc - int(cPlayer.y)
		var k int
		switch q & 3 {
		case 0:
			k = b2int(dy < 0)
		case 1:
			k = b2int(-dy > ddx)
		case 2:
			k = b2int(ddx < 0)
		case 3:
			k = b2int(ddx < dy)
		}
		k = q/4 + o.DoorDir() + k
		if k&1 == 0 {
			first, step = 1, -1
		}
	}
	movec(Clk(3), o.Flags()&7+b2int(z >= 0))
	wall := tileAt(tmptr).Wall()
	for pass := first; pass <= 1 && pass >= 0; pass += step {
		if pass == 0 {
			if int8(tCacheOK) < 1 {
				txtwal(nil, locsqmod, byte(objyloc>>6), byte(wall))
			}
			movec(bmhgtoff()+cTmBm<<3, (cTmDm*cTmDm>>8)*h-1)
			movec(Clk(7), cTmDm*cTmDm-1)
			movec(Clk(6), bmhgtoff()+cTmBm<<3)
			if PickUp != 0 {
				dbw(OP_SETBMCOL)
				dbw(wall + PICK_WALL)
			}
			dbw(OP_SET_TMCTXT)
			dbw(cTmBm)
			doRect(1, o, o.HeadingBits()<<1, wall)
		} else {
			if PickUp != 0 {
				dbw(OP_SETBMCOL)
				dbw(int(PickUp) - 1)
			}
			if z >= 0 {
				objyloc = z
				doRect(0xC, o, o.HeadingBits()<<1, 0)
				objyloc = oldz
			} else if k := item & 7; k == 7 {
				if int8(tCacheOK) < 1 {
					txtwal(nil, locsqmod, byte(objyloc>>6), byte(wall))
				}
				dbw(OP_SET_TMCTXT)
				dbw(cTmBm)
				movec(bmhgtoff()+cTmBm<<3, cTmDm*cTmDm-1)
				doRect(0xF, o, o.HeadingBits()<<1, wall)
			} else {
				doRect(0xE, o, o.HeadingBits()<<1, firstTmobj+k+0x30)
			}
		}
	}
}
