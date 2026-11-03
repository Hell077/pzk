package main

import (
	"fmt"
	"github.com/Hell077/portguard/internal/features/terminal"
	"os"
	"runtime/debug"
)

var version string

func main() {
	if version == "" {
		version = "dev"
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}

	if err := terminal.Run(os.Args[1:], version, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "portguard:", err)
		os.Exit(1)
	}
}
