package pools

import (
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
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
	s := configured(t, "--banded", "1")
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
	s := configured(t, "--banded", "1")
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
	s := configured(t)
	rng := testCtx(t, 1).RNG(streamLayout)
	_, _, ramp := s.inks(palette.ByLuminance(paletteFor(t, s, testCtx(t, 1)).Colors))

	// The pitch is only free between the two limits: below a couple of
	// bands a disc still has to come out whole, and above the cap it is the
	// cap that decides.
	floored := 2 * s.BandWidth
	capped := float64(s.MaxBands) * s.BandWidth
	var lastPitch float64
	for _, r := range []float64{0.05, 0.09, 0.15, 0.22, 0.35} {
		p := s.planBands(rng, r, scheme{dir: 1}, ramp)
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
	s := configured(t)
	ctx := testCtx(t, 1)
	radii, _ := s.layoutFor(s.Traits(ctx), ctx.RNG(streamFill)).ladder()
	big := radii[len(radii)-1]
	rng := testCtx(t, 1).RNG(streamLayout)
	_, _, ramp := s.inks(palette.ByLuminance(paletteFor(t, s, testCtx(t, 1)).Colors))

	p := s.planBands(rng, big, scheme{dir: 1}, ramp)
	if n := len(p.mid); n > s.MaxBands {
		t.Errorf("the largest circle (r=%v) gets %d rings, over the cap of %d", big, n, s.MaxBands)
	}
	// And they are wider than the nominal pitch, not merely fewer.
	if step := p.mid[0] - p.mid[1]; step < s.BandWidth {
		t.Errorf("the largest circle's rings are %.4f wide, no wider than the %.4f pitch",
			step, s.BandWidth)
	}
}

// TestBandRampWalksNeighbours pins the colouring rule: a mark's anchors are
// *consecutive* entries of the luminance-ordered pigments, walked in one
// direction. A ramp between two colours from opposite ends of a palette
// spends most of its length in the muddy middle between them, and the mark
// comes out grey whatever its endpoints were.
//
// Checked at the ends, where the interpolation is the identity, so this is
// exact. Two earlier versions bounded the *step* and then its direction,
// and both were really testing the palette: neighbours on a five-colour
// palette can sit far apart in luminance, and interpolating HSL lightness
// between them does not move luminance monotonically, since hue carries
// some of it. The endpoints say what the code does regardless.
func TestBandRampWalksNeighbours(t *testing.T) {
	s := configured(t, "--banded", "1")
	ctx := testCtx(t, 1)
	rng := ctx.RNG(streamLayout)
	_, _, ramp := s.inks(palette.ByLuminance(paletteFor(t, s, ctx).Colors))

	// The anchors round-trip through HSL, so compare within a hair rather
	// than exactly.
	near := func(a, b palette.Color) bool {
		return math.Abs(a.R-b.R) < 1e-9 && math.Abs(a.G-b.G) < 1e-9 && math.Abs(a.B-b.B) < 1e-9
	}
	step := func(at, k, dir int) int {
		return ((at+k*dir)%len(ramp) + len(ramp)) % len(ramp)
	}

	for at := range ramp {
		for _, dir := range []int{1, -1} {
			p := s.planBands(rng, 0.2, scheme{at: at, dir: dir}, ramp)
			last := step(at, rampAnchors-1, dir)
			if got := p.colors[0]; !near(got, ramp[at]) {
				t.Errorf("at=%d dir=%d: the rim band is %v, want the anchor %v", at, dir, got, ramp[at])
			}
			if got := p.colors[len(p.colors)-1]; !near(got, ramp[last]) {
				t.Errorf("at=%d dir=%d: the centre band is %v, want ramp[%d] = %v — the anchors are not consecutive",
					at, dir, got, last, ramp[last])
			}
		}
	}
}
