package palette_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func TestHSBToColor(t *testing.T) {
	tests := []struct {
		name string
		hsb  palette.HSB
		want palette.Color
	}{
		{"black", palette.HSB{H: 0, S: 0, B: 0}, palette.Color{}},
		{"white", palette.HSB{H: 0, S: 0, B: 100}, palette.Color{R: 1, G: 1, B: 1}},
		{"red", palette.HSB{H: 0, S: 100, B: 100}, palette.Color{R: 1}},
		{"green", palette.HSB{H: 120, S: 100, B: 100}, palette.Color{G: 1}},
		{"blue", palette.HSB{H: 240, S: 100, B: 100}, palette.Color{B: 1}},
		{"cyan", palette.HSB{H: 180, S: 100, B: 100}, palette.Color{G: 1, B: 1}},
		{"mid grey", palette.HSB{H: 0, S: 0, B: 50}, palette.Color{R: 0.5, G: 0.5, B: 0.5}},
		{"hue wraps", palette.HSB{H: 360, S: 100, B: 100}, palette.Color{R: 1}},
		{"negative hue wraps", palette.HSB{H: -120, S: 100, B: 100}, palette.Color{B: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.hsb.Color()
			if math.Abs(got.R-tc.want.R) > 1e-9 ||
				math.Abs(got.G-tc.want.G) > 1e-9 ||
				math.Abs(got.B-tc.want.B) > 1e-9 {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestHSBRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 2000; i++ {
		want := palette.Color{R: rng.Float64(), G: rng.Float64(), B: rng.Float64()}
		got := want.HSB().Color()
		if math.Abs(got.R-want.R) > 1e-9 ||
			math.Abs(got.G-want.G) > 1e-9 ||
			math.Abs(got.B-want.B) > 1e-9 {
			t.Fatalf("round trip %+v → %+v (via %+v)", want, got, want.HSB())
		}
	}
}

func testSwatch() palette.Swatch {
	return palette.Swatch{
		Name: "test", Base: palette.HSB{H: 205, S: 85, B: 70},
		HMin: 203, HMax: 207, HStd: 1,
		SMin: 83, SMax: 87, SStd: 1,
		BMin: 68, BMax: 72, BStd: 1,
	}
}

func TestSwatchStaysInsideItsBox(t *testing.T) {
	s := testSwatch()
	rng := rand.New(rand.NewPCG(4, 5))
	c := s.Draw(rng)
	for i := 0; i < 20000; i++ {
		c = s.Step(rng, c)
		if c.H < s.HMin || c.H > s.HMax {
			t.Fatalf("iteration %d: hue %v outside [%v,%v]", i, c.H, s.HMin, s.HMax)
		}
		if c.S < s.SMin || c.S > s.SMax {
			t.Fatalf("iteration %d: sat %v outside [%v,%v]", i, c.S, s.SMin, s.SMax)
		}
		if c.B < s.BMin || c.B > s.BMax {
			t.Fatalf("iteration %d: bright %v outside [%v,%v]", i, c.B, s.BMin, s.BMax)
		}
	}
}

// The point of Step is that it is a walk: consecutive values must be
// correlated, unlike an independent resample of the base.
func TestSwatchStepIsAWalkNotAResample(t *testing.T) {
	s := testSwatch()
	s.HMin, s.HMax, s.HStd = 0, 360, 3 // a wide box so clamping does not mask the effect

	rng := rand.New(rand.NewPCG(6, 7))
	var walkDrift, resampleDrift float64
	const n = 20000

	c := s.Draw(rng)
	for i := 0; i < n; i++ {
		next := s.Step(rng, c)
		walkDrift += math.Abs(next.H - c.H)
		c = next
	}
	prev := s.Draw(rng)
	for i := 0; i < n; i++ {
		next := s.Draw(rng)
		resampleDrift += math.Abs(next.H - prev.H)
		prev = next
	}
	if walkDrift/n > resampleDrift/n {
		t.Errorf("walk steps (%.3f) should be smaller than resample jumps (%.3f)",
			walkDrift/n, resampleDrift/n)
	}
}

func TestSwatchWalkCoversItsBox(t *testing.T) {
	s := testSwatch()
	rng := rand.New(rand.NewPCG(8, 9))
	c := s.Draw(rng)
	lo, hi := math.Inf(1), math.Inf(-1)
	for i := 0; i < 20000; i++ {
		c = s.Step(rng, c)
		lo, hi = math.Min(lo, c.B), math.Max(hi, c.B)
	}
	if lo > s.BMin+0.1 || hi < s.BMax-0.1 {
		t.Errorf("walk reached only [%v,%v] of [%v,%v]", lo, hi, s.BMin, s.BMax)
	}
}

func TestSwatchAround(t *testing.T) {
	base := palette.MustHex("#4488cc")
	s := palette.SwatchAround("blue", base, 6, 10, 10)
	got := s.Color()
	if math.Abs(got.R-base.R) > 1e-9 || math.Abs(got.G-base.G) > 1e-9 || math.Abs(got.B-base.B) > 1e-9 {
		t.Errorf("base colour not preserved: %+v vs %+v", got, base)
	}
	rng := rand.New(rand.NewPCG(10, 11))
	c := s.Draw(rng)
	for i := 0; i < 5000; i++ {
		c = s.Step(rng, c)
		if c.S < 0 || c.S > 100 || c.B < 0 || c.B > 100 {
			t.Fatalf("iteration %d escaped the legal range: %+v", i, c)
		}
	}
}

func TestSwatchIsDeterministic(t *testing.T) {
	s := testSwatch()
	a := s.Draw(rand.New(rand.NewPCG(12, 13)))
	b := s.Draw(rand.New(rand.NewPCG(12, 13)))
	if a != b {
		t.Errorf("same seed gave %+v then %+v", a, b)
	}
}
