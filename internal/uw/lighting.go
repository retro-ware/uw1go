package uw

import (
	"os"
	"strconv"

	"github.com/memmaker/uw1go/internal/r3d"
)

// Port of LIGHTING.C: the light level of the 3D view. lightabs (expand.go) holds the 16
// shading maps from DATA\LIGHT.DAT (MONO.DAT's grey ones at level 5); set_light reads a
// level's row of DATA\SHADES.DAT. The renderer gets copies of the tables (r3d).

var curLightLevel = 0xFF

// The distance shading parameters (renderer data the C reads as smooth_*).
var smoothDiv, smoothBase, smoothLowpass int16

func loadLightTabs(name string) {
	if b := readFile(name); b != nil {
		copy(lightabs[:], b)
		r3dInit()
		r3d.SetLightTabs(lightabs[:])
	}
}

// setLight sets light level n (SHADES.DAT's rows, 0 darkest to 7 brightest).
func setLight(n int) {
	if v := os.Getenv("UW1GO_LIGHT"); v != "" { // debugging: a fixed light level
		n, _ = strconv.Atoi(v)
	}
	if curLightLevel == n {
		return
	}
	if curLightLevel == 5 {
		loadLightTabs("DATA\\light.dat")
	} else if n == 5 {
		loadLightTabs("DATA\\mono.dat")
	}
	curLightLevel = n
	b := readFile("DATA\\shades.dat")
	if b == nil || len(b) < n*12+12 {
		return
	}
	row := func(i int) int16 { return int16(le16(b, n*12+2*i)) }
	smoothDiv = row(0)
	if smoothDiv <= 1 {
		smoothDiv = 1
	}
	smoothBase = row(1)
	smoothLowpass = row(2)
	curvrad = int(row(3))
	distpoly = int(row(4))
	dist8 = int(row(5))
	r3dInit()
	r3d.SetSmooth(smoothDiv, smoothBase, smoothLowpass)
	presetGrid(curvrad)
	editchng(2)
}

// initLighting loads LIGHT.DAT and XFER.DAT (start-up).
func initLighting() {
	loadLightTabs("DATA\\light.dat")
	if b := readFile("DATA\\xfer.dat"); b != nil {
		r3dInit()
		r3d.SetXfer(b)
	}
}

// randomLight is the mushroom effect: the light maps replaced with 4 KB of low memory
// (the interrupt vector table, in DOS; here whatever the renderer's memory holds there),
// or put back.
func randomLight(on bool) {
	if on {
		r3dInit()
		src := int(r3d.Bytes(r3d.Seg051+0x28A0, 1)[0])
		copy(lightabs[:], r3d.Bytes(src, 0x1000))
		for i := 0; i < 16; i++ {
			lightabs[i*256] = 0
			lightabs[i*256+1] = 0
		}
		r3d.SetLightTabs(lightabs[:])
	} else if curLightLevel != 5 {
		loadLightTabs("DATA\\light.dat")
	} else {
		loadLightTabs("DATA\\mono.dat")
	}
}
