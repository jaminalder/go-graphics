package hatch

import (
	"math"
	"testing"
)

// unit is a sample in the middle of a unit-ish region, the shape most of
// these tests reason about: centred at (0.5, 0.5) with half a canvas of
// room and no boundary in sight.
func unit(u, v float64) Sample {
	return Sample{U: u, V: v, CX: 0.5, CY: 0.5, Reach: 0.4, Wall: math.Inf(1), Tone: 1}
}

// flat is a parallel hatch running along +u, so the across coordinate is
// simply v and every claim about spacing can be read off a vertical scan.
func flat() Spec {
	s := Defaults()
	s.Angle = 0
	s.Spacing = 0.05
	s.Thickness = 0.3
	return s
}

// TestSpacingIsTheDistanceBetweenMarks pins the meaning of the single most
// used parameter: marks sit on multiples of Spacing from the phase origin
// and the paper between them is bare. If this drifts, every hatch in the
// repo silently changes density.
func TestSpacingIsTheDistanceBetweenMarks(t *testing.T) {
	h := New(flat())
	for k := -3; k <= 3; k++ {
		on := float64(k) * 0.05
		if c := h.Cover(unit(0.3, on)); c < 0.99 {
			t.Errorf("mark %d at v=%g: coverage %.3f, want ~1", k, on, c)
		}
		off := (float64(k) + 0.5) * 0.05
		if c := h.Cover(unit(0.3, off)); c > 0.001 {
			t.Errorf("gap after mark %d at v=%g: coverage %.3f, want 0", k, off, c)
		}
	}
}

// TestThicknessIsAFractionOfTheSpacing defends the decision that thickness
// is dimensionless. A hatch fitted to a small region must come out with
// proportionally finer lines rather than with the same absolute width, or
// every fitted hatch turns solid as the region shrinks.
func TestThicknessIsAFractionOfTheSpacing(t *testing.T) {
	for _, tc := range []struct{ spacing, thickness float64 }{
		{0.05, 0.3}, {0.05, 0.6}, {0.02, 0.3}, {0.1, 0.15},
	} {
		s := flat()
		s.Spacing, s.Thickness = tc.spacing, tc.thickness
		h := New(s)
		got := inkedShare(func(v float64) float64 { return h.Cover(unit(0.3, v)) }, 0, tc.spacing, 20000)
		want := tc.thickness
		if math.Abs(got-want) > 0.01 {
			t.Errorf("spacing %g thickness %g: inked share %.3f, want %.3f",
				tc.spacing, tc.thickness, got, want)
		}
	}
}

// TestAHatchIsIdenticalAtAnyResolution is the invariant most easily broken
// here (docs/ARCHITECTURE.md §4.2): nothing in a hatch may be measured in
// pixels. Averaging a fine sampling grid down to a coarse one must give what
// the coarse grid gives directly, for every structure.
func TestAHatchIsIdenticalAtAnyResolution(t *testing.T) {
	const coarse, factor = 40, 6
	for _, st := range allStructures() {
		s := Defaults()
		s.Structure = st
		s.Spacing = 0.06
		s.Amplitude = 2
		s.Seed = 7
		h := New(s)
		sample := func(u, v float64) float64 {
			return h.Cover(Sample{U: u, V: v, CX: 0.5, CY: 0.5, Reach: 0.4, Wall: dist(u, v), Tone: 0.7})
		}
		worst := 0.0
		for j := range coarse {
			for i := range coarse {
				// The coarse pixel's own average, and the same area averaged
				// from a grid factor² times finer.
				lo := boxAverage(sample, i, j, coarse, 4)
				hi := boxAverage(sample, i, j, coarse, 4*factor)
				worst = math.Max(worst, math.Abs(lo-hi))
			}
		}
		// Not zero: the two grids sample different points, so a mark edge
		// crossing a pixel is resolved differently. It has to be small —
		// a shift in the pattern itself would show up as a whole mark.
		if worst > 0.35 {
			t.Errorf("%s: coarse and fine sampling disagree by %.3f", st, worst)
		}
	}
}

