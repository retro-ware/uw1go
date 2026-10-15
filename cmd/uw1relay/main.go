// Command uw1relay is the relay for uw1's online play: it keeps the rooms and passes
// each player's messages to the other. Under systemd it takes the socket it is given
// (deploy/relay) and stops after a while with no one connected.
package main

import (
	"flag"
	"log"

	"github.com/memmaker/uw1go/internal/mp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7781", "where to listen without a socket from systemd")
	idle := flag.Duration("idle", 0, "stop after this long with no one connected (0: never)")
	flag.Parse()
	if err := mp.Serve(*addr, *idle); err != nil {
		log.Fatal(err)
	}
}
