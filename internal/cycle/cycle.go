package cycle

import (
	"math"
	"sort"

	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

const (
	snapshotVersion        = 1
	chebyshevMoveThreshold = 8
)

// Action is the transition-table result: "fresh" | "merge" | "reuse".
type Action string

const (
	ActionFresh Action = "fresh"
	ActionMerge Action = "merge"
	ActionReuse Action = "reuse"
)

const (
	MembershipMerge   = "merge"
	MembershipRebuild = "rebuild"
)

// Snapshot is the on-disk sticky-cycle state.
// Frames is rewritten from this invocation's eligible set on every write;
// it must not use omitempty (missing key for an ID means "no movement").
type Snapshot struct {
	Version    int                   `json:"version"`
	PID        int32                 `json:"pid"`
	Policy     string                `json:"policy"`
	Order      []string              `json:"order"`
	LastRaised string                `json:"last_raised"`
	LastUsedMS int64                 `json:"last_used_ms"`
	StartedMS  int64                 `json:"started_ms"`
	Frames     map[string][4]float64 `json:"frames"`
}

type Input struct {
	PID                int32
	Policy             string
	StickyMS           int
	Wrap               bool
	OnMembershipChange string // "merge" | "rebuild"
	NowMS              int64
	Eligible           []types.Window // IDs already assigned
	Current            types.WindowID
	CurrentOK          bool
	Signal             types.WindowID
	SignalOK           bool
	Direction          int      // +1 next, -1 prev
	InsertOrder        []string // Merge: placement guide for new IDs; empty appends
}

type Result struct {
	Action   Action
	Order    []string
	RaiseID  string
	Snapshot Snapshot
	// NoOp is set when len(Eligible) <= 1: no raise, caller must not write.
	NoOp bool
}

// Decide applies the sticky-cycle transition table (first match) and
// returns the ID this invocation should raise as LastRaised.
//
//  1. empty snap / Version != 1 → Fresh
//  2. now - LastUsedMS > sticky_ms → Fresh
//  3. PID mismatch → Fresh
//  4. Policy mismatch → Fresh
//  5. !CurrentOK or current ∉ Order → Merge or Fresh per OnMembershipChange
//  6. current ∈ Order, current != LastRaised, SignalOK && signal != LastRaised → Fresh
//     (stale focused: Current=A, LastRaised=B, Signal=B does NOT match this row)
//  7. eligible ID set != Order set → Merge or Fresh per OnMembershipChange
//  8. policy=="spatial" and any surviving ID Chebyshev-moved > 8pt → Fresh
//  9. else Reuse
func Decide(snap Snapshot, in Input) Result {
	if len(in.Eligible) <= 1 {
		return Result{NoOp: true, Snapshot: snap, Order: snap.Order}
	}

	action := classify(snap, in)
	order := orderFor(action, snap, in)
	raise := raiseID(action, order, snap, in)
	return Result{
		Action:   action,
		Order:    order,
		RaiseID:  raise,
		Snapshot: buildSnapshot(action, order, raise, snap, in),
	}
}

func classify(snap Snapshot, in Input) Action {
	if snap.Version != snapshotVersion {
		return ActionFresh
	}
	if in.NowMS-snap.LastUsedMS > int64(in.StickyMS) {
		return ActionFresh
	}
	if snap.PID != in.PID {
		return ActionFresh
	}
	if snap.Policy != in.Policy {
		return ActionFresh
	}

	cur := in.Current.String()
	if !in.CurrentOK || !contains(snap.Order, cur) {
		return membershipAction(in)
	}

	// Row 6: user picked another window (click / ⌘Tab). Stale focused
	// (Current drifted, Main/signal still LastRaised) must not match.
	if cur != snap.LastRaised && in.SignalOK && in.Signal.String() != snap.LastRaised {
		return ActionFresh
	}

	if !sameIDSet(snap.Order, in.Eligible) {
		return membershipAction(in)
	}

	if in.Policy == "spatial" && spatialMoved(snap, in) {
		return ActionFresh
	}
	return ActionReuse
}

func membershipAction(in Input) Action {
	if in.OnMembershipChange == MembershipMerge {
		return ActionMerge
	}
	return ActionFresh
}

func orderFor(action Action, snap Snapshot, in Input) []string {
	switch action {
	case ActionReuse:
		return append([]string{}, snap.Order...)
	case ActionMerge:
		return mergeIDs(snap.Order, in.Eligible, in.InsertOrder)
	default:
		return eligibleIDs(in.Eligible)
	}
}

func raiseID(action Action, order []string, snap Snapshot, in Input) string {
	if len(order) == 0 {
		return ""
	}
	dir := direction(in.Direction)
	switch action {
	case ActionFresh:
		if in.CurrentOK && contains(order, in.Current.String()) {
			return step(order, in.Current.String(), dir, in.Wrap)
		}
		return firstOrLast(order, dir)
	case ActionMerge:
		if contains(order, snap.LastRaised) {
			return step(order, snap.LastRaised, dir, in.Wrap)
		}
		return firstOrLast(order, dir)
	default: // Reuse
		return step(order, snap.LastRaised, dir, in.Wrap)
	}
}

func buildSnapshot(action Action, order []string, raise string, snap Snapshot, in Input) Snapshot {
	started := snap.StartedMS
	if action == ActionFresh {
		started = in.NowMS
	}
	return Snapshot{
		Version:    snapshotVersion,
		PID:        in.PID,
		Policy:     in.Policy,
		Order:      order,
		LastRaised: raise,
		LastUsedMS: in.NowMS,
		StartedMS:  started,
		Frames:     framesFromEligible(in.Eligible),
	}
}

func direction(d int) int {
	if d < 0 {
		return -1
	}
	return 1
}

func firstOrLast(order []string, dir int) string {
	if dir < 0 {
		return order[len(order)-1]
	}
	return order[0]
}

func step(order []string, from string, dir int, wrap bool) string {
	i := indexOf(order, from)
	if i < 0 {
		return firstOrLast(order, dir)
	}
	n := i + dir
	if wrap {
		n %= len(order)
		if n < 0 {
			n += len(order)
		}
		return order[n]
	}
	if n < 0 {
		return order[0]
	}
	if n >= len(order) {
		return order[len(order)-1]
	}
	return order[n]
}

func contains(order []string, id string) bool {
	return indexOf(order, id) >= 0
}

func indexOf(order []string, id string) int {
	for i, s := range order {
		if s == id {
			return i
		}
	}
	return -1
}

func eligibleIDs(eligible []types.Window) []string {
	out := make([]string, 0, len(eligible))
	seen := make(map[string]struct{}, len(eligible))
	for _, w := range eligible {
		id := w.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func sameIDSet(order []string, eligible []types.Window) bool {
	a := make(map[string]struct{}, len(order))
	for _, id := range order {
		a[id] = struct{}{}
	}
	b := make(map[string]struct{}, len(eligible))
	for _, w := range eligible {
		b[w.ID.String()] = struct{}{}
	}
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			return false
		}
	}
	return true
}

func framesFromEligible(eligible []types.Window) map[string][4]float64 {
	m := make(map[string][4]float64, len(eligible))
	for _, w := range eligible {
		m[w.ID.String()] = [4]float64{w.Frame.X, w.Frame.Y, w.Frame.W, w.Frame.H}
	}
	return m
}

func spatialMoved(snap Snapshot, in Input) bool {
	elig := make(map[string]types.Window, len(in.Eligible))
	for _, w := range in.Eligible {
		elig[w.ID.String()] = w
	}
	for _, id := range snap.Order {
		w, ok := elig[id]
		if !ok {
			continue
		}
		old, ok := snap.Frames[id]
		if !ok {
			continue // missing Frames key → not a move
		}
		dx := math.Abs(w.Frame.X - old[0])
		dy := math.Abs(w.Frame.Y - old[1])
		dw := math.Abs(w.Frame.W - old[2])
		dh := math.Abs(w.Frame.H - old[3])
		if max(dx, dy, dw, dh) > chebyshevMoveThreshold {
			return true
		}
	}
	return false
}

// InsertSpatially drops IDs gone from eligible and inserts brand-new IDs at
// spatial positions (X, then Y, then ID.String()). Surviving IDs keep order.
func InsertSpatially(order []string, eligible []types.Window) []string {
	sorted := append([]types.Window(nil), eligible...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return spatialLess(sorted[i], sorted[j])
	})
	return mergeIDs(order, eligible, eligibleIDs(sorted))
}

