//go:build !darwin

package cli

import (
	"fmt"
	"io"
)

func runCommand(cmd string, f Flags, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "not implemented on this platform")
	return ExitRuntime
}
