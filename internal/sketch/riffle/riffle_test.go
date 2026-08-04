package riffle

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

// reaches is every level of the energy axis, so the tests sweep the whole
// dimension rather than whichever one seed 42 happens to draw.
var reaches = []string{"pool", "glide", "run", "riffle", "rapid", "cascade"}

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("hokusai-great-wave")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal}
}

// configured returns a sketch with its options resolved, optionally pinning
// flags the way the CLI would. Tests go through the real path rather than
// setting fields, because which flags were *given* is what tells an override
// from what the seed's traits drew.
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

func built(t *testing.T, s *Sketch, seed uint64) *plan {
	t.Helper()
	p, err := s.build(testCtx(t, seed))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 42), testCtx(t, 43))
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/riffle_seed42_96.png", *update)
}

func TestOverlayIsAnAllWaterAlphaLayer(t *testing.T) {
	s := configured(t,
		"--medium", "overlay",
		"--ground", "transparent",
		"--boulders", "field",
	)
	p := built(t, s, 42)
	if len(p.rocks) != 0 {
		t.Fatalf("overlay retained %d physical rocks", len(p.rocks))
	}
	if len(p.dots) == 0 {
		t.Fatal("overlay lost the washed-dot placements")
	}
	for j := range 40 {
		for i := range 40 {
			u := (float64(i) + 0.5) / 40 * p.aspect
			v := (float64(j) + 0.5) / 40
			if depth := p.read(u, v).depth; depth <= 0 {
				t.Fatalf("overlay exposed a bank at (%v,%v), depth %v", u, v, depth)
			}
		}
	}

	img := sketchtest.RenderNRGBA(t, s, testCtx(t, 42))
	lo, hi := uint8(255), uint8(0)
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			px := img.NRGBAAt(x, y)
			lo = min(lo, px.A)
			hi = max(hi, px.A)
		}
	}
	if lo == 0 || hi == 255 || lo == hi {
		t.Fatalf("overlay alpha range = %d..%d, want varied intermediate alpha", lo, hi)
	}
	center := img.NRGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
	if center.B <= center.R {
		t.Fatalf("overlay center = %#v, want a blue-dominant layer", center)
	}
}

func TestOverlayGrayGroundIsOpaque(t *testing.T) {
	s := configured(t, "--medium", "overlay", "--ground", "gray-mid")
	img := sketchtest.RenderNRGBA(t, s, testCtx(t, 42))
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if a := img.NRGBAAt(x, y).A; a != 255 {
				t.Fatalf("gray preview alpha at (%d,%d) = %d, want 255", x, y, a)
			}
		}
	}
}

// TestSchemaIsValid pins the shape of the output space. If it fails, either a
// dimension has lost its weights (and seeds stop reaching part of the space)
// or two dimensions have collided on a filename key.
func TestSchemaIsValid(t *testing.T) {
	if err := schema.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := len(schema); got != 6 {
		t.Fatalf("schema has %d dimensions, want 6", got)
	}
	// cascade and from-flag are deliberate departures, reachable only by an
	// explicit override. A weight above zero would put them in every sweep.
	for _, unreachable := range []struct{ dim, value string }{
		{dimReach, "cascade"},
		{dimColourway, fromFlag},
	} {
		d, ok := schema.Dim(unreachable.dim)
		if !ok {
			t.Fatalf("no dimension %q", unreachable.dim)
		}
		if w := d.Weight(unreachable.value); w != 0 {
			t.Errorf("%s=%s carries weight %v, want 0", unreachable.dim, unreachable.value, w)
		}
	}
}

// TestFlagsDoNotShadowTheRenderFlags guards the flat CLI namespace: a sketch's
// options are registered on the *same* FlagSet as the render command's, so a
// collision is a panic at startup rather than a compile error.
func TestFlagsDoNotShadowTheRenderFlags(t *testing.T) {
	reserved := map[string]bool{
		"profile": true, "width": true, "height": true, "seed": true,
		"aa": true, "deep": true, "palette": true, "format": true, "out": true,
	}
	s := configured(t)
	for _, name := range s.knobs.Names() {
		if reserved[name] {
			t.Errorf("knob --%s shadows a render flag", name)
		}
	}
	for _, d := range schema {
		if reserved[d.Name] {
			t.Errorf("trait --%s shadows a render flag", d.Name)
		}
	}
}

