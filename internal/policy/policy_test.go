package policy

import (
	"reflect"
	"testing"

	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

func cg(n uint32) types.WindowID {
	return types.WindowID{Kind: types.IDCGWindow, CG: n}
}

func fb(key string) types.WindowID {
	return types.WindowID{Kind: types.IDFallback, Key: key}
}

func win(id uint32, x, y float64) types.Window {
	return types.Window{ID: cg(id), Frame: types.Rectangle{X: x, Y: y}}
}

func idsOf(windows []types.Window) []types.WindowID {
	out := make([]types.WindowID, len(windows))
	for i, w := range windows {
		out[i] = w.ID
	}
	return out
}

func mustLookup(t *testing.T, name string) SortPolicy {
	t.Helper()
	p, err := Lookup(name)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", name, err)
	}
	return p
}

func TestLookup(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: "spatial", want: "spatial"},
		{name: "window-id", want: "window-id"},
		{name: "z-order", want: "z-order"},
		{name: "mru", want: "mru"},
		{name: "unknown", wantErr: true},
		{name: "Spatial", wantErr: true},
		{name: "mruApprox", wantErr: true},
		{name: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Lookup(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Lookup(%q) err = nil, want error", tt.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("Lookup(%q) unexpected err: %v", tt.name, err)
			}
			if got.Name() != tt.want {
				t.Fatalf("Name() = %q, want %q", got.Name(), tt.want)
			}
		})
	}
}

func TestSortPolicies(t *testing.T) {
	spatialFixture := []types.Window{
		win(1, 100, 0),
		win(2, 0, 0),
		win(3, 0, 100),
	}
	// spatial next from w:2 → w:3 → w:1 → w:2

	tests := []struct {
		name    string
		policy  string
		windows []types.Window
		ctx     Context
		want    []types.WindowID
	}{
		{
			name:    "spatial left-to-right then top-to-bottom",
			policy:  "spatial",
			windows: spatialFixture,
			ctx:     Context{LastOrder: idsOf(spatialFixture)},
			want:    []types.WindowID{cg(2), cg(3), cg(1)},
		},
		{
			name:   "spatial tie-break on ID.String",
			policy: "spatial",
			windows: []types.Window{
				win(2, 0, 0),
				win(10, 0, 0),
			},
			ctx: Context{LastOrder: []types.WindowID{cg(2), cg(10)}},
			// "w:10" < "w:2" (string), not CG numeric order
			want: []types.WindowID{cg(10), cg(2)},
		},
		{
			name:   "window-id CG then fallback Key",
			policy: "window-id",
			windows: []types.Window{
				{ID: fb("b")},
				{ID: cg(10)},
				{ID: fb("a")},
				{ID: cg(2)},
			},
			ctx:  Context{LastOrder: []types.WindowID{fb("b"), cg(10), fb("a"), cg(2)}},
			want: []types.WindowID{cg(2), cg(10), fb("a"), fb("b")},
		},
		{
			name:   "z-order by AXIndex",
			policy: "z-order",
			windows: []types.Window{
				{ID: cg(1), AXIndex: 5},
				{ID: cg(2), AXIndex: 1},
				{ID: cg(3), AXIndex: 3},
			},
			ctx:  Context{LastOrder: []types.WindowID{cg(1), cg(2), cg(3)}},
			want: []types.WindowID{cg(2), cg(3), cg(1)},
		},
		{
			name:   "mru current first, LastOrder relative, new spatial-inserted",
			policy: "mru",
			windows: []types.Window{
				win(1, 100, 0),
				win(2, 0, 0),
				win(3, 50, 0),
				win(4, 50, 0),
			},
			ctx: Context{
				Current:   cg(3),
				CurrentOK: true,
				LastOrder: []types.WindowID{cg(2), cg(1)},
			},
			// current w:3; remaining LastOrder w:2,w:1; w:4 at x=50 inserts between them
			want: []types.WindowID{cg(3), cg(2), cg(4), cg(1)},
		},
		{
			name:   "mru ignores Current when not in windows",
			policy: "mru",
			windows: []types.Window{
				win(1, 100, 0),
				win(2, 0, 0),
				win(4, 50, 0),
			},
			ctx: Context{
				Current:   cg(99),
				CurrentOK: true,
				LastOrder: []types.WindowID{cg(2), cg(1)},
			},
			want: []types.WindowID{cg(2), cg(4), cg(1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustLookup(t, tt.policy).Sort(tt.windows, tt.ctx)
			if !reflect.DeepEqual(idsOf(got), tt.want) {
				t.Fatalf("ids = %v, want %v", idsOf(got), tt.want)
			}
		})
	}
}

func TestSortDoesNotMutateInput(t *testing.T) {
	policies := []string{"spatial", "window-id", "z-order", "mru"}
	in := []types.Window{
		{ID: cg(1), Frame: types.Rectangle{X: 100, Y: 0}, Title: "a", AXIndex: 2},
		{ID: cg(2), Frame: types.Rectangle{X: 0, Y: 0}, Title: "b", AXIndex: 0},
		{ID: fb("z"), Frame: types.Rectangle{X: 50, Y: 0}, Title: "c", AXIndex: 1},
	}
	ctx := Context{
		Current:   cg(2),
		CurrentOK: true,
		LastOrder: []types.WindowID{cg(1), cg(2)},
	}
	for _, name := range policies {
		t.Run(name, func(t *testing.T) {
			orig := copyWindows(in)
			out := mustLookup(t, name).Sort(in, ctx)
			if !reflect.DeepEqual(in, orig) {
				t.Fatalf("input mutated: got %+v, want %+v", in, orig)
			}
			if len(out) == 0 {
				t.Fatal("empty output")
			}
			out[0].Title = "mutated"
			if in[0].Title == "mutated" {
				t.Fatal("output aliases input")
			}
		})
	}
}
