package app

import (
	"fmt"
	"io"
	"time"

	"github.com/haruyama480/macos-same-window-switcher/internal/ax"
	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/filter"
)

func fmtMS(d time.Duration) string {
	return fmt.Sprintf("%.1fms", d.Seconds()*1000)
}

func Doctor(stdout, stderr io.Writer, configPath string) int {
	cfg, used, err := config.Load(configPath)
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}

	exe := executablePath()

	t0 := time.Now()
	trusted, trustErr := ax.Trusted(true)
	trustedDur := time.Since(t0)

	if trustErr != nil {
		fmt.Fprintln(stdout, "trusted: (check failed)")
	} else {
		fmt.Fprintf(stdout, "trusted: %v\n", trusted)
	}
	fmt.Fprintf(stdout, "executable: %s\n", exe)
	fmt.Fprintf(stdout, "codesign identifier: %s\n", codesignIdentifier(exe))
	fmt.Fprintf(stdout, "bundle path: %s\n", bundlePath(exe))
	if used == "" {
		fmt.Fprintln(stdout, "config: (defaults)")
	} else {
		fmt.Fprintf(stdout, "config: %s\n", used)
	}

	var openDur, pidDur, copyDur time.Duration
	focusedPID := "-"
	nElig := 0
	code := exitSuccess
	var runtimeErr error

	if trustErr != nil {
		code = exitRuntime
		runtimeErr = trustErr
	} else if !trusted {
		code = exitAccessibility
	}

	if trustErr == nil {
		tOpen := time.Now()
		sess, err := ax.Open(timeoutMS(cfg))
		openDur = time.Since(tOpen)
		if err != nil {
			if code == exitSuccess {
				code = exitRuntime
				runtimeErr = err
			}
		} else {
			defer sess.Close()
			tPID := time.Now()
			pid, err := sess.FocusedPID()
			pidDur = time.Since(tPID)
			if err != nil {
				if code == exitSuccess {
					code = exitFocusedPID(err)
					runtimeErr = err
				}
			} else {
				focusedPID = fmt.Sprintf("%d", pid)
				tCopy := time.Now()
				wins, err := sess.Windows(pid)
				copyDur = time.Since(tCopy)
				if err != nil {
					if code == exitSuccess {
						code = exitRuntime
						runtimeErr = err
					}
				} else {
					elig, _ := filter.Eligible(wins, cfg.Filter)
					nElig = len(elig)
				}
			}
		}
	}

	fmt.Fprintf(stdout, "timing: trusted=%s open=%s focused_pid=%s copy_windows=%s\n",
		fmtMS(trustedDur), fmtMS(openDur), fmtMS(pidDur), fmtMS(copyDur))
	fmt.Fprintf(stdout, "focused pid: %s\n", focusedPID)
	fmt.Fprintf(stdout, "eligible windows: %d\n", nElig)
	fmt.Fprintln(stdout, "If Settings does not list this executable, use the .app (make install-app)")
	fmt.Fprintln(stdout, "  and point skhd at Contents/MacOS/same-window-switcher.")

	if runtimeErr != nil && code != exitAccessibility {
		writeRuntime(stderr, runtimeErr)
	}
	if code == exitAccessibility {
		openAccessibilitySettings()
	}
	return code
}
