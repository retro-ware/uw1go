//go:build !js

package main

import "github.com/memmaker/uw1go/internal/mp"

func serveRelay(addr string) error { return mp.Serve(addr) }
