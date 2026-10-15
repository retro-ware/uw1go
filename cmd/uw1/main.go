// Command uw1 runs Ultima Underworld on Ebiten from the original game files.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/memmaker/uw1go/internal/host"
	"github.com/memmaker/uw1go/internal/uw"
)

func main() {
	defData := "gamedata/UW"
	if d := os.Getenv("GOPLAY_DATA"); d != "" { // goplay (the browser): the imported game, saves beside it
		defData = d
	}
	data := flag.String("data", defData, "the game directory (with DATA, CRIT, CUTS, SOUND and UW.EXE)")
	saves := flag.String("saves", "", "where saved games go (default: the user config directory)")
	importFrom := flag.String("import", "", "goplay: find the game under this folder, copy it to -data, and exit")
	scale := flag.Int("scale", 3, "window scale")
	defRelay := os.Getenv("UW1GO_RELAY")
	if defRelay == "" {
		defRelay = os.Getenv("GOPLAY_RELAY")
	}
	relay := flag.String("relay", defRelay, "online play: the relay's address (ws://host:port); empty, no online menu")
	serve := flag.String("serve-relay", "", "run an online play relay on this address (such as :7777) instead of the game")
	flag.Parse()
	if *serve != "" {
		if err := serveRelay(*serve); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	uw.RelayURL = *relay
	if *importFrom != "" {
		if err := importGame(*importFrom, *data); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	uw.GameDir = *data
	uw.SaveDir = *saves
	if uw.SaveDir == "" && os.Getenv("GOPLAY_DATA") != "" {
		uw.SaveDir = *data
	}
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
