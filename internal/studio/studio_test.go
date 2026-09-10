package studio_test

import (
	"testing"

	"github.com/jaminalder/go-graphics/internal/studio"
)

// TestWorkspaceOwnershipAndStaleChoicesAreEnforced defends cross-tab isolation.
func TestWorkspaceOwnershipAndStaleChoicesAreEnforced(t *testing.T) {
	s := studio.New(nil, "test")
	w, e := s.Create()
	if e != nil {
		t.Fatal(e)
	}
	x, e := s.Start(w.Token, "iris", "fine", "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get("another", x.ID); e == nil {
		t.Fatal("cross-workspace read")
	}
	if e = s.Choices(w.Token, x.ID, 9, "winding", ""); e != studio.ErrConflict {
		t.Fatal(e)
	}
}
