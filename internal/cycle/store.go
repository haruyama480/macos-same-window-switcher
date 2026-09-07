package cycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	lockName = "cycle.lock"
	jsonName = "cycle.json"
	tmpName  = "cycle.json.tmp"
	filePerm = 0600
	dirPerm  = 0700
)

// Store is the $TMPDIR sticky-cycle snapshot. flock is on cycle.lock only;
// cycle.json is replaced by tmp+rename (inode changes, so it is not flocked).
type Store struct {
	Dir string
}

// DefaultDir is $TMPDIR/same-window-switcher-$UID.
func DefaultDir() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("same-window-switcher-%d", os.Getuid()))
}

func (s *Store) lockPath() string { return filepath.Join(s.Dir, lockName) }
func (s *Store) jsonPath() string { return filepath.Join(s.Dir, jsonName) }
func (s *Store) tmpPath() string  { return filepath.Join(s.Dir, tmpName) }

func (s *Store) ensureDir() error {
	return os.MkdirAll(s.Dir, dirPerm)
}

// WithLock holds a blocking LOCK_EX on cycle.lock for the duration of fn.
// PR 5 raises the chosen window inside this callback so the next one-shot
// cannot observe LastRaised written ahead of AX.
func (s *Store) WithLock(fn func() error) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}

// Read returns an empty Snapshot (Fresh) when the file is missing, JSON
// does not decode, or Version != 1. Other I/O errors are returned.
func (s *Store) Read() (Snapshot, error) {
	data, err := os.ReadFile(s.jsonPath())
	if err != nil {
		if os.IsNotExist(err) {
			return Snapshot{}, nil
		}
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, nil
	}
	if snap.Version != snapshotVersion {
		return Snapshot{}, nil
	}
	return snap, nil
}

// Write marshals snap to cycle.json.tmp then os.Rename onto cycle.json.
func (s *Store) Write(snap Snapshot) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	if snap.Frames == nil {
		snap.Frames = map[string][4]float64{}
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	tmp := s.tmpPath()
	if err := os.WriteFile(tmp, data, filePerm); err != nil {
		return err
	}
	_ = os.Chmod(tmp, filePerm)
	if err := os.Rename(tmp, s.jsonPath()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Chmod(s.jsonPath(), filePerm)
	return nil
}
