package circles

import (
	"flag"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("degas-milliner")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, New(), testCtx(t, 42), testCtx(t, 43))
}

func TestPacking(t *testing.T) {
	s := New()
	for seed := uint64(0); seed < 20; seed++ {
		p := s.plan(testCtx(t, seed))
		if len(p.circles) < 80 {
			t.Fatalf("seed %d: only %d circles packed", seed, len(p.circles))
		}
		for i := range p.circles {
			a := &p.circles[i]
			if a.R < s.MinR-1e-12 || a.R > s.MaxR+1e-12 {
				t.Fatalf("seed %d: radius %v out of range", seed, a.R)
			}
			if a.X-a.R < 0 || a.X+a.R > 1 || a.Y-a.R < 0 || a.Y+a.R > 1 {
				t.Fatalf("seed %d: circle %d leaves the canvas", seed, i)
			}
			for j := i + 1; j < len(p.circles); j++ {
				b := &p.circles[j]
				dx, dy := a.X-b.X, a.Y-b.Y
				if math.Sqrt(dx*dx+dy*dy) < a.R+b.R+s.Gap-1e-9 {
					t.Fatalf("seed %d: circles %d and %d overlap", seed, i, j)
				}
			}
		}
	}
}

func TestPlanBounds(t *testing.T) {
	s := New()
	for seed := uint64(0); seed < 20; seed++ {
		p := s.plan(testCtx(t, seed))
		kinds := map[fillKind]int{}
		for i := range p.circles {
			c := &p.circles[i]
			kinds[c.kind]++
			if len(c.shades) < 3 || len(c.shades) > 5 {
				t.Fatalf("seed %d: %d shades", seed, len(c.shades))
			}
			if c.scale <= 0 || c.scale > 1 {
				t.Fatalf("seed %d: scale %v out of range", seed, c.scale)
			}
			if feature := c.scale * c.R; feature < 0.0079 && c.scale < 1 {
				t.Fatalf("seed %d: feature size %v below floor", seed, feature)
			}
			if c.kind == fillDots && (c.dotSize < 0.55 || c.dotSize > 0.8) {
				t.Fatalf("seed %d: dot size %v out of range", seed, c.dotSize)
			}
		}
		if len(kinds) < 3 {
			t.Fatalf("seed %d: only %d fill kinds present", seed, len(kinds))
		}
	}
}

func TestGridMatchesBruteForce(t *testing.T) {
	s := New()
	p := s.plan(testCtx(t, 7))
	for i := range 500 {
		u := float64(i%25) / 24
		v := float64(i/25) / 19.96
		got := p.index.At(u, v)
		want := -1
		for j := range p.circles {
			c := &p.circles[j]
			dx, dy := u-c.X, v-c.Y
			if dx*dx+dy*dy <= c.R*c.R {
				want = j
				break
			}
		}
		if got != want {
			t.Fatalf("at (%v, %v): grid %d, brute force %d", u, v, got, want)
		}
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/circles_seed42_64.png", *update)
}
