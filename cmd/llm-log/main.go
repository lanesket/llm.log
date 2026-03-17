package main

import (
	"os"

	"github.com/lanesket/llm.log/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