func spatialLess(a, b types.Window) bool {
	if a.Frame.X != b.Frame.X {
		return a.Frame.X < b.Frame.X
	}
	if a.Frame.Y != b.Frame.Y {
		return a.Frame.Y < b.Frame.Y
	}
	return a.ID.String() < b.ID.String()
}

// mergeIDs keeps surviving IDs in snapshot order and inserts brand-new IDs.
// If insertOrder is empty, new IDs are appended in Eligible order.
// Otherwise each new ID is placed immediately before the next surviving ID
// that follows it in insertOrder (or appended).
func mergeIDs(order []string, eligible []types.Window, insertOrder []string) []string {
	elig := make(map[string]struct{}, len(eligible))
	eligList := make([]string, 0, len(eligible))
	seenEl := make(map[string]struct{}, len(eligible))
	for _, w := range eligible {
		id := w.ID.String()
		elig[id] = struct{}{}
		if _, ok := seenEl[id]; ok {
			continue
		}
		seenEl[id] = struct{}{}
		eligList = append(eligList, id)
	}

	surviving := make([]string, 0, len(order))
	survSet := make(map[string]struct{}, len(order))
	for _, id := range order {
		if _, ok := elig[id]; !ok {
			continue
		}
		if _, dup := survSet[id]; dup {
			continue
		}
		survSet[id] = struct{}{}
		surviving = append(surviving, id)
	}

	newIDs := make([]string, 0)
	newSet := make(map[string]struct{})
	if len(insertOrder) > 0 {
		for _, id := range insertOrder {
			if _, ok := elig[id]; !ok {
				continue
			}
			if _, ok := survSet[id]; ok {
				continue
			}
			if _, ok := newSet[id]; ok {
				continue
			}
			newSet[id] = struct{}{}
			newIDs = append(newIDs, id)
		}
	}
	for _, id := range eligList {
		if _, ok := survSet[id]; ok {
			continue
		}
		if _, ok := newSet[id]; ok {
			continue
		}
		newSet[id] = struct{}{}
		newIDs = append(newIDs, id)
	}

	if len(insertOrder) == 0 {
		return append(append([]string{}, surviving...), newIDs...)
	}

	guidePos := make(map[string]int, len(insertOrder))
	for i, id := range insertOrder {
		if _, ok := guidePos[id]; !ok {
			guidePos[id] = i
		}
	}
	out := append([]string{}, surviving...)
	for _, nid := range newIDs {
		pos, ok := guidePos[nid]
		idx := len(out)
		if ok {
			for i, id := range out {
				gp, exists := guidePos[id]
				if exists && gp > pos {
					idx = i
					break
				}
			}
		}
		out = append(out, "")
		copy(out[idx+1:], out[idx:])
		out[idx] = nid
	}
	return out
}
