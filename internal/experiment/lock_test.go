package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockSameNameTimesOutWithOwnerDetails(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseID("foam/hatching-by-depth")
	if err != nil {
		t.Fatal(err)
	}

	first, err := manager.AcquireExperimentLock(context.Background(), id, "first command")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	data, err := os.ReadFile(filepath.Join(first.path, "owner.json"))
	if err != nil {
		t.Fatal(err)
	}
	var owner LockOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	if owner.PID != os.Getpid() || owner.Hostname == "" || owner.Command != "first command" || owner.AcquiredAt.Location() != time.UTC {
		t.Fatalf("owner = %#v", owner)
	}
	if owner.Token == "" {
		t.Fatal("owner token is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err = manager.AcquireExperimentLock(ctx, id, "second command")
	if err == nil {
		t.Fatal("second acquisition unexpectedly succeeded")
	}
	for _, detail := range []string{"first command", owner.Hostname, "PID", "foam--hatching-by-depth.lock"} {
		if !strings.Contains(err.Error(), detail) {
			t.Errorf("timeout error %q does not contain %q", err, detail)
		}
	}
}

func TestLockDifferentExperimentIDsCanBeHeldConcurrently(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	firstID, _ := ParseID("foam/first")
	secondID, _ := ParseID("foam/second")
	first, err := manager.AcquireExperimentLock(context.Background(), firstID, "first")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := manager.AcquireExperimentLock(context.Background(), secondID, "second")
	if err != nil {
		t.Fatalf("different ID lock: %v", err)
	}
	t.Cleanup(func() { _ = second.Release() })
}

func TestLockReleaseIsIdempotentAndAllowsReacquisition(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}

	lock, err := manager.AcquireGlobalLock(context.Background(), "create")
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}

	reacquired, err := manager.AcquireGlobalLock(context.Background(), "archive")
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	if err := reacquired.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestLockReleaseRefusesChangedOwnership(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := manager.AcquireGlobalLock(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(lock.path) })

	other := LockOwner{
		PID:        os.Getpid() + 1,
		Hostname:   "other-host",
		Command:    "other command",
		AcquiredAt: time.Now().UTC(),
		Token:      "other-token",
	}
	data, err := json.Marshal(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lock.path, "owner.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := lock.Release(); err == nil {
		t.Fatal("Release removed a lock after its ownership changed")
	}
	if _, err := os.Stat(lock.path); err != nil {
		t.Fatalf("changed lock was removed: %v", err)
	}
}

func TestLockCleanupFailureDoesNotBlockReacquisition(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := manager.AcquireGlobalLock(context.Background(), "create")
	if err != nil {
		t.Fatal(err)
	}
	cleanupFailure := errors.New("cleanup failed")
	var tombstone string
	lock.removeTombstone = func(path string) error {
		tombstone = path
		return cleanupFailure
	}
	if err := lock.Release(); !errors.Is(err, cleanupFailure) {
		t.Fatalf("Release error = %v, want cleanup failure", err)
	}
	if _, err := os.Stat(lock.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original lock remains after atomic release: %v", err)
	}
	owner, err := readLockOwner(tombstone)
	if err != nil {
		t.Fatalf("tombstone owner: %v", err)
	}
	if owner.Token != lock.owner.Token {
		t.Fatalf("tombstone token = %q, want %q", owner.Token, lock.owner.Token)
	}

	reacquired, err := manager.AcquireGlobalLock(context.Background(), "retry")
	if err != nil {
		t.Fatalf("reacquire after tombstone cleanup failure: %v", err)
	}
	if err := reacquired.Release(); err != nil {
		t.Fatal(err)
	}

	lock.removeTombstone = os.RemoveAll
	if err := lock.Release(); err != nil {
		t.Fatalf("retry tombstone cleanup: %v", err)
	}
}