// TestZeroCurvatureIsExactlyStraight guards the numerically awkward limit:
// the arc form divides by the curvature, so the straight case must be taken
// by a different path and must agree with the arc form as it is approached.
func TestZeroCurvatureIsExactlyStraight(t *testing.T) {
	straight := New(flat())
	s := flat()
	s.Curvature = 1e-7
	nearly := New(s)
	for _, p := range []struct{ u, v float64 }{{0.1, 0.2}, {0.5, 0.5}, {0.9, 0.77}} {
		a, b := straight.Cover(unit(p.u, p.v)), nearly.Cover(unit(p.u, p.v))
		if math.Abs(a-b) > 1e-4 {
			t.Errorf("at (%g,%g): straight %.6f vs curvature 1e-7 %.6f", p.u, p.v, a, b)
		}
	}
}

// TestCurvatureBendsMarksByTheRadiusItNames says what the knob means: a
// mark leaves its straight line by a²/2R after running a distance a. Without
// this, "curvature" is a number that bends things by an unknown amount and
// cannot be dialled in from a sketch.
func TestCurvatureBendsMarksByTheRadiusItNames(t *testing.T) {
	const k = 2.0 // radius 0.5
	s := flat()
	s.Curvature = k
	h := New(s)
	for _, along := range []float64{0.05, 0.1, 0.2} {
		// The mark through the origin, followed a distance `along`, should
		// have moved across by about along²·k/2.
		want := along * along * k / 2
		got := findMark(t, h, along, want, 0.4*s.Spacing)
		if math.Abs(got-want) > 0.1*want+1e-4 {
			t.Errorf("at along=%g the mark is at across=%.5f, want ~%.5f", along, got, want)
		}
	}
}

// TestContourMarksAreLevelSetsOfTheWallDistance is what makes contour
// hatching follow a shape it has never been told the outline of: the only
// thing it may read is Wall. If position leaks in, the marks stop hugging
// concave boundaries, which is the entire point of the structure.
func TestContourMarksAreLevelSetsOfTheWallDistance(t *testing.T) {
	s := Defaults()
	s.Structure = Contour
	s.Continuity = 1
	h := New(s)
	for _, wall := range []float64{0.01, 0.033, 0.07, 0.12} {
		want := h.Cover(Sample{U: 0.5, V: 0.5, CX: 0.5, CY: 0.5, Reach: 0.4, Wall: wall})
		for _, p := range []struct{ u, v float64 }{{0.1, 0.9}, {0.77, 0.2}, {0.5, 0.05}} {
			got := h.Cover(Sample{U: p.u, V: p.v, CX: 0.5, CY: 0.5, Reach: 0.4, Wall: wall})
			if math.Abs(got-want) > 1e-9 {
				t.Errorf("wall %g: coverage %.6f at (%g,%g) but %.6f at the centre",
					wall, got, p.u, p.v, want)
			}
		}
	}
}

// TestContourDrawsNothingWithoutABoundary: cells.Hit.Wall is +Inf where only
// one cell is in range, and a caller that hands that on must get bare paper
// rather than a NaN smeared across the frame.
func TestContourDrawsNothingWithoutABoundary(t *testing.T) {
	s := Defaults()
	s.Structure = Contour
	h := New(s)
	if c := h.Cover(unit(0.3, 0.4)); c != 0 {
		t.Errorf("coverage %v with Wall = +Inf, want 0", c)
	}
}

// TestRadialRaysCloseWithoutASeam: the across coordinate of a radial family
// is an angle, and an angle wraps. Unless the pitch is quantised to fit the
// turn a whole number of times there is a discontinuity at ±π, which reads
// as a crack running out of the centre of every region.
func TestRadialRaysCloseWithoutASeam(t *testing.T) {
	s := Defaults()
	s.Structure = Radial
	s.Spacing = 0.037 // deliberately not a divisor of the circumference
	h := New(s)
	const r = 0.3
	for _, eps := range []float64{1e-5, 1e-4} {
		a := h.Cover(radial(r, math.Pi-eps))
		b := h.Cover(radial(r, -math.Pi+eps))
		if math.Abs(a-b) > 0.02 {
			t.Errorf("across the ±π seam at eps=%g: %.4f vs %.4f", eps, a, b)
		}
	}
}

func radial(r, theta float64) Sample {
	return Sample{
		U: 0.5 + r*math.Cos(theta), V: 0.5 + r*math.Sin(theta),
		CX: 0.5, CY: 0.5, Reach: 0.4, Wall: math.Inf(1), Tone: 1,
	}
}

