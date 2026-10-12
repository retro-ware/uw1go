//go:build !js

package snd

// Volume is the goplay desk's fader (volume_js.go); there is none outside the browser.
func Volume(string) float64 { return 1 }
