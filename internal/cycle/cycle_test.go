package cycle

import (
	"reflect"
	"testing"

	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

func cg(id uint32) types.WindowID {
	return types.WindowID{Kind: types.IDCGWindow, CG: id}
}

func win(id uint32, x, y, w, h float64) types.Window {
	return types.Window{
		ID:    cg(id),
		Frame: types.Rectangle{X: x, Y: y, W: w, H: h},
	}
}

func frames(ws ...types.Window) map[string][4]float64 {
	return framesFromEligible(ws)
}

func baseInput(ws []types.Window, current, signal types.WindowID) Input {
	return Input{
		PID:                10,
		Policy:             "spatial",
		StickyMS:           2000,
		Wrap:               true,
		OnMembershipChange: MembershipMerge,
		NowMS:              1500,
		Eligible:           ws,
		Current:            current,
		CurrentOK:          true,
		Signal:             signal,
		SignalOK:           true,
		Direction:          1,
	}
}

func baseSnap(ws []types.Window, last string) Snapshot {
	ids := eligibleIDs(ws)
	return Snapshot{
		Version:    1,
		PID:        10,
		Policy:     "spatial",
		Order:      ids,
		LastRaised: last,
		LastUsedMS: 1000,
		StartedMS:  900,
		Frames:     frames(ws...),
	}
}

func TestDecide_StaleFocusedReuseRaisesC(t *testing.T) {
	// eligible [A,B,C], LastRaised=B, focused/current=A (stale), Main/signal=B
	// next is Reuse, raise C. Must not Fresh.
	a, b, c := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10), win(3, 40, 0, 10, 10)
	ws := []types.Window{a, b, c}
	snap := baseSnap(ws, "w:2")
	in := baseInput(ws, a.ID, b.ID)

	got := Decide(snap, in)
	if got.NoOp {
		t.Fatal("NoOp = true, want false")
	}
	if got.Action != ActionReuse {
		t.Fatalf("Action = %q, want reuse (stale focused must not Fresh)", got.Action)
	}
	if got.RaiseID != "w:3" {
		t.Fatalf("RaiseID = %q, want w:3", got.RaiseID)
	}
	if !reflect.DeepEqual(got.Order, []string{"w:1", "w:2", "w:3"}) {
		t.Fatalf("Order = %v, want [w:1 w:2 w:3]", got.Order)
	}
	if got.Snapshot.LastRaised != "w:3" {
		t.Fatalf("Snapshot.LastRaised = %q, want w:3 (this invocation's raise)", got.Snapshot.LastRaised)
	}
}

func TestDecide_StaleFocusedPrevFromLastRaised(t *testing.T) {
	// If Reuse used Current (A) instead of LastRaised (B), prev would wrap to C.
	a, b, c := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10), win(3, 40, 0, 10, 10)
	ws := []types.Window{a, b, c}
	in := baseInput(ws, a.ID, b.ID)
	in.Direction = -1
	got := Decide(baseSnap(ws, "w:2"), in)
	if got.Action != ActionReuse {
		t.Fatalf("Action = %q, want reuse", got.Action)
	}
	if got.RaiseID != "w:1" {
		t.Fatalf("RaiseID = %q, want w:1 (prev from LastRaised B, not from stale A)", got.RaiseID)
	}
}

func TestDecide_Row6RealClickFresh(t *testing.T) {
	// current=C, signal=C, LastRaised=B → Fresh
	a, b, c := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10), win(3, 40, 0, 10, 10)
	ws := []types.Window{a, b, c}
	in := baseInput(ws, c.ID, c.ID)
	got := Decide(baseSnap(ws, "w:2"), in)
	if got.Action != ActionFresh {
		t.Fatalf("Action = %q, want fresh (real click)", got.Action)
	}
	// Fresh steps from current C → next wraps to A
	if got.RaiseID != "w:1" {
		t.Fatalf("RaiseID = %q, want w:1 (next of current C)", got.RaiseID)
	}
	if got.Snapshot.StartedMS != in.NowMS {
		t.Fatalf("StartedMS = %d, want NowMS %d on Fresh", got.Snapshot.StartedMS, in.NowMS)
	}
}

