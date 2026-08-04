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

func TestLockReleaseRevalidatesOwnershipAfterRename(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := manager.AcquireGlobalLock(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	other := LockOwner{
		PID:        os.Getpid() + 1,
		Hostname:   "other-host",
		Command:    "replacement",
		AcquiredAt: time.Now().UTC(),
		Token:      "replacement-token",
	}
	lock.afterRename = func(_, tombstone string) error {
		data, err := json.Marshal(other)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(tombstone, "owner.json"), data, 0o644)
	}

	if err := lock.Release(); err == nil || !strings.Contains(err.Error(), "ownership changed after rename") {
		t.Fatalf("Release error = %v, want post-rename ownership error", err)
	}
	restored, err := readLockOwner(lock.path)
	if err != nil {
		t.Fatalf("restored owner: %v", err)
	}
	if restored.Token != other.Token {
		t.Fatalf("restored token = %q, want %q", restored.Token, other.Token)
	}
}

func TestLockReleasePreservesMismatchedTombstoneWhenOriginalIsReacquired(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := manager.AcquireGlobalLock(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	other := LockOwner{
		PID:        os.Getpid() + 1,
		Hostname:   "other-host",
		Command:    "replacement",
		AcquiredAt: time.Now().UTC(),
		Token:      "replacement-token",
	}
	lock.afterRename = func(path, tombstone string) error {
		data, err := json.Marshal(other)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(tombstone, "owner.json"), data, 0o644); err != nil {
			return err
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(path, "owner.json"), data, 0o644)
	}

	if err := lock.Release(); err == nil || !strings.Contains(err.Error(), "severe lock ownership error") {
		t.Fatalf("Release error = %v, want severe ownership error", err)
	}
	for _, path := range []string{lock.path, lock.tombstone} {
		owner, err := readLockOwner(path)
		if err != nil {
			t.Fatalf("preserved owner %q: %v", path, err)
		}
		if owner.Token != other.Token {
			t.Fatalf("owner token at %q = %q, want %q", path, owner.Token, other.Token)
		}
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

	lock.removeTombstone = removeLockTombstone
	if err := lock.Release(); err != nil {
		t.Fatalf("retry tombstone cleanup: %v", err)
	}
}

func TestLockReleaseDoesNotRecursivelyRemoveUnexpectedContents(t *testing.T) {
	repo := newTestRepo(t)
	manager, err := NewManager(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := manager.AcquireGlobalLock(context.Background(), "create")
	if err != nil {
		t.Fatal(err)
	}
	unexpectedName := "unexpected"
	if err := os.WriteFile(filepath.Join(lock.path, unexpectedName), []byte("preserve me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := lock.Release(); err == nil {
		t.Fatal("Release recursively removed unexpected lock contents")
	}
	if _, err := os.Stat(lock.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original lock remains after release: %v", err)
	}
	if lock.tombstone == "" {
		t.Fatal("diagnostic tombstone path is empty")
	}
	if _, err := os.Stat(filepath.Join(lock.tombstone, unexpectedName)); err != nil {
		t.Fatalf("unexpected tombstone content was removed: %v", err)
	}
	owner, err := readLockOwner(lock.tombstone)
	if err != nil {
		t.Fatalf("diagnostic owner was removed: %v", err)
	}
	if owner.Token != lock.owner.Token {
		t.Fatalf("diagnostic token = %q, want %q", owner.Token, lock.owner.Token)
	}
}