// TestFanArcsPassThroughBothPoles pins the geometry the fan is built on: the
// angle a point subtends between two fixed poles is constant along a
// circular arc through both, so the family really is a fan spreading from
// one pole to the other rather than a set of bowed lines.
func TestFanArcsPassThroughBothPoles(t *testing.T) {
	s := Defaults()
	s.Structure = Fan
	s.Angle = 0
	h := New(s)
	const r = 0.4
	ax, ay := 0.5-r, 0.5 // the poles, one Reach either side of the centre

	// A circle through both poles, centred above them. The inscribed angle
	// is constant on *one* arc — on the other it is that angle less π, which
	// is a different mark of the family — so the walk stays between the two
	// poles, on the arc below the centre.
	const cy = 0.7
	rad := math.Hypot(ax-0.5, ay-cy)
	lo := math.Atan2(ay-cy, ax-0.5) + 2*math.Pi // 3.605 rad
	hi := math.Atan2(ay-cy, 0.5+r-0.5)          // -0.464 rad, i.e. 5.819
	hi += 2 * math.Pi
	var first float64
	for i := range 9 {
		th := lo + (hi-lo)*(0.08+0.84*float64(i)/8)
		u, v := 0.5+rad*math.Cos(th), cy+rad*math.Sin(th)
		across, _, ok := h.Coords(Sample{U: u, V: v, CX: 0.5, CY: 0.5, Reach: r})
		if !ok {
			t.Fatal("fan refused a sample")
		}
		if i == 0 {
			first = across
			continue
		}
		if math.Abs(across-first) > 1e-6 {
			t.Errorf("point %d on an arc through both poles has across %.8f, want %.8f",
				i, across, first)
		}
	}
}

// TestConcentricRingsIgnoreEverythingButTheRadius separates the two
// round structures: Concentric is about the centre and Contour is about the
// boundary. Confusing them makes a crescent's rings pinch along its medial
// axis instead of nesting.
func TestConcentricRingsIgnoreEverythingButTheRadius(t *testing.T) {
	s := Defaults()
	s.Structure = Concentric
	s.Continuity = 1
	h := New(s)
	for _, r := range []float64{0.05, 0.13, 0.27} {
		var want float64
		for i := range 12 {
			th := float64(i) * math.Pi / 6
			got := h.Cover(radial(r, th))
			if i == 0 {
				want = got
				continue
			}
			if math.Abs(got-want) > 1e-9 {
				t.Errorf("radius %g: coverage %.6f at angle %.2f, %.6f at 0", r, got, th, want)
			}
		}
	}
}

// TestFlowWithoutAmplitudeIsPlainParallel: the flow structure adds a stream
// function to the parallel coordinate, so with the field turned off it must
// reduce exactly to parallel. It is the cheapest guard that the field is
// added and not substituted.
func TestFlowWithoutAmplitudeIsPlainParallel(t *testing.T) {
	base := flat()
	base.Amplitude = 0
	flow := base
	flow.Structure = Flow
	a, b := New(base), New(flow)
	for i := range 50 {
		u, v := float64(i)*0.019, float64(i)*0.037
		if x, y := a.Cover(unit(u, v)), b.Cover(unit(u, v)); math.Abs(x-y) > 1e-12 {
			t.Fatalf("at (%.3f,%.3f): parallel %.9f, flow %.9f", u, v, x, y)
		}
	}
}

// TestToneThinsByHalvingAndLeavesTheMarksWhereTheyWere is the density
// gradient's contract. Stretching the pitch to grade a tone would force the
// marks to split or merge somewhere; halving keeps every surviving mark in
// exactly the place it had at full tone, which is why a graded hatch does
// not appear to slide as it lightens.
func TestToneThinsByHalvingAndLeavesTheMarksWhereTheyWere(t *testing.T) {
	s := flat()
	s.ToneDensity = 3
	h := New(s)
	for _, tc := range []struct {
		tone   float64
		keep   int // one mark in this many survives
		levels string
	}{
		{1, 1, "full tone keeps every mark"},
		{2.0 / 3.0, 2, "one octave down keeps every second"},
		{1.0 / 3.0, 4, "two octaves down keeps every fourth"},
		{0, 8, "three octaves down keeps every eighth"},
	} {
		for k := -8; k <= 8; k++ {
			sm := unit(0.3, float64(k)*0.05)
			sm.Tone = tc.tone
			c := h.Cover(sm)
			want := k%tc.keep == 0
			if got := c > 0.5; got != want {
				t.Errorf("%s: mark %d coverage %.3f (drawn=%v), want drawn=%v",
					tc.levels, k, c, got, want)
			}
		}
	}
}

