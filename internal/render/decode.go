package render

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Decoding the models from UW.EXE. The EXE keeps them in the renderer's data segment as
// programs for the original's model interpreter (INTERP.ASM): a stream of 16-bit words,
// an opcode and then its operands. This file is the one place that knows that format;
// it turns each program into a Model once, at load.
//
// The format's conventions: point operands are point numbers times 8 (byte offsets into
// the original's point table), except in the compact opcodes, which give point numbers
// as bytes; jump and skip operands are byte offsets from the end of their operand or
// instruction; variable operands are data segment addresses of model variables.

// Where things are in UW.EXE: its load image starts after the MZ header; the renderer's
// data segment (seg051) is this many 16-byte paragraphs into it in the release the
// listing was made from. Other releases have it further on (see Load's shift).
const (
	dataSegmentParagraph = 18211
	dataSegmentSize      = 65536
	modelTableOffset     = 10400 // the models' entry points, a word each
	modelProgramsOffset  = 10552 // what the entry points are relative to
	variablesOffset      = 10528 // the model variables, a word each
	modelCount           = 64
)

// The interpreter's opcodes (each the offset of its handler in the original's table).
const (
	opEnd             = 0
	opSortPlane       = 6
	opSortPlaneYZ     = 12
	opSortPlaneXZ     = 14
	opSortPlaneXY     = 16
	opCall            = 18
	opUnlessAtMost    = 20
	opUnlessAtLeast   = 22
	opYOffsetB        = 28
	opPolygonB        = 34
	opXOffsetB        = 42
	opZOffsetB        = 44
	opSetColour       = 46
	opTexturedB       = 52
	opTextured        = 54
	opCaveFill        = 64
	opFlatFill        = 68
	opJump            = 72
	opMoveOrigin      = 74
	opDelta           = 76
	opTurned          = 80
	opFacing          = 88
	opSetColourVar    = 92
	opFacingYZ        = 94
	opFacingXZ        = 96
	opFacingXY        = 98
	opFacingX         = 100
	opFacingY         = 102
	opFacingZ         = 104
	opSortVar         = 108
	opBox             = 120
	opPoint           = 122
	opPolygon         = 126
	opPoints          = 130
	opHull            = 132
	opXOffset         = 134
	opYOffset         = 136
	opZOffset         = 138
	opSum             = 140
	opXYOffset        = 144
	opXZOffset        = 146
	opYZOffset        = 148
	opTexturedTri     = 158
	opCompact         = 160
	opCompactB        = 162
	opCompactMirror   = 164
	opCompactMirrorB  = 166
	opUVTextured      = 168
	opUVTexturedB     = 170
	opNothing         = 172
	opSetPickColour   = 174
	opSetContext      = 178
	opContextUV       = 180
	opSmallPoint      = 182
	opSmallPoints     = 184
	opTurnedVar       = 186
	opShadedColour    = 188
	opMoveIndirect    = 190
	opShadedFill      = 200
	opShadedFillV     = 202
	opSetShades       = 204
	opContextUVB      = 206
	opSubtract        = 198
	opSetShadesVar    = 212
	opShadedOrPick    = 214
	opTranslucentFill = 216
)

// decoder reads one data segment's programs.
type decoder struct {
	seg    []byte
	blocks map[int]Block // the parts decoded so far, by address
	busy   map[int]bool  // parts being decoded (a jump back to one would loop)
}

func (d *decoder) word(at int) int {
	return int(binary.LittleEndian.Uint16(d.seg[at%dataSegmentSize:]))
}
func (d *decoder) sword(at int) int { return int(int16(d.word(at))) }
func (d *decoder) byte_(at int) int { return int(d.seg[at%dataSegmentSize]) }
func (d *decoder) sbyte(at int) int { return int(int8(d.seg[at%dataSegmentSize])) }

// variable is the model variable at a data segment address.
func (d *decoder) variable(addr int) (int, error) {
	n := (addr - variablesOffset) / 2
	if addr%2 != 0 || n < 0 || n >= numVars {
		return 0, fmt.Errorf("address %d is not a model variable", addr)
	}
	return n, nil
}

// decodeModels reads the models from the data segment.
func decodeModels(seg []byte) ([]*Model, error) {
	if len(seg) < dataSegmentSize {
		return nil, errors.New("render: UW.EXE is too short")
	}
	d := &decoder{seg: seg, blocks: map[int]Block{}, busy: map[int]bool{}}
	models := make([]*Model, modelCount)
	for n := range models {
		w := d.sword(modelTableOffset + 2*n)
		if w == -1 {
			continue
		}
		body, err := d.part((modelProgramsOffset + 2*int(uint16(w)>>1)) % dataSegmentSize)
		if err != nil {
			return nil, fmt.Errorf("render: model %d: %w", n, err)
		}
		models[n] = &Model{Body: body}
	}
	return models, nil
}

