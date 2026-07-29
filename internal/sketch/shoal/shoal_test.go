package shoal

import (
	"flag"
	"math"
	"sort"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("hopper-night-windows")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, New(), testCtx(t, 42), testCtx(t, 43))
}

// planner builds the layout state for one seed on a 1×1 canvas.
func newTestPlanner(t *testing.T, s *Sketch, seed uint64) *planner {
	t.Helper()
	ctx := testCtx(t, seed)
	byLum := append([]palette.Color(nil), ctx.Palette.Colors...)
	sort.SliceStable(byLum, func(i, j int) bool {
		return byLum[i].Luminance() < byLum[j].Luminance()
	})
	_, inks, ink := s.palette(byLum, ctx.RNG(streamPaint))
	return newPlanner(s, ctx, 1, inks, ink)
}

// plan runs the layout for one seed and returns the dots.
func plan(t *testing.T, s *Sketch, seed uint64) []dot {
	t.Helper()
	return newTestPlanner(t, s, seed).run()
}

func TestPlanStaysInBoundsAndApart(t *testing.T) {
	for _, f := range []Field{FieldCurl, FieldFlow, FieldRidge} {
		for seed := uint64(1); seed <= 6; seed++ {
			s := New()
			s.Field = f
			dots := plan(t, s, seed)
			if len(dots) < 100 {
				t.Fatalf("field %d seed %d: only %d dots, layout is starving", f, seed, len(dots))
			}
			for _, d := range dots {
				if d.R < s.MinR-1e-9 || d.R > s.MaxR+1e-9 {
					t.Fatalf("radius %v outside [%v, %v]", d.R, s.MinR, s.MaxR)
				}
				if d.X-d.R < s.Margin-1e-9 || d.X+d.R > 1-s.Margin+1e-9 ||
					d.Y-d.R < s.Margin-1e-9 || d.Y+d.R > 1-s.Margin+1e-9 {
					t.Fatalf("dot %v breaks the margin", d.Circle)
				}
			}
			// Dots must not overlap: the field is a packing, and an
			// overlap means the collision index was bypassed.
			for i := range dots {
				for j := i + 1; j < len(dots); j++ {
					a, b := dots[i], dots[j]
					if math.Hypot(a.X-b.X, a.Y-b.Y) < a.R+b.R-1e-9 {
						t.Fatalf("dots %v and %v overlap", a.Circle, b.Circle)
					}
				}
			}
		}
	}
}

// TestCulledChainsLeaveNoGhosts guards the bug where a stub chain was
// inserted into the collision index and then dropped from the output: it
// went on blocking ground that nothing occupied, and enough of those
// starved the field so that adding seeds stopped adding dots. The index
// and the returned dots must describe the same set of circles.
func TestCulledChainsLeaveNoGhosts(t *testing.T) {
	s := New()
	s.MinChain = 8 // cull aggressively, so ghosts would be plentiful
	p := newTestPlanner(t, s, 9)
	dots := p.run()
	if got, want := len(p.index.Circles()), len(dots); got != want {
		t.Errorf("index holds %d circles but %d dots were returned — %d ghosts blocking ground", got, want, got-want)
	}
	// The check above is only meaningful if chains were actually culled.
	loose := New()
	loose.MinChain = 1
	if a, b := len(dots), len(plan(t, loose, 9)); a >= b {
		t.Fatalf("MinChain=8 kept %d dots and MinChain=1 kept %d — nothing was culled, so the test proves nothing", a, b)
	}
}

func TestColorRunsAlongChains(t *testing.T) {
	// With colour chosen per chain, consecutive dots usually share it.
	// Per-dot choice would drop agreement to roughly 1/len(inks).
	dots := plan(t, New(), 4)
	same := 0
	for i := 1; i < len(dots); i++ {
		if dots[i].main == dots[i-1].main {
			same++
		}
	}
	if frac := float64(same) / float64(len(dots)-1); frac < 0.5 {
		t.Errorf("only %.0f%% of neighbouring dots share a colour, want ≥50%% — colour is not running along chains", frac*100)
	}
}

func TestFieldAndGradeChangeOutput(t *testing.T) {
	base := sketchtest.RenderNRGBA(t, New(), testCtx(t, 42))
	for _, tc := range []struct {
		name string
		mut  func(*Sketch)
	}{
		{"curl", func(s *Sketch) { s.Field = FieldCurl }},
		{"ridge", func(s *Sketch) { s.Field = FieldRidge }},
		{"patches", func(s *Sketch) { s.Grade = GradePatches }},
		{"ribbon", func(s *Sketch) { s.Mark = MarkRibbon }},
		{"mixed", func(s *Sketch) { s.Mark = MarkMixed }},
		{"wash", func(s *Sketch) { s.Mark = MarkWash }},
		{"dark", func(s *Sketch) { s.Ground = GroundDark }},
		{"mono", func(s *Sketch) { s.Mono = true }},
		{"overlap", func(s *Sketch) { s.Overlap = 0.5 }},
	} {
		s := New()
		tc.mut(s)
		got := sketchtest.RenderNRGBA(t, s, testCtx(t, 42))
		if string(got.Pix) == string(base.Pix) {
			t.Errorf("%s changed nothing", tc.name)
		}
	}
}

// TestOverlapLetsMarksCrowd checks that the knob does what it says: with
// Overlap on, marks may run into each other, and the default keeps them
// strictly apart (which TestPlanStaysInBoundsAndApart relies on).
func TestOverlapLetsMarksCrowd(t *testing.T) {
	s := New()
	s.Overlap = 0.6
	dots := plan(t, s, 5)
	crowded := 0
	for i := range dots {
		for j := i + 1; j < len(dots); j++ {
			a, b := dots[i], dots[j]
			if math.Hypot(a.X-b.X, a.Y-b.Y) < a.R+b.R {
				crowded++
			}
		}
	}
	if crowded == 0 {
		t.Error("Overlap 0.6 produced no overlapping marks")
	}
}

// TestRunsGroupByChainAndColour guards the unit a ribbon is painted as:
// a run must never span two chains or two colours, or a stroke would jump
// across the canvas between unrelated dots.
func TestRunsGroupByChainAndColour(t *testing.T) {
	s := New()
	dots := plan(t, s, 5)
	total := 0
	for _, run := range s.runs(dots) {
		total += len(run)
		for _, d := range run {
			if d.chain != run[0].chain || d.main != run[0].main {
				t.Fatalf("run mixes chains or colours: %v vs %v", d, run[0])
			}
		}
	}
	if total != len(dots) {
		t.Errorf("runs cover %d dots, want all %d", total, len(dots))
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/shoal_seed42_64.png", *update)
}
