package main

import (
	"os"

	"github.com/haruyama480/macos-same-window-switcher/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args, os.Stdout, os.Stderr, version))
}
