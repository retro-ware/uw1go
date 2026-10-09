// Command uw1 runs Ultima Underworld on Ebiten from the original game files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/felix/uw1go/internal/host"
	"github.com/felix/uw1go/internal/uw"
)

func main() {
	data := flag.String("data", "gamedata/UW", "the game directory (with DATA, CRIT, CUTS, SOUND and UW.EXE)")
	saves := flag.String("saves", "", "where saved games go (default: the user config directory)")
	scale := flag.Int("scale", 3, "window scale")
	flag.Parse()
	uw.GameDir = *data
	uw.SaveDir = *saves
	if uw.SaveDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			dir = "."
		}
		uw.SaveDir = filepath.Join(dir, "uw1go")
	}
	if err := os.MkdirAll(uw.SaveDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := host.Run("Ultima Underworld", *scale, uw.Main); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