// TestToneWidthMakesAMarkHeavierWithoutMovingIt is the other tonal lever:
// variable-width hatching. The mark must grow about its own centre, or a
// tonal gradient shears the whole family sideways.
func TestToneWidthMakesAMarkHeavierWithoutMovingIt(t *testing.T) {
	s := flat()
	s.ToneWidth = 1
	s.Softness = 0
	h := New(s)
	prev := -1.0
	for _, tone := range []float64{0.1, 0.4, 0.7, 1.0} {
		w := inkedShare(func(v float64) float64 {
			return h.Cover(Sample{U: 0.3, V: v, CX: 0.5, CY: 0.5, Reach: 0.4, Wall: math.Inf(1), Tone: tone})
		}, 0, 0.05, 20000)
		if w <= prev {
			t.Errorf("tone %g: inked share %.3f did not exceed %.3f", tone, w, prev)
		}
		prev = w
		// The mark centre must not move: v=0 is on it at every tone.
		if c := h.Cover(Sample{U: 0.3, V: 0, CX: 0.5, CY: 0.5, Reach: 0.4, Wall: math.Inf(1), Tone: tone}); c < 0.99 {
			t.Errorf("tone %g: the mark centre lost coverage (%.3f)", tone, c)
		}
	}
}

// TestContinuityIsTheShareOfAMarkThatIsDrawn: broken hatching is measured
// along the mark, not across it, so a dashed hatch must keep the density of
// a solid one in the across direction and lose length only.
func TestContinuityIsTheShareOfAMarkThatIsDrawn(t *testing.T) {
	for _, want := range []float64{0.3, 0.5, 0.8} {
		s := flat()
		s.Continuity = want
		s.Dash = 0.04
		h := New(s)
		// Walk along the mark at v = 0, which is a mark spine.
		got := inkedShare(func(u float64) float64 { return h.Cover(unit(u, 0)) }, 0, 4, 40000)
		if math.Abs(got-want) > 0.03 {
			t.Errorf("continuity %g: drawn share %.3f", want, got)
		}
	}
}

// TestFitPutsTheSameNumberOfMarksInEveryRegion is the alignment knob. Two
// regions of different size must come out with the same number of marks —
// that is what makes hatching read as belonging to the shape it fills
// rather than as a screen laid over it.
func TestFitPutsTheSameNumberOfMarksInEveryRegion(t *testing.T) {
	s := flat()
	s.Fit = 6
	s.Align = AlignRegion
	h := New(s)
	for _, r := range []float64{0.1, 0.25, 0.45} {
		n := 0
		prev := false
		const steps = 4000
		for i := range steps {
			v := 0.5 - r + 2*r*float64(i)/float64(steps-1)
			on := h.Cover(Sample{U: 0.5, V: v, CX: 0.5, CY: 0.5, Reach: r, Wall: math.Inf(1), Tone: 1}) > 0.5
			if on && !prev {
				n++
			}
			prev = on
		}
		if n != 7 { // 6 pitches across 2·Reach spans 7 mark centres
			t.Errorf("reach %g: %d marks across the region, want 7", r, n)
		}
	}
}

// TestAlignRegionGivesEveryRegionItsOwnHatch: under AlignCanvas two
// neighbouring cells share one continuous screen; under AlignRegion each
// carries its own, anchored at its centre. It is the difference between a
// mechanical fill and a hand filling each shape in turn, and it is the only
// thing Sample.CX/CY does for a plain parallel hatch.
func TestAlignRegionGivesEveryRegionItsOwnHatch(t *testing.T) {
	s := flat()
	canvas, region := New(s), New(s.With(func(c *Spec) { c.Align = AlignRegion }))
	a := Sample{U: 0.2, V: 0.2, CX: 0.2, CY: 0.2, Reach: 0.1, Wall: math.Inf(1), Tone: 1}
	b := Sample{U: 0.77, V: 0.63, CX: 0.77, CY: 0.63, Reach: 0.1, Wall: math.Inf(1), Tone: 1}
	if x, y := region.Cover(a), region.Cover(b); math.Abs(x-y) > 1e-9 {
		t.Errorf("region-aligned: centre coverage %.4f vs %.4f — phase is not anchored at the centre", x, y)
	}
	if x, y := canvas.Cover(a), canvas.Cover(b); math.Abs(x-y) < 1e-6 {
		t.Error("canvas-aligned: two unrelated centres landed on the same phase — the test lost its grip")
	}
}

