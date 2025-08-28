package main

import (
	"fmt"
	"os"

	"github.com/jjasghar/ollama-lancache/cmd"
)

// Version information (set via build flags)
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func init() {
	// Set version information for the CLI
	cmd.SetVersionInfo(version, commit, buildTime)
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
