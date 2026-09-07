package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/haruyama480/macos-same-window-switcher/internal/ax"
	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

const (
	exitSuccess       = 0
	exitRuntime       = 1
	exitAccessibility = 2
	exitNoFocusedApp  = 3
)

func timeoutMS(cfg config.Config) int {
	ms := cfg.Focus.AXTimeoutMS
	if ms < 50 {
		return 50
	}
	return ms
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

func writeUntrusted(stderr io.Writer, exe string) {
	fmt.Fprintln(stderr, "same-window-switcher: accessibility permission is not granted.")
	fmt.Fprintln(stderr, "Add this binary (or SameWindowSwitcher.app) in")
	fmt.Fprintln(stderr, "  System Settings → Privacy & Security → Accessibility")
	if exe != "" {
		fmt.Fprintf(stderr, "  %s\n", exe)
	}
	fmt.Fprintln(stderr, "If the path does not appear in the list, run: make install-app")
	fmt.Fprintln(stderr, "Then: same-window-switcher doctor")
}

func writeRuntime(stderr io.Writer, err error) {
	fmt.Fprintf(stderr, "same-window-switcher: %v\n", err)
}

func exitFocusedPID(err error) int {
	if ax.IsNoValue(err) {
		return exitNoFocusedApp
	}
	return exitRuntime
}

func sanitizeTitle(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func b2i(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatWindow(w types.Window) string {
	return fmt.Sprintf("id=%s ax=%d focused=%d main=%d minimized=%d subrole=%s title=%s",
		w.ID.String(), w.AXIndex, b2i(w.Focused), b2i(w.Main), b2i(w.Minimized), w.Subrole, sanitizeTitle(w.Title))
}

func bundlePath(exe string) string {
	if exe == "" {
		return "(none)"
	}
	dir := filepath.Dir(exe)
	if filepath.Base(dir) != "MacOS" {
		return "(none)"
	}
	contents := filepath.Dir(dir)
	if filepath.Base(contents) != "Contents" {
		return "(none)"
	}
	app := filepath.Dir(contents)
	if !strings.HasSuffix(app, ".app") {
		return "(none)"
	}
	return app
}

func codesignIdentifier(exe string) string {
	if exe == "" {
		return "-"
	}
	cmd := exec.Command("codesign", "-dv", exe)
	out, _ := cmd.CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Identifier=") {
			id := strings.TrimPrefix(line, "Identifier=")
			if id == "" {
				return "-"
			}
			return id
		}
	}
	return "-"
}

func openAccessibilitySettings() {
	urls := []string{
		"x-apple.systemsettings:com.apple.settings.PrivacySecurity.extension?Privacy_Accessibility",
		"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility",
	}
	for _, u := range urls {
		if exec.Command("open", u).Run() == nil {
			return
		}
	}
}
