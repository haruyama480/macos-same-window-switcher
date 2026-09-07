//go:build !darwin || !cgo

// Stub used on non-darwin and on darwin with CGO_ENABLED=0.

package ax

import (
	"errors"

	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

var errUnavailable = errors.New("accessibility APIs unavailable")

type Session struct{}

func Trusted(prompt bool) (bool, error) {
	return false, errUnavailable
}

func Open(timeoutMS int) (*Session, error) {
	return nil, errUnavailable
}

func (s *Session) Close() {}

func (s *Session) FocusedPID() (int32, error) {
	return 0, errUnavailable
}

func (s *Session) Windows(pid int32) ([]types.Window, error) {
	return nil, errUnavailable
}

func (s *Session) FocusedWindowID(pid int32) (types.WindowID, types.Window, error) {
	return types.WindowID{}, types.Window{}, errUnavailable
}

func (s *Session) RaiseIndex(index int, unminimize, doRaise, setMain bool) error {
	return errUnavailable
}
