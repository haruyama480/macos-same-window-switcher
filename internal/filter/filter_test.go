package filter

import (
	"testing"

	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

func TestEligible(t *testing.T) {
	cfg := config.Default.Filter
	std := types.Window{Role: "AXWindow", Subrole: "AXStandardWindow", Title: "Doc"}
	tests := []struct {
		name       string
		in         []types.Window
		cfg        config.FilterConfig
		wantElig   int
		wantReason string
	}{
		{
			name:       "drop non-window",
			in:         []types.Window{{Role: "AXSheet", Subrole: "AXStandardWindow"}},
			cfg:        cfg,
			wantReason: "role",
		},
		{
			name:       "empty subrole",
			in:         []types.Window{{Role: "AXWindow"}},
			cfg:        cfg,
			wantReason: "subrole",
		},
		{
			name:       "dialog",
			in:         []types.Window{{Role: "AXWindow", Subrole: "AXDialog"}},
			cfg:        cfg,
			wantReason: "subrole",
		},
		{
			name:       "floating",
			in:         []types.Window{{Role: "AXWindow", Subrole: "AXFloatingWindow"}},
			cfg:        cfg,
			wantReason: "subrole",
		},
		{
			name:       "minimized",
			in:         []types.Window{{Role: "AXWindow", Subrole: "AXStandardWindow", Minimized: true}},
			cfg:        cfg,
			wantReason: "minimized",
		},
		{
			name:     "missing minimized passes",
			in:       []types.Window{std},
			cfg:      cfg,
			wantElig: 1,
		},
		{
			name:     "standard window",
			in:       []types.Window{std},
			cfg:      cfg,
			wantElig: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elig, dropped := Eligible(tt.in, tt.cfg)
			if len(elig) != tt.wantElig {
				t.Fatalf("eligible = %d, want %d", len(elig), tt.wantElig)
			}
			if tt.wantReason == "" {
				if len(dropped) != 0 {
					t.Fatalf("dropped = %+v, want none", dropped)
				}
				return
			}
			if len(dropped) != 1 {
				t.Fatalf("dropped count = %d, want 1 (%+v)", len(dropped), dropped)
			}
			if dropped[0].Reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", dropped[0].Reason, tt.wantReason)
			}
		})
	}
}

func TestEligibleIncludeFlags(t *testing.T) {
	std := types.Window{Role: "AXWindow", Subrole: "AXStandardWindow", Title: "Doc"}
	dialog := types.Window{Role: "AXWindow", Subrole: "AXDialog", Title: "Save"}
	sysDialog := types.Window{Role: "AXWindow", Subrole: "AXSystemDialog", Title: "Sys"}
	floating := types.Window{Role: "AXWindow", Subrole: "AXFloatingWindow", Title: "Palette"}
	mini := types.Window{Role: "AXWindow", Subrole: "AXStandardWindow", Title: "Mini", Minimized: true}

	t.Run("include_dialogs", func(t *testing.T) {
		cfg := config.Default.Filter
		cfg.IncludeDialogs = true
		elig, dropped := Eligible([]types.Window{std, dialog, sysDialog, floating}, cfg)
		if len(elig) != 2 {
			t.Fatalf("eligible = %d, want 2 (%+v)", len(elig), elig)
		}
		if elig[0].Subrole != "AXStandardWindow" || elig[1].Subrole != "AXDialog" {
			t.Fatalf("eligible = %+v, want standard+dialog", elig)
		}
		if !hasReason(dropped, "subrole") {
			t.Fatalf("dropped = %+v, want subrole for system dialog/floating", dropped)
		}
	})

	t.Run("include_floating", func(t *testing.T) {
		cfg := config.Default.Filter
		cfg.IncludeFloating = true
		elig, dropped := Eligible([]types.Window{std, dialog, floating}, cfg)
		if len(elig) != 2 {
			t.Fatalf("eligible = %d, want 2 (%+v)", len(elig), elig)
		}
		if elig[0].Subrole != "AXStandardWindow" || elig[1].Subrole != "AXFloatingWindow" {
			t.Fatalf("eligible = %+v, want standard+floating", elig)
		}
		if len(dropped) != 1 || dropped[0].Reason != "subrole" || dropped[0].Window.Subrole != "AXDialog" {
			t.Fatalf("dropped = %+v, want dialog subrole", dropped)
		}
	})

	t.Run("include_minimized", func(t *testing.T) {
		cfg := config.Default.Filter
		cfg.IncludeMinimized = true
		elig, dropped := Eligible([]types.Window{std, mini}, cfg)
		if len(elig) != 2 {
			t.Fatalf("eligible = %d, want 2 (%+v)", len(elig), elig)
		}
		if len(dropped) != 0 {
			t.Fatalf("dropped = %+v, want none", dropped)
		}
	})
}

func TestEligibleAllowedSubrolesReplace(t *testing.T) {
	cfg := config.Default.Filter
	cfg.AllowedSubroles = []string{"AXDialog"}
	std := types.Window{Role: "AXWindow", Subrole: "AXStandardWindow", Title: "Doc"}
	dialog := types.Window{Role: "AXWindow", Subrole: "AXDialog", Title: "Save"}
	floating := types.Window{Role: "AXWindow", Subrole: "AXFloatingWindow", Title: "Palette"}

	elig, dropped := Eligible([]types.Window{std, dialog, floating}, cfg)
	if len(elig) != 1 || elig[0].Subrole != "AXDialog" {
		t.Fatalf("eligible = %+v, want only dialog (default not merged)", elig)
	}
	if len(dropped) != 2 {
		t.Fatalf("dropped = %+v, want 2 (standard+floating)", dropped)
	}
	for _, d := range dropped {
		if d.Reason != "subrole" {
			t.Fatalf("reason = %q, want subrole", d.Reason)
		}
	}

	cfg.IncludeFloating = true
	elig, dropped = Eligible([]types.Window{std, dialog, floating}, cfg)
	if len(elig) != 2 {
		t.Fatalf("eligible = %+v, want dialog+floating", elig)
	}
	if len(dropped) != 1 || dropped[0].Window.Subrole != "AXStandardWindow" {
		t.Fatalf("dropped = %+v, want standard", dropped)
	}
}

func TestEligibleEmptyAllowedSubrolesUsesDefault(t *testing.T) {
	cfg := config.FilterConfig{}
	std := types.Window{Role: "AXWindow", Subrole: "AXStandardWindow"}
	dialog := types.Window{Role: "AXWindow", Subrole: "AXDialog"}
	elig, dropped := Eligible([]types.Window{std, dialog}, cfg)
	if len(elig) != 1 || elig[0].Subrole != "AXStandardWindow" {
		t.Fatalf("eligible = %+v, want default standard", elig)
	}
	if len(dropped) != 1 || dropped[0].Reason != "subrole" {
		t.Fatalf("dropped = %+v, want dialog subrole", dropped)
	}
}

func hasReason(dropped []Dropped, reason string) bool {
	for _, d := range dropped {
		if d.Reason == reason {
			return true
		}
	}
	return false
}
