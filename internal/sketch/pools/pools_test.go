package pools

import (
	"flag"
	"io"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

// fills is every level of the fill trait, so tests sweep the whole axis
// rather than whichever one seed 42 happens to draw.
var fills = []string{"sparse", "open", "medium", "busy", "packed"}

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("tchelitchew-hide-and-seek")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal}
}

// configured returns a sketch with its options resolved, optionally pinning
// flags the way the CLI would. Tests go through the real path rather than
// setting fields, because which flags were *given* is what tells an
// override from what the seed's fill level drew.
func configured(t *testing.T, args ...string) *Sketch {
	t.Helper()
	s := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Configure(); err != nil {
		t.Fatal(err)
	}
	return s
}

// plan runs the layout for one seed on a 1×1 canvas.
func plan(t *testing.T, s *Sketch, seed uint64) []circle {
	t.Helper()
	ctx := testCtx(t, seed)
	rng := ctx.RNG(streamLayout)
	l := s.layoutFor(s.Traits(ctx), ctx.RNG(streamFill))
	_, _, bag, ramp := s.inks(palette.ByLuminance(ctx.Palette.Colors), rng)
	return s.plan(l, rng, 1, bag, ramp)
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 42), testCtx(t, 43))
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/pools_seed42_96.png", *update)
}

func TestPlanStaysOnTheLadderAndOnThePaper(t *testing.T) {
	// Every level of the fill axis, since each draws its own ladder.
	for seed := uint64(1); seed <= 12; seed++ {
		s := configured(t, "--fill", fills[int(seed)%len(fills)])
		ctx := testCtx(t, seed)
		radii, _ := s.layoutFor(s.Traits(ctx), ctx.RNG(streamFill)).ladder()
		onLadder := func(r float64) bool {
			for _, v := range radii {
				if math.Abs(v-r) < 1e-9 {
					return true
				}
			}
			// The one exception: a bottom-rung anchor's companion, which has
			// no rung below it to come from.
			return math.Abs(r-radii[0]*0.72) < 1e-9
		}
		cs := plan(t, s, seed)
		if len(cs) < 3 {
			t.Fatalf("seed %d: only %d circles — the layout is starving", seed, len(cs))
		}
		for _, c := range cs {
			if !onLadder(c.R) {
				t.Fatalf("seed %d: radius %v is not a rung of the ladder %v", seed, c.R, radii)
			}
			// Satellites are allowed the tighter half-margin; nothing may
			// be clipped by the edge of the paper.
			if c.X-c.R < 0 || c.X+c.R > 1 || c.Y-c.R < 0 || c.Y+c.R > 1 {
				t.Fatalf("seed %d: circle %v runs off the paper", seed, c.Circle)
			}
		}
	}
}

// TestLargestPaintedFirst pins the paint order: transparent marks stack,
// so a small disc laid after a large one settles on top of it the way a
// later touch of a brush does. Reversed, every small mark is buried.
func TestLargestPaintedFirst(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		cs := plan(t, configured(t), seed)
		for i := 1; i < len(cs); i++ {
			if cs[i].R > cs[i-1].R+1e-9 {
				t.Fatalf("seed %d: circle %d (r=%v) is painted after a smaller one (r=%v)",
					seed, i, cs[i].R, cs[i-1].R)
			}
		}
	}
}

// TestSatellitesActuallyCross is the reason satellites are placed by hand
// rather than left to the packing rule: the crossings are the subject, so
// a run has to contain some. A companion is only worth placing if it
// genuinely overlaps its parent.
func TestSatellitesActuallyCross(t *testing.T) {
	crossings := 0
	for seed := uint64(1); seed <= 8; seed++ {
		cs := plan(t, configured(t), seed)
		for i := range cs {
			for j := i + 1; j < len(cs); j++ {
				d := math.Hypot(cs[i].X-cs[j].X, cs[i].Y-cs[j].Y)
				// Strictly inside both: touching rims do not mix pigment.
				if d < cs[i].R+cs[j].R-1e-9 && d > math.Abs(cs[i].R-cs[j].R) {
					crossings++
				}
			}
		}
	}
	if crossings < 8 {
		t.Errorf("only %d crossings over 8 seeds — the pools never mix", crossings)
	}
}

// TestOpenRingsKeepTheirHole guards the annulus band against swallowing
// the middle: an open mark whose band reaches the centre is just a disc.
func TestOpenRingsKeepTheirHole(t *testing.T) {
	seen := 0
	for seed := uint64(1); seed <= 12; seed++ {
		for _, c := range plan(t, New(), seed) {
			if c.kind != kindOpen {
				continue
			}
			seen++
			if c.band >= c.R {
				t.Fatalf("seed %d: open ring r=%v has band %v — no hole left", seed, c.R, c.band)
			}
		}
	}
	if seen == 0 {
		t.Error("no open rings in twelve seeds")
	}
}

func TestConfigureRejectsOutOfRange(t *testing.T) {
	for _, args := range [][]string{
		{"--alpha", "2"},
		{"--ragged", "0.9"},
		{"--count", "-2"},
		{"--ratio", "0.5"},
		{"--pigments", "99"},
	} {
		s := New()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		s.Flags(fs)
		if err := fs.Parse(args); err != nil {
			continue // the flag package rejected it first, which is fine
		}
		if _, err := s.Configure(); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
}
