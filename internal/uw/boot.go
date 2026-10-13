package uw

import (
	"fmt"
	"os"

	"github.com/memmaker/uw1go/internal/gr"
	"github.com/memmaker/uw1go/internal/host"
)

var frame host.Frame

// Main is the game's entry point on the host's game goroutine.
func Main() {
	defer func() {
		if r := recover(); r != nil {
			if fe, ok := r.(fatalError); ok {
				fmt.Fprintln(os.Stderr, fe.msg)
				return
			}
			panic(r)
		}
	}()
	host.ShowFrame = func() *host.Frame {
		frame.Pix = gr.Shown()
		frame.W = gr.W
		frame.Palette = &gr.DAC
		return &frame
	}
	gr.FlipHook = host.Yield
	if err := loadExe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	uwMain()
}

// waitTicks lets n ticks of the game clock pass.
func waitTicks(n uint32) {
	start := GameTime()
	for GameTime()-start < n {
		host.Yield()
	}
}