func TestDecide_Wrap(t *testing.T) {
	a, b, c := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10), win(3, 40, 0, 10, 10)
	ws := []types.Window{a, b, c}
	snap := baseSnap(ws, "w:3")

	t.Run("next wrap", func(t *testing.T) {
		in := baseInput(ws, c.ID, c.ID)
		in.Current = cg(3)
		// current == LastRaised == signal → Reuse
		got := Decide(snap, in)
		if got.Action != ActionReuse {
			t.Fatalf("Action = %q, want reuse", got.Action)
		}
		if got.RaiseID != "w:1" {
			t.Fatalf("RaiseID = %q, want w:1", got.RaiseID)
		}
	})
	t.Run("prev wrap", func(t *testing.T) {
		s := baseSnap(ws, "w:1")
		in := baseInput(ws, a.ID, a.ID)
		in.Direction = -1
		got := Decide(s, in)
		if got.Action != ActionReuse {
			t.Fatalf("Action = %q, want reuse", got.Action)
		}
		if got.RaiseID != "w:3" {
			t.Fatalf("RaiseID = %q, want w:3", got.RaiseID)
		}
	})
	t.Run("next no wrap stays last", func(t *testing.T) {
		in := baseInput(ws, c.ID, c.ID)
		in.Wrap = false
		got := Decide(snap, in)
		if got.RaiseID != "w:3" {
			t.Fatalf("RaiseID = %q, want w:3", got.RaiseID)
		}
	})
	t.Run("prev no wrap stays first", func(t *testing.T) {
		s := baseSnap(ws, "w:1")
		in := baseInput(ws, a.ID, a.ID)
		in.Wrap = false
		in.Direction = -1
		got := Decide(s, in)
		if got.RaiseID != "w:1" {
			t.Fatalf("RaiseID = %q, want w:1", got.RaiseID)
		}
	})
}

func TestDecide_EmptyAndVersionFresh(t *testing.T) {
	a, b := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10)
	ws := []types.Window{a, b}
	in := baseInput(ws, a.ID, a.ID)

	t.Run("empty snap", func(t *testing.T) {
		got := Decide(Snapshot{}, in)
		if got.Action != ActionFresh {
			t.Fatalf("Action = %q, want fresh", got.Action)
		}
		if !reflect.DeepEqual(got.Order, []string{"w:1", "w:2"}) {
			t.Fatalf("Order = %v (Eligible slice order)", got.Order)
		}
		if got.RaiseID != "w:2" {
			t.Fatalf("RaiseID = %q, want w:2 (next of current A)", got.RaiseID)
		}
	})
	t.Run("version != 1", func(t *testing.T) {
		snap := baseSnap(ws, "w:1")
		snap.Version = 2
		got := Decide(snap, in)
		if got.Action != ActionFresh {
			t.Fatalf("Action = %q, want fresh", got.Action)
		}
	})
}

func TestDecide_StickyPIDPolicy(t *testing.T) {
	a, b := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10)
	ws := []types.Window{a, b}
	snap := baseSnap(ws, "w:1")

	t.Run("sticky equal is not expired", func(t *testing.T) {
		in := baseInput(ws, a.ID, a.ID)
		in.NowMS = snap.LastUsedMS + int64(in.StickyMS) // 1000+2000=3000; delta==sticky, not >
		got := Decide(snap, in)
		if got.Action != ActionReuse {
			t.Fatalf("Action = %q, want reuse (delta == sticky_ms is not >)", got.Action)
		}
	})
	t.Run("sticky expired", func(t *testing.T) {
		in := baseInput(ws, a.ID, a.ID)
		in.NowMS = snap.LastUsedMS + int64(in.StickyMS) + 1
		got := Decide(snap, in)
		if got.Action != ActionFresh {
			t.Fatalf("Action = %q, want fresh", got.Action)
		}
	})
	t.Run("pid mismatch", func(t *testing.T) {
		in := baseInput(ws, a.ID, a.ID)
		in.PID = 99
		got := Decide(snap, in)
		if got.Action != ActionFresh {
			t.Fatalf("Action = %q, want fresh", got.Action)
		}
	})
	t.Run("policy mismatch", func(t *testing.T) {
		in := baseInput(ws, a.ID, a.ID)
		in.Policy = "z-order"
		got := Decide(snap, in)
		if got.Action != ActionFresh {
			t.Fatalf("Action = %q, want fresh", got.Action)
		}
	})
}

