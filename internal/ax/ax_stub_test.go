//go:build !darwin || !cgo

package ax

import "testing"

func TestStubReturnsError(t *testing.T) {
	if _, err := Trusted(false); err == nil {
		t.Fatal("Trusted: expected error")
	}
	if _, err := Open(250); err == nil {
		t.Fatal("Open: expected error")
	}
	s := &Session{}
	if _, err := s.FocusedPID(); err == nil {
		t.Fatal("FocusedPID: expected error")
	}
	if _, err := s.Windows(1); err == nil {
		t.Fatal("Windows: expected error")
	}
	if _, _, err := s.FocusedWindowID(1); err == nil {
		t.Fatal("FocusedWindowID: expected error")
	}
	if err := s.RaiseIndex(0, false, true, true); err == nil {
		t.Fatal("RaiseIndex: expected error")
	}
	s.Close()
}
