package cycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/haruyama480/macos-same-window-switcher/internal/types"
)

func TestDefaultDir(t *testing.T) {
	dir := DefaultDir()
	want := "same-window-switcher-"
	if !strings.Contains(dir, want) {
		t.Fatalf("DefaultDir() = %q, want substring %q", dir, want)
	}
	if !strings.Contains(dir, os.TempDir()) {
		t.Fatalf("DefaultDir() = %q, want under TempDir", dir)
	}
}

func TestStore_MissingAndBrokenJSON(t *testing.T) {
	s := &Store{Dir: t.TempDir()}

	snap, err := s.Read()
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if snap.Version != 0 {
		t.Fatalf("missing file Version = %d, want 0 (empty → Fresh)", snap.Version)
	}

	path := filepath.Join(s.Dir, jsonName)
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err = s.Read()
	if err != nil {
		t.Fatalf("broken JSON should not error: %v", err)
	}
	if snap.Version != 0 || snap.LastRaised != "" {
		t.Fatalf("broken JSON snap = %+v, want empty (Fresh)", snap)
	}

	a, b := win(1, 0, 0, 10, 10), win(2, 20, 0, 10, 10)
	got := Decide(snap, baseInput([]types.Window{a, b}, a.ID, a.ID))
	if got.Action != ActionFresh {
		t.Fatalf("broken JSON Decide Action = %q, want fresh", got.Action)
	}

	if err := os.WriteFile(path, []byte(`{"version":2,"order":["w:1"],"last_raised":"w:1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	snap, err = s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Version != 0 {
		t.Fatalf("Version!=1 must Read as empty, got %+v", snap)
	}
}

func TestStore_WriteReadRoundTripAndFrames(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	a, b := win(1, 1, 2, 3, 4), win(2, 5, 6, 7, 8)
	in := baseInput([]types.Window{a, b}, a.ID, a.ID)
	res := Decide(Snapshot{}, in)

	var locked bool
	if err := s.WithLock(func() error {
		locked = true
		if err := s.Write(res.Snapshot); err != nil {
			return err
		}
		got, err := s.Read()
		if err != nil {
			return err
		}
		if got.Version != 1 {
			t.Errorf("Version = %d", got.Version)
		}
		if got.LastRaised != res.RaiseID {
			t.Errorf("LastRaised = %q, want %q", got.LastRaised, res.RaiseID)
		}
		if got.LastUsedMS != in.NowMS {
			t.Errorf("LastUsedMS = %d", got.LastUsedMS)
		}
		wantFrames := map[string][4]float64{
			"w:1": {1, 2, 3, 4},
			"w:2": {5, 6, 7, 8},
		}
		if !framesEq(got.Frames, wantFrames) {
			t.Errorf("Frames = %#v, want %#v", got.Frames, wantFrames)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !locked {
		t.Fatal("WithLock did not run callback")
	}

	raw, err := os.ReadFile(filepath.Join(s.Dir, jsonName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"frames"`) {
		t.Fatalf("JSON missing frames key (must not omitempty): %s", raw)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if _, ok := probe["frames"]; !ok {
		t.Fatal("frames key absent")
	}

	info, err := os.Stat(filepath.Join(s.Dir, jsonName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("cycle.json perm = %o, want 0600", info.Mode().Perm())
	}
	lockInfo, err := os.Stat(filepath.Join(s.Dir, lockName))
	if err != nil {
		t.Fatal(err)
	}
	if lockInfo.Mode().Perm() != 0600 {
		t.Errorf("cycle.lock perm = %o, want 0600", lockInfo.Mode().Perm())
	}
}

func TestStore_WriteEmptyFramesKey(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.Write(Snapshot{Version: 1, Frames: nil}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Dir, jsonName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"frames"`) {
		t.Fatalf("nil Frames must still be written: %s", raw)
	}
}

func TestStore_FlockSerializesWriters(t *testing.T) {
	s := &Store{Dir: t.TempDir()}

	firstInside := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan struct{})
	secondAcquired := make(chan struct{})

	var secondSaw string
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := s.WithLock(func() error {
			close(firstInside)
			<-releaseFirst
			return s.Write(Snapshot{Version: 1, LastRaised: "first", Frames: map[string][4]float64{}})
		})
		if err != nil {
			t.Errorf("first writer: %v", err)
		}
		close(firstDone)
	}()

	<-firstInside

	go func() {
		defer wg.Done()
		err := s.WithLock(func() error {
			close(secondAcquired)
			snap, err := s.Read()
			if err != nil {
				return err
			}
			secondSaw = snap.LastRaised
			return s.Write(Snapshot{Version: 1, LastRaised: "second", Frames: map[string][4]float64{}})
		})
		if err != nil {
			t.Errorf("second writer: %v", err)
		}
	}()

	select {
	case <-secondAcquired:
		t.Fatal("second writer acquired lock while first still holds it")
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseFirst)
	select {
	case <-secondAcquired:
	case <-time.After(2 * time.Second):
		t.Fatal("second writer did not acquire lock after first released")
	}
	wg.Wait()

	if secondSaw != "first" {
		t.Fatalf("second writer saw LastRaised=%q, want first (must wait for unlock+write)", secondSaw)
	}
	snap, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if snap.LastRaised != "second" {
		t.Fatalf("final LastRaised = %q, want second", snap.LastRaised)
	}
}

func framesEq(a, b map[string][4]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va != vb {
			return false
		}
	}
	return true
}