// TestEveryReachIsMostlyWater is the plan-bounds test. A river whose frame is
// all dry gravel, or one whose channel never reaches a bank, is a failure of
// the depth field that no amount of surface detail rescues — and both are one
// bad range in reachLevel or channelLevel away.
func TestEveryReachIsMostlyWater(t *testing.T) {
	for _, reach := range reaches {
		for seed := uint64(1); seed <= 8; seed++ {
			s := configured(t, "--reach", reach)
			p := built(t, s, seed)
			wet, n := 0, 0
			for i := range 40 {
				for j := range 40 {
					u := (float64(i) + 0.5) / 40 * p.aspect
					v := (float64(j) + 0.5) / 40
					d := p.read(u, v).depth
					if math.IsNaN(d) || math.IsInf(d, 0) {
						t.Fatalf("%s seed %d: depth is %v at (%v,%v)", reach, seed, d, u, v)
					}
					if d > 0 {
						wet++
					}
					n++
				}
			}
			if share := float64(wet) / float64(n); share < 0.35 {
				t.Errorf("%s seed %d: only %.0f%% of the frame is water", reach, seed, 100*share)
			}
		}
	}
}

// TestARiffleBreaksWhereItShallows defends the single expression the whole
// sketch turns on. Water breaks at high Froude number, and the pool–riffle
// term makes the crest shallow *and* fast at once — so if the mean Froude on
// the crests ever stops exceeding the mean in the pools, the white water has
// come unstuck from the bed and will scatter evenly over the sheet.
func TestARiffleBreaksWhereItShallows(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		s := configured(t, "--reach", "riffle", "--boulders", "clear")
		p := built(t, s, seed)
		var crest, pool, nc, np float64
		for i := range 60 {
			for j := range 60 {
				u := (float64(i) + 0.5) / 60 * p.aspect
				v := (float64(j) + 0.5) / 60
				r := p.read(u, v)
				if r.depth <= 0.05 || math.Abs(r.x) > 0.6 {
					continue // banks are slow for reasons of their own
				}
				fu, fv := p.velocity(u, v, r)
				f := froude(math.Hypot(fu, fv), r.depth)
				switch {
				case r.grade < 0.8:
					crest, nc = crest+f, nc+1
				case r.grade > 1.2:
					pool, np = pool+f, np+1
				}
			}
		}
		if nc == 0 || np == 0 {
			t.Fatalf("seed %d: the pool–riffle sequence produced no crest or no pool", seed)
		}
		if crest/nc <= pool/np*1.5 {
			t.Errorf("seed %d: crest Froude %.2f is not clearly above pool Froude %.2f",
				seed, crest/nc, pool/np)
		}
	}
}

// TestFoamStaysOffMostOfTheSheet is a regression fence around the failure
// this sketch spent longest on. Foam thresholds were absolute, and because
// depth and speed are in arbitrary units a value that left a pool clean
// turned a riffle entirely white. They are relative to the reach's own
// nominal Froude now; if that comes undone, this catches it before a sweep
// does.
func TestFoamStaysOffMostOfTheSheet(t *testing.T) {
	for _, reach := range []string{"pool", "glide", "run", "riffle", "rapid"} {
		for seed := uint64(1); seed <= 4; seed++ {
			s := configured(t, "--reach", reach)
			p := built(t, s, seed)
			white, n := 0, 0
			for i := range 32 {
				for j := range 32 {
					u := (float64(i) + 0.5) / 32 * p.aspect
					v := (float64(j) + 0.5) / 32
					r := p.read(u, v)
					if r.depth <= 0 {
						continue
					}
					n++
					if w := p.upstream(u, v, r); w.foam > p.foamFull {
						white++
					}
				}
			}
			if n == 0 {
				continue
			}
			if share := float64(white) / float64(n); share > 0.30 {
				t.Errorf("%s seed %d: %.0f%% of the water is solid foam", reach, seed, 100*share)
			}
		}
	}
}

// TestDryGravelHasNoRiverOnIt: wetness is the one gate every consumer shares,
// so a bar has to come out with no current, no streaks and no foam on it. If
// this fails, an exposed gravel bar will have the water's texture laid over
// it and stop reading as land.
func TestDryGravelHasNoRiverOnIt(t *testing.T) {
	s := configured(t, "--channel", "bar")
	for seed := uint64(1); seed <= 6; seed++ {
		p := built(t, s, seed)
		found := 0
		for i := range 50 {
			for j := range 50 {
				u := (float64(i) + 0.5) / 50 * p.aspect
				v := (float64(j) + 0.5) / 50
				r := p.read(u, v)
				if r.depth > -0.02 {
					continue
				}
				found++
				fu, fv := p.velocity(u, v, r)
				if sp := math.Hypot(fu, fv); sp > 1e-9 {
					t.Fatalf("seed %d: dry gravel at (%v,%v) is flowing at %v", seed, u, v, sp)
				}
				if f := p.foamSource(u, v, r, 0); f != 0 {
					t.Fatalf("seed %d: dry gravel at (%v,%v) is making foam (%v)", seed, u, v, f)
				}
			}
		}
		if found == 0 {
			t.Errorf("seed %d: --channel bar exposed no dry gravel at all", seed)
		}
	}
}

