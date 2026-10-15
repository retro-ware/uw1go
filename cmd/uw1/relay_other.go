//go:build !js

package main

func pageRelay() string { return "wss://ruzzoli.de/goplay/uw1relay" }

func goplayURL() string { return "https://ruzzoli.de/goplay/" }

func pageJoin() string { return "" }

func share(string) bool { return false }
