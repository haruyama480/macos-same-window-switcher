package app

import (
	"fmt"
	"io"

	"github.com/haruyama480/macos-same-window-switcher/internal/ax"
	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/filter"
)

func List(stdout, stderr io.Writer, verbose bool, configPath string) int {
	cfg, _, err := config.Load(configPath)
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}
	exe := executablePath()
	trusted, err := ax.Trusted(false)
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}
	if !trusted {
		writeUntrusted(stderr, exe)
		return exitAccessibility
	}

	sess, err := ax.Open(timeoutMS(cfg))
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}
	defer sess.Close()

	pid, err := sess.FocusedPID()
	if err != nil {
		writeRuntime(stderr, err)
		return exitFocusedPID(err)
	}
	wins, err := sess.Windows(pid)
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}
	elig, dropped := filter.Eligible(wins, cfg.Filter)
	for _, w := range elig {
		fmt.Fprintln(stdout, formatWindow(w))
	}
	if verbose {
		for _, d := range dropped {
			fmt.Fprintf(stdout, "dropped=%s: %s\n", d.Reason, formatWindow(d.Window))
		}
	}
	return exitSuccess
}
