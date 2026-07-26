// Command codelens mines a git log and runs evolutionary code analyses,
// emitting structured JSON by default.
package main

import (
	"os"

	"github.com/andreswebs/codelens/internal/command"
)

// main is the only place the real process environment is touched: it hands the
// arguments and streams to the delegate and exits with its code. It never
// inspects errors; the exit boundary lives in internal/command.
func main() {
	os.Exit(command.Run(os.Args[1:], command.Deps{
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
	}))
}
