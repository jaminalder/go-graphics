package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseSeeds(t *testing.T) {
	tests := []struct {
		in   string
		want []uint64
	}{
		{"3", []uint64{3}},
		{"1-4", []uint64{1, 2, 3, 4}},
		{"3,7,11", []uint64{3, 7, 11}},
		{"1-3,9", []uint64{1, 2, 3, 9}},
	}
	for _, tc := range tests {
		got, err := parseSeeds(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("%q gave %v, want %v", tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"4-1", "x", "1-x"} {
		if _, err := parseSeeds(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

// TestSweepPassesUnknownFlagsThrough is what keeps this command from having
// to know anything about the sketches: its own flags are stripped, and
// every other argument goes to render untouched, so a knob added to a
// sketch tomorrow is sweepable today.
func TestSweepPassesUnknownFlagsThrough(t *testing.T) {
	o, rest, err := splitSweepArgs([]string{
		"--seeds", "1-2", "--vary", "fill=busy,packed", "--out", "tmp",
		"--profile", "web", "--band-width", "0.03", "--mono",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.seeds) != 2 || o.out != "tmp" || len(o.vary) != 1 {
		t.Fatalf("sweep flags not consumed: %+v", o)
	}
	want := []string{"--profile", "web", "--band-width", "0.03", "--mono"}
	if !slices.Equal(rest, want) {
		t.Errorf("passthrough %v, want %v", rest, want)
	}
}

// TestExpandIsTheCartesianProduct pins the shape of a sweep: every seed
// against every value of every varied flag, because the question a sweep
// answers is what the *space* looks like, not what one corner of it does.
func TestExpandIsTheCartesianProduct(t *testing.T) {
	o := sweepOpts{
		seeds: []uint64{1, 2},
		vary:  []varied{{"fill", []string{"busy", "packed"}}, {"palette", []string{"a"}}},
	}
	combos, labels, err := expand(o)
	if err != nil {
		t.Fatal(err)
	}
	if len(combos) != 4 || len(labels) != 4 {
		t.Fatalf("got %d combinations, want 4", len(combos))
	}
	for i, c := range combos {
		if len(c) != 6 {
			t.Errorf("combination %d has %d args, want 3 flag pairs: %v", i, len(c), c)
		}
		if !strings.Contains(labels[i], "seed=") || !strings.Contains(labels[i], "fill=") {
			t.Errorf("label %q does not identify its tile", labels[i])
		}
	}
	if _, _, err := expand(sweepOpts{}); err == nil {
		t.Error("a sweep with nothing to vary was accepted")
	}
}
