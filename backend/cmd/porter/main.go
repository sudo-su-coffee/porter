// Command porter is the single Porter binary. It dispatches to subcommands
// (server, worker, version, help) from the internals/command package; this
// file only exists so a `package main` owns `Version` for -ldflags
// injection (-X main.Version=...) from the Makefile and CI.
package main

import (
	"fmt"
	"os"

	"porter/internal/command"
)

// Version is Porter's release version (see internal/command, versions.md).
// Overridden at build time via -ldflags "-X main.Version=v...", e.g. from
// the Makefile or .github/workflows/release.yml.
var Version = "v0.1.0-beta-dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println(Version)
			return
		}
	}
	os.Exit(command.Run(os.Args[1:], Version))
}