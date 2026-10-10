package host

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
)

// The crash report: what crashed, in which build, after which input. recent keeps the last
// input events (keys, buttons, mouse look) for it.
var (
	recent    [40]string
	recentN   int
	crashOnce bool
)

// note records an input event for a crash report.
func note(format string, args ...any) {
	recent[recentN%len(recent)] = fmt.Sprintf("%7.2fs ", float64(Clock)/256) + fmt.Sprintf(format, args...)
	recentN++
}

// crashReport describes the panic r, raised on the game goroutine (stack is its stack).
func crashReport(r any, stack []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "panic: %v\n\n", r)
	rev, when, dirty := "unknown", "", ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.time":
				when = " " + s.Value
			case "vcs.modified":
				if s.Value == "true" {
					dirty = " (modified)"
				}
			}
		}
	}
	fmt.Fprintf(&b, "build %s%s%s, %s/%s, %s\n", rev, when, dirty, runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Fprintf(&b, "game time %.2fs, mouse look %v\n\nlast input:\n", float64(Clock)/256, MouseLook)
	for i := max(recentN-len(recent), 0); i < recentN; i++ {
		b.WriteString("  " + recent[i%len(recent)] + "\n")
	}
	b.WriteString("\n")
	b.Write(stack)
	return b.String()
}

// crashed handles a panic on the game goroutine: the report goes to stderr and, on the web,
// into the page (ReportCrash). Off the web the program then exits as a panic does.
func crashed(r any) {
	if crashOnce {
		return
	}
	crashOnce = true
	report := crashReport(r, debug.Stack())
	fmt.Fprint(os.Stderr, report)
	ReportCrash(report)
	if runtime.GOOS != "js" {
		os.Exit(2)
	}
}
