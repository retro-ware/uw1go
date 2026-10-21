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
	} else if _, err := os.Stat(filepath.Join("data", "UW.EXE")); err == nil {
		defData = "data" // goplay: started in games/uw1 with no arguments
	} else if _, err := os.Stat(defData); err != nil {
		defData = bundledData(defData)
	}
	data := flag.String("data", defData, "the game directory (with DATA, CRIT, CUTS, SOUND and UW.EXE)")
	saves := flag.String("saves", "", "where saved games go (default: the user config directory)")
	importFrom := flag.String("import", "", "goplay: find the game under this folder, copy it to -data, and exit")
	scale := flag.Int("scale", 3, "window scale")
	relay := flag.String("relay", defaultRelay(), "online play: the rendezvous server (host:port or a ws:// URL); \"off\" for none (the host's machine is the meeting point then)")
	hostGame := flag.Bool("host", false, "online play: host a game right away")
	join := flag.String("join", "", "online play: join the game with this code, invite link or ws://HOST:PORT/CODE right away")
	flag.Parse()
	uw.RelayURL, uw.DefaultRelay = *relay, pageRelay()
	uw.AutoHost, uw.AutoJoin = *hostGame, *join
	if uw.AutoJoin == "" {
		uw.AutoJoin = pageJoin()
	}
	uw.Invite, uw.Share = invite, share
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
	uw.ClaimWorkDir()
	if err := host.Run("Ultima Underworld", *scale, uw.Main); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// invite is the link that joins the hosted game code: goplay's page, which starts the game
// joining it (a desktop player gives the link or the code to -join).
func invite(code string) string { return goplayURL() + "?join=" + code }

// defaultRelay is UW1GO_RELAY, or the rendezvous server at ruzzoli.de.
func defaultRelay() string {
	if r := os.Getenv("UW1GO_RELAY"); r != "" {
		return r
	}
	return pageRelay()
}

// bundledData finds the game shipped with a release: beside the executable, or in
// the .app's Resources on macOS (a double-click starts with / as working directory).
func bundledData(def string) string {
	exe, err := os.Executable()
	if err != nil {
		return def
	}
	dir := filepath.Dir(exe)
	for _, d := range []string{filepath.Join(dir, def), filepath.Join(dir, "..", "Resources", "UW")} {
		if _, err := os.Stat(d); err == nil {
			return d
		}
	}
	return def
}
