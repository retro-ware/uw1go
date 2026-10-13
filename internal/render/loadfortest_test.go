package render

func loadForTest(exe []byte) (*Renderer, error) { return Load(exe, 0) }

func goldenDoor(r *Renderer, slot int) { r.SetVar(VarSlot, slot) }
