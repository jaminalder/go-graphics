package trait

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"strings"
)

// unset is the flag default for every dimension; it means "let the seed
// decide". No dimension may declare a value with this name.
const unset = ""

// Options plugs a Schema into a sketch's CLI options: it registers one
// string flag per dimension, validates what the user typed, and applies the
// overrides on top of the seed-derived set.
//
// The lifecycle mirrors sketch.Configurable — Flags runs before parsing,
// Configure after it, and Resolve during the render, once a generator is
// available.
type Options struct {
	schema Schema
	raw    map[string]*string
	over   Set
}

// NewOptions prepares CLI plumbing for a schema. A malformed schema is a
// programming error and panics.
func NewOptions(s Schema) *Options {
	if err := s.Validate(); err != nil {
		panic(err.Error())
	}
	return &Options{schema: s, raw: make(map[string]*string, len(s)), over: Set{}}
}

// Schema returns the schema these options steer.
func (o *Options) Schema() Schema { return o.schema }

// Flags registers one flag per dimension. Flag names are stable — they are
// embedded in the recipe metadata of every rendered file.
func (o *Options) Flags(fs *flag.FlagSet) {
	for _, d := range o.schema {
		v := new(string)
		o.raw[d.Name] = v
		fs.StringVar(v, d.Name, unset, dimUsage(d))
	}
}

func dimUsage(d Dim) string {
	var b strings.Builder
	if d.Doc != "" {
		b.WriteString(d.Doc)
		b.WriteString(": ")
	}
	b.WriteString(strings.Join(d.ValueNames(), "|"))
	b.WriteString(" (default: from seed)")
	return b.String()
}

// Configure validates the overrides. It deliberately returns no filename
// suffix: the trait part of a filename needs the resolved set, which only
// exists once the seed is known, so it is built by NameSuffix at render
// time instead.
func (o *Options) Configure() error {
	o.over = Set{}
	for _, d := range o.schema {
		v, ok := o.raw[d.Name]
		if !ok || *v == unset {
			continue
		}
		if !d.Has(*v) {
			return fmt.Errorf("unknown --%s value %q (have %s)",
				d.Name, *v, strings.Join(d.ValueNames(), "|"))
		}
		o.over[d.Name] = *v
	}
	return nil
}

// Overrides returns the dimensions the user pinned, if any.
func (o *Options) Overrides() Set {
	out := make(Set, len(o.over))
	for k, v := range o.over {
		out[k] = v
	}
	return out
}

// Resolve derives a set from rng and applies the overrides on top. The full
// draw happens either way, so pinning one dimension leaves the others
// exactly as that seed would have had them.
func (o *Options) Resolve(rng *rand.Rand) Set {
	set := o.schema.Derive(rng)
	for k, v := range o.over {
		set[k] = v
	}
	return set
}

// NameSuffix is the output-filename fragment for a resolved set.
func (o *Options) NameSuffix(set Set) string {
	return o.schema.NameSuffix(set, o.over)
}