// TestJitterMovesAMarkAndTheLookupStillFindsIt: the coverage lookup rounds
// to the nearest lattice slot, so a displaced mark can end up outside the
// slot the point falls in. Testing three candidates rather than one is what
// keeps a jittered hatch from developing gaps, and this is the test that
// fails if that is ever simplified away.
func TestJitterMovesAMarkAndTheLookupStillFindsIt(t *testing.T) {
	s := flat()
	s.Jitter = 0.4
	s.Softness = 0
	h := New(s)
	for k := -6; k <= 6; k++ {
		off := s.Jitter * s.Spacing * (hash(s.Seed^saltLine, k) - 0.5) * 2
		v := float64(k)*s.Spacing + off
		if c := h.Cover(unit(0.3, v)); c < 0.99 {
			t.Errorf("mark %d displaced to v=%.5f: coverage %.3f, want ~1", k, v, c)
		}
	}
}

// TestChordMarksRunFromBoundaryToBoundary: a chord family's marks must
// touch the region's edge at both ends and stay inside it. A chord hatch
// whose marks stop short is just a radial one with a hole.
func TestChordMarksRunFromBoundaryToBoundary(t *testing.T) {
	s := Defaults()
	s.Structure = Chord
	s.Angle = math.Pi / 2
	s.Spacing = 0.05
	h := New(s)
	const r = 0.4

	touched := 0
	for i := range 2000 {
		th := 2 * math.Pi * float64(i) / 2000
		if h.Cover(radialAt(r*0.995, th, r)) > 0.5 {
			touched++
		}
	}
	if touched == 0 {
		t.Error("no chord reaches the boundary")
	}
	for i := range 400 {
		th := 2 * math.Pi * float64(i) / 400
		if c := h.Cover(radialAt(r*1.1, th, r)); c > 0.001 {
			t.Errorf("coverage %.3f outside the region at angle %.2f", c, th)
		}
	}
	// The envelope: with a quarter-turn chord nothing reaches inside
	// Reach·cos(π/4).
	inner := r*math.Cos(math.Pi/4) - 0.02
	for i := range 400 {
		th := 2 * math.Pi * float64(i) / 400
		if c := h.Cover(radialAt(inner, th, r)); c > 0.001 {
			t.Errorf("coverage %.3f inside the chord envelope at angle %.2f", c, th)
		}
	}
}

func radialAt(rr, theta, reach float64) Sample {
	return Sample{
		U: 0.5 + rr*math.Cos(theta), V: 0.5 + rr*math.Sin(theta),
		CX: 0.5, CY: 0.5, Reach: reach, Wall: math.Inf(1), Tone: 1,
	}
}

// TestEveryStructureStaysWithinTheUnitRange is the blanket guard: coverage
// is a fraction, and a caller lerping a colour with it must never be handed
// a NaN or a value outside [0,1], whatever region description it passes —
// including the degenerate ones (a point at the centre, no reach, no wall).
func TestEveryStructureStaysWithinTheUnitRange(t *testing.T) {
	for _, st := range allStructures() {
		for _, wf := range []Waveform{Straight, Sine, Zigzag} {
			s := Defaults()
			s.Structure, s.Waveform = st, wf
			s.Amplitude, s.Jitter, s.Continuity = 2, 0.3, 0.6
			s.ToneDensity, s.ToneWidth, s.Curvature = 2, 0.8, 1.5
			s.Seed = 99
			h := New(s)
			for j := range 60 {
				for i := range 60 {
					u, v := float64(i)/59, float64(j)/59
					for _, sm := range []Sample{
						{U: u, V: v, CX: 0.5, CY: 0.5, Reach: 0.4, Wall: dist(u, v), Tone: u},
						{U: u, V: v},                              // no region at all
						{U: 0.5, V: 0.5, CX: 0.5, CY: 0.5},        // exactly on the centre
						{U: u, V: v, Reach: -1, Wall: math.NaN()}, // nonsense in
					} {
						c := h.Cover(sm)
						if math.IsNaN(c) || c < 0 || c > 1 {
							t.Fatalf("%s/%s: coverage %v at %+v", st, wf, c, sm)
						}
					}
				}
			}
		}
	}
}

