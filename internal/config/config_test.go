package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default
	if d.Cycle.Policy != "spatial" {
		t.Errorf("Cycle.Policy = %q, want spatial", d.Cycle.Policy)
	}
	if d.Cycle.StickyMS != 2000 {
		t.Errorf("Cycle.StickyMS = %d, want 2000", d.Cycle.StickyMS)
	}
	if !d.Cycle.Wrap {
		t.Error("Cycle.Wrap = false, want true")
	}
	if d.Cycle.OnMembershipChange != "merge" {
		t.Errorf("Cycle.OnMembershipChange = %q, want merge", d.Cycle.OnMembershipChange)
	}
	if !reflect.DeepEqual(d.Filter.AllowedSubroles, []string{"AXStandardWindow"}) {
		t.Errorf("Filter.AllowedSubroles = %#v", d.Filter.AllowedSubroles)
	}
	if d.Filter.IncludeMinimized || d.Filter.IncludeDialogs || d.Filter.IncludeFloating {
		t.Errorf("Filter include_* = minimized=%v dialogs=%v floating=%v, want all false",
			d.Filter.IncludeMinimized, d.Filter.IncludeDialogs, d.Filter.IncludeFloating)
	}
	if !d.Focus.Raise || !d.Focus.SetMain {
		t.Errorf("Focus Raise=%v SetMain=%v, want true, true", d.Focus.Raise, d.Focus.SetMain)
	}
	if d.Focus.AXTimeoutMS != 250 {
		t.Errorf("Focus.AXTimeoutMS = %d, want 250", d.Focus.AXTimeoutMS)
	}
	if d.Focus.RaiseRetry != 1 {
		t.Errorf("Focus.RaiseRetry = %d, want 1", d.Focus.RaiseRetry)
	}
	if d.Focus.Unminimize {
		t.Error("Focus.Unminimize = true, want false")
	}
	if d.State.Dir != "" {
		t.Errorf("State.Dir = %q, want empty", d.State.Dir)
	}
}

func testdata(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	p := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "config", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	return p
}

func isolateEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv(envConfig, "")
	return home
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultsWhenNoFile(t *testing.T) {
	isolateEnv(t)
	cfg, path, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
	if !reflect.DeepEqual(cfg, Default) {
		t.Fatalf("cfg = %#v, want Default", cfg)
	}
}

func TestLoadDefaultLikeTestdata(t *testing.T) {
	isolateEnv(t)
	cfg, path, err := Load(testdata(t, "default-like.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if path != testdata(t, "default-like.toml") {
		t.Fatalf("path = %q", path)
	}
	if !reflect.DeepEqual(cfg, Default) {
		t.Fatalf("cfg = %#v, want Default", cfg)
	}
}

func TestLoadUnknownKeyTestdata(t *testing.T) {
	isolateEnv(t)
	_, _, err := Load(testdata(t, "unknown-key.toml"))
	if err == nil {
		t.Fatal("expected unknown key error")
	}
	if !strings.Contains(err.Error(), "unknown keys") {
		t.Fatalf("err = %v, want unknown keys", err)
	}
	if !strings.Contains(err.Error(), "not_a_real_key") {
		t.Fatalf("err = %v, want not_a_real_key", err)
	}
}

func TestLoadInvalidTestdata(t *testing.T) {
	isolateEnv(t)
	_, _, err := Load(testdata(t, "invalid.toml"))
	if err == nil {
		t.Fatal("expected invalid TOML error")
	}
}

func TestLoadExplicitMissingFile(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "missing.toml")
	_, _, err := Load(p)
	if err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestLoadOverlayKeepsDefaults(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "partial.toml")
	writeFile(t, p, "[cycle]\nsticky_ms = 500\n")
	cfg, path, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if path != p {
		t.Fatalf("path = %q, want %q", path, p)
	}
	if cfg.Cycle.StickyMS != 500 {
		t.Fatalf("StickyMS = %d, want 500", cfg.Cycle.StickyMS)
	}
	if cfg.Cycle.Policy != "spatial" || !cfg.Cycle.Wrap || cfg.Cycle.OnMembershipChange != "merge" {
		t.Fatalf("cycle overlay lost defaults: %+v", cfg.Cycle)
	}
	if cfg.Focus.AXTimeoutMS != 250 || cfg.Focus.RaiseRetry != 1 || !cfg.Focus.Raise {
		t.Fatalf("focus overlay lost defaults: %+v", cfg.Focus)
	}
	if Default.Cycle.StickyMS != 2000 {
		t.Fatal("Default mutated")
	}
}

