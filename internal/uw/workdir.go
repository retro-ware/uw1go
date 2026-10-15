package uw

import (
	"os"
	"path/filepath"
	"strconv"
)

// workSuffix follows the save folders' names (SAVE0, the working one, and the slots
// SAVE1 to SAVE5) for this running copy: none, or -2, -3, ... while other copies play
// from SaveDir (two on one machine, as for trying online play), so that no copy empties
// another's level files or overwrites the save it just made.
var (
	workSuffix string
	workLock   *os.File // held open (a closed file, or one collected, lets the lock go)
)

// ClaimWorkDir picks workSuffix: the first that no other running copy holds, held until
// this one exits.
func ClaimWorkDir() {
	for n := 1; n < 100; n++ {
		suffix := ""
		if n > 1 {
			suffix = "-" + strconv.Itoa(n)
		}
		f, err := os.OpenFile(filepath.Join(SaveDir, "SAVE"+suffix+".lock"), os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			return
		}
		if lockFile(f) {
			workSuffix, workLock = suffix, f
			return
		}
		f.Close()
	}
}
