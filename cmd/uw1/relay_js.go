package main

import "syscall/js"

// pageRelay is the relay on the server that serves the page (goplay): wss://HOST/goplay/uw1relay.
func pageRelay() string {
	loc := js.Global().Get("location")
	scheme := "wss://"
	if loc.Get("protocol").String() == "http:" {
		scheme = "ws://"
	}
	return scheme + loc.Get("host").String() + "/goplay/uw1relay"
}
