package main

import (
	"syscall/js"
	"time"
)

// pageRelay is the relay on the server that serves the page (goplay): wss://HOST/goplay/uw1relay.
func pageRelay() string {
	loc := js.Global().Get("location")
	scheme := "wss://"
	if loc.Get("protocol").String() == "http:" {
		scheme = "ws://"
	}
	return scheme + loc.Get("host").String() + "/goplay/uw1relay"
}

func goplayURL() string { return js.Global().Get("location").Get("origin").String() + "/goplay/" }

// top is goplay's page, which the game's frame is in.
func top() js.Value {
	if t := js.Global().Get("top"); t.Truthy() {
		return t
	}
	return js.Global()
}

// pageJoin is the code of an invite link the page was opened with (?join=CODE), taken
// once: it is cleared from the address bar, so that the next start does not join again.
func pageJoin() (code string) {
	defer func() { recover() }() // a page of another origin cannot be read
	t := top()
	q := js.Global().Get("URLSearchParams").New(t.Get("location").Get("search"))
	if !q.Call("has", "join").Bool() {
		return ""
	}
	code = q.Call("get", "join").String()
	q.Call("delete", "join")
	u := t.Get("location").Get("pathname").String()
	if s := q.Call("toString").String(); s != "" {
		u += "?" + s
	}
	t.Get("history").Call("replaceState", nil, "", u)
	return code
}

// share puts link on the clipboard; false if the browser would not.
func share(link string) bool {
	clip := js.Global().Get("navigator").Get("clipboard")
	if !clip.Truthy() {
		return false
	}
	done := make(chan bool, 1)
	ok := js.FuncOf(func(js.Value, []js.Value) any { done <- true; return nil })
	no := js.FuncOf(func(js.Value, []js.Value) any { done <- false; return nil })
	defer ok.Release()
	defer no.Release()
	clip.Call("writeText", link).Call("then", ok, no)
	select {
	case r := <-done:
		return r
	case <-time.After(time.Second):
		return false
	}
}
