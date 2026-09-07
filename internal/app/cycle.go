package app

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/haruyama480/macos-same-window-switcher/internal/ax"
	"github.com/haruyama480/macos-same-window-switcher/internal/config"
	"github.com/haruyama480/macos-same-window-switcher/internal/cycle"
	"github.com/haruyama480/macos-same-window-switcher/internal/filter"
	"github.com/haruyama480/macos-same-window-switcher/internal/policy"
	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

// CycleFlags are the next/prev CLI flags. Config is --config; Policy overrides TOML.
type CycleFlags struct {
	Config  string
	Policy  string
	Verbose bool
	DryRun  bool
}

type cyclePlan struct {
	Result  cycle.Result
	Policy  string
	CopyIdx int
	Window  types.Window
}

// Cycle raises the next (direction >= 0) or previous (direction < 0) eligible
// window of the focused app. Success is silent unless Verbose, DryRun, or
// SAME_WINDOW_SWITCHER_DEBUG=1. Dry-run selects but does not write or raise.
func Cycle(stdout, stderr io.Writer, direction int, flags CycleFlags) int {
	_ = stdout
	start := time.Now()
	cfg, _, err := config.Load(flags.Config)
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}
	verbose := flags.Verbose || os.Getenv("SAME_WINDOW_SWITCHER_DEBUG") == "1"

	trusted, err := ax.Trusted(false)
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}
	if !trusted {
		writeUntrusted(stderr, executablePath())
		return exitAccessibility
	}

	sess, err := ax.Open(timeoutMS(cfg))
	if err != nil {
		writeRuntime(stderr, err)
		return exitRuntime
	}
	defer sess.Close()

	pid, err := sess.FocusedPID()
	if err != nil {
		writeRuntime(stderr, err)
		return exitFocusedPID(err)
	}

	dir := cycle.DefaultDir()
	if cfg.State.Dir != "" {
		dir = cfg.State.Dir
	}
	store := &cycle.Store{Dir: dir}

	var plan cyclePlan
	var raiseAttempted bool
	err = store.WithLock(func() error {
		// Windows/focus/nowMS must be sampled after acquiring the lock so a
		// key-repeat waiter sees Main from the previous raise, not a stale copy.
		nowMS := time.Now().UnixMilli()
		windows, err := sess.Windows(pid)
		if err != nil {
			return err
		}
		eligible, _ := filter.Eligible(windows, cfg.Filter)
		if len(eligible) <= 1 {
			plan = cyclePlan{Result: cycle.Result{NoOp: true}}
			return nil
		}
		pol, err := resolvePolicy(flags.Policy, cfg)
		if err != nil {
			return err
		}
		focused := focusedID(sess, pid, windows)
		plan, err = decideAndMaybeWrite(store, cfg, pol, pid, windows, eligible, focused, nowMS, direction, flags.DryRun)
		if err != nil {
			return err
		}
		if plan.Result.NoOp || flags.DryRun {
			return nil
		}
		// C raise_index already retries kAXErrorCannotComplete once; do not double.
		raiseAttempted = true
		return sess.RaiseIndex(plan.CopyIdx, raiseUnminimize(cfg), cfg.Focus.Raise, cfg.Focus.SetMain)
	})
	if err != nil {
		if raiseAttempted {
			writeRaiseError(stderr, err, plan.Window)
		} else {
			writeRuntime(stderr, err)
		}
		return exitRuntime
	}
	if plan.Result.NoOp {
		return exitSuccess
	}
	if flags.DryRun || verbose {
		idx := indexOfOrder(plan.Result.Order, plan.Result.RaiseID)
		fmt.Fprintln(stderr, formatCycleLine(pid, plan.Policy, string(plan.Result.Action), plan.Result.RaiseID, idx, len(plan.Result.Order), time.Since(start)))
	}
	return exitSuccess
}

// Minimized eligible windows cannot be raised until restored.
func raiseUnminimize(cfg config.Config) bool {
	return cfg.Focus.Unminimize || cfg.Filter.IncludeMinimized
}

func resolvePolicy(override string, cfg config.Config) (policy.SortPolicy, error) {
	name := cfg.Cycle.Policy
	if override != "" {
		name = override
	}
	if name == "" {
		name = "spatial"
	}
	return policy.Lookup(name)
}

