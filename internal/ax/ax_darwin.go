//go:build darwin && cgo

package ax

/*
#cgo darwin LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include "ax_darwin.h"
*/
import "C"

import (
	"runtime"
	"unsafe"

	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

type Session struct {
	s      *C.ax_session_t
	locked bool
}

func Trusted(prompt bool) (bool, error) {
	var trusted C.bool
	ret := C.ax_is_trusted(C.bool(prompt), &trusted)
	if ret != 0 {
		return false, Error{Op: "is_trusted", Code: int(ret)}
	}
	return bool(trusted), nil
}

func Open(timeoutMS int) (*Session, error) {
	runtime.LockOSThread()
	var s *C.ax_session_t
	ret := C.ax_session_open(C.int(timeoutMS), &s)
	if ret != 0 {
		runtime.UnlockOSThread()
		return nil, Error{Op: "session_open", Code: int(ret)}
	}
	return &Session{s: s, locked: true}, nil
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.s != nil {
		C.ax_session_close(s.s)
		s.s = nil
	}
	if s.locked {
		runtime.UnlockOSThread()
		s.locked = false
	}
}

func (s *Session) closed() error {
	if s == nil || s.s == nil {
		return Error{Op: "session", Code: -25201}
	}
	return nil
}

func (s *Session) FocusedPID() (int32, error) {
	if err := s.closed(); err != nil {
		return 0, err
	}
	var pid C.int32_t
	ret := C.ax_session_focused_pid(s.s, &pid)
	if ret != 0 {
		return 0, Error{Op: "focused_pid", Code: int(ret)}
	}
	return int32(pid), nil
}

func goWindow(w *C.ax_window_t) types.Window {
	if w == nil {
		return types.Window{}
	}
	return types.Window{
		ID:         types.WindowID{CG: uint32(w.cg_window_id)},
		PID:        int32(w.pid),
		Role:       C.GoString(&w.role[0]),
		Subrole:    C.GoString(&w.subrole[0]),
		Title:      C.GoString(&w.title[0]),
		Frame:      types.Rectangle{X: float64(w.x), Y: float64(w.y), W: float64(w.w), H: float64(w.h)},
		Minimized:  bool(w.minimized),
		Fullscreen: bool(w.fullscreen),
		Main:       bool(w.main),
		Focused:    bool(w.focused),
		AXIndex:    int(w.ax_index),
	}
}

func (s *Session) Windows(pid int32) ([]types.Window, error) {
	if err := s.closed(); err != nil {
		return nil, err
	}
	var out *C.ax_window_t
	var n C.int
	ret := C.ax_session_copy_windows(s.s, C.int32_t(pid), &out, &n)
	if ret != 0 {
		return nil, Error{Op: "copy_windows", Code: int(ret)}
	}
	defer C.ax_free_windows(out)
	if n <= 0 || out == nil {
		return []types.Window{}, nil
	}
	slice := unsafe.Slice(out, int(n))
	windows := make([]types.Window, int(n))
	for i := range slice {
		windows[i] = goWindow(&slice[i])
	}
	return types.AssignIDs(windows), nil
}

// FocusedWindowID returns the focused window's fields with RawID (CG or unsalt
// fallback). Callers with a Windows() snapshot should use types.IdentifyFocused
// so collision salts and AXIndex match that slice. The raw AXIndex is not a raise index.
func (s *Session) FocusedWindowID(pid int32) (types.WindowID, types.Window, error) {
	if err := s.closed(); err != nil {
		return types.WindowID{}, types.Window{}, err
	}
	var cg C.uint32_t
	var meta C.ax_window_t
	ret := C.ax_session_focused_window_id(s.s, C.int32_t(pid), &cg, &meta)
	if ret != 0 {
		return types.WindowID{}, types.Window{}, Error{Op: "focused_window_id", Code: int(ret)}
	}
	if cg == 0 && meta.role[0] == 0 && meta.subrole[0] == 0 && meta.title[0] == 0 &&
		meta.x == 0 && meta.y == 0 && meta.w == 0 && meta.h == 0 {
		return types.WindowID{}, types.Window{}, nil
	}
	w := goWindow(&meta)
	// RawID only: collision salts and raise AXIndex come from IdentifyFocused(Windows()).
	w.ID = types.RawID(w)
	return w.ID, w, nil
}

func (s *Session) RaiseIndex(index int, unminimize, doRaise, setMain bool) error {
	if err := s.closed(); err != nil {
		return err
	}
	ret := C.ax_session_raise_index(s.s, C.int(index), C.bool(unminimize), C.bool(doRaise), C.bool(setMain))
	if ret != 0 {
		return Error{Op: "raise_index", Code: int(ret)}
	}
	return nil
}