// TestABoulderSitsInWaterAndKeepsItsDistance: a rock is placed on land is
// invisible, and two rocks that touch merge into one lumpy dome and stop
// reading as separate objects. Both are properties of the pack, not of the
// render, so they are checked here rather than by eye.
func TestABoulderSitsInWaterAndKeepsItsDistance(t *testing.T) {
	for seed := uint64(1); seed <= 8; seed++ {
		s := configured(t, "--boulders", "field")
		p := built(t, s, seed)
		if len(p.rocks) < 3 {
			t.Fatalf("seed %d: --boulders field placed only %d rocks", seed, len(p.rocks))
		}
		for i, r := range p.rocks {
			if p.bareDepth(r.X, r.Y) <= 0 {
				t.Errorf("seed %d: rock %d is planted on dry land", seed, i)
			}
			if math.Abs(math.Hypot(r.DU, r.DV)-1) > 1e-9 {
				t.Errorf("seed %d: rock %d has a downstream direction of length %v",
					seed, i, math.Hypot(r.DU, r.DV))
			}
			for j := i + 1; j < len(p.rocks); j++ {
				o := p.rocks[j]
				// A mid-channel bar is placed first and deliberately large,
				// and boulders are allowed to sit on the shoal beside it.
				if r.Long > 2 || o.Long > 2 {
					continue
				}
				if math.Hypot(r.X-o.X, r.Y-o.Y) < 0.35*(r.area()+o.area()) {
					t.Errorf("seed %d: rocks %d and %d are on top of one another", seed, i, j)
				}
			}
		}
	}
}

// TestThePictureIsTheSameAtAnyResolution is invariant 2, checked where it can
// actually be checked: the pixel function is pure in normalized coordinates,
// so a preview-sized plan and a print-sized plan of one seed must agree
// exactly at the same (u, v). A frequency taken per pixel rather than per
// canvas unit fails here and nowhere else until someone orders a print.
func TestThePictureIsTheSameAtAnyResolution(t *testing.T) {
	s := configured(t)
	small, err := s.build(sketch.Context{Width: 300, Height: 300, Seed: 7, Palette: testCtx(t, 7).Palette})
	if err != nil {
		t.Fatal(err)
	}
	big, err := s.build(sketch.Context{Width: 4800, Height: 4800, Seed: 7, Palette: testCtx(t, 7).Palette})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 17 {
		for j := range 17 {
			u := (float64(i) + 0.5) / 17
			v := (float64(j) + 0.5) / 17
			a, b := small.pixel(u, v), big.pixel(u, v)
			if math.Abs(a.R-b.R)+math.Abs(a.G-b.G)+math.Abs(a.B-b.B) > 1e-12 {
				t.Fatalf("(%v,%v): preview says %v, print says %v", u, v, a, b)
			}
		}
	}
}

// TestAKnobOverridesOnlyItselfAndOnlyWhenGiven: the trait draws every number,
// and a flag lays one value over the top. A knob that applies when it was not
// given would silently pin that value for every seed, which is exactly the
// bug the sentinel-default approach used to have.
func TestAKnobOverridesOnlyItselfAndOnlyWhenGiven(t *testing.T) {
	ctx := testCtx(t, 11)
	free := configured(t)
	base := free.settingsFor(free.Traits(ctx), ctx.RNG(streamReach))

	pinned := configured(t, "--speed", "1.75")
	got := pinned.settingsFor(pinned.Traits(ctx), ctx.RNG(streamReach))
	if got.speed != 1.75 {
		t.Errorf("--speed 1.75 gave %v", got.speed)
	}
	if got.depth != base.depth || got.foam != base.foam || got.rocks != base.rocks {
		t.Error("pinning --speed disturbed what the seed drew for everything else")
	}
	if base.speed == 1.75 {
		t.Error("the unpinned draw happened to be the pinned value; pick another")
	}
}
