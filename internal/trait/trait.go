// Package trait models an artwork's output space as a set of orthogonal,
// weighted, discrete dimensions derived from the seed — the design idea
// behind Tyler Hobbs' QQL. A sketch declares a Schema once; a seed resolves
// it to a Set; single dimensions can be overridden from the CLI to steer a
// composition without giving up the rest of the draw.
//
// Dimensions are meant to be independent: each one adds a axis to the
// output space rather than restricting the others. Weights express relative
// rarity, so a schema can keep its common values common and still reach the
// rare combinations that make a long-form algorithm worth exploring.
//
// Stdlib-only leaf.
package trait

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
)

// Value is one option of a Dim. Weight is its relative rarity within the
// dimension; a weight of 0 means the value is never drawn from a seed and
// is reachable only by an explicit override.
type Value struct {
	Name   string
	Weight float64
}

// Dim is one orthogonal dimension of an output space.
type Dim struct {
	// Name is the kebab-case identifier; it doubles as the CLI flag name,
	// so it must not collide with the generic render flags.
	Name string
	// Key is a short stable token used in output filenames.
	Key string
	// Doc is the one-line description shown by --help.
	Doc string
	// InName records the resolved value in the filename even when the
	// dimension was not overridden — for dimensions that change the piece
	// so much that a file is not identifiable without them.
	InName bool
	// Values are the options, in a stable order.
	Values []Value
}

// Has reports whether value is one of the dimension's options.
func (d Dim) Has(value string) bool {
	for _, v := range d.Values {
		if v.Name == value {
			return true
		}
	}
	return false
}

// ValueNames lists the option names in declaration order.
func (d Dim) ValueNames() []string {
	out := make([]string, len(d.Values))
	for i, v := range d.Values {
		out[i] = v.Name
	}
	return out
}

// Weight returns the weight of value, or 0 if it is not an option.
func (d Dim) Weight(value string) float64 {
	for _, v := range d.Values {
		if v.Name == value {
			return v.Weight
		}
	}
	return 0
}

// TotalWeight is the sum of all option weights — the denominator of a
// value's rarity.
func (d Dim) TotalWeight() float64 {
	var sum float64
	for _, v := range d.Values {
		if v.Weight > 0 {
			sum += v.Weight
		}
	}
	return sum
}

// draw picks one value with probability proportional to its weight.
func (d Dim) draw(rng *rand.Rand) string {
	total := d.TotalWeight()
	if total <= 0 {
		panic(fmt.Sprintf("trait: dimension %q has no drawable value", d.Name))
	}
	bisect := rng.Float64() * total
	var cum float64
	for _, v := range d.Values {
		if v.Weight <= 0 {
			continue
		}
		cum += v.Weight
		if cum > bisect {
			return v.Name
		}
	}
	// Only reachable through float rounding at the very top of the range.
	for i := len(d.Values) - 1; i >= 0; i-- {
		if d.Values[i].Weight > 0 {
			return d.Values[i].Name
		}
	}
	panic("unreachable")
}

// Schema is an ordered set of dimensions. The order is stable and fixes the
// order values are drawn in, so appending a dimension never changes what an
// existing seed draws for the others.
type Schema []Dim

// Dim looks a dimension up by name.
func (s Schema) Dim(name string) (Dim, bool) {
	for _, d := range s {
		if d.Name == name {
			return d, true
		}
	}
	return Dim{}, false
}

// Validate reports the first structural problem with the schema. It catches
// programming errors, not user input.
func (s Schema) Validate() error {
	names := make(map[string]bool, len(s))
	keys := make(map[string]bool, len(s))
	for _, d := range s {
		switch {
		case d.Name == "":
			return fmt.Errorf("trait: dimension with empty name")
		case d.Key == "":
			return fmt.Errorf("trait: dimension %q has no key", d.Name)
		case names[d.Name]:
			return fmt.Errorf("trait: duplicate dimension %q", d.Name)
		case keys[d.Key]:
			return fmt.Errorf("trait: duplicate key %q on dimension %q", d.Key, d.Name)
		case len(d.Values) == 0:
			return fmt.Errorf("trait: dimension %q has no values", d.Name)
		case d.TotalWeight() <= 0:
			return fmt.Errorf("trait: dimension %q has no drawable value", d.Name)
		}
		names[d.Name] = true
		keys[d.Key] = true
		seen := make(map[string]bool, len(d.Values))
		for _, v := range d.Values {
			if v.Name == "" {
				return fmt.Errorf("trait: dimension %q has a value with an empty name", d.Name)
			}
			if seen[v.Name] {
				return fmt.Errorf("trait: dimension %q has duplicate value %q", d.Name, v.Name)
			}
			seen[v.Name] = true
		}
	}
	return nil
}

// Derive draws one value per dimension, in schema order.
func (s Schema) Derive(rng *rand.Rand) Set {
	set := make(Set, len(s))
	for _, d := range s {
		set[d.Name] = d.draw(rng)
	}
	return set
}

// NameSuffix builds the output-filename fragment for a resolved set: one
// "-<key>-<value>" part per dimension that was overridden or is marked
// InName, in schema order.
func (s Schema) NameSuffix(set Set, overrides Set) string {
	var b strings.Builder
	for _, d := range s {
		if _, over := overrides[d.Name]; !over && !d.InName {
			continue
		}
		if v := set[d.Name]; v != "" {
			fmt.Fprintf(&b, "-%s-%s", d.Key, v)
		}
	}
	return b.String()
}

// Format renders a set as a single stable line, for metadata.
func (s Schema) Format(set Set) string {
	parts := make([]string, 0, len(s))
	for _, d := range s {
		if v, ok := set[d.Name]; ok {
			parts = append(parts, d.Name+"="+v)
		}
	}
	return strings.Join(parts, " ")
}

// Set is a resolved point in the output space: dimension name → value name.
type Set map[string]string

// Get returns the value of a dimension, or "" if it is not in the set.
func (s Set) Get(dim string) string { return s[dim] }

// Is reports whether a dimension resolved to a particular value.
func (s Set) Is(dim, value string) bool { return s[dim] == value }

// Names lists the dimensions present, sorted.
func (s Set) Names() []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
