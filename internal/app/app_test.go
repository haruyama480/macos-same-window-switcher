package app

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/haruyama480/macos-same-window-switcher/internal/ax"
	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/filter"
	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

func TestListVerboseDropReasons(t *testing.T) {
	wins := []types.Window{
		{Role: "AXSheet", Subrole: "AXStandardWindow", Title: "Sheet"},
		{Role: "AXWindow", Subrole: "AXDialog", Title: "Save"},
		{Role: "AXWindow", Subrole: "AXFloatingWindow", Title: "Palette"},
		{Role: "AXWindow", Subrole: "AXStandardWindow", Title: "Mini", Minimized: true},
	}
	_, dropped := filter.Eligible(wins, config.Default.Filter)
	got := map[string]string{}
	for _, d := range dropped {
		got[d.Window.Title] = fmt.Sprintf("dropped=%s: %s", d.Reason, formatWindow(d.Window))
	}
	wantPrefix := map[string]string{
		"Sheet":   "dropped=role:",
		"Save":    "dropped=subrole:",
		"Palette": "dropped=subrole:",
		"Mini":    "dropped=minimized:",
	}
	for title, prefix := range wantPrefix {
		line, ok := got[title]
		if !ok {
			t.Fatalf("missing drop line for %q: %+v", title, got)
		}
		if !strings.HasPrefix(line, prefix) {
			t.Fatalf("%s drop = %q, want prefix %q", title, line, prefix)
		}
	}
}

func TestCycleBrokenConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Cycle(&stdout, &stderr, 1, CycleFlags{Config: filepath.Join(t.TempDir(), "missing.toml")})
	if code != exitRuntime {
		t.Fatalf("code = %d, want %d; stderr=%q", code, exitRuntime, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "missing.toml") {
		t.Fatalf("stderr = %q, want missing.toml", stderr.String())
	}
}

func TestExitFocusedPID(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "no value", err: ax.Error{Op: "focused_pid", Code: ax.CodeNoValue}, want: exitNoFocusedApp},
		{name: "cannot complete", err: ax.Error{Op: "focused_pid", Code: -25204}, want: exitRuntime},
		{name: "api disabled", err: ax.Error{Op: "focused_pid", Code: -25211}, want: exitRuntime},
		{name: "failure", err: ax.Error{Op: "focused_pid", Code: -25200}, want: exitRuntime},
		{name: "plain", err: errors.New("unavailable"), want: exitRuntime},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitFocusedPID(tt.err); got != tt.want {
				t.Fatalf("exitFocusedPID = %d, want %d", got, tt.want)
			}
		})
	}
}