func TestDecide_Row5Membership(t *testing.T) {
	a, b, c := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10), win(3, 40, 0, 10, 10)
	ws := []types.Window{a, b, c}
	snap := baseSnap([]types.Window{a, b}, "w:2") // Order [A,B], current C not in Order

	t.Run("merge", func(t *testing.T) {
		in := baseInput(ws, c.ID, c.ID)
		got := Decide(snap, in)
		if got.Action != ActionMerge {
			t.Fatalf("Action = %q, want merge", got.Action)
		}
		if !reflect.DeepEqual(got.Order, []string{"w:1", "w:2", "w:3"}) {
			t.Fatalf("Order = %v, want append new C", got.Order)
		}
		// LastRaised B still in order → next of B → C
		if got.RaiseID != "w:3" {
			t.Fatalf("RaiseID = %q, want w:3", got.RaiseID)
		}
	})
	t.Run("rebuild", func(t *testing.T) {
		in := baseInput(ws, c.ID, c.ID)
		in.OnMembershipChange = MembershipRebuild
		got := Decide(snap, in)
		if got.Action != ActionFresh {
			t.Fatalf("Action = %q, want fresh", got.Action)
		}
	})
	t.Run("current ok false", func(t *testing.T) {
		in := baseInput(ws, types.WindowID{}, types.WindowID{})
		in.CurrentOK = false
		in.SignalOK = false
		got := Decide(baseSnap(ws, "w:2"), in)
		if got.Action != ActionMerge {
			t.Fatalf("Action = %q, want merge", got.Action)
		}
	})
}

func TestDecide_Row7OtherMembership(t *testing.T) {
	a, b, c := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10), win(3, 40, 0, 10, 10)
	// current and LastRaised both B, new C opened
	snap := baseSnap([]types.Window{a, b}, "w:2")
	in := baseInput([]types.Window{a, b, c}, b.ID, b.ID)
	got := Decide(snap, in)
	if got.Action != ActionMerge {
		t.Fatalf("Action = %q, want merge", got.Action)
	}
	if !reflect.DeepEqual(got.Order, []string{"w:1", "w:2", "w:3"}) {
		t.Fatalf("Order = %v", got.Order)
	}
}

func TestDecide_MergeLastRaisedGone(t *testing.T) {
	a, c := win(1, 0, 0, 10, 10), win(3, 40, 0, 10, 10)
	// B gone; current A still in Order so row 5 does not fire.
	// signal A != LastRaised B → row 6 Fresh. Use signal still B? B isn't eligible.
	// To hit Merge with LastRaised gone: current in Order, current==LastRaised is
	// impossible if LastRaised is gone. So this is row 6 or row 5.
	// LastRaised gone + current remaining + signal remaining (≠ LastRaised) = row 6 Fresh.
	snap := Snapshot{
		Version: 1, PID: 10, Policy: "spatial",
		Order:      []string{"w:1", "w:2", "w:3"},
		LastRaised: "w:2",
		LastUsedMS: 1000, StartedMS: 900,
		Frames: frames(a, win(2, 20, 0, 10, 10), c),
	}
	in := baseInput([]types.Window{a, c}, a.ID, a.ID)
	got := Decide(snap, in)
	if got.Action != ActionFresh {
		t.Fatalf("Action = %q, want fresh (row 6: signal != LastRaised)", got.Action)
	}

	// LastRaised gone but current not in Order (!CurrentOK) → row 5 Merge.
	in2 := baseInput([]types.Window{a, c}, types.WindowID{}, types.WindowID{})
	in2.CurrentOK = false
	in2.SignalOK = false
	got = Decide(snap, in2)
	if got.Action != ActionMerge {
		t.Fatalf("Action = %q, want merge (row 5)", got.Action)
	}
	if !reflect.DeepEqual(got.Order, []string{"w:1", "w:3"}) {
		t.Fatalf("Order = %v, want dropped B", got.Order)
	}
	// LastRaised B not in new Order → next → [0]
	if got.RaiseID != "w:1" {
		t.Fatalf("RaiseID = %q, want w:1", got.RaiseID)
	}
	in2.Direction = -1
	got = Decide(snap, in2)
	if got.RaiseID != "w:3" {
		t.Fatalf("prev RaiseID = %q, want w:3 (last)", got.RaiseID)
	}
}