func TestLoadAcceptsFilterTable(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "filter.toml")
	writeFile(t, p, "[filter]\ninclude_dialogs = true\ninclude_floating = true\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Filter.IncludeDialogs || !cfg.Filter.IncludeFloating {
		t.Fatalf("filter = %+v, want include_dialogs and include_floating", cfg.Filter)
	}
	if !reflect.DeepEqual(cfg.Filter.AllowedSubroles, []string{"AXStandardWindow"}) {
		t.Fatalf("AllowedSubroles = %#v, want default", cfg.Filter.AllowedSubroles)
	}
}

func TestLoadAcceptsUnminimize(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "unmin.toml")
	writeFile(t, p, "[focus]\nunminimize = true\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Focus.Unminimize {
		t.Fatal("Unminimize = false, want true")
	}
}

func TestLoadUnknownPolicyDeferred(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "policy.toml")
	writeFile(t, p, "[cycle]\npolicy = \"nope\"\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatalf("unknown policy should be deferred: %v", err)
	}
	if cfg.Cycle.Policy != "nope" {
		t.Fatalf("Policy = %q, want nope", cfg.Cycle.Policy)
	}
}

func TestLoadValidatesStickyMS(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "sticky.toml")
	writeFile(t, p, "[cycle]\nsticky_ms = -1\n")
	_, _, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "sticky_ms") {
		t.Fatalf("err = %v, want sticky_ms validation", err)
	}
}

func TestLoadValidatesOnMembershipChange(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "mem.toml")
	writeFile(t, p, "[cycle]\non_membership_change = \"drop\"\n")
	_, _, err := Load(p)
	if err == nil || !strings.Contains(err.Error(), "on_membership_change") {
		t.Fatalf("err = %v, want on_membership_change validation", err)
	}
}

func TestLoadClampsAXTimeoutMS(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "timeout.toml")
	writeFile(t, p, "[focus]\nax_timeout_ms = 10\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Focus.AXTimeoutMS != 50 {
		t.Fatalf("AXTimeoutMS = %d, want 50", cfg.Focus.AXTimeoutMS)
	}
}

func TestLoadSearchOrderExplicitWins(t *testing.T) {
	home := isolateEnv(t)
	envPath := filepath.Join(t.TempDir(), "env.toml")
	writeFile(t, envPath, "[cycle]\nsticky_ms = 111\n")
	t.Setenv(envConfig, envPath)
	writeFile(t, filepath.Join(home, ".config", "same-window-switcher", "config.toml"), "[cycle]\nsticky_ms = 222\n")
	explicit := filepath.Join(t.TempDir(), "explicit.toml")
	writeFile(t, explicit, "[cycle]\nsticky_ms = 333\n")

	cfg, path, err := Load(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if path != explicit {
		t.Fatalf("path = %q, want %q", path, explicit)
	}
	if cfg.Cycle.StickyMS != 333 {
		t.Fatalf("StickyMS = %d, want 333", cfg.Cycle.StickyMS)
	}
}

func TestLoadSearchOrderEnvWins(t *testing.T) {
	home := isolateEnv(t)
	envPath := filepath.Join(t.TempDir(), "env.toml")
	writeFile(t, envPath, "[cycle]\nsticky_ms = 111\n")
	t.Setenv(envConfig, envPath)
	writeFile(t, filepath.Join(home, ".config", "same-window-switcher", "config.toml"), "[cycle]\nsticky_ms = 222\n")

	cfg, path, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if path != envPath {
		t.Fatalf("path = %q, want %q", path, envPath)
	}
	if cfg.Cycle.StickyMS != 111 {
		t.Fatalf("StickyMS = %d, want 111", cfg.Cycle.StickyMS)
	}
}

func TestLoadSearchOrderXDGWinsOverHome(t *testing.T) {
	home := isolateEnv(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	writeFile(t, filepath.Join(xdg, "same-window-switcher", "config.toml"), "[cycle]\nsticky_ms = 444\n")
	writeFile(t, filepath.Join(home, ".config", "same-window-switcher", "config.toml"), "[cycle]\nsticky_ms = 555\n")

	cfg, path, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "same-window-switcher", "config.toml")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if cfg.Cycle.StickyMS != 444 {
		t.Fatalf("StickyMS = %d, want 444", cfg.Cycle.StickyMS)
	}
}

func TestLoadSearchOrderHomeConfig(t *testing.T) {
	home := isolateEnv(t)
	p := filepath.Join(home, ".config", "same-window-switcher", "config.toml")
	writeFile(t, p, "[cycle]\nsticky_ms = 666\n")
	cfg, path, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if path != p {
		t.Fatalf("path = %q, want %q", path, p)
	}
	if cfg.Cycle.StickyMS != 666 {
		t.Fatalf("StickyMS = %d, want 666", cfg.Cycle.StickyMS)
	}
}

func TestLoadDoesNotSearchLibraryApplicationSupport(t *testing.T) {
	home := isolateEnv(t)
	writeFile(t, filepath.Join(home, "Library", "Application Support", "same-window-switcher", "config.toml"),
		"[cycle]\nsticky_ms = 999\n")
	cfg, path, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("path = %q, want empty (Library must not be searched)", path)
	}
	if cfg.Cycle.StickyMS != 2000 {
		t.Fatalf("StickyMS = %d, want default 2000", cfg.Cycle.StickyMS)
	}
}

