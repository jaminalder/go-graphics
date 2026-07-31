package opt

import (
	"flag"
	"io"
	"strings"
	"testing"
)

type knobs struct {
	count int
	alpha float64
	deep  bool
	style string
	mode  int
}

func newSet(k *knobs) (*Set, *flag.FlagSet) {
	s := New()
	s.Int("count", "how many", "co", 0, 100, &k.count)
	s.Float("alpha", "how strong", "al", 0, 1, &k.alpha)
	s.Bool("deep", "go deep", "deep", &k.deep)
	s.Choice("style", "which style", "st", []string{"a", "b", "c"}, &k.style, func(i int) { k.mode = i })
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	return s, fs
}

func run(t *testing.T, args ...string) (*knobs, *Set, string, error) {
	t.Helper()
	k := &knobs{count: 7, alpha: 0.5, style: "b", mode: 1}
	s, fs := newSet(k)
	if err := fs.Parse(args); err != nil {
		return k, s, "", err
	}
	suffix, err := s.Configure()
	return k, s, suffix, err
}

// TestDefaultsSurvive is the reason the sentinel had to go. A knob the
// user did not touch keeps whatever the sketch put there — including, for
// a trait-driven sketch, a value its seed just derived — and says nothing
// in the filename.
func TestDefaultsSurvive(t *testing.T) {
	k, s, suffix, err := run(t)
	if err != nil {
		t.Fatal(err)
	}
	if k.count != 7 || k.alpha != 0.5 || k.style != "b" {
		t.Errorf("defaults were overwritten: %+v", k)
	}
	if suffix != "" {
		t.Errorf("untouched knobs named the file %q", suffix)
	}
	for _, n := range []string{"count", "alpha", "deep", "style"} {
		if s.WasSet(n) {
			t.Errorf("--%s reported as set", n)
		}
	}
}

func TestSetKnobsApplyAndName(t *testing.T) {
	k, s, suffix, err := run(t, "--count", "12", "--alpha", "0.25", "--deep", "--style", "c")
	if err != nil {
		t.Fatal(err)
	}
	if k.count != 12 || k.alpha != 0.25 || !k.deep || k.mode != 2 {
		t.Errorf("knobs not applied: %+v", k)
	}
	// Declaration order, so a filename is comparable across renders.
	if want := "-co12-al0.25-deep-stc"; suffix != want {
		t.Errorf("suffix %q, want %q", suffix, want)
	}
	if !s.WasSet("count") || s.WasSet("style") != true {
		t.Error("WasSet disagrees with the command line")
	}
}

// TestOutOfRangeIsRejected covers the bug the hand-written version shipped:
// with a negative sentinel for "unset", a negative value was silently
// ignored rather than refused.
func TestOutOfRangeIsRejected(t *testing.T) {
	for _, args := range [][]string{
		{"--count", "-2"},
		{"--count", "101"},
		{"--alpha", "-0.5"},
		{"--alpha", "2"},
		{"--style", "z"},
	} {
		if _, _, _, err := run(t, args...); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
}

// TestNegativeRangesAreExpressible is the other thing a sentinel cost: a
// knob whose useful range crosses zero could not be declared at all.
func TestNegativeRangesAreExpressible(t *testing.T) {
	gap := 0.1
	s := New()
	s.Float("gap", "clearance", "ga", -0.5, 2, &gap)
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse([]string{"--gap", "-0.2"}); err != nil {
		t.Fatal(err)
	}
	suffix, err := s.Configure()
	if err != nil {
		t.Fatalf("a legal negative value was refused: %v", err)
	}
	if gap != -0.2 || !strings.Contains(suffix, "ga-0.2") {
		t.Errorf("gap %v, suffix %q", gap, suffix)
	}
}

// TestBoolNamesOnlyWhenTrue keeps "--crackle" out of the filename of every
// render that did not ask for it.
func TestBoolNamesOnlyWhenTrue(t *testing.T) {
	on := true
	s := New()
	s.Bool("plain", "plain mode", "plain", &on)
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse([]string{"--plain=false"}); err != nil {
		t.Fatal(err)
	}
	suffix, err := s.Configure()
	if err != nil {
		t.Fatal(err)
	}
	if suffix != "" {
		t.Errorf("a false bool named the file %q", suffix)
	}
}
