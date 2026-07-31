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

// TestBandPitchIsResolutionIndependent guards the reason the pitch is a
// width and not a count: the ring texture has to weigh the same on a large
// mark as on a small one, so a mark twice the radius gets twice the rings
// rather than rings twice as fat.
func TestBandPitchIsResolutionIndependent(t *testing.T) {
	s := New()
	rng := testCtx(t, 1).RNG(streamLayout)
	_, _, ramp := s.inks(byLuminance(testCtx(t, 1).Palette.Colors), rng)

	small := s.planBands(rng, 0.05, ramp)
	large := s.planBands(rng, 0.20, ramp)
	if ratio := float64(len(large.mid)) / float64(len(small.mid)); math.Abs(ratio-4) > 0.35 {
		t.Errorf("a 4x radius gave %.2fx the bands, want ~4x — the pitch is not fixed", ratio)
	}
	mean := func(w []float64) float64 {
		t := 0.0
		for _, v := range w {
			t += v
		}
		return t / float64(len(w))
	}
	if a, b := mean(small.width), mean(large.width); math.Abs(a-b)/b > 0.2 {
		t.Errorf("band widths differ by size: %v small vs %v large", a, b)
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
