package render

// The 3D models (doors, tables, chairs, pillars, ...) as the renderer keeps them: a tree of
// steps, decoded once from UW.EXE (see decode.go) and walked to draw. A model places
// numbered points (in its own frame, turned and moved by the steps), then draws polygons
// through them, choosing which parts to draw and in what order from where the eye is and
// from the model variables the game sets.

// Model is one 3D model.
type Model struct {
	Body Block
}

// Block is a sequence of steps.
type Block []Step

// Step is one thing a model does. draw does it and reports whether the model's current
// part ends there (a part is a Model's Body or a Call's): a hull or box test that finds it
// out of view, a jump's tail having run, or something that could not be decoded.
type Step interface {
	draw(r *Renderer) (end bool)
}

// Model variables: what the game tells a model (GRIDDB.C's Clk(n)), a 16-bit value each.
const (
	VarColour     = 0  // a base colour (shaded faces, colour-from-variable faces)
	VarColour2    = 1  // a second colour
	VarDoorState  = 3  // a door's state: open steps, and whether it is in the near half
	VarSmooth     = 4  // distance shading of textured faces
	VarTurn       = 5  // a heading for turned parts (doors' swing, a lever's pitch)
	VarSlot       = 6  // a texture slot, for SetSlotHeight
	VarSlotHeight = 7  // the height SetSlotHeight gives it
	VarFlat       = 8  // textured faces drawn flat in the current colour
	VarPick       = 9  // this is a pick frame
	VarShade      = 10 // a shade added to shaded colours
	VarFlagColour = 11 // a colour a flag picks
	numVars       = 12
)

// Plane is a plane a model tests the eye against: the sum over its terms of
// Normal * (Offset + the eye's coordinate along Axis), in the original's 16-bit
// arithmetic. The eye is in front when the sum is not negative.
type Plane []PlaneTerm

// PlaneTerm is one axis's part of a Plane.
type PlaneTerm struct {
	Axis           int
	Normal, Offset int
}

// Corner is a corner of a texture.
type Corner struct {
	Right, Bottom bool
}

// UV is a place in a texture, as fractions of its width and height.
type UV struct {
	U, V float64
}

// FillStyle is how polygons are filled.
type FillStyle int

const (
	FillFlat         FillStyle = iota // in the current colour
	FillShaded                        // the current colour shaded by each point's shade (Gouraud)
	FillShadedOrPick                  // shaded, or flat in the pick colour in a pick frame
	FillTranslucent                   // darkening what is behind by the points' shades
	FillConcave                       // flat in the current colour, the polygon maybe concave
)

// The steps.
type (
	// Call draws Body as a part of its own: an end in it ends only Body.
	Call struct{ Body Block }

	// Jump draws Body and then ends the current part (the original's jump to shared code).
	Jump struct{ Body Block }

	// Unless draws Body unless variable Var compares to Value: at most it when AtMost,
	// at least it otherwise.
	Unless struct {
		Var    int
		AtMost bool
		Value  int
		Body   Block
	}

	// InOrder draws First then Second, or Second first when variable Var is below Value.
	InOrderByVar struct {
		Var           int
		Value         int
		First, Second Block
	}

	// InOrder draws First then Second, or Second first when the eye is behind Plane:
	// the half the eye is in comes last.
	InOrder struct {
		Plane         Plane
		First, Second Block
	}

	// IfFacing draws Body when the eye is in front of Plane.
	IfFacing struct {
		Plane Plane
		Body  Block
	}

	// IfFacingAxis draws Body when the eye is on Normal's side of the plane across Axis
	// at -Offset (the one-axis test).
	IfFacingAxis struct {
		Axis           int
		Normal, Offset int
		Body           Block
	}

	// MoveOrigin moves the model's origin by Offset.
	MoveOrigin struct{ Offset [3]int }

	// Turned draws Body turned by Heading, or by variable Var's value when ByVar.
	Turned struct {
		Heading int16
		ByVar   bool
		Var     int
		Body    Block
	}

	// PlacePoint places point Index at Pos.
	PlacePoint struct {
		Index int
		Pos   [3]int
	}

	// OffsetPoint places point Index at point From moved by Offset.
	OffsetPoint struct {
		Index, From int
		Offset      [3]int
	}

	// SumPoint places point Index at point A plus point B (minus it when Subtract).
	SumPoint struct {
		Index, A, B int
		Subtract    bool
	}

	// Delta keeps Offset, turned with the model, as point Index, for SumPoint to add.
	Delta struct {
		Index  int
		Offset [3]int
	}

	// SetShades gives points their shades for shaded faces.
	SetShades struct{ Shades []PointShade }

	// SetShadesFromVar takes the shaded faces' base colour from variable Var and gives
	// points their shades, each plus VarShade's.
	SetShadesFromVar struct {
		Var    int
		Shades []PointShade
	}

	// EndUnlessVisible ends the part when all of Points are outside one side of the view.
	EndUnlessVisible struct{ Points []int }

	// EndUnlessBoxVisible ends the part when the box round point Center, with half
	// extents Extent along the model's axes, is wholly outside one side of the view.
	EndUnlessBoxVisible struct {
		Center int
		Extent [3]int
	}

	// SetColour makes Colour (plus variable Var's value when FromVar) the flat colour.
	SetColour struct {
		Colour  int
		FromVar bool
		Var     int
	}

	// ShadedColour makes the flat colour variable Var's colour plus Offset, shaded by
	// Shade plus VarShade's; in a pick frame the pick colour.
	ShadedColour struct {
		Var           int
		Offset, Shade int
	}

	// SetFill sets how the next polygons are filled.
	SetFill struct{ Style FillStyle }

	// Polygon fills a polygon through Points.
	Polygon struct{ Points []int }

	// SetPickColour sets the colour pick frames draw the model in.
	SetPickColour struct{ Colour byte }

	// SetContext makes Slot the slot ContextTextured faces take.
	SetContext struct{ Slot int }

	// CornerTextured maps the texture in Slot onto a polygon, each point at a corner.
	CornerTextured struct {
		Slot    int
		Points  []int
		Corners []Corner
	}

	// Textured maps a texture onto a polygon at UVs: the one in Slot (which becomes the
	// context slot), or the context slot's when Context.
	Textured struct {
		Slot    int
		Context bool
		Points  []int
		UVs     []UV
	}

	// SetSlotHeight sets the height (v limit) of the texture slot variable VarSlot names
	// to variable VarSlotHeight's value.
	SetSlotHeight struct{}

	// Stop ends the part: what the decoder could not read (Why says what).
	Stop struct{ Why string }
)

// PointShade is a point's shade.
type PointShade struct {
	Point int
	Shade int
}