// part decodes the program at pc to its end, once.
func (d *decoder) part(pc int) (Block, error) {
	if b, ok := d.blocks[pc]; ok {
		return b, nil
	}
	if d.busy[pc] {
		return nil, fmt.Errorf("the program at %d jumps back into itself", pc)
	}
	d.busy[pc] = true
	defer delete(d.busy, pc)
	b, _, err := d.steps(pc, -1)
	if err != nil {
		return nil, err
	}
	d.blocks[pc] = b
	return b, nil
}

// steps decodes from pc up to stop (or, when stop is -1, to the program's end) and is
// where it stopped.
func (d *decoder) steps(pc, stop int) (Block, int, error) {
	var b Block
	for stop < 0 || pc < stop {
		s, next, done, err := d.step(pc)
		if err != nil {
			return nil, 0, fmt.Errorf("at %d: %w", pc, err)
		}
		if s != nil {
			b = append(b, s)
		}
		if done {
			if stop >= 0 {
				next = stop // the rest of the range is not reached this way
			}
			return b, next, nil
		}
		pc = next
	}
	if pc != stop {
		return nil, 0, fmt.Errorf("an instruction runs past the end of its block at %d", stop)
	}
	return b, pc, nil
}

// body decodes the skipped range of a conditional step: from pc to pc+skip.
func (d *decoder) body(pc, skip int) (Block, error) {
	if skip < 0 {
		return nil, fmt.Errorf("a backward skip")
	}
	b, _, err := d.steps(pc, pc+skip)
	return b, err
}

// target is a jump operand at pc's destination.
func (d *decoder) target(pc int) int { return (pc + 2 + d.sword(pc)) % dataSegmentSize }

// pointAt is the point operand at pc.
func (d *decoder) pointAt(pc int) int { return d.word(pc) / 8 }

// pos reads three coordinates stored x, z (ahead), y (up) and gives them x, up, ahead.
func pos(x, z, y int) [3]int { return [3]int{x, y, z} }

// plane reads a plane's terms (normal, offset pairs) for the axes listed.
func (d *decoder) plane(pc int, axes ...int) (Plane, int) {
	p := make(Plane, len(axes))
	for i, a := range axes {
		p[i] = PlaneTerm{Axis: a, Normal: d.sword(pc), Offset: d.sword(pc + 2)}
		pc += 4
	}
	return p, pc
}

var planeAxes = map[int][]int{
	opSortPlane: {0, 1, 2}, opSortPlaneYZ: {1, 2}, opSortPlaneXZ: {0, 2}, opSortPlaneXY: {0, 1},
	opFacing: {0, 1, 2}, opFacingYZ: {1, 2}, opFacingXZ: {0, 2}, opFacingXY: {0, 1},
}

