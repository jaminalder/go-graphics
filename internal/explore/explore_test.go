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

// TestClearingExplicitWashRestoresBaseMaterialsAndRejectsPartialParents defends pin removal.
func TestClearingExplicitWashRestoresBaseMaterialsAndRejectsPartialParents(t *testing.T) {
	s := trait.Schema{{Name: "fill", Key: "f", Values: []trait.Value{{Name: "ink", Weight: 1}, {Name: "wash", Weight: 0}}}, {Name: "size", Key: "s", Values: []trait.Value{{Name: "large", Weight: 1}}}}
	parents := []explore.Candidate{{Seed: 42, Traits: trait.Set{"fill": "wash", "size": "large"}}}
	batch, e := explore.Public(s, nil, parents, 99, int(^uint(0)>>1), nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range batch {
		if c.Traits["fill"] != "ink" {
			t.Fatal("cleared wash pin still inherited")
		}
	}
	parents[0].Traits = trait.Set{"fill": "ink"}
	if _, e := explore.Public(s, nil, parents, 99, 0, nil); e == nil {
		t.Fatal("accepted incomplete public parent")
	}
}
