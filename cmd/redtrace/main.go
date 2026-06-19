// Command redtrace is the entrypoint for the RedTrace security testing platform.
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("redtrace %s\n", version)
		return
	}

	fmt.Fprintln(os.Stderr, "redtrace: command wiring is implemented in a later step")
	os.Exit(0)
}
