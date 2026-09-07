package filter

import (
	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

type Dropped struct {
	Window types.Window
	Reason string
}

// Eligible keeps AXWindow + allowed subrole + (visible unless IncludeMinimized).
// Missing minimized is treated as visible (false).
func Eligible(windows []types.Window, cfg config.FilterConfig) (eligible []types.Window, dropped []Dropped) {
	allowed := allowedSubroles(cfg)
	for _, w := range windows {
		if w.Role != "AXWindow" {
			dropped = append(dropped, Dropped{Window: w, Reason: "role"})
			continue
		}
		if _, ok := allowed[w.Subrole]; !ok {
			dropped = append(dropped, Dropped{Window: w, Reason: "subrole"})
			continue
		}
		if w.Minimized && !cfg.IncludeMinimized {
			dropped = append(dropped, Dropped{Window: w, Reason: "minimized"})
			continue
		}
		eligible = append(eligible, w)
	}
	return eligible, dropped
}

func allowedSubroles(cfg config.FilterConfig) map[string]struct{} {
	roles := cfg.AllowedSubroles
	if len(roles) == 0 {
		roles = []string{"AXStandardWindow"}
	}
	allowed := make(map[string]struct{}, len(roles)+2)
	for _, s := range roles {
		allowed[s] = struct{}{}
	}
	if cfg.IncludeDialogs {
		allowed["AXDialog"] = struct{}{}
	}
	if cfg.IncludeFloating {
		allowed["AXFloatingWindow"] = struct{}{}
	}
	return allowed
}
