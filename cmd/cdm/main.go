package main

import (
	"fmt"
	"os"

	"github.com/woodgear/cdm/internal/cli"
)

var (
	// Set at build time via ldflags
	version   = "1.0.0"
	gitCommit = "unknown"
	buildDate = "unknown"
)

func main() {
	cli.Version = version

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
}
