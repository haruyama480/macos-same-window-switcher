package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const envConfig = "SAME_WINDOW_SWITCHER_CONFIG"

type Config struct {
	Cycle  CycleConfig  `toml:"cycle"`
	Filter FilterConfig `toml:"filter"`
	Focus  FocusConfig  `toml:"focus"`
	State  StateConfig  `toml:"state"`
}

type CycleConfig struct {
	Policy             string `toml:"policy"`
	StickyMS           int    `toml:"sticky_ms"`
	Wrap               bool   `toml:"wrap"`
	OnMembershipChange string `toml:"on_membership_change"`
}

type FilterConfig struct {
	AllowedSubroles  []string `toml:"allowed_subroles"`
	IncludeMinimized bool     `toml:"include_minimized"`
	IncludeDialogs   bool     `toml:"include_dialogs"`
	IncludeFloating  bool     `toml:"include_floating"`
}

type FocusConfig struct {
	Raise       bool `toml:"raise"`
	SetMain     bool `toml:"set_main"`
	AXTimeoutMS int  `toml:"ax_timeout_ms"`
	RaiseRetry  int  `toml:"raise_retry"`
	Unminimize  bool `toml:"unminimize"`
}

type StateConfig struct {
	Dir string `toml:"dir"`
}

var Default = Config{
	Cycle: CycleConfig{
		Policy:             "spatial",
		StickyMS:           2000,
		Wrap:               true,
		OnMembershipChange: "merge",
	},
	Filter: FilterConfig{
		AllowedSubroles: []string{"AXStandardWindow"},
	},
	Focus: FocusConfig{
		Raise: true, SetMain: true,
		AXTimeoutMS: 250, RaiseRetry: 1,
		Unminimize: false,
	},
}

// Load overlays an optional TOML file onto Default. Search order (first wins):
//  1. explicit (--config)
//  2. $SAME_WINDOW_SWITCHER_CONFIG
//  3. $XDG_CONFIG_HOME/same-window-switcher/config.toml
//  4. ~/.config/same-window-switcher/config.toml
//  5. Default (no file is success)
//
// ~/Library/Application Support is never searched. Unknown keys and broken
// TOML are errors. explicit and $SAME_WINDOW_SWITCHER_CONFIG fail if missing.
func Load(explicit string) (Config, string, error) {
	cfg := Default
	path, err := find(explicit)
	if err != nil {
		return Config{}, "", err
	}
	if path == "" {
		return Default, "", nil
	}
	md, err := overlay(path, &cfg)
	if err != nil {
		return Config{}, path, err
	}
	if err := validate(&cfg, md); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

func find(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if p := os.Getenv(envConfig); p != "" {
		return p, nil
	}
	var paths []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "same-window-switcher", "config.toml"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, ".config", "same-window-switcher", "config.toml"))
	}
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		_, err := os.Stat(p)
		if err == nil {
			return p, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", nil
}

func overlay(path string, cfg *Config) (toml.MetaData, error) {
	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return md, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return md, fmt.Errorf("%s: unknown keys: %s", path, strings.Join(keys, ", "))
	}
	return md, nil
}

func validate(cfg *Config, md toml.MetaData) error {
	if cfg.Cycle.StickyMS < 0 {
		return fmt.Errorf("cycle.sticky_ms must be >= 0")
	}
	switch cfg.Cycle.OnMembershipChange {
	case "merge", "rebuild":
	default:
		return fmt.Errorf("cycle.on_membership_change must be merge or rebuild")
	}
	if cfg.Focus.AXTimeoutMS < 50 {
		cfg.Focus.AXTimeoutMS = 50
	}
	if len(cfg.Filter.AllowedSubroles) == 0 {
		cfg.Filter.AllowedSubroles = []string{"AXStandardWindow"}
	}
	if cfg.Filter.IncludeMinimized {
		if md.IsDefined("focus", "unminimize") && !cfg.Focus.Unminimize {
			return fmt.Errorf("include_minimized=true requires unminimize=true")
		}
		cfg.Focus.Unminimize = true
	}
	return nil
}
