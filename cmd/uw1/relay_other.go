//go:build !js

package main

// pageRelay is the rendezvous server at ruzzoli.de (github.com/memmaker/rendezvous),
// plain TCP so that the UDP rendezvous on the same port works (the browser's wss://
// way in, behind nginx, has no port for it and stays relayed).
func pageRelay() string { return "ruzzoli.de:7781" }

func goplayURL() string { return "https://ruzzoli.de/goplay/" }

func pageJoin() string { return "" }

func share(string) bool { return false }
