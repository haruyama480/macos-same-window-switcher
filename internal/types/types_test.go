package types

import (
	"fmt"
	"testing"
)

func TestWindowIDString(t *testing.T) {
	tests := []struct {
		name string
		id   WindowID
		want string
	}{
		{
			name: "cg window",
			id:   WindowID{Kind: IDCGWindow, CG: 1234},
			want: "w:1234",
		},
		{
			name: "fallback",
			id:   WindowID{Kind: IDFallback, Key: "1|AXWindow|AXStandardWindow|0|0|100|100"},
			want: "f:1|AXWindow|AXStandardWindow|0|0|100|100",
		},
		{
			name: "fallback empty key",
			id:   WindowID{Kind: IDFallback},
			want: "f:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAssignIDs(t *testing.T) {
	frame := Rectangle{X: 10.4, Y: 20.5, W: 100.2, H: 200.6}
	tests := []struct {
		name    string
		windows []Window
		want    []WindowID
	}{
		{
			name: "cg non-zero",
			windows: []Window{
				{ID: WindowID{CG: 42}, PID: 1, Role: "AXWindow", Subrole: "AXStandardWindow", Frame: frame},
			},
			want: []WindowID{{Kind: IDCGWindow, CG: 42}},
		},
		{
			name: "fallback key rounds frame",
			windows: []Window{
				{
					PID:     7,
					Role:    "AXWindow",
					Subrole: "AXStandardWindow",
					Title:   "ignored",
					Frame:   frame,
				},
			},
			want: []WindowID{{
				Kind: IDFallback,
				Key:  fmt.Sprintf("%d|%s|%s|%.0f|%.0f|%.0f|%.0f", 7, "AXWindow", "AXStandardWindow", 10.0, 21.0, 100.0, 201.0),
			}},
		},
		{
			name: "collision salts only colliding windows",
			windows: []Window{
				{
					PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow",
					Title: "A", Frame: Rectangle{X: 1, Y: 2, W: 3, H: 4},
				},
				{
					PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow",
					Title: "B", Frame: Rectangle{X: 1, Y: 2, W: 3, H: 4},
				},
				{
					PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow",
					Title: "C", Frame: Rectangle{X: 9, Y: 9, W: 9, H: 9},
				},
			},
			want: []WindowID{
				{Kind: IDFallback, Key: "3|AXWindow|AXStandardWindow|1|2|3|4|t:A"},
				{Kind: IDFallback, Key: "3|AXWindow|AXStandardWindow|1|2|3|4|t:B"},
				{Kind: IDFallback, Key: "3|AXWindow|AXStandardWindow|9|9|9|9"},
			},
		},
		{
			name: "cg and fallback mixed; cg does not collide",
			windows: []Window{
				{ID: WindowID{CG: 8}, PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow", Frame: Rectangle{X: 1, Y: 2, W: 3, H: 4}},
				{PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow", Title: "only", Frame: Rectangle{X: 1, Y: 2, W: 3, H: 4}},
			},
			want: []WindowID{
				{Kind: IDCGWindow, CG: 8},
				{Kind: IDFallback, Key: "3|AXWindow|AXStandardWindow|1|2|3|4"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssignIDs(tt.windows)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].ID != tt.want[i] {
					t.Errorf("[%d] ID = %+v (%s), want %+v (%s)", i, got[i].ID, got[i].ID, tt.want[i], tt.want[i])
				}
			}
		})
	}
}

func TestAssignIDsDoesNotFoldTo32Bit(t *testing.T) {
	w := Window{
		PID:     1,
		Role:    "AXWindow",
		Subrole: "AXStandardWindow",
		Frame:   Rectangle{X: 1, Y: 2, W: 3, H: 4},
	}
	got := AssignIDs([]Window{w})
	if got[0].ID.Kind != IDFallback {
		t.Fatalf("Kind = %v, want IDFallback", got[0].ID.Kind)
	}
	if got[0].ID.CG != 0 {
		t.Fatalf("CG = %d, want 0 (must not fold fallback into CG space)", got[0].ID.CG)
	}
	if got[0].ID.Key != "1|AXWindow|AXStandardWindow|1|2|3|4" {
		t.Fatalf("Key = %q", got[0].ID.Key)
	}
}

func cg(id uint32) WindowID {
	return WindowID{Kind: IDCGWindow, CG: id}
}

func TestCurrentWindow(t *testing.T) {
	w := func(id uint32, main bool, ax int) Window {
		return Window{ID: cg(id), Main: main, AXIndex: ax}
	}
	tests := []struct {
		name     string
		focused  WindowID
		eligible []Window
		want     WindowID
		wantOK   bool
	}{
		{
			name:     "empty eligible",
			focused:  cg(1),
			eligible: nil,
			wantOK:   false,
		},
		{
			name:     "focused valid and present",
			focused:  cg(2),
			eligible: []Window{w(1, true, 0), w(2, false, 1)},
			want:     cg(2),
			wantOK:   true,
		},
		{
			name:     "focused not in eligible uses Main",
			focused:  cg(99),
			eligible: []Window{w(1, false, 0), w(2, true, 1)},
			want:     cg(2),
			wantOK:   true,
		},
		{
			name:     "invalid focused CG=0 skips to Main",
			focused:  WindowID{Kind: IDCGWindow, CG: 0},
			eligible: []Window{w(1, false, 0), w(2, true, 1)},
			want:     cg(2),
			wantOK:   true,
		},
		{
			name:     "invalid fallback empty key skips to Main",
			focused:  WindowID{Kind: IDFallback, Key: ""},
			eligible: []Window{w(4, true, 3)},
			want:     cg(4),
			wantOK:   true,
		},
		{
			name:     "no Main uses min AXIndex",
			focused:  cg(99),
			eligible: []Window{w(3, false, 5), w(1, false, 1), w(2, false, 2)},
			want:     cg(1),
			wantOK:   true,
		},
		{
			name:     "multiple Mains uses lowest AXIndex",
			focused:  cg(99),
			eligible: []Window{w(8, true, 4), w(7, true, 1), w(9, true, 9)},
			want:     cg(7),
			wantOK:   true,
		},
		{
			name:    "Window.Focused is not the step-1 source",
			focused: cg(99),
			eligible: []Window{
				{ID: cg(1), Focused: true, AXIndex: 0},
				{ID: cg(2), Main: true, AXIndex: 1},
			},
			want:   cg(2),
			wantOK: true,
		},
		{
			name:     "fallback focused present",
			focused:  WindowID{Kind: IDFallback, Key: "abc"},
			eligible: []Window{{ID: WindowID{Kind: IDFallback, Key: "abc"}, AXIndex: 2}},
			want:     WindowID{Kind: IDFallback, Key: "abc"},
			wantOK:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CurrentWindow(tt.focused, tt.eligible)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (id=%s)", ok, tt.wantOK, got)
			}
			if got != tt.want {
				t.Fatalf("id = %s (%+v), want %s (%+v)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestRaiseSignal(t *testing.T) {
	w := func(id uint32, main bool, ax int) Window {
		return Window{ID: cg(id), Main: main, AXIndex: ax}
	}
	tests := []struct {
		name     string
		eligible []Window
		want     WindowID
		wantOK   bool
	}{
		{
			name:     "empty",
			eligible: nil,
			wantOK:   false,
		},
		{
			name:     "first Main by AXIndex",
			eligible: []Window{w(3, true, 4), w(1, true, 1), w(2, false, 0)},
			want:     cg(1),
			wantOK:   true,
		},
		{
			name:     "no Main uses min AXIndex",
			eligible: []Window{w(3, false, 5), w(8, false, 2)},
			want:     cg(8),
			wantOK:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := RaiseSignal(tt.eligible)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (id=%s)", ok, tt.wantOK, got)
			}
			if got != tt.want {
				t.Fatalf("id = %s (%+v), want %s (%+v)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestEmptyEligibleOKFalse(t *testing.T) {
	if _, ok := CurrentWindow(cg(1), nil); ok {
		t.Fatal("CurrentWindow empty: ok=true")
	}
	if _, ok := CurrentWindow(WindowID{}, []Window{}); ok {
		t.Fatal("CurrentWindow empty slice: ok=true")
	}
	if _, ok := RaiseSignal(nil); ok {
		t.Fatal("RaiseSignal empty: ok=true")
	}
}

func TestRawID(t *testing.T) {
	cgw := Window{ID: WindowID{CG: 9}, PID: 1, Role: "AXWindow", Subrole: "AXStandardWindow"}
	if got := RawID(cgw); got != (WindowID{Kind: IDCGWindow, CG: 9}) {
		t.Fatalf("RawID CG = %+v", got)
	}
	fb := Window{PID: 7, Role: "AXWindow", Subrole: "AXStandardWindow", Frame: Rectangle{X: 1, Y: 2, W: 3, H: 4}}
	want := WindowID{Kind: IDFallback, Key: "7|AXWindow|AXStandardWindow|1|2|3|4"}
	if got := RawID(fb); got != want {
		t.Fatalf("RawID fallback = %+v, want %+v", got, want)
	}
}

func TestIdentifyFocused(t *testing.T) {
	frame := Rectangle{X: 1, Y: 2, W: 3, H: 4}
	a := Window{PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow", Title: "A", Frame: frame, AXIndex: 0}
	b := Window{PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow", Title: "B", Frame: frame, AXIndex: 1}
	c := Window{ID: WindowID{CG: 8}, PID: 3, Role: "AXWindow", Subrole: "AXStandardWindow", Frame: Rectangle{X: 9, Y: 9, W: 9, H: 9}, AXIndex: 2}
	assigned := AssignIDs([]Window{a, b, c})

	t.Run("cg", func(t *testing.T) {
		meta := Window{ID: WindowID{CG: 8}}
		got, ok := IdentifyFocused(meta, assigned)
		if !ok || got.ID != assigned[2].ID || got.AXIndex != 2 {
			t.Fatalf("got %+v ok=%v, want %s ax=2", got, ok, assigned[2].ID)
		}
	})
	t.Run("fallback collision uses title not singleton AssignIDs", func(t *testing.T) {
		meta := b
		meta.AXIndex = -1
		got, ok := IdentifyFocused(meta, assigned)
		if !ok {
			t.Fatal("expected match")
		}
		if got.ID != assigned[1].ID {
			t.Fatalf("ID = %s, want %s", got.ID, assigned[1].ID)
		}
		if got.AXIndex != 1 {
			t.Fatalf("AXIndex = %d, want 1 (raise index from snapshot)", got.AXIndex)
		}
		singleton := AssignIDs([]Window{meta})[0]
		if singleton.ID == assigned[1].ID {
			t.Fatal("singleton AssignIDs unexpectedly equals snapshot ID")
		}
		if RawID(meta) == assigned[1].ID {
			t.Fatal("RawID unexpectedly equals salted snapshot ID")
		}
	})
	t.Run("missing", func(t *testing.T) {
		if _, ok := IdentifyFocused(Window{ID: WindowID{CG: 99}}, assigned); ok {
			t.Fatal("expected no match")
		}
	})
}
