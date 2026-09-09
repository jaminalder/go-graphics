package explore_test

import (
	"reflect"
	"testing"

	"github.com/jaminalder/go-graphics/internal/explore"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// TestPublicRefinementPreservesPinsAndDiversity defends the four-sample product promise.
func TestPublicRefinementPreservesPinsAndDiversity(t *testing.T) {
	s := trait.Schema{{Name: "a", Key: "a", Values: []trait.Value{{Name: "x", Weight: 1}, {Name: "y", Weight: 1}}}}
	p := []explore.Candidate{{Seed: 42, Traits: trait.Set{"a": "x"}}}
	a, e := explore.Public(s, trait.Set{"a": "y"}, p, 9, 0, nil)
	b, _ := explore.Public(s, trait.Set{"a": "y"}, p, 9, 0, nil)
	if e != nil || len(a) != 4 || !reflect.DeepEqual(a, b) {
		t.Fatal(a, e)
	}
	for _, c := range a {
		if c.Traits["a"] != "y" {
			t.Fatal(c)
		}
	}
	if a[3].Mode != "explore" {
		t.Fatal(a)
	}
}
