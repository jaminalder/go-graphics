// Package explore plans deterministic candidates without filesystem, HTTP or clocks.
package explore

import (
	"errors"
	"math/rand/v2"
	"strconv"

	"github.com/jaminalder/go-graphics/internal/trait"
)

// Candidate records the composition seed, complete traits and parent relationship.
type Candidate struct {
	Seed   uint64
	Traits trait.Set
	Mode   string
	Parent uint64
}

// NeighborStride preserves the established local flock seed policy.
const NeighborStride uint64 = 100003

// LocalBreed preserves the CLI's half boosted, half neighboring-parent policy.
func LocalBreed(schema trait.Schema, parents []Candidate, count int, seed uint64, strength float64) []Candidate {
	if len(parents) == 0 || count < 1 || count > 120 {
		return nil
	}
	sets := make([]trait.Set, len(parents))
	for i, p := range parents {
		sets[i] = p.Traits
	}
	boosted := schema.Boost(sets, strength)
	out := make([]Candidate, 0, count)
	for i := 0; i < count/2; i++ {
		n := seed + uint64(i)
		out = append(out, Candidate{Seed: n, Traits: boosted.Derive(rand.New(rand.NewPCG(n, 2))), Mode: "boosted"})
	}
	for i := 0; i < count-count/2; i++ {
		p := parents[i%len(parents)]
		out = append(out, Candidate{Seed: p.Seed + uint64(i+1)*NeighborStride, Traits: clone(p.Traits), Mode: "neighbor", Parent: p.Seed})
	}
	return out
}

// Public proposes two parent-guided, one boosted and one base-distribution candidate.
// Pins always win; round rotates representation when more than two parents are selected.
func Public(s trait.Schema, pins trait.Set, parents []Candidate, seed uint64, round int, recent []uint64) ([]Candidate, error) {
	if len(parents) > 4 || len(recent) > 128 || round < 0 {
		return nil, errors.New("exploration limit")
	}
	valid := func(set trait.Set) bool {
		for k, v := range set {
			d, ok := s.Dim(k)
			if !ok || !d.Has(v) {
				return false
			}
		}
		return true
	}
	if !valid(pins) {
		return nil, errors.New("invalid pin")
	}
	sets := make([]trait.Set, len(parents))
	for i, p := range parents {
		if !valid(p.Traits) {
			return nil, errors.New("invalid parent")
		}
		sets[i] = p.Traits
	}
	boosted := s.Boost(sets, 3)
	seen := map[string]bool{}
	for _, n := range recent {
		seen[strconv.FormatUint(n, 10)] = true
	}
	out := make([]Candidate, 0, 4)
	for attempt := 0; attempt < 128 && len(out) < 4; attempt++ {
		n := seed + uint64(attempt)
		key := strconv.FormatUint(n, 10)
		if seen[key] {
			continue
		}
		seen[key] = true
		rng := rand.New(rand.NewPCG(n, 2))
		c := Candidate{Seed: n, Traits: s.Derive(rng), Mode: "explore"}
		i := len(out)
		if len(parents) > 0 && i < 2 {
			p := parents[(round*2+i)%len(parents)]
			c.Traits = clone(p.Traits)
			c.Mode = "neighbor"
			c.Parent = p.Seed
		} else if len(parents) > 0 && i == 2 {
			c.Traits = boosted.Derive(rand.New(rand.NewPCG(n, 2)))
			c.Mode = "boosted"
		}
		for k, v := range pins {
			c.Traits[k] = v
		}
		out = append(out, c)
	}
	return out, nil
}

func clone(set trait.Set) trait.Set {
	out := trait.Set{}
	for k, v := range set {
		out[k] = v
	}
	return out
}
