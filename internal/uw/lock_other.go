//go:build !unix && !windows

package uw

import "os"

func lockFile(*os.File) bool { return true } // the browser: one copy per page
