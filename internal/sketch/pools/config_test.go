package pools_test

import (
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/sketch/pools"
)

// TestTypedOverridesKeepPresenceAndRejectNonfinite defends the public configuration boundary.
func TestTypedOverridesKeepPresenceAndRejectNonfinite(t *testing.T) {
	a := 0.74
	s, err := pools.FromConfig(pools.Config{Alpha: &a})
	if err != nil || s.Alpha != a {
		t.Fatal(s, err)
	}
	a = math.NaN()
	if _, err := pools.FromConfig(pools.Config{Alpha: &a}); err == nil {
		t.Fatal("accepted NaN")
	}
}