func focusedID(sess *ax.Session, pid int32, windows []types.Window) types.WindowID {
	_, meta, err := sess.FocusedWindowID(pid)
	if err != nil {
		return types.WindowID{}
	}
	if w, ok := types.IdentifyFocused(meta, windows); ok {
		return w.ID
	}
	return types.WindowID{}
}

// decideAndMaybeWrite is the lock body minus raise: Read → Sort → Decide → Write
// (skipped on dry-run so LastRaised cannot desync from AX Main).
func decideAndMaybeWrite(store *cycle.Store, cfg config.Config, pol policy.SortPolicy, pid int32, all, eligible []types.Window, focused types.WindowID, nowMS int64, direction int, dryRun bool) (cyclePlan, error) {
	snap, err := store.Read()
	if err != nil {
		return cyclePlan{}, err
	}
	current, currentOK := types.CurrentWindow(focused, eligible)
	signal, signalOK := types.RaiseSignal(eligible)
	sorted := pol.Sort(eligible, policy.Context{
		Current:   current,
		CurrentOK: currentOK,
		Now:       time.UnixMilli(nowMS),
		LastOrder: parseLastOrder(snap.Order),
	})
	res := cycle.Decide(snap, cycle.Input{
		PID:                pid,
		Policy:             pol.Name(),
		StickyMS:           cfg.Cycle.StickyMS,
		Wrap:               cfg.Cycle.Wrap,
		OnMembershipChange: cfg.Cycle.OnMembershipChange,
		NowMS:              nowMS,
		Eligible:           sorted,
		Current:            current,
		CurrentOK:          currentOK,
		Signal:             signal,
		SignalOK:           signalOK,
		Direction:          direction,
		InsertOrder:        mergeInsertOrder(pol.Name(), snap.Order, sorted),
	})
	plan := cyclePlan{Result: res, Policy: pol.Name(), CopyIdx: -1}
	if res.NoOp {
		return plan, nil
	}
	idx, w, ok := copyIndex(all, res.RaiseID)
	if !ok {
		return plan, fmt.Errorf("raise id %s not in window list", res.RaiseID)
	}
	plan.CopyIdx = idx
	plan.Window = w
	if dryRun {
		return plan, nil
	}
	if err := store.Write(res.Snapshot); err != nil {
		return plan, err
	}
	return plan, nil
}

// mergeInsertOrder places new IDs on Merge: spatial splices by frame; other
// policies pass empty InsertOrder so mergeIDs appends in Eligible order.
func mergeInsertOrder(policyName string, snapOrder []string, sorted []types.Window) []string {
	if policyName == "spatial" {
		return cycle.InsertSpatially(snapOrder, sorted)
	}
	return nil
}

// copyIndex is the Session.Windows() slice index RaiseIndex expects (last
// copy_windows array index). Do not pass Window.AXIndex.
func copyIndex(all []types.Window, raiseID string) (int, types.Window, bool) {
	for i, w := range all {
		if w.ID.String() == raiseID {
			return i, w, true
		}
	}
	return -1, types.Window{}, false
}

func parseLastOrder(order []string) []types.WindowID {
	out := make([]types.WindowID, 0, len(order))
	for _, s := range order {
		if id, ok := parseWindowID(s); ok {
			out = append(out, id)
		}
	}
	return out
}

func parseWindowID(s string) (types.WindowID, bool) {
	switch {
	case strings.HasPrefix(s, "w:"):
		n, err := strconv.ParseUint(s[2:], 10, 32)
		if err != nil || n == 0 {
			return types.WindowID{}, false
		}
		return types.WindowID{Kind: types.IDCGWindow, CG: uint32(n)}, true
	case strings.HasPrefix(s, "f:"):
		key := s[2:]
		if key == "" {
			return types.WindowID{}, false
		}
		return types.WindowID{Kind: types.IDFallback, Key: key}, true
	default:
		return types.WindowID{}, false
	}
}

func indexOfOrder(order []string, id string) int {
	for i, s := range order {
		if s == id {
			return i
		}
	}
	return -1
}

func formatCycleLine(pid int32, policyName, action, id string, index, n int, elapsed time.Duration) string {
	return fmt.Sprintf("pid=%d policy=%s action=%s index=%d/%d id=%s elapsed=%dms",
		pid, policyName, action, index, n, id, elapsed.Milliseconds())
}

func writeRaiseError(stderr io.Writer, err error, w types.Window) {
	fmt.Fprintf(stderr, "same-window-switcher: raise %q: %v\n", sanitizeTitle(w.Title), err)
}
