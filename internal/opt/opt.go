// Package opt turns a sketch's CLI knobs into a declaration.
//
// A sketch names each knob once — flag name, help text, valid range, where
// the value goes, how it appears in the filename — and gets flag
// registration, validation and the output-name suffix from that. Written
// by hand, every sketch grew the same forty lines of range checks and the
// same fencepost bugs in them.
//
// The design turns on one thing: knobs are registered with the sketch's
// *real* defaults, and which ones the user actually set is read afterwards
// from the flag package itself. Hand-written versions used a sentinel
// default — "-1 means leave it alone" — which put the true default only in
// the help text, made a negative range unexpressible, and silently
// swallowed a typo like `--count -2` as "unset". Asking the FlagSet which
// flags were visited has none of those problems, and it is what makes a
// value derived from a seed distinguishable from one the user chose: see
// Set.WasSet, which is how a trait-driven sketch knows an override from a
// default.
//
// Stdlib-only leaf.
package opt

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

// Set is one sketch's collection of knobs.
type Set struct {
	knobs []knob
	fs    *flag.FlagSet
	seen  map[string]bool
}

// New creates an empty set.
func New() *Set { return &Set{seen: map[string]bool{}} }

// knob is one CLI option: how to describe it, how to check it and how it
// shows up in a filename.
type knob struct {
	name  string
	doc   string
	tag   string // filename prefix; empty leaves the knob out of the name
	reg   func(fs *flag.FlagSet)
	check func() error  // range validation, run only if the flag was set
	show  func() string // the filename fragment, run only if the flag was set
}

// Float declares a float knob constrained to [lo, hi]. dst carries the
// sketch's default in and the user's value out.
func (s *Set) Float(name, doc, tag string, lo, hi float64, dst *float64) {
	s.knobs = append(s.knobs, knob{
		name: name, doc: doc, tag: tag,
		reg: func(fs *flag.FlagSet) { fs.Float64Var(dst, name, *dst, doc) },
		check: func() error {
			if *dst < lo || *dst > hi {
				return fmt.Errorf("--%s must be within [%g, %g]", name, lo, hi)
			}
			return nil
		},
		show: func() string { return fmt.Sprintf("%s%g", tag, *dst) },
	})
}

// Int declares an int knob constrained to [lo, hi].
func (s *Set) Int(name, doc, tag string, lo, hi int, dst *int) {
	s.knobs = append(s.knobs, knob{
		name: name, doc: doc, tag: tag,
		reg: func(fs *flag.FlagSet) { fs.IntVar(dst, name, *dst, doc) },
		check: func() error {
			if *dst < lo || *dst > hi {
				return fmt.Errorf("--%s must be within [%d, %d]", name, lo, hi)
			}
			return nil
		},
		show: func() string { return fmt.Sprintf("%s%d", tag, *dst) },
	})
}

// Uint64 declares an unbounded uint64 knob, for sub-seeds.
func (s *Set) Uint64(name, doc, tag string, dst *uint64) {
	s.knobs = append(s.knobs, knob{
		name: name, doc: doc, tag: tag,
		reg:   func(fs *flag.FlagSet) { fs.Uint64Var(dst, name, *dst, doc) },
		check: func() error { return nil },
		show:  func() string { return fmt.Sprintf("%s%d", tag, *dst) },
	})
}

// Bool declares a boolean knob. It appears in the filename only when true,
// under its tag alone, since "--crackle" reads better than "--crackle1".
func (s *Set) Bool(name, doc, tag string, dst *bool) {
	s.knobs = append(s.knobs, knob{
		name: name, doc: doc, tag: tag,
		reg:   func(fs *flag.FlagSet) { fs.BoolVar(dst, name, *dst, doc) },
		check: func() error { return nil },
		show: func() string {
			if !*dst {
				return ""
			}
			return tag
		},
	})
}

// Choice declares a knob whose value is one of a fixed list of names. apply
// receives the index of the chosen name, so a sketch can map it onto its
// own iota-ordered constants; the help text lists the options.
func (s *Set) Choice(name, doc, tag string, values []string, dst *string, apply func(int)) {
	full := doc + ": " + strings.Join(values, "|")
	s.knobs = append(s.knobs, knob{
		name: name, doc: full, tag: tag,
		reg: func(fs *flag.FlagSet) { fs.StringVar(dst, name, *dst, full) },
		check: func() error {
			for i, v := range values {
				if v == *dst {
					// A knob whose whole answer is the string itself has
					// nothing to apply, and passing a no-op closure to say so
					// is noise at every call site.
					if apply != nil {
						apply(i)
					}
					return nil
				}
			}
			return fmt.Errorf("--%s must be one of %s", name, strings.Join(values, "|"))
		},
		show: func() string { return tag + *dst },
	})
}

// Flags registers every knob. It keeps the FlagSet so Configure can ask
// which flags were actually given.
func (s *Set) Flags(fs *flag.FlagSet) {
	s.fs = fs
	for _, k := range s.knobs {
		k.reg(fs)
	}
}

// Configure validates the knobs the user set and returns the filename
// suffix built from them, in declaration order. Knobs left alone are not
// validated and not named: whatever the sketch or its seed put there is by
// definition in range, and naming a file after a default says nothing.
func (s *Set) Configure() (string, error) {
	s.seen = map[string]bool{}
	if s.fs != nil {
		s.fs.Visit(func(f *flag.Flag) { s.seen[f.Name] = true })
	}
	var parts []string
	for _, k := range s.knobs {
		if !s.seen[k.name] {
			continue
		}
		if err := k.check(); err != nil {
			return "", err
		}
		if k.tag == "" {
			continue
		}
		if p := k.show(); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "-" + strings.Join(parts, "-"), nil
}

// WasSet reports whether the user gave this flag on the command line. A
// sketch that derives a value from its seed uses it to tell an override
// from a default, which a sentinel cannot express.
func (s *Set) WasSet(name string) bool { return s.seen[name] }

// Names lists the declared flag names, sorted — for tests that guard the
// flat CLI namespace against collisions with the generic render flags.
func (s *Set) Names() []string {
	out := make([]string, 0, len(s.knobs))
	for _, k := range s.knobs {
		out = append(out, k.name)
	}
	sort.Strings(out)
	return out
}
