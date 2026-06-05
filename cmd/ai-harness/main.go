package main

import (
	"os"

	"ai-harness/internal/cli"
)

var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
