package experiment

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	Token      string    `json:"token"`
}

// Lock is an acquired lifecycle lock.
type Lock struct {
	path  string
	owner LockOwner

	mu              sync.Mutex
	tombstone       string
	released        bool
	removeTombstone func(string) error
	afterRename     func(path, tombstone string) error
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
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("create lock token: %w", err)
	}
	owner := LockOwner{
		PID:        os.Getpid(),
		Hostname:   hostname,
		Command:    command,
		AcquiredAt: time.Now().UTC(),
		Token:      fmt.Sprintf("%x", token),
	}

	ticker := time.NewTicker(lockRetryInterval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil, lockTimeoutError(path, err)
		}
		err := os.Mkdir(path, 0o755)
		if err == nil {
			data, ownerErr := json.MarshalIndent(owner, "", "  ")
			if ownerErr == nil {
				data = append(data, '\n')
				ownerErr = os.WriteFile(filepath.Join(path, "owner.json"), data, 0o644)
			}
			if ownerErr != nil {
				cleanupErr := cleanupIncompleteLock(path, owner.Token)
				return nil, errors.Join(fmt.Errorf("write lock owner %q: %w", path, ownerErr), cleanupErr)
			}
			return &Lock{path: path, owner: owner, removeTombstone: removeLockTombstone}, nil
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

func cleanupIncompleteLock(path, token string) error {
	tombstone := path + ".failed-" + token
	if err := os.Rename(path, tombstone); err != nil {
		return fmt.Errorf("release incomplete lock %q: %w", path, err)
	}
	if err := removeLockDirectory(tombstone, false); err != nil {
		return fmt.Errorf("clean incomplete lock %q: %w", tombstone, err)
	}
	return nil
}

func removeLockTombstone(path string) error {
	return removeLockDirectory(path, true)
}

func removeLockDirectory(path string, requireOwner bool) error {
	ownerPath := filepath.Join(path, "owner.json")
	ownerData, readErr := os.ReadFile(ownerPath)
	if readErr != nil && (requireOwner || !errors.Is(readErr, os.ErrNotExist)) {
		return fmt.Errorf("read lock owner %q: %w", ownerPath, readErr)
	}
	if readErr == nil {
		if err := os.Remove(ownerPath); err != nil {
			return fmt.Errorf("remove lock owner %q: %w", ownerPath, err)
		}
	}
	if err := os.Remove(path); err != nil {
		if readErr == nil {
			if restoreErr := os.WriteFile(ownerPath, ownerData, 0o644); restoreErr != nil {
				return errors.Join(
					fmt.Errorf("remove lock tombstone %q: %w", path, err),
					fmt.Errorf("restore lock owner %q: %w", ownerPath, restoreErr),
				)
			}
		}
		return fmt.Errorf("remove lock tombstone %q: %w", path, err)
	}
	return nil
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
	if l.tombstone != "" {
		owner, err := readLockOwner(l.tombstone)
		if err != nil {
			return fmt.Errorf("revalidate released lock ownership %q: %w", l.tombstone, err)
		}
		if owner.Token != l.owner.Token {
			return fmt.Errorf("severe lock ownership error: tombstone %q has token %q, expected %q", l.tombstone, owner.Token, l.owner.Token)
		}
		if err := l.removeTombstone(l.tombstone); err != nil {
			return fmt.Errorf("clean released lock %q: %w", l.tombstone, err)
		}
		l.tombstone = ""
		l.released = true
		return nil
	}
	owner, err := readLockOwner(l.path)
	if err != nil {
		return fmt.Errorf("verify lock ownership %q: %w", l.path, err)
	}
	if owner.Token != l.owner.Token {
		return fmt.Errorf("refuse to release lock %q owned by PID %d on %s for %q", l.path, owner.PID, owner.Hostname, owner.Command)
	}
	tombstone := l.path + ".released-" + l.owner.Token
	if err := os.Rename(l.path, tombstone); err != nil {
		return fmt.Errorf("release lock %q: %w", l.path, err)
	}
	l.tombstone = tombstone
	if l.afterRename != nil {
		if err := l.afterRename(l.path, tombstone); err != nil {
			return fmt.Errorf("after lock rename %q: %w", tombstone, err)
		}
	}
	renamedOwner, err := readLockOwner(tombstone)
	if err != nil {
		return fmt.Errorf("revalidate lock ownership %q: %w", tombstone, err)
	}
	if renamedOwner.Token != l.owner.Token {
		restoreErr := restoreLockTombstone(tombstone, l.path)
		if restoreErr == nil {
			l.tombstone = ""
			return fmt.Errorf("lock ownership changed after rename; restored token %q to %q", renamedOwner.Token, l.path)
		}
		return errors.Join(
			fmt.Errorf("severe lock ownership error: token changed after rename from %q to %q; preserved tombstone %q", l.owner.Token, renamedOwner.Token, tombstone),
			restoreErr,
		)
	}
	if err := l.removeTombstone(tombstone); err != nil {
		return fmt.Errorf("clean released lock %q: %w", tombstone, err)
	}
	l.tombstone = ""
	l.released = true
	return nil
}

func restoreLockTombstone(tombstone, path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("cannot restore lock tombstone %q: original path %q is occupied", tombstone, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect original lock path %q: %w", path, err)
	}
	if err := os.Rename(tombstone, path); err != nil {
		return fmt.Errorf("restore lock tombstone %q to %q: %w", tombstone, path, err)
	}
	return nil
}
