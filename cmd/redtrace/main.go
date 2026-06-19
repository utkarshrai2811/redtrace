// Command redtrace is the entrypoint for the RedTrace security testing platform.
package main

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	Execute()
}