// TestTheSameSpecAlwaysDrawsTheSameHatch: randomness comes from hashes of
// the seed and the position, never from a generator, which is what lets one
// Hatch serve the parallel pixel loop (invariant 1).
func TestTheSameSpecAlwaysDrawsTheSameHatch(t *testing.T) {
	s := Defaults()
	s.Structure, s.Jitter, s.Continuity, s.Seed = Flow, 0.3, 0.5, 12345
	a, b := New(s), New(s)
	s.Seed = 12346
	other := New(s)
	same := true
	for i := range 500 {
		u, v := float64(i%25)/25, float64(i/25)/20
		if a.Cover(unit(u, v)) != b.Cover(unit(u, v)) {
			t.Fatalf("two hatches from one spec differ at (%g,%g)", u, v)
		}
		if a.Cover(unit(u, v)) != other.Cover(unit(u, v)) {
			same = false
		}
	}
	if same {
		t.Error("a different seed drew the identical hatch")
	}
}

// TestStructureNamesRoundTrip keeps the CLI and manifest names in step with
// the constants; a mismatch silently renames a specimen.
func TestStructureNamesRoundTrip(t *testing.T) {
	for i, n := range StructureNames() {
		got, ok := StructureByName(n)
		if !ok || int(got) != i {
			t.Errorf("%q resolved to %v (ok=%v), want %d", n, got, ok, i)
		}
		if Structure(i).String() != n {
			t.Errorf("Structure(%d).String() = %q, want %q", i, Structure(i).String(), n)
		}
	}
	if _, ok := StructureByName("spiral"); ok {
		t.Error("an unknown structure name resolved")
	}
}

// --- helpers -------------------------------------------------------------

func allStructures() []Structure {
	out := make([]Structure, len(StructureNames()))
	for i := range out {
		out[i] = Structure(i)
	}
	return out
}

// dist is a stand-in wall distance: how far the point is from the edge of
// the unit square, which is a real region a caller could have.
func dist(u, v float64) float64 {
	return math.Min(math.Min(u, 1-u), math.Min(v, 1-v))
}

// inkedShare is the fraction of [lo, hi) where f is inked.
func inkedShare(f func(float64) float64, lo, hi float64, steps int) float64 {
	n := 0
	for i := range steps {
		x := lo + (hi-lo)*float64(i)/float64(steps)
		if f(x) > 0.5 {
			n++
		}
	}
	return float64(n) / float64(steps)
}

// boxAverage is the mean coverage over pixel (i, j) of an n×n grid, taken
// with sub×sub samples — the renderer's own supersampling, in miniature.
func boxAverage(f func(u, v float64) float64, i, j, n, sub int) float64 {
	sum := 0.0
	for b := range sub {
		for a := range sub {
			u := (float64(i) + (float64(a)+0.5)/float64(sub)) / float64(n)
			v := (float64(j) + (float64(b)+0.5)/float64(sub)) / float64(n)
			sum += f(u, v)
		}
	}
	return sum / float64(sub*sub)
}

// findMark walks across the family at a given distance along it and returns
// the centre of the mark nearest a given position — the coverage-weighted
// centroid over a window narrower than one pitch, so a saturated mark gives
// its middle rather than its leading edge.
func findMark(t *testing.T, h *Hatch, along, near, window float64) float64 {
	t.Helper()
	sum, weight := 0.0, 0.0
	const steps = 20001
	for i := range steps {
		w := near - window + 2*window*float64(i)/(steps-1)
		c := h.Cover(unit(along, w))
		sum += c * w
		weight += c
	}
	if weight == 0 {
		t.Fatalf("no mark found near across=%g at along=%g", near, along)
	}
	return sum / weight
}