func TestDecide_Chebyshev8pt(t *testing.T) {
	a, b := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10)
	ws := []types.Window{a, b}
	snap := baseSnap(ws, "w:1")

	t.Run("dx=8 is not a move", func(t *testing.T) {
		moved := []types.Window{win(1, 8, 0, 10, 10), b}
		in := baseInput(moved, a.ID, a.ID)
		got := Decide(snap, in)
		if got.Action != ActionReuse {
			t.Fatalf("Action = %q, want reuse (dx=8 is not > 8)", got.Action)
		}
	})
	t.Run("dx=9 is a move", func(t *testing.T) {
		moved := []types.Window{win(1, 9, 0, 10, 10), b}
		in := baseInput(moved, a.ID, a.ID)
		got := Decide(snap, in)
		if got.Action != ActionFresh {
			t.Fatalf("Action = %q, want fresh (dx=9 > 8)", got.Action)
		}
	})
	t.Run("missing Frames key is not a move", func(t *testing.T) {
		s := baseSnap(ws, "w:1")
		delete(s.Frames, "w:1")
		moved := []types.Window{win(1, 100, 100, 50, 50), b}
		in := baseInput(moved, a.ID, a.ID)
		got := Decide(s, in)
		if got.Action != ActionReuse {
			t.Fatalf("Action = %q, want reuse (missing Frames)", got.Action)
		}
	})
	t.Run("non-spatial ignores movement", func(t *testing.T) {
		s := baseSnap(ws, "w:1")
		s.Policy = "z-order"
		moved := []types.Window{win(1, 99, 0, 10, 10), b}
		in := baseInput(moved, a.ID, a.ID)
		in.Policy = "z-order"
		got := Decide(s, in)
		if got.Action != ActionReuse {
			t.Fatalf("Action = %q, want reuse", got.Action)
		}
	})
}

func TestDecide_FramesRewritten(t *testing.T) {
	a, b := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10)
	snap := baseSnap([]types.Window{a, b}, "w:1")
	// small nudge (not a Chebyshev move) plus gone extra frame
	snap.Frames["w:99"] = [4]float64{1, 2, 3, 4}
	nudged := []types.Window{win(1, 1, 0, 10, 10), b}
	in := baseInput(nudged, a.ID, a.ID)
	got := Decide(snap, in)
	if got.Action != ActionReuse {
		t.Fatalf("Action = %q, want reuse", got.Action)
	}
	want := map[string][4]float64{
		"w:1": {1, 0, 10, 10},
		"w:2": {20, 0, 10, 10},
	}
	if !reflect.DeepEqual(got.Snapshot.Frames, want) {
		t.Fatalf("Frames = %#v, want %#v (rewritten from this eligible; no omitempty leftovers)", got.Snapshot.Frames, want)
	}
	if got.Snapshot.LastUsedMS != in.NowMS {
		t.Fatalf("LastUsedMS = %d, want %d", got.Snapshot.LastUsedMS, in.NowMS)
	}
	if got.Snapshot.StartedMS != snap.StartedMS {
		t.Fatalf("StartedMS = %d, want kept %d on Reuse", got.Snapshot.StartedMS, snap.StartedMS)
	}
}

