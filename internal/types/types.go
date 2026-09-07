package types

import (
	"fmt"
	"math"
)

type IDKind int

const (
	IDCGWindow IDKind = iota // "_AXUIElementGetWindow" が 0 以外を返した
	IDFallback               // それ以外。CGWindowID 空間とは混ぜない
)

type WindowID struct {
	Kind IDKind
	CG   uint32 // Kind==IDCGWindow のときのみ。0 は無効
	Key  string // Kind==IDFallback のとき必須
}

func (id WindowID) String() string {
	if id.Kind == IDCGWindow {
		return fmt.Sprintf("w:%d", id.CG)
	}
	return "f:" + id.Key
}

func (id WindowID) valid() bool {
	switch id.Kind {
	case IDCGWindow:
		return id.CG != 0
	case IDFallback:
		return id.Key != ""
	default:
		return false
	}
}

type Rectangle struct{ X, Y, W, H float64 }

type Window struct {
	ID         WindowID
	PID        int32
	Role       string
	Subrole    string
	Title      string
	Frame      Rectangle // global, top-left origin (AX)
	Minimized  bool
	Fullscreen bool // 私有 AXFullScreen。無ければ false
	Main       bool
	Focused    bool
	AXIndex    int // kAXWindowsAttribute における元の添字（z-order / raise）
}

func fallbackKey(w Window) string {
	return fmt.Sprintf("%d|%s|%s|%.0f|%.0f|%.0f|%.0f",
		w.PID, w.Role, w.Subrole,
		math.Round(w.Frame.X), math.Round(w.Frame.Y),
		math.Round(w.Frame.W), math.Round(w.Frame.H))
}

// RawID is this window's identity from its own fields (CG or unsalt fallback).
// It does not apply snapshot collision salting; use AssignIDs / IdentifyFocused for that.
func RawID(w Window) WindowID {
	if w.ID.CG != 0 {
		return WindowID{Kind: IDCGWindow, CG: w.ID.CG}
	}
	return WindowID{Kind: IDFallback, Key: fallbackKey(w)}
}

// IdentifyFocused maps a focused-window read onto a Windows() snapshot so IDs
// (including |t: collision salts) and AXIndex match that slice.
func IdentifyFocused(meta Window, windows []Window) (Window, bool) {
	if meta.ID.CG != 0 {
		for _, w := range windows {
			if w.ID.Kind == IDCGWindow && w.ID.CG == meta.ID.CG {
				return w, true
			}
		}
		return Window{}, false
	}
	key := fallbackKey(meta)
	var hits []Window
	for _, w := range windows {
		if w.ID.Kind == IDCGWindow {
			continue
		}
		if fallbackKey(w) == key {
			hits = append(hits, w)
		}
	}
	switch len(hits) {
	case 0:
		return Window{}, false
	case 1:
		return hits[0], true
	default:
		for _, w := range hits {
			if w.Title == meta.Title {
				return w, true
			}
		}
		return Window{}, false
	}
}

// AssignIDs sets Window.ID from CG (when non-zero) or a fallback key.
// Title is appended as "|t:"+title only for fallback keys that collide in this slice.
func AssignIDs(windows []Window) []Window {
	out := make([]Window, len(windows))
	copy(out, windows)

	counts := make(map[string]int, len(out))
	base := make([]string, len(out))
	for i := range out {
		if out[i].ID.CG != 0 {
			out[i].ID = WindowID{Kind: IDCGWindow, CG: out[i].ID.CG}
			continue
		}
		k := fallbackKey(out[i])
		base[i] = k
		counts[k]++
	}
	for i := range out {
		if out[i].ID.Kind == IDCGWindow && out[i].ID.CG != 0 {
			continue
		}
		k := base[i]
		if counts[k] > 1 {
			k = k + "|t:" + out[i].Title
		}
		out[i].ID = WindowID{Kind: IDFallback, Key: k}
	}
	return out
}

func containsID(windows []Window, id WindowID) bool {
	for _, w := range windows {
		if w.ID == id {
			return true
		}
	}
	return false
}

func minAXIndex(windows []Window) (Window, bool) {
	if len(windows) == 0 {
		return Window{}, false
	}
	best := windows[0]
	for _, w := range windows[1:] {
		if w.AXIndex < best.AXIndex {
			best = w
		}
	}
	return best, true
}

func firstMain(windows []Window) (Window, bool) {
	found := false
	var best Window
	for _, w := range windows {
		if !w.Main {
			continue
		}
		if !found || w.AXIndex < best.AXIndex {
			best = w
			found = true
		}
	}
	return best, found
}

// CurrentWindow picks the current window among eligible only:
// focusedAX if valid and present, else Main (lowest AXIndex), else min AXIndex.
func CurrentWindow(focusedAX WindowID, eligible []Window) (id WindowID, ok bool) {
	if focusedAX.valid() && containsID(eligible, focusedAX) {
		return focusedAX, true
	}
	if w, ok := firstMain(eligible); ok {
		return w.ID, true
	}
	if w, ok := minAXIndex(eligible); ok {
		return w.ID, true
	}
	return WindowID{}, false
}

// RaiseSignal is the window this tool's raise writes (Main), else min AXIndex.
func RaiseSignal(eligible []Window) (id WindowID, ok bool) {
	if w, ok := firstMain(eligible); ok {
		return w.ID, true
	}
	if w, ok := minAXIndex(eligible); ok {
		return w.ID, true
	}
	return WindowID{}, false
}
