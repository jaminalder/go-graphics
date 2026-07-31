package pools

import (
	"math"
	"testing"
)

// bandedCircles collects the banded marks a seed produces.
func bandedCircles(t *testing.T, s *Sketch, seed uint64) []circle {
	t.Helper()
	var out []circle
	for _, c := range plan(t, s, seed) {
		if c.kind == kindBanded {
			out = append(out, c)
		}
	}
	return out
}

// TestBandsFillTheDisc is the point of the mark: it is a disc *made of*
// rings, so the rings have to reach both the rim and the centre. A stack
// that stops short leaves either an unpainted margin inside the silhouette
// or a pinhole in the middle, and both read as a fault rather than a
// choice.
func TestBandsFillTheDisc(t *testing.T) {
	s := New()
	s.Banded = 1
	seen := 0
	for seed := uint64(1); seed <= 10; seed++ {
		for _, c := range bandedCircles(t, s, seed) {
			seen++
			b := c.bands
			if len(b.mid) < 2 {
				t.Fatalf("seed %d: banded circle r=%v has %d bands", seed, c.R, len(b.mid))
			}
			// The outermost band reaches the rim.
			if out := b.mid[0] + b.width[0]/2; out < c.R {
				t.Errorf("seed %d: outermost band ends at %v, inside the rim at %v", seed, out, c.R)
			}
			// The innermost band closes over the centre.
			if in := b.mid[len(b.mid)-1] - b.width[len(b.width)-1]/2; in > 0 {
				t.Errorf("seed %d: innermost band leaves a hole of radius %v", seed, in)
			}
			if len(b.colors) != len(b.mid) {
				t.Errorf("seed %d: %d bands but %d colours", seed, len(b.mid), len(b.colors))
			}
		}
	}
	if seen == 0 {
		t.Fatal("no banded circles at --banded 1")
	}
}

// TestBandsOverlapTheirNeighbours pins the mechanism that makes the seams.
// Neighbouring bands have to cross, because the fine contour line at every
// ring boundary is two transparent glazes passing through one another —
// butt them together and the mark is a flat gradient with hairline gaps.
func TestBandsOverlapTheirNeighbours(t *testing.T) {
	s := New()
	s.Banded = 1
	for seed := uint64(1); seed <= 6; seed++ {
		for _, c := range bandedCircles(t, s, seed) {
			b := c.bands
			for i := 1; i < len(b.mid); i++ {
				inner := b.mid[i-1] - b.width[i-1]/2 // inside edge of the outer band
				outer := b.mid[i] + b.width[i]/2     // outside edge of the inner one
				if outer <= inner {
					t.Fatalf("seed %d: bands %d and %d do not meet (%v vs %v)",
						seed, i-1, i, outer, inner)
				}
			}
		}
	}
}

// TestBandPitchHoldsUntilTheCapBinds covers both halves of the sizing rule.
//
// Below MaxBands the pitch is a width, so a mark twice the radius gets
// twice the rings rather than rings twice as fat and the ring texture keeps
// its weight across the ladder. The count is a rounded quotient — a disc
// has to come out whole — so the realised pitch cannot land on the nominal
// one exactly, and the bound below is that rounding and nothing more.
//
// At the cap the rule inverts: the count stops and the rings widen instead,
// which is what keeps a large disc a few concentric washes rather than a
// target.
func TestBandPitchHoldsUntilTheCapBinds(t *testing.T) {
	s := New()
	rng := testCtx(t, 1).RNG(streamLayout)
	_, _, _, ramp := s.inks(byLuminance(testCtx(t, 1).Palette.Colors), rng)

	// The pitch is only free between the two limits: below a couple of
	// bands a disc still has to come out whole, and above the cap it is the
	// cap that decides.
	floored := 2 * s.BandWidth
	capped := float64(s.MaxBands) * s.BandWidth
	var lastPitch float64
	for _, r := range []float64{0.05, 0.09, 0.15, 0.22, 0.35} {
		p := s.planBands(rng, r, ramp)
		n := len(p.mid)
		step := p.mid[0] - p.mid[1] // consecutive centres are one pitch apart

		if n > s.MaxBands {
			t.Fatalf("radius %v got %d bands, over the cap of %d", r, n, s.MaxBands)
		}
		if r >= floored && r <= capped {
			bound := 0.5 * s.BandWidth / float64(n)
			if math.Abs(step-s.BandWidth) > bound+1e-9 {
				t.Errorf("radius %v under the cap: %d bands at pitch %.5f, want %.5f ±%.5f",
					r, n, step, s.BandWidth, bound)
			}
			continue
		}
		// Over the cap the rings must actually be getting wider, not just
		// stopping at the nominal pitch and leaving the centre unpainted.
		if step <= lastPitch {
			t.Errorf("radius %v is over the cap but its pitch %.5f did not widen past %.5f",
				r, step, lastPitch)
		}
		lastPitch = step
	}
}

// TestBigCircleKeepsFewWideRings is the rule the mark lives or dies by. A
// disc that goes on accumulating rings as it grows reads as a target; what
// makes this mark is a ring wide enough to be a band of colour in its own
// right, with its own wet edge and rim. So past a point the count stops and
// the rings widen — measured at the top of the ladder, where it shows.
func TestBigCircleKeepsFewWideRings(t *testing.T) {
	s := New()
	radii, _ := s.ladder()
	big := radii[len(radii)-1]
	rng := testCtx(t, 1).RNG(streamLayout)
	_, _, _, ramp := s.inks(byLuminance(testCtx(t, 1).Palette.Colors), rng)

	p := s.planBands(rng, big, ramp)
	if n := len(p.mid); n > s.MaxBands {
		t.Errorf("the largest circle (r=%v) gets %d rings, over the cap of %d", big, n, s.MaxBands)
	}
	// And they are wider than the nominal pitch, not merely fewer.
	if step := p.mid[0] - p.mid[1]; step < s.BandWidth {
		t.Errorf("the largest circle's rings are %.4f wide, no wider than the %.4f pitch",
			step, s.BandWidth)
	}
}

// TestBandColoursGraduate pins the colouring rule. Anchors are neighbours
// on the luminance-ordered pigments walked in one direction, so successive
// bands stay related; drawing each band freely gives confetti, and ramping
// between colours from opposite ends of a palette spends most of the mark
// in the muddy middle.
func TestBandColoursGraduate(t *testing.T) {
	s := New()
	s.Banded = 1
	for seed := uint64(1); seed <= 8; seed++ {
		for _, c := range bandedCircles(t, s, seed) {
			cols := c.bands.colors
			if len(cols) < 4 {
				continue
			}
			for i := 1; i < len(cols); i++ {
				d := math.Abs(cols[i].Luminance() - cols[i-1].Luminance())
				if d > 0.3 {
					t.Errorf("seed %d: bands %d→%d jump %.2f in luminance — the ramp is not graduating",
						seed, i-1, i, d)
				}
			}
		}
	}
}