func TestDecide_NoOp(t *testing.T) {
	snap := Snapshot{Version: 1, Order: []string{"w:9"}, LastRaised: "w:9"}
	got := Decide(snap, Input{Eligible: nil})
	if !got.NoOp {
		t.Fatal("empty eligible: want NoOp")
	}
	if got.RaiseID != "" {
		t.Fatalf("RaiseID = %q, want empty", got.RaiseID)
	}
	if got.Snapshot.LastRaised != "w:9" {
		t.Fatal("NoOp must not rewrite snapshot")
	}

	got = Decide(snap, Input{Eligible: []types.Window{win(1, 0, 0, 1, 1)}})
	if !got.NoOp {
		t.Fatal("one eligible: want NoOp")
	}
}

func TestDecide_FreshUsesEligibleOrder(t *testing.T) {
	// caller (PR 5) sorts first; Decide must not re-sort
	c, a, b := win(3, 40, 0, 10, 10), win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10)
	ws := []types.Window{c, a, b}
	in := baseInput(ws, a.ID, a.ID)
	got := Decide(Snapshot{}, in)
	if got.Action != ActionFresh {
		t.Fatalf("Action = %q", got.Action)
	}
	if !reflect.DeepEqual(got.Order, []string{"w:3", "w:1", "w:2"}) {
		t.Fatalf("Order = %v, want Eligible slice order", got.Order)
	}
	if got.RaiseID != "w:2" {
		t.Fatalf("RaiseID = %q, want next of current A in that order", got.RaiseID)
	}
}

func TestDecide_FreshNoCurrent(t *testing.T) {
	a, b := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10)
	in := baseInput([]types.Window{a, b}, types.WindowID{}, types.WindowID{})
	in.CurrentOK = false
	in.SignalOK = false
	got := Decide(Snapshot{}, in)
	if got.RaiseID != "w:1" {
		t.Fatalf("next RaiseID = %q, want w:1", got.RaiseID)
	}
	in.Direction = -1
	got = Decide(Snapshot{}, in)
	if got.RaiseID != "w:2" {
		t.Fatalf("prev RaiseID = %q, want w:2", got.RaiseID)
	}
}

func TestInsertSpatially(t *testing.T) {
	a := win(1, 0, 0, 10, 10)
	b := win(2, 100, 0, 10, 10)
	c := win(3, 200, 0, 10, 10)
	got := InsertSpatially([]string{"w:1", "w:3"}, []types.Window{a, b, c})
	if !reflect.DeepEqual(got, []string{"w:1", "w:2", "w:3"}) {
		t.Fatalf("got %v, want [w:1 w:2 w:3]", got)
	}
}

func TestDecide_MergeInsertOrder(t *testing.T) {
	a, b, c := win(1, 0, 0, 10, 10), win(2, 100, 0, 10, 10), win(3, 200, 0, 10, 10)
	snap := baseSnap([]types.Window{a, c}, "w:1")
	in := baseInput([]types.Window{a, b, c}, a.ID, a.ID)
	in.InsertOrder = []string{"w:1", "w:2", "w:3"}
	got := Decide(snap, in)
	if got.Action != ActionMerge {
		t.Fatalf("Action = %q, want merge", got.Action)
	}
	if !reflect.DeepEqual(got.Order, []string{"w:1", "w:2", "w:3"}) {
		t.Fatalf("Order = %v, want spatial insert via InsertOrder", got.Order)
	}
}

func TestDecide_MergeDefaultAppend(t *testing.T) {
	a, b, c := win(1, 0, 0, 10, 10), win(2, 100, 0, 10, 10), win(3, 200, 0, 10, 10)
	snap := baseSnap([]types.Window{a, c}, "w:1")
	in := baseInput([]types.Window{a, c, b}, a.ID, a.ID) // B last in Eligible
	got := Decide(snap, in)
	if got.Action != ActionMerge {
		t.Fatalf("Action = %q", got.Action)
	}
	if !reflect.DeepEqual(got.Order, []string{"w:1", "w:3", "w:2"}) {
		t.Fatalf("Order = %v, want new IDs appended", got.Order)
	}
}