func TestLoadEnvMissingFileErrors(t *testing.T) {
	isolateEnv(t)
	missing := filepath.Join(t.TempDir(), "nope.toml")
	t.Setenv(envConfig, missing)
	_, _, err := Load("")
	if err == nil {
		t.Fatal("expected missing env config error")
	}
}

func TestLoadLiveKeys(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "live.toml")
	writeFile(t, p, `
[cycle]
policy = "z-order"
sticky_ms = 1500
wrap = false
on_membership_change = "rebuild"

[focus]
raise = false
set_main = false
ax_timeout_ms = 80
raise_retry = 2

[state]
dir = "/tmp/sws-state"
`)
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Cycle.Policy != "z-order" || cfg.Cycle.StickyMS != 1500 || cfg.Cycle.Wrap || cfg.Cycle.OnMembershipChange != "rebuild" {
		t.Fatalf("cycle = %+v", cfg.Cycle)
	}
	if cfg.Focus.Raise || cfg.Focus.SetMain || cfg.Focus.AXTimeoutMS != 80 || cfg.Focus.RaiseRetry != 2 {
		t.Fatalf("focus = %+v", cfg.Focus)
	}
	if cfg.State.Dir != "/tmp/sws-state" {
		t.Fatalf("State.Dir = %q", cfg.State.Dir)
	}
	if cfg.Filter.IncludeDialogs || cfg.Focus.Unminimize {
		t.Fatal("filter extras / unminimize must stay default-false")
	}
}

func TestLoadAllowedSubrolesReplace(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "roles.toml")
	writeFile(t, p, "[filter]\nallowed_subroles = [\"AXDialog\"]\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Filter.AllowedSubroles, []string{"AXDialog"}) {
		t.Fatalf("AllowedSubroles = %#v, want [AXDialog] (replace, not merge)", cfg.Filter.AllowedSubroles)
	}
}

func TestLoadEmptyAllowedSubrolesKeepsDefault(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "empty-roles.toml")
	writeFile(t, p, "[filter]\nallowed_subroles = []\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Filter.AllowedSubroles, []string{"AXStandardWindow"}) {
		t.Fatalf("AllowedSubroles = %#v, want default", cfg.Filter.AllowedSubroles)
	}
}

func TestLoadIncludeMinimizedForcesUnminimize(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "force.toml")
	writeFile(t, p, "[filter]\ninclude_minimized = true\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Filter.IncludeMinimized {
		t.Fatal("IncludeMinimized = false, want true")
	}
	if !cfg.Focus.Unminimize {
		t.Fatal("Unminimize = false, want forced true")
	}
}

func TestLoadIncludeMinimizedWithUnminimizeTrue(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "both.toml")
	writeFile(t, p, "[filter]\ninclude_minimized = true\n[focus]\nunminimize = true\n")
	cfg, _, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Filter.IncludeMinimized || !cfg.Focus.Unminimize {
		t.Fatalf("IncludeMinimized=%v Unminimize=%v, want true, true",
			cfg.Filter.IncludeMinimized, cfg.Focus.Unminimize)
	}
}

func TestLoadIncludeMinimizedContradiction(t *testing.T) {
	isolateEnv(t)
	p := filepath.Join(t.TempDir(), "contradiction.toml")
	writeFile(t, p, "[filter]\ninclude_minimized = true\n[focus]\nunminimize = false\n")
	_, _, err := Load(p)
	if err == nil {
		t.Fatal("expected contradiction error")
	}
	if !strings.Contains(err.Error(), "include_minimized") || !strings.Contains(err.Error(), "unminimize") {
		t.Fatalf("err = %v, want include_minimized/unminimize contradiction", err)
	}
}
