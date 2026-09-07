//go:build darwin

package cli

import (
	"fmt"
	"io"

	"github.com/haruyama480/macos-same-window-switcher/internal/app"
)

func runCommand(cmd string, f Flags, stdout, stderr io.Writer) int {
	switch cmd {
	case "list":
		return app.List(stdout, stderr, f.Verbose, f.Config)
	case "doctor":
		return app.Doctor(stdout, stderr, f.Config)
	case "next":
		return app.Cycle(stdout, stderr, +1, app.CycleFlags{Config: f.Config, Policy: f.Policy, Verbose: f.Verbose, DryRun: f.DryRun})
	case "prev":
		return app.Cycle(stdout, stderr, -1, app.CycleFlags{Config: f.Config, Policy: f.Policy, Verbose: f.Verbose, DryRun: f.DryRun})
	default:
		fmt.Fprintln(stderr, "not implemented yet")
		return ExitRuntime
	}
}
