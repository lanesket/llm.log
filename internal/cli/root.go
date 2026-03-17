package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time: -ldflags "-X github.com/lanesket/llm.log/internal/cli.Version=v0.1.0"
var Version = "dev"

const proxyAddr = "127.0.0.1:9922"

var rootCmd = &cobra.Command{
	Use:     "llm-log",
	Short:   "Intercept and log all LLM API calls from your device",
	Long:    "llm.log — local MITM proxy that tracks prompts, tokens, and costs across all LLM providers.",
	Version: Version,
}

func Execute() error {
	return rootCmd.Execute()
}

// DataDir returns the path to ~/.llm.log, creating it if needed.
func DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	dir := filepath.Join(home, ".llm.log")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot create data directory %s: %v\n", dir, err)
		os.Exit(1)
	}
	return dir
}
