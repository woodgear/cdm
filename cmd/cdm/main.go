package main

import (
	"fmt"
	"os"

	"github.com/woodgear/cdm/internal/cli"
	versionpkg "github.com/woodgear/cdm/internal/version"
)

var (
	// Set at build time via ldflags
	version   = "auto"
	gitCommit = "unknown"
	gitBranch = "unknown"
	buildDate = "auto"
)

func main() {
	versionpkg.Value = version
	versionpkg.GitCommit = gitCommit
	versionpkg.GitBranch = gitBranch
	versionpkg.BuildDate = buildDate

	info := versionpkg.Current()
	cli.Version = info.Version
	cli.GitCommit = info.GitCommit
	cli.GitBranch = info.GitBranch
	cli.BuildDate = info.BuildDate

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}
}
