package trait_test

import (
	"flag"
	"io"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/jaminalder/go-graphics/internal/trait"
)

func schema() trait.Schema {
	return trait.Schema{
		{
			Name: "structure", Key: "str", Doc: "how start points are laid out",
			Values: []trait.Value{{Name: "orbital", Weight: 1}, {Name: "formation", Weight: 1}, {Name: "shadows", Weight: 1}},
		},
		{
			Name: "ring-size", Key: "rs", Doc: "size of the ring dots",
			Values: []trait.Value{{Name: "small", Weight: 4}, {Name: "medium", Weight: 3}, {Name: "large", Weight: 1}},
		},
		{
			Name: "palette", Key: "p", Doc: "colour set", InName: true,
			Values: []trait.Value{{Name: "berlin", Weight: 1}, {Name: "miami", Weight: 1}, {Name: "external", Weight: 0}},
		},
	}
}

func rng(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 1)) }

func TestValidate(t *testing.T) {
	if err := schema().Validate(); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}

	tests := []struct {
		name string
		s    trait.Schema
		want string
	}{
		{"empty name", trait.Schema{{Key: "k", Values: []trait.Value{{Name: "a", Weight: 1}}}}, "empty name"},
		{"no key", trait.Schema{{Name: "a", Values: []trait.Value{{Name: "a", Weight: 1}}}}, "no key"},
		{"no values", trait.Schema{{Name: "a", Key: "a"}}, "no values"},
		{
			"all weights zero",
			trait.Schema{{Name: "a", Key: "a", Values: []trait.Value{{Name: "x"}}}},
			"no drawable value",
		},
		{
			"duplicate dimension",
			trait.Schema{
				{Name: "a", Key: "a", Values: []trait.Value{{Name: "x", Weight: 1}}},
				{Name: "a", Key: "b", Values: []trait.Value{{Name: "x", Weight: 1}}},
			},
			"duplicate dimension",
		},
		{
			"duplicate key",
			trait.Schema{
				{Name: "a", Key: "k", Values: []trait.Value{{Name: "x", Weight: 1}}},
				{Name: "b", Key: "k", Values: []trait.Value{{Name: "x", Weight: 1}}},
			},
			"duplicate key",
		},
		{
			"duplicate value",
			trait.Schema{{Name: "a", Key: "a", Values: []trait.Value{{Name: "x", Weight: 1}, {Name: "x", Weight: 1}}}},
			"duplicate value",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.Validate()
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestDeriveIsDeterministic(t *testing.T) {
	s := schema()
	for seed := uint64(0); seed < 50; seed++ {
		a := s.Derive(rng(seed))
		b := s.Derive(rng(seed))
		for _, d := range s {
			if a[d.Name] != b[d.Name] {
				t.Fatalf("seed %d dim %s: %q then %q", seed, d.Name, a[d.Name], b[d.Name])
			}
			if !d.Has(a[d.Name]) {
				t.Fatalf("seed %d dim %s: drew unknown value %q", seed, d.Name, a[d.Name])
			}
		}
	}
}

// Appending a dimension must not disturb what an existing seed draws for
// the ones already there — that is what makes a schema safe to grow.
func TestAppendingADimensionIsStable(t *testing.T) {
	base := schema()
	grown := append(schema(), trait.Dim{
		Name: "margin", Key: "mg",
		Values: []trait.Value{{Name: "none", Weight: 1}, {Name: "wide", Weight: 2}},
	})
	for seed := uint64(0); seed < 50; seed++ {
		a, b := base.Derive(rng(seed)), grown.Derive(rng(seed))
		for _, d := range base {
			if a[d.Name] != b[d.Name] {
				t.Fatalf("seed %d dim %s changed: %q → %q", seed, d.Name, a[d.Name], b[d.Name])
			}
		}
	}
}

func TestDrawHonoursWeights(t *testing.T) {
	s := schema()
	const n = 20000
	counts := map[string]int{}
	r := rng(7)
	for i := 0; i < n; i++ {
		counts[s.Derive(r)["ring-size"]]++
	}
	// small 4, medium 3, large 1 out of 8.
	for _, tc := range []struct {
		value string
		want  float64
	}{{"small", 0.5}, {"medium", 0.375}, {"large", 0.125}} {
		got := float64(counts[tc.value]) / n
		if math.Abs(got-tc.want) > 0.02 {
			t.Errorf("%s: got %.3f, want ≈%.3f", tc.value, got, tc.want)
		}
	}
}

// Weight 0 is the "override only" marker: reachable by hand, never by seed.
func TestZeroWeightIsNeverDrawn(t *testing.T) {
	s := schema()
	r := rng(3)
	for i := 0; i < 20000; i++ {
		if v := s.Derive(r)["palette"]; v == "external" {
			t.Fatalf("drew a zero-weight value on iteration %d", i)
		}
	}
}

func TestOptionsOverride(t *testing.T) {
	o := trait.NewOptions(schema())
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o.Flags(fs)
	if err := fs.Parse([]string{"--structure", "shadows", "--palette", "external"}); err != nil {
		t.Fatal(err)
	}
	if err := o.Configure(); err != nil {
		t.Fatal(err)
	}

	set := o.Resolve(rng(11))
	if set["structure"] != "shadows" {
		t.Errorf("structure = %q, want shadows", set["structure"])
	}
	if set["palette"] != "external" {
		t.Errorf("palette = %q, want external", set["palette"])
	}
	// The unpinned dimension must still be whatever the seed drew.
	if want := schema().Derive(rng(11))["ring-size"]; set["ring-size"] != want {
		t.Errorf("ring-size = %q, want the seed's %q", set["ring-size"], want)
	}
}

func TestOptionsRejectUnknownValue(t *testing.T) {
	o := trait.NewOptions(schema())
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o.Flags(fs)
	if err := fs.Parse([]string{"--ring-size", "gigantic"}); err != nil {
		t.Fatal(err)
	}
	err := o.Configure()
	if err == nil {
		t.Fatal("want an error for an unknown value")
	}
	if !strings.Contains(err.Error(), "small|medium|large") {
		t.Errorf("error should list the options, got: %v", err)
	}
}

func TestNameSuffix(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		// palette is InName, so it shows up even untouched.
		{"nothing pinned", nil, "-p-berlin"},
		{"one pinned", []string{"--structure", "shadows"}, "-str-shadows-p-berlin"},
		{
			"pinned in schema order",
			[]string{"--ring-size", "large", "--structure", "orbital"},
			"-str-orbital-rs-large-p-berlin",
		},
		{"pinned InName dim appears once", []string{"--palette", "miami"}, "-p-miami"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := trait.NewOptions(schema())
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			o.Flags(fs)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if err := o.Configure(); err != nil {
				t.Fatal(err)
			}
			// Seed 11 draws palette=berlin; pinned values override it.
			if got := o.NameSuffix(o.Resolve(rng(11))); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	s := schema()
	got := s.Format(trait.Set{"structure": "orbital", "ring-size": "small", "palette": "miami"})
	want := "structure=orbital ring-size=small palette=miami"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWeightReporting(t *testing.T) {
	d, ok := schema().Dim("ring-size")
	if !ok {
		t.Fatal("dimension missing")
	}
	if got := d.Weight("small"); got != 4 {
		t.Errorf("weight of small = %v, want 4", got)
	}
	if got := d.TotalWeight(); got != 8 {
		t.Errorf("total weight = %v, want 8", got)
	}
	if got := d.Weight("nope"); got != 0 {
		t.Errorf("weight of an unknown value = %v, want 0", got)
	}
}
