package policy

import (
	"fmt"
	"sort"
	"time"

	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

type Context struct {
	Current   types.WindowID
	CurrentOK bool
	Now       time.Time
	LastOrder []types.WindowID // always pass snapshot.Order; nil if none. mru uses it
}

type SortPolicy interface {
	Name() string
	Sort(windows []types.Window, ctx Context) []types.Window
}

func Lookup(name string) (SortPolicy, error) {
	switch name {
	case "spatial":
		return spatial{}, nil
	case "window-id":
		return windowIDPolicy{}, nil
	case "z-order":
		return zOrder{}, nil
	case "mru":
		return mruApprox{}, nil
	default:
		return nil, fmt.Errorf("unknown sort policy: %q", name)
	}
}

func copyWindows(windows []types.Window) []types.Window {
	out := make([]types.Window, len(windows))
	copy(out, windows)
	return out
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

type spatial struct{}

func (spatial) Name() string { return "spatial" }

func (spatial) Sort(windows []types.Window, _ Context) []types.Window {
	out := copyWindows(windows)
	sort.Slice(out, func(i, j int) bool { return spatialLess(out[i], out[j]) })
	return out
}

type windowIDPolicy struct{}

func (windowIDPolicy) Name() string { return "window-id" }

func (windowIDPolicy) Sort(windows []types.Window, _ Context) []types.Window {
	out := copyWindows(windows)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].ID, out[j].ID
		aCG := a.Kind == types.IDCGWindow
		bCG := b.Kind == types.IDCGWindow
		if aCG != bCG {
			return aCG
		}
		if aCG {
			return a.CG < b.CG
		}
		return a.Key < b.Key
	})
	return out
}

type zOrder struct{}

func (zOrder) Name() string { return "z-order" }

func (zOrder) Sort(windows []types.Window, _ Context) []types.Window {
	out := copyWindows(windows)
	sort.Slice(out, func(i, j int) bool { return out[i].AXIndex < out[j].AXIndex })
	return out
}

type mruApprox struct{}

func (mruApprox) Name() string { return "mru" }

func (mruApprox) Sort(windows []types.Window, ctx Context) []types.Window {
	byID := make(map[types.WindowID]types.Window, len(windows))
	for _, w := range windows {
		byID[w.ID] = w
	}

	used := make(map[types.WindowID]bool, len(windows))
	out := make([]types.Window, 0, len(windows))

	if ctx.CurrentOK {
		if w, ok := byID[ctx.Current]; ok {
			out = append(out, w)
			used[ctx.Current] = true
		}
	}

	remaining := make([]types.Window, 0, len(windows))
	for _, id := range ctx.LastOrder {
		if used[id] {
			continue
		}
		w, ok := byID[id]
		if !ok {
			continue
		}
		remaining = append(remaining, w)
		used[id] = true
	}

	newcomers := make([]types.Window, 0, len(windows))
	for _, w := range windows {
		if used[w.ID] {
			continue
		}
		newcomers = append(newcomers, w)
		used[w.ID] = true
	}
	sort.Slice(newcomers, func(i, j int) bool { return spatialLess(newcomers[i], newcomers[j]) })
	for _, w := range newcomers {
		remaining = insertSpatial(remaining, w)
	}

	return append(out, remaining...)
}

func insertSpatial(dst []types.Window, w types.Window) []types.Window {
	i := 0
	for ; i < len(dst); i++ {
		if spatialLess(w, dst[i]) {
			break
		}
	}
	dst = append(dst, types.Window{})
	copy(dst[i+1:], dst[i:])
	dst[i] = w
	return dst
}
