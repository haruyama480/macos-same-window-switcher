package app

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/haruyama480/macos-same-window-switcher/internal/ax"
	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/cycle"
	"github.com/haruyama480/macos-same-window-switcher/internal/policy"
	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

func stdWin(id uint32, x, y float64, axIndex int) types.Window {
	return types.Window{
		ID:      types.WindowID{Kind: types.IDCGWindow, CG: id},
		PID:     42,
		Role:    "AXWindow",
		Subrole: "AXStandardWindow",
		Title:   fmt.Sprintf("w%d", id),
		Frame:   types.Rectangle{X: x, Y: y, W: 10, H: 10},
		AXIndex: axIndex,
	}
}

func mustPolicy(t *testing.T, name string) policy.SortPolicy {
	t.Helper()
	p, err := policy.Lookup(name)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", name, err)
	}
	return p
}

func TestParseWindowID(t *testing.T) {
	tests := []struct {
		in   string
		want types.WindowID
		ok   bool
	}{
		{in: "w:1235", want: types.WindowID{Kind: types.IDCGWindow, CG: 1235}, ok: true},
		{in: "f:1|AXWindow|AXStandardWindow|0|0|10|10", want: types.WindowID{Kind: types.IDFallback, Key: "1|AXWindow|AXStandardWindow|0|0|10|10"}, ok: true},
		{in: "w:0", ok: false},
		{in: "w:nope", ok: false},
		{in: "f:", ok: false},
		{in: "x:1", ok: false},
		{in: "", ok: false},
	}
	for _, tt := range tests {
		got, ok := parseWindowID(tt.in)
		if ok != tt.ok {
			t.Errorf("parseWindowID(%q) ok=%v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("parseWindowID(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseLastOrder(t *testing.T) {
	got := parseLastOrder([]string{"w:1", "f:abc", "w:nope", "f:", "w:2"})
	want := []types.WindowID{
		{Kind: types.IDCGWindow, CG: 1},
		{Kind: types.IDFallback, Key: "abc"},
		{Kind: types.IDCGWindow, CG: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLastOrder = %#v, want %#v", got, want)
	}
}

func TestCopyIndexUsesSliceIndexNotAXIndex(t *testing.T) {
	// Non-AXUIElement slots can occupy earlier C-array indices; AXIndex is the
	// kAXWindowsAttribute index and must not be passed to RaiseIndex.
	skip := types.Window{
		ID:      types.WindowID{Kind: types.IDFallback, Key: "skip"},
		AXIndex: 0,
	}
	a := stdWin(1, 0, 0, 5)
	b := stdWin(2, 50, 0, 9)
	all := []types.Window{skip, a, b}

	idx, w, ok := copyIndex(all, "w:2")
	if !ok {
		t.Fatal("copyIndex ok=false")
	}
	if idx != 2 {
		t.Fatalf("copyIndex = %d, want 2 (slice index), not AXIndex %d", idx, b.AXIndex)
	}
	if w.ID.String() != "w:2" {
		t.Fatalf("window id = %s, want w:2", w.ID.String())
	}
	if _, _, ok := copyIndex(all, "w:99"); ok {
		t.Fatal("missing id must not match")
	}
}

func TestFormatCycleLine(t *testing.T) {
	got := formatCycleLine(99, "spatial", "reuse", "w:1235", 2, 4, 12*time.Millisecond)
	want := "pid=99 policy=spatial action=reuse index=2/4 id=w:1235 elapsed=12ms"
	if got != want {
		t.Fatalf("formatCycleLine = %q, want %q", got, want)
	}
}

func TestResolvePolicy(t *testing.T) {
	cfg := config.Default
	p, err := resolvePolicy("", cfg)
	if err != nil || p.Name() != "spatial" {
		t.Fatalf("default: %v name=%q", err, nameOf(p))
	}
	p, err = resolvePolicy("z-order", cfg)
	if err != nil || p.Name() != "z-order" {
		t.Fatalf("override: %v name=%q", err, nameOf(p))
	}
	if _, err := resolvePolicy("nope", cfg); err == nil {
		t.Fatal("unknown policy: expected error")
	}
}

func nameOf(p policy.SortPolicy) string {
	if p == nil {
		return ""
	}
	return p.Name()
}

func TestDecideAndMaybeWrite_FreshSortsSpatially(t *testing.T) {
	store := &cycle.Store{Dir: t.TempDir()}
	// AX / slice order is right-to-left; spatial next from left must raise right.
	right := stdWin(2, 100, 0, 0)
	left := stdWin(1, 0, 0, 1)
	all := []types.Window{right, left}
	pol := mustPolicy(t, "spatial")

	plan, err := decideAndMaybeWrite(store, config.Default, pol, 42, all, all, left.ID, 1000, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Result.NoOp {
		t.Fatal("NoOp")
	}
	if plan.Result.Action != cycle.ActionFresh {
		t.Fatalf("Action = %q, want fresh", plan.Result.Action)
	}
	if plan.Result.RaiseID != "w:2" {
		t.Fatalf("RaiseID = %q, want w:2 (spatial next from left)", plan.Result.RaiseID)
	}
	if plan.CopyIdx != 0 {
		t.Fatalf("CopyIdx = %d, want 0 (right is slice[0], AXIndex 0)", plan.CopyIdx)
	}
	if !reflect.DeepEqual(plan.Result.Order, []string{"w:1", "w:2"}) {
		t.Fatalf("Order = %v, want spatial [w:1 w:2]", plan.Result.Order)
	}
	snap, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if snap.LastRaised != "w:2" || snap.Policy != "spatial" || snap.PID != 42 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.Frames["w:1"] != [4]float64{0, 0, 10, 10} || snap.Frames["w:2"] != [4]float64{100, 0, 10, 10} {
		t.Fatalf("Frames = %v", snap.Frames)
	}
}

func TestDecideAndMaybeWrite_StaleFocusedReuse(t *testing.T) {
	store := &cycle.Store{Dir: t.TempDir()}
	a := stdWin(1, 0, 0, 0)
	b := stdWin(2, 50, 0, 1)
	b.Main = true
	c := stdWin(3, 100, 0, 2)
	all := []types.Window{a, b, c}
	pol := mustPolicy(t, "spatial")
	cfg := config.Default

	first, err := decideAndMaybeWrite(store, cfg, pol, 42, all, all, a.ID, 1000, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.Result.RaiseID != "w:2" {
		t.Fatalf("first RaiseID = %q, want w:2", first.Result.RaiseID)
	}

	// Stale focused A, Main/signal still B = LastRaised → Reuse, next C.
	second, err := decideAndMaybeWrite(store, cfg, pol, 42, all, all, a.ID, 1100, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if second.Result.Action != cycle.ActionReuse {
		t.Fatalf("Action = %q, want reuse", second.Result.Action)
	}
	if second.Result.RaiseID != "w:3" {
		t.Fatalf("RaiseID = %q, want w:3", second.Result.RaiseID)
	}
	if second.CopyIdx != 2 {
		t.Fatalf("CopyIdx = %d, want 2", second.CopyIdx)
	}
}

func TestDecideAndMaybeWrite_Prev(t *testing.T) {
	store := &cycle.Store{Dir: t.TempDir()}
	a := stdWin(1, 0, 0, 0)
	b := stdWin(2, 50, 0, 1)
	b.Main = true
	c := stdWin(3, 100, 0, 2)
	all := []types.Window{a, b, c}
	pol := mustPolicy(t, "spatial")

	if _, err := decideAndMaybeWrite(store, config.Default, pol, 42, all, all, b.ID, 1000, 1, false); err != nil {
		t.Fatal(err)
	}
	got, err := decideAndMaybeWrite(store, config.Default, pol, 42, all, all, c.ID, 1100, -1, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result.RaiseID != "w:2" {
		t.Fatalf("prev RaiseID = %q, want w:2", got.Result.RaiseID)
	}
}

func TestDecideAndMaybeWrite_DryRunDoesNotWrite(t *testing.T) {
	store := &cycle.Store{Dir: t.TempDir()}
	a := stdWin(1, 0, 0, 0)
	b := stdWin(2, 50, 0, 1)
	pol := mustPolicy(t, "spatial")

	plan, err := decideAndMaybeWrite(store, config.Default, pol, 42, []types.Window{a, b}, []types.Window{a, b}, a.ID, 1000, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Result.RaiseID == "" || plan.Result.Action != cycle.ActionFresh {
		t.Fatalf("dry-run plan = %+v", plan.Result)
	}
	snap, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Version != 0 || snap.LastRaised != "" {
		t.Fatalf("dry-run wrote snapshot: %+v", snap)
	}
}

func TestDecideAndMaybeWrite_NoOpTwoOrFewer(t *testing.T) {
	store := &cycle.Store{Dir: t.TempDir()}
	a := stdWin(1, 0, 0, 0)
	pol := mustPolicy(t, "spatial")
	plan, err := decideAndMaybeWrite(store, config.Default, pol, 42, []types.Window{a}, []types.Window{a}, a.ID, 1000, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Result.NoOp {
		t.Fatal("expected NoOp")
	}
	snap, _ := store.Read()
	if snap.Version != 0 {
		t.Fatalf("NoOp wrote snapshot: %+v", snap)
	}
}

func TestMergeInsertOrder(t *testing.T) {
	a := stdWin(1, 0, 0, 0)
	c := stdWin(3, 100, 0, 1)
	b := stdWin(2, 50, 0, 2)
	sorted := []types.Window{a, b, c}
	order := []string{"w:1", "w:3"}

	spatial := mergeInsertOrder("spatial", order, sorted)
	if !reflect.DeepEqual(spatial, []string{"w:1", "w:2", "w:3"}) {
		t.Fatalf("spatial InsertOrder = %v, want [w:1 w:2 w:3]", spatial)
	}
	for _, name := range []string{"z-order", "window-id", "mru"} {
		if got := mergeInsertOrder(name, order, sorted); got != nil {
			t.Fatalf("%s InsertOrder = %v, want nil (append)", name, got)
		}
	}
}

func TestDecideAndMaybeWrite_MergeInsertSpatially(t *testing.T) {
	store := &cycle.Store{Dir: t.TempDir()}
	a := stdWin(1, 0, 0, 0)
	c := stdWin(3, 100, 0, 1)
	c.Main = true
	pol := mustPolicy(t, "spatial")
	cfg := config.Default

	if _, err := decideAndMaybeWrite(store, cfg, pol, 42, []types.Window{a, c}, []types.Window{a, c}, a.ID, 1000, 1, false); err != nil {
		t.Fatal(err)
	}
	b := stdWin(2, 50, 0, 2)
	all := []types.Window{a, c, b}
	got, err := decideAndMaybeWrite(store, cfg, pol, 42, all, all, c.ID, 1100, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result.Action != cycle.ActionMerge {
		t.Fatalf("Action = %q, want merge", got.Result.Action)
	}
	if !reflect.DeepEqual(got.Result.Order, []string{"w:1", "w:2", "w:3"}) {
		t.Fatalf("Order = %v, want [w:1 w:2 w:3] (B inserted spatially)", got.Result.Order)
	}
}

func TestDecideAndMaybeWrite_MergeNonSpatialAppends(t *testing.T) {
	store := &cycle.Store{Dir: t.TempDir()}
	a := stdWin(1, 0, 0, 0)
	c := stdWin(3, 100, 0, 1)
	c.Main = true
	pol := mustPolicy(t, "z-order")
	cfg := config.Default
	cfg.Cycle.Policy = "z-order"

	if _, err := decideAndMaybeWrite(store, cfg, pol, 42, []types.Window{a, c}, []types.Window{a, c}, a.ID, 1000, 1, false); err != nil {
		t.Fatal(err)
	}
	b := stdWin(2, 50, 0, 2)
	all := []types.Window{a, c, b}
	got, err := decideAndMaybeWrite(store, cfg, pol, 42, all, all, c.ID, 1100, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Result.Action != cycle.ActionMerge {
		t.Fatalf("Action = %q, want merge", got.Result.Action)
	}
	if !reflect.DeepEqual(got.Result.Order, []string{"w:1", "w:3", "w:2"}) {
		t.Fatalf("Order = %v, want [w:1 w:3 w:2] (B appended, not spatial splice)", got.Result.Order)
	}
}

func TestRaiseUnminimize(t *testing.T) {
	if raiseUnminimize(config.Default) {
		t.Fatal("default must not unminimize")
	}
	cfg := config.Default
	cfg.Filter.IncludeMinimized = true
	if !raiseUnminimize(cfg) {
		t.Fatal("include_minimized must pass unminimize=true to RaiseIndex")
	}
	cfg = config.Default
	cfg.Focus.Unminimize = true
	if !raiseUnminimize(cfg) {
		t.Fatal("unminimize=true must pass unminimize=true to RaiseIndex")
	}
}

func TestWriteRaiseErrorIncludesTitleAndAXError(t *testing.T) {
	var buf bytes.Buffer
	w := types.Window{Title: "README.md"}
	writeRaiseError(&buf, ax.Error{Op: "raise_index", Code: -25204}, w)
	got := buf.String()
	if !strings.Contains(got, "README.md") {
		t.Fatalf("missing title: %q", got)
	}
	if !strings.Contains(got, "AXError -25204") {
		t.Fatalf("missing AXError: %q", got)
	}
}
