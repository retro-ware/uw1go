package uw

import (
	"fmt"
	"os"

	"github.com/memmaker/uw1go/internal/host"
)

// UTIL.C: small helpers.

// mvcheck steps *val by step in direction dir (-1 or 1) unless that passes limit.
func mvcheck(val *int16, limit, step, dir int) bool {
	var ok bool
	if dir == -1 {
		ok = int(*val)-step >= limit
	} else {
		ok = int(*val)+step <= limit
	}
	if !ok {
		return false
	}
	*val += int16(dir * step)
	return true
}

// moveAlong moves (*x, *y) dist along heading, a byte angle; each nonzero step is
// rounded away from zero.
func moveAlong(heading, dist int, x, y *int16) {
	heading = (0x140 - heading) & 0xFF
	heading <<= 8
	dy, dx := cFstSinCos(heading)
	dy = dy / 0x80
	dy = int16(int(dy) * dist)
	dy = dy / 0x100
	dx = dx / 0x80
	dx = int16(int(dx) * dist)
	dx = dx / 0x100
	if dy > 0 {
		dy++
	} else if dy < 0 {
		dy--
	}
	if dx > 0 {
		dx++
	} else if dx < 0 {
		dx--
	}
	*y += dy
	*x += dx
}

// rollem rolls dice dice of sides sides; the total starts at dice.
func rollem(dice, sides int) int {
	total := dice
	if sides > 0 && dice > 0 {
		for ; dice > 0; dice-- {
			total += int((int32(rand()) * int32(sides)) / 0x8000)
		}
	}
	return total
}

// GameTime is GAME_TIME(): the 1/256 s clock. Reading it lets time pass in a busy wait.
func GameTime() uint32 {
	host.Busy()
	return host.Clock
}

// ERROR.C: fatal errors. The game goroutine stops with a fatalError, which Main reports.

type fatalError struct {
	msg  string
	code int
}

func (e fatalError) Error() string { return e.msg }

func errorCodeText(code int) string {
	kind := (code & 0xF000) >> 12
	reason := "Resource problem or internal error."
	switch kind {
	case 1:
		reason = "Out of Low Memory."
	case 2:
		reason = "Out of EMS Memory."
	case 3:
		reason = "Could not read data."
	case 4:
		reason = "Could not write data."
	}
	c := code & 0xFFF
	return fmt.Sprintf("%s Error code %c%d%d%d", reason, 'A'+kind, (c>>6)&7, (c>>3)&7, c&7)
}

// firstPunt is a fatal error during start-up.
func firstPunt(code int) {
	panic(fatalError{"Cannot run Underworld.\n" + errorCodeText(code), -1})
}

// pfatalCode is a fatal error once the game is running.
func pfatalCode(code int) {
	c := code & 0xFFF
	msg := fmt.Sprintf("Underworld can no longer run.  Error code %c%d%d%d.", 'A'+((code&0xF000)>>12), (c>>6)&7, (c>>3)&7, c&7)
	freeWorld(false)
	panic(fatalError{msg, -24})
}

// pfatal is pfatal_code with the caller's message.
func pfatal(msg string) {
	if len(msg) > 0 && msg[len(msg)-1] == '$' {
		msg = msg[:len(msg)-1]
	}
	freeWorld(false)
	panic(fatalError{msg, -24})
}

func dprintf(format string, args ...any) {
	if os.Getenv("UW1GO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, format, args...)
	}
}
