package statefile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"nasmon/internal/model"
)

const Version = 1

type stateFile struct {
	Version   int            `json:"version"`
	WrittenAt time.Time      `json:"written_at"`
	Snapshot  model.Snapshot `json:"snapshot"`
}

type State struct {
	WrittenAt time.Time
	Snapshot  model.Snapshot
}

func ensureDir(path string) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// AcquireWriterLock prevents accidentally running more than one collector for
// the same state directory. The lock is released automatically when the file is
// closed or the process exits.
func AcquireWriterLock(path string) (*os.File, error) {
	dir, err := ensureDir(path)
	if err != nil {
		return nil, err
	}
	lockPath := filepath.Join(dir, "nasmond.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another nasmond is already using %s: %w", dir, err)
	}
	return f, nil
}

// WriteAtomic writes a complete JSON snapshot to a temporary file in the same
// directory and atomically replaces the public state file with rename(2).
func WriteAtomic(path string, snapshot model.Snapshot) error {
	dir, err := ensureDir(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(stateFile{Version: Version, WrittenAt: time.Now(), Snapshot: snapshot}); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func ReadState(path string) (State, error) {
	f, err := os.Open(path)
	if err != nil {
		return State{}, err
	}
	defer f.Close()

	var state stateFile
	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return State{}, err
	}
	if state.Version != Version {
		return State{}, fmt.Errorf("unsupported state version %d (expected %d)", state.Version, Version)
	}
	return State{WrittenAt: state.WrittenAt, Snapshot: state.Snapshot}, nil
}

func Read(path string) (model.Snapshot, error) {
	state, err := ReadState(path)
	if err != nil {
		return model.Snapshot{}, err
	}
	return state.Snapshot, nil
}
