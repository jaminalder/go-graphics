package hatch

import (
	"math"
	"testing"
)

// TestCrossHatchIsDarkerOnlyWhereTheFamiliesMeet: cross-hatching darkens by
// *crossing*, so a point on one family alone must be exactly as dark as that
// family alone, and only the intersections may be darker. Adding coverage
// instead of compositing it would lift the whole fill and lose the plaid.
func TestCrossHatchIsDarkerWhereTheFamiliesMeet(t *testing.T) {
	s := flat()
	s.Jitter = 0
	one := New(s)
	both := Cross(s, math.Pi/2)

	// v = 0 is a mark of the horizontal family; u = 0.025 is between marks
	// of the vertical one, so only the horizontal family is present.
	alone := both(unit(0.025, 0))
	if math.Abs(alone-one.Cover(unit(0.025, 0))) > 1e-9 {
		t.Errorf("on one family only: crossed %.6f, single %.6f", alone, one.Cover(unit(0.025, 0)))
	}
	// The origin is on a mark of both families.
	crossing := both(unit(0, 0))
	if crossing < alone-1e-9 {
		t.Errorf("at a crossing the coverage fell: %.6f vs %.6f", crossing, alone)
	}
	// And the paper between them stays paper.
	if c := both(unit(0.025, 0.025)); c > 1e-6 {
		t.Errorf("coverage %.6f between both families, want bare paper", c)
	}
}

// TestOverNeverExceedsFullCoverage: layers composite as transparency, so
// however many families are stacked the result stays a fraction a caller can
// lerp with.
func TestOverNeverExceedsFullCoverage(t *testing.T) {
	s := flat()
	s.Thickness = 0.9
	f := Cross(s, 0.4, 0.9, 1.4, 2.1, 2.7)
	for j := range 50 {
		for i := range 50 {
			c := f(unit(float64(i)/49, float64(j)/49))
			if c < 0 || c > 1 || math.IsNaN(c) {
				t.Fatalf("coverage %v", c)
			}
		}
	}
}

// TestCrossFamiliesDoNotWanderInStep: each family of a cross-hatch is given
// its own seed, because two families that jitter identically read as one
// hatch drawn twice rather than as two passes of a hand.
func TestCrossFamiliesDoNotWanderInStep(t *testing.T) {
	s := flat()
	s.Jitter = 0.35
	s.Seed = 5
	a := New(s)
	b := New(s.With(func(c *Spec) { c.Angle = math.Pi / 2 }))
	// Rotating by 90° maps (u,v) to (v,-u); if the seeds were shared the two
	// families would be the same displaced marks and these would agree.
	same := 0
	for k := -6; k <= 6; k++ {
		x := a.Cover(unit(0.3, float64(k)*0.05))
		y := b.Cover(unit(float64(k)*0.05, 0.3))
		if math.Abs(x-y) < 1e-12 {
			same++
		}
	}
	crossed := Cross(s, math.Pi/2)
	if crossed(unit(0.3, 0.3)) < 0 { // keeps Cross exercised
		t.Fatal("impossible")
	}
	if same > 3 {
		t.Errorf("%d of 13 marks landed identically — the families share a seed", same)
	}
}

// TestWeaveThreadsPassOverAndUnderEachOther is what makes a weave cloth
// rather than two stacked hatchings: at some crossings the first family
// occludes the second and at others the second occludes the first, and both
// happen often.
func TestWeaveThreadsPassOverAndUnderEachOther(t *testing.T) {
	warp := flat()
	warp.Thickness, warp.Jitter = 0.55, 0
	weft := warp.Rotated(math.Pi / 2)
	w := Weave(warp, weft)

	aOver, bOver, crossings := 0, 0, 0
	for j := range 240 {
		for i := range 240 {
			s := unit(float64(i)/239, float64(j)/239)
			ca, cb := w(s)
			if ca < 0 || cb < 0 || ca+cb > 1+1e-9 {
				t.Fatalf("visible coverage %.4f + %.4f at (%v,%v)", ca, cb, s.U, s.V)
			}
			if ca > 0.9 && cb > 0.0 && cb < 0.1 {
				aOver++
				crossings++
			}
			if cb > 0.9 && ca > 0.0 && ca < 0.1 {
				bOver++
				crossings++
			}
		}
	}
	if crossings == 0 {
		t.Fatal("the two families never crossed")
	}
	if aOver == 0 || bOver == 0 {
		t.Errorf("one family is always on top: %d crossings with warp over, %d with weft over", aOver, bOver)
	}
	// The alternation is what a weave is; a strong imbalance means the
	// parity rule has been lost.
	ratio := float64(min(aOver, bOver)) / float64(max(aOver, bOver))
	if ratio < 0.5 {
		t.Errorf("over-under is lopsided: %d vs %d", aOver, bOver)
	}
}

// TestNestedFillsEachSubRegionWithItsOwnHatch: nesting works by
// re-describing the sample for the inner region, so an inner hatch fitted to
// its sub-region must actually see that sub-region's centre and reach — not
// the outer one's.
func TestNestedFillsEachSubRegionWithItsOwnHatch(t *testing.T) {
	s := flat()
	s.Fit, s.Align = 3, AlignRegion
	inner := Of(s)
	// Two panels side by side, each half a canvas wide.
	f := Nested(func(sm Sample) (Sample, int) {
		i := 0
		cx := 0.25
		if sm.U >= 0.5 {
			i, cx = 1, 0.75
		}
		sm.CX, sm.CY, sm.Reach = cx, 0.5, 0.25
		return sm, i
	}, inner, Of(s.With(func(c *Spec) { c.Angle = math.Pi / 2 })))

	// The panel centres are equivalent points, so they must look the same.
	left := f(Sample{U: 0.25, V: 0.5})
	if left < 0.99 {
		t.Errorf("the left panel's centre mark is missing (coverage %.3f)", left)
	}
	// And the right panel is hatched the other way: walking down through its
	// centre crosses no mark but the middle one, where a horizontal hatch
	// would cross seven.
	crossings := 0
	prev := false
	for i := range 2000 {
		v := 0.25 + 0.5*float64(i)/1999
		on := f(Sample{U: 0.75, V: v}) > 0.5
		if on && !prev {
			crossings++
		}
		prev = on
	}
	if crossings != 1 {
		t.Errorf("walking down the right panel crossed %d marks, want 1 (it is hatched vertically)", crossings)
	}
}

// TestMaskConfinesAHatchToAShapeItCannotSee: a coverage function has no
// outline, so confining one is the caller's job. Multiplying is the whole
// mechanism and this pins that it is multiplication, not replacement.
func TestMaskConfinesAHatchToAShapeItCannotSee(t *testing.T) {
	s := flat()
	base := Of(s)
	disc := func(sm Sample) float64 {
		if math.Hypot(sm.U-0.5, sm.V-0.5) < 0.2 {
			return 1
		}
		return 0
	}
	m := Mask(base, disc)
	if c := m(unit(0.5, 0.5)); math.Abs(c-base(unit(0.5, 0.5))) > 1e-12 {
		t.Errorf("inside the mask the hatch changed: %.6f", c)
	}
	if c := m(unit(0.5, 0.95)); c != 0 {
		t.Errorf("outside the mask the coverage is %.6f, want 0", c)
	}
}
