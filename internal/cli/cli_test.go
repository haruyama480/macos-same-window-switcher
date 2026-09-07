package cli

import (
	"bytes"
	"flag"
	"runtime"
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		cmd     string
		flags   Flags
		rest    []string
		wantErr bool
	}{
		{
			name: "command only",
			args: []string{"next"},
			cmd:  "next",
		},
		{
			name:  "flags before command",
			args:  []string{"--verbose", "--policy", "spatial", "next"},
			cmd:   "next",
			flags: Flags{Verbose: true, Policy: "spatial"},
		},
		{
			name:  "single-dash value flags before command",
			args:  []string{"-policy", "spatial", "-config", "/tmp/c.toml", "next"},
			cmd:   "next",
			flags: Flags{Policy: "spatial", Config: "/tmp/c.toml"},
		},
		{
			name:  "equals form",
			args:  []string{"--policy=window-id", "list"},
			cmd:   "list",
			flags: Flags{Policy: "window-id"},
		},
		{
			name:  "flags after command",
			args:  []string{"prev", "-v", "--dry-run", "--config", "/tmp/c.toml"},
			cmd:   "prev",
			flags: Flags{Verbose: true, DryRun: true, Config: "/tmp/c.toml"},
		},
		{
			name:  "short help",
			args:  []string{"-h"},
			flags: Flags{Help: true},
		},
		{
			name:  "long help with command",
			args:  []string{"list", "--help"},
			cmd:   "list",
			flags: Flags{Help: true},
		},
		{
			name: "extra positional",
			args: []string{"next", "extra"},
			cmd:  "next",
			rest: []string{"extra"},
		},
		{
			name:    "unknown flag",
			args:    []string{"next", "--nope"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, flags, rest, err := Parse(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if cmd != tt.cmd {
				t.Errorf("cmd = %q, want %q", cmd, tt.cmd)
			}
			if flags != tt.flags {
				t.Errorf("flags = %+v, want %+v", flags, tt.flags)
			}
			if strings.Join(rest, ",") != strings.Join(tt.rest, ",") {
				t.Errorf("rest = %q, want %q", rest, tt.rest)
			}
		})
	}
}

func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, err bytes.Buffer
	argv := append([]string{"same-window-switcher"}, args...)
	code = Run(argv, &out, &err, "0.1.0")
	return out.String(), err.String(), code
}

func TestRunVersion(t *testing.T) {
	stdout, stderr, code := run(t, "version")
	if code != ExitSuccess {
		t.Fatalf("exit %d, want %d; stderr=%q", code, ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	want := "same-window-switcher 0.1.0 " + runtime.GOOS + "/" + runtime.GOARCH + "\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, code := run(t, args...)
			if code != ExitSuccess {
				t.Fatalf("exit %d; stderr=%q", code, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
			if !strings.Contains(stdout, "Usage: same-window-switcher") {
				t.Fatalf("stdout missing usage: %q", stdout)
			}
		})
	}
}

func TestRunUnknownAndMissing(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown", args: []string{"foo"}, want: "unknown command: foo"},
		{name: "missing", args: nil, want: "missing command"},
		{name: "extra", args: []string{"version", "nope"}, want: "unexpected arguments: nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := run(t, tt.args...)
			if code != ExitUsage {
				t.Fatalf("exit %d, want %d; stderr=%q", code, ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("stderr = %q, want substring %q", stderr, tt.want)
			}
		})
	}
}

func TestRunUnknownFlag(t *testing.T) {
	stdout, stderr, code := run(t, "next", "--bogus")
	if code != ExitUsage {
		t.Fatalf("exit %d, want %d", code, ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr == "" {
		t.Fatal("expected stderr")
	}
}

func TestRunNextPrevWiredOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("next/prev are wired only on darwin")
	}
	for _, cmd := range []string{"next", "prev"} {
		t.Run(cmd, func(t *testing.T) {
			stdout, stderr, code := run(t, cmd, "--dry-run")
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if strings.Contains(stderr, "not implemented") {
				t.Fatalf("%s still stubbed: %q", cmd, stderr)
			}
			if code == ExitUsage {
				t.Fatalf("exit %d (usage); stderr=%q", code, stderr)
			}
		})
	}
}

func TestRunPlatformStubs(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("next/prev/list/doctor are implemented on darwin")
	}
	for _, cmd := range []string{"next", "prev", "list", "doctor"} {
		t.Run(cmd, func(t *testing.T) {
			stdout, stderr, code := run(t, cmd, "--verbose", "--dry-run", "--policy", "spatial")
			if code != ExitRuntime {
				t.Fatalf("exit %d, want %d; stderr=%q", code, ExitRuntime, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "not implemented on this platform") {
				t.Fatalf("stderr = %q, want substring %q", stderr, "not implemented on this platform")
			}
		})
	}
}

func TestExitConstants(t *testing.T) {
	if ExitSuccess != 0 || ExitRuntime != 1 || ExitAccessibility != 2 || ExitNoFocusedApp != 3 || ExitUsage != 64 {
		t.Fatalf("unexpected exit constants")
	}
	if flag.ErrHelp == nil {
		t.Fatal("flag.ErrHelp is nil")
	}
}