// step decodes the instruction at pc: its step (nil for none), where the next one is,
// and whether the program ends with it.
func (d *decoder) step(pc int) (s Step, next int, done bool, err error) {
	op := d.word(pc)
	pc += 2
	switch op {
	case opEnd:
		return nil, pc, true, nil

	case opCall:
		b, err := d.part(d.target(pc))
		return Call{b}, pc + 2, false, err
	case opJump:
		b, err := d.part(d.target(pc))
		return Jump{b}, pc + 2, true, err

	case opUnlessAtMost, opUnlessAtLeast:
		skip := d.sword(pc)
		v, err := d.variable(d.word(pc + 2))
		if err != nil {
			return nil, 0, false, err
		}
		value := d.sword(pc + 4)
		pc += 6
		b, err := d.body(pc, skip)
		return Unless{Var: v, AtMost: op == opUnlessAtMost, Value: value, Body: b}, pc + skip, false, err

	case opSortVar:
		v, err := d.variable(d.word(pc))
		if err != nil {
			return nil, 0, false, err
		}
		value := d.sword(pc + 2)
		first, err := d.part(d.target(pc + 4))
		if err != nil {
			return nil, 0, false, err
		}
		second, err := d.part(d.target(pc + 6))
		return InOrderByVar{Var: v, Value: value, First: first, Second: second}, pc + 8, false, err

	case opSortPlane, opSortPlaneYZ, opSortPlaneXZ, opSortPlaneXY:
		p, at := d.plane(pc, planeAxes[op]...)
		first, err := d.part(d.target(at))
		if err != nil {
			return nil, 0, false, err
		}
		second, err := d.part(d.target(at + 2))
		return InOrder{Plane: p, First: first, Second: second}, at + 4, false, err

	case opFacing, opFacingYZ, opFacingXZ, opFacingXY:
		skip := d.sword(pc)
		p, at := d.plane(pc+2, planeAxes[op]...)
		b, err := d.body(at, skip)
		return IfFacing{Plane: p, Body: b}, at + skip, false, err

	case opFacingX, opFacingY, opFacingZ:
		skip := d.sword(pc)
		f := IfFacingAxis{Axis: (op - opFacingX) / 2, Normal: d.sword(pc + 2), Offset: d.sword(pc + 4)}
		pc += 6
		f.Body, err = d.body(pc, skip)
		return f, pc + skip, false, err

	case opMoveOrigin:
		return MoveOrigin{[3]int{d.sword(pc), d.sword(pc + 2), d.sword(pc + 4)}}, pc + 6, false, nil

	case opTurned, opTurnedVar:
		t := Turned{Heading: int16(d.word(pc))}
		if op == opTurnedVar {
			t.ByVar = true
			if t.Var, err = d.variable(d.word(pc)); err != nil {
				return nil, 0, false, err
			}
			t.Heading = 0
		}
		t.Body, err = d.part(d.target(pc + 2))
		return t, pc + 4, false, err

	case opPoint:
		p := pos(d.sword(pc), d.sword(pc+2), d.sword(pc+4))
		return PlacePoint{Index: d.pointAt(pc + 6), Pos: p}, pc + 8, false, nil
	case opSmallPoint:
		p := pos(d.sbyte(pc)*32, d.sbyte(pc+1)*32, d.sbyte(pc+2)*32)
		return PlacePoint{Index: d.byte_(pc + 3), Pos: p}, pc + 4, false, nil
	case opPoints:
		n, first := d.word(pc), d.pointAt(pc+2)
		pc += 4
		var b Block
		for i := 0; i < n; i++ {
			b = append(b, PlacePoint{Index: first + i, Pos: pos(d.sword(pc), d.sword(pc+2), d.sword(pc+4))})
			pc += 6
		}
		return Call{b}, pc, false, nil // a Call of placements ends nothing
	case opSmallPoints:
		n, first := d.byte_(pc), d.byte_(pc+1)
		pc += 2
		var b Block
		for i := 0; i < n; i++ {
			b = append(b, PlacePoint{Index: first + i, Pos: pos(d.sbyte(pc)*32, d.sbyte(pc+1)*32, d.sbyte(pc+2)*32)})
			pc += 3
		}
		return Call{b}, pc, false, nil

	case opXOffset, opXOffsetB, opYOffset, opYOffsetB, opZOffset, opZOffsetB:
		axis := map[int]int{opXOffset: 0, opXOffsetB: 0, opYOffset: 1, opYOffsetB: 1, opZOffset: 2, opZOffsetB: 2}[op]
		o := OffsetPoint{From: d.pointAt(pc)}
		o.Offset[axis] = d.sword(pc + 2)
		o.Index = d.pointAt(pc + 4)
		return o, pc + 6, false, nil
	case opXYOffset, opXZOffset, opYZOffset:
		a, b := d.sword(pc), d.sword(pc+2)
		off := map[int][3]int{opXYOffset: {a, b, 0}, opXZOffset: {a, 0, b}, opYZOffset: {0, b, a}}[op]
		return OffsetPoint{From: d.pointAt(pc + 4), Index: d.pointAt(pc + 6), Offset: off}, pc + 8, false, nil
	case opDelta:
		p := pos(d.sword(pc), d.sword(pc+2), d.sword(pc+4))
		return Delta{Index: d.pointAt(pc + 6), Offset: p}, pc + 8, false, nil
	case opSum, opSubtract:
		return SumPoint{A: d.pointAt(pc), B: d.pointAt(pc + 2), Index: d.pointAt(pc + 4), Subtract: op == opSubtract}, pc + 6, false, nil

	case opSetShades, opSetShadesVar:
		n := d.word(pc)
		pc += 2
		var v int
		if op == opSetShadesVar {
			if v, err = d.variable(d.word(pc)); err != nil {
				return nil, 0, false, err
			}
			pc += 2
		}
		shades := make([]PointShade, n)
		for i := range shades {
			shades[i] = PointShade{Point: d.pointAt(pc), Shade: d.byte_(pc + 2)}
			pc += 3
		}
		pc += pc % 2 // the next instruction is word aligned
		if op == opSetShadesVar {
			return SetShadesFromVar{Var: v, Shades: shades}, pc, false, nil
		}
		return SetShades{shades}, pc, false, nil

	case opHull:
		n := d.word(pc)
		pts := make([]int, n)
		for i := range pts {
			pts[i] = d.pointAt(pc + 2 + 2*i)
		}
		return EndUnlessVisible{pts}, pc + 2 + 2*n, false, nil
	case opBox:
		bx := EndUnlessBoxVisible{Center: d.pointAt(pc), Extent: [3]int{d.sword(pc + 2), d.sword(pc + 4), d.sword(pc + 6)}}
		if d.sword(pc+8) != 0 {
			return nil, 0, false, fmt.Errorf("a box test that skips instead of ending")
		}
		return bx, pc + 10, false, nil

	case opSetColour:
		return SetColour{Colour: d.word(pc)}, pc + 2, false, nil
	case opSetColourVar:
		v, err := d.variable(d.word(pc + 2))
		return SetColour{Colour: d.word(pc), FromVar: true, Var: v}, pc + 4, false, err
	case opShadedColour:
		v, err := d.variable(d.word(pc))
		s := d.word(pc + 2)
		return ShadedColour{Var: v, Offset: s / 256, Shade: s % 256}, pc + 4, false, err
	case opFlatFill:
		return SetFill{FillFlat}, pc, false, nil
	case opShadedFill, opShadedFillV:
		return SetFill{FillShaded}, pc + 2, false, nil
	case opShadedOrPick:
		return SetFill{FillShadedOrPick}, pc, false, nil
	case opTranslucentFill:
		return SetFill{FillTranslucent}, pc, false, nil
	case opCaveFill:
		// The original selects a fill routine here that was never traced; the parts that
		// use it are left out (as the interpreter port did).
		return Stop{"cave fill"}, pc, true, nil

	case opPolygon, opPolygonB:
		n := d.word(pc)
		pts := make([]int, n)
		for i := range pts {
			pts[i] = d.pointAt(pc + 2 + 2*i)
		}
		return Polygon{pts}, pc + 2 + 2*n, false, nil

	case opSetPickColour:
		return SetPickColour{byte(d.word(pc))}, pc + 2, false, nil
	case opSetContext:
		return SetContext{d.word(pc) % 256}, pc + 2, false, nil

	case opTextured, opTexturedB, opTexturedTri:
		n := 4
		if op == opTexturedTri {
			n = 3
		}
		t := CornerTextured{Slot: d.word(pc) % 256}
		pc += 2
		for i := 0; i < n; i++ {
			c := d.word(pc + 2)
			t.Points = append(t.Points, d.pointAt(pc))
			t.Corners = append(t.Corners, Corner{Right: c%2 != 0, Bottom: c >= 2})
			pc += 4
		}
		return t, pc, false, nil
	case opCompact, opCompactB, opCompactMirror, opCompactMirrorB:
		t := CornerTextured{Slot: d.word(pc) % 256, Corners: compactCorners}
		if op == opCompactMirror || op == opCompactMirrorB {
			t.Corners = mirroredCorners
		}
		for i := 0; i < 4; i++ {
			t.Points = append(t.Points, d.byte_(pc+2+i))
		}
		return t, pc + 6, false, nil
	case opUVTextured, opUVTexturedB, opContextUV, opContextUVB:
		t := Textured{Context: true}
		if op == opUVTextured || op == opUVTexturedB {
			t.Slot, t.Context = d.word(pc)%256, false
			pc += 2
		}
		n := d.word(pc)
		pc += 2
		for i := 0; i < n; i++ {
			t.Points = append(t.Points, d.pointAt(pc))
			t.UVs = append(t.UVs, UV{float64(d.word(pc+2)) / 65536, float64(d.word(pc+4)) / 65536})
			pc += 6
		}
		return t, pc, false, nil
	case opNothing:
		return nil, pc, false, nil

	case opMoveIndirect:
		// [[b]] = [a]: the only use is the door frame's, a slot's height through a
		// variable holding where it is
		a, errA := d.variable(d.word(pc))
		b, errB := d.variable(d.word(pc + 2))
		if errA != nil || errB != nil || a != VarSlotHeight || b != VarSlot {
			return nil, 0, false, fmt.Errorf("an indirect move other than the door frame's")
		}
		return SetSlotHeight{}, pc + 4, false, nil
	}
	return Stop{fmt.Sprintf("opcode %d", op)}, pc, true, nil
}

// The corners of a compact quad: round the texture from its top left, or mirrored.
var (
	compactCorners  = []Corner{{}, {Right: true}, {Right: true, Bottom: true}, {Bottom: true}}
	mirroredCorners = []Corner{{Right: true}, {}, {Bottom: true}, {Right: true, Bottom: true}}
)
