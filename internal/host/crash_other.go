//go:build !js

package host

// ReportCrash does nothing off the web: the report on stderr is all there is.
func ReportCrash(string) {}
