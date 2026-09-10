package publish_test

import (
	"testing"

	"github.com/jaminalder/go-graphics/internal/explore"
	"github.com/jaminalder/go-graphics/internal/publish"
)

// TestEveryVisualChoiceMakesAnAdmittedRecipe keeps the catalogue and renderer policy aligned.
func TestEveryVisualChoiceMakesAnAdmittedRecipe(t *testing.T) {
	if err := publish.Check(); err != nil {
		t.Fatal(err)
	}
	for _, e := range publish.All() {
		space, _ := publish.Space(e.ID)
		for _, s := range e.Styles {
			for _, c := range e.Colours {
				pins, pal, err := publish.Pins(e.ID, s.ID, c.ID)
				if err != nil {
					t.Fatal(err)
				}
				batch, err := explore.Public(space, pins, nil, 42, 0, nil)
				if err != nil {
					t.Fatal(err)
				}
				for _, candidate := range batch {
					r, err := publish.Complete(e.ID, candidate.Seed, pal, candidate.Traits)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := publish.Validate(r.Bytes()); err != nil {
						t.Fatal(e.ID, s.ID, c.ID, err)
					}
				}
			}
		}
	}
}
