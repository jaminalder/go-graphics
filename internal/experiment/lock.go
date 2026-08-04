package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

const lockRetryInterval = 20 * time.Millisecond

// LockOwner identifies the process that acquired a lifecycle lock.
type LockOwner struct {
	PID        int       `json:"pid"`
	Hostname   string    `json:"hostname"`
	Command    string    `json:"command"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// Lock is an acquired lifecycle lock.
type Lock struct {
	path  string
	owner LockOwner

	mu       sync.Mutex
	released bool
}

// AcquireGlobalLock acquires the lock for repository-wide lifecycle mutations.
func (m *Manager) AcquireGlobalLock(ctx context.Context, command string) (*Lock, error) {
	return acquireLock(ctx, filepath.Join(m.CommonDir, "experiment-locks", "global.lock"), command)
}

// AcquireExperimentLock acquires the lock for mutations to one experiment.
func (m *Manager) AcquireExperimentLock(ctx context.Context, id ID, command string) (*Lock, error) {
	name := id.Flat() + ".lock"
	return acquireLock(ctx, filepath.Join(m.CommonDir, "experiment-locks", name), command)
}

func acquireLock(ctx context.Context, path, command string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock root: %w", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("read hostname for lock: %w", err)
	}
	owner := LockOwner{
		PID:        os.Getpid(),
		Hostname:   hostname,
		Command:    command,
		AcquiredAt: time.Now().UTC(),
	}

	ticker := time.NewTicker(lockRetryInterval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, lockTimeoutError(path, err)
		}
		err := os.Mkdir(path, 0o755)
		if err == nil {
			data, marshalErr := json.MarshalIndent(owner, "", "  ")
			if marshalErr == nil {
				data = append(data, '\n')
				marshalErr = os.WriteFile(filepath.Join(path, "owner.json"), data, 0o644)
			}
			if marshalErr != nil {
				_ = os.RemoveAll(path)
				return nil, fmt.Errorf("write lock owner %q: %w", path, marshalErr)
			}
			return &Lock{path: path, owner: owner}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire lock %q: %w", path, err)
		}

		select {
		case <-ctx.Done():
			return nil, lockTimeoutError(path, ctx.Err())
		case <-ticker.C:
		}
	}
}

func lockTimeoutError(path string, cause error) error {
	owner, err := readLockOwner(path)
	if err != nil {
		return fmt.Errorf("acquire lock %q: %w; owner details unavailable: %v", path, cause, err)
	}
	return fmt.Errorf(
		"acquire lock %q: %w; held by PID %d on %s for %q since %s",
		path, cause, owner.PID, owner.Hostname, owner.Command, owner.AcquiredAt.Format(time.RFC3339Nano),
	)
}

func readLockOwner(path string) (LockOwner, error) {
	data, err := os.ReadFile(filepath.Join(path, "owner.json"))
	if err != nil {
		return LockOwner{}, err
	}
	var owner LockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return LockOwner{}, err
	}
	return owner, nil
}

// Release releases this lock. Repeated calls are harmless.
func (l *Lock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.released {
		return nil
	}
	owner, err := readLockOwner(l.path)
	if err != nil {
		return fmt.Errorf("verify lock ownership %q: %w", l.path, err)
	}
	if !reflect.DeepEqual(owner, l.owner) {
		return fmt.Errorf("refuse to release lock %q owned by PID %d on %s for %q", l.path, owner.PID, owner.Hostname, owner.Command)
	}
	entries, err := os.ReadDir(l.path)
	if err != nil {
		return fmt.Errorf("inspect lock %q: %w", l.path, err)
	}
	if len(entries) != 1 || entries[0].Name() != "owner.json" || entries[0].IsDir() {
		return fmt.Errorf("refuse to release lock %q with unexpected contents", l.path)
	}
	ownerPath := filepath.Join(l.path, "owner.json")
	if err := os.Remove(ownerPath); err != nil {
		return fmt.Errorf("remove lock owner %q: %w", ownerPath, err)
	}
	if err := os.Remove(l.path); err != nil {
		return fmt.Errorf("release lock %q: %w", l.path, err)
	}
	l.released = true
	return nil
}
