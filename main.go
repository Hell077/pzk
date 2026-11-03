package main

import (
	"fmt"
	"github.com/Hell077/portguard/internal/features/terminal"
	"os"
)

var version = "dev"

func main() {
	if err := terminal.Run(os.Args[1:], version, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "portguard:", err)
		os.Exit(1)
	}
}
