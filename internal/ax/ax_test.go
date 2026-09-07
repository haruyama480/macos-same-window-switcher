package ax

import (
	"errors"
	"testing"
)

func TestErrorString(t *testing.T) {
	e := Error{Op: "copy_windows", Code: -25200}
	got := e.Error()
	want := "copy_windows: AXError -25200"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestIsNoValue(t *testing.T) {
	if !IsNoValue(Error{Op: "focused_pid", Code: CodeNoValue}) {
		t.Fatal("expected NoValue")
	}
	if IsNoValue(Error{Op: "focused_pid", Code: -25204}) {
		t.Fatal("CannotComplete must not be NoValue")
	}
	if IsNoValue(Error{Op: "session_open", Code: -25200}) {
		t.Fatal("Failure must not be NoValue")
	}
	if IsNoValue(errors.New("accessibility APIs unavailable")) {
		t.Fatal("plain error must not be NoValue")
	}
	if IsNoValue(nil) {
		t.Fatal("nil must not be NoValue")
	}
}
