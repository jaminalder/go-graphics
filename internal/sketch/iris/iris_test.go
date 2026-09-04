package iris

import (
	"flag"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

var fixedSeeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34}

func testCtx(t testing.TB, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("vermeer-pearl-earring")
	if !ok {
		t.Fatal("vermeer-pearl-earring palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

// configured builds a sketch with CLI arguments applied, the way cmd does.
func configured(t testing.TB, args ...string) *Sketch {
	t.Helper()
	s := New()
	fs := flag.NewFlagSet("iris", flag.ContinueOnError)
	s.Flags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Configure(); err != nil {
		t.Fatal(err)
	}
	return s
}

func planFor(t testing.TB, s *Sketch, ctx sketch.Context) plan {
	t.Helper()
	p, err := s.plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSameSeedDrawsTheSameIris(t *testing.T) {
	sketchtest.AssertDeterministic(t, New(), testCtx(t, 7), testCtx(t, 8))
}

// The remap sends the stroma round a closed circle in noise space, so the
// coordinate the fields are read at must be continuous on the negative x axis,
// where a naive fbm(angle, radius) formulation would tear. If this fails, the
// seam is back and every field in the sketch has a visible join.
func TestTheRemapIsContinuousAcrossTheAngularSeam(t *testing.T) {
	for _, seed := range fixedSeeds {
		p := planFor(t, New(), testCtx(t, seed))
		const step = 1e-5
		above := p.remap(-1, step, 0.5)
		below := p.remap(-1, -step, 0.5)
		for _, d := range []float64{above.warpX - below.warpX, above.warpY - below.warpY} {
			if math.Abs(d) > 1e-3 {
				t.Fatalf("seed %d: warped coordinate jumps by %v across the seam", seed, d)
			}
		}
	}
}

// And the same must hold of the finished pixel: a step across the seam may not
// be any larger than the same step taken anywhere else on the ring.
func TestTheSeamIsNoSharperThanAnyOtherAngle(t *testing.T) {
	p := planFor(t, configured(t, "--structure", "fiber"), testCtx(t, 2))
	radius := p.set.limbus * (p.set.pupil + 1) / 2 // mid-annulus
	const step = 1e-4

	delta := func(angle float64) float64 {
		cos, sin := math.Cos(angle), math.Sin(angle)
		normalX, normalY := -sin*step, cos*step
		a := p.At(p.centreU+cos*radius+normalX, p.centreV+sin*radius+normalY)
		b := p.At(p.centreU+cos*radius-normalX, p.centreV+sin*radius-normalY)
		return math.Abs(a.R-b.R) + math.Abs(a.G-b.G) + math.Abs(a.B-b.B)
	}

	elsewhere := 0.0
	for i := range 64 {
		elsewhere = math.Max(elsewhere, delta(float64(i)*math.Pi/32.5))
	}
	if seam := delta(math.Pi); seam > elsewhere {
		t.Errorf("step across the seam is %v, larger than the %v seen anywhere else", seam, elsewhere)
	}
}

// A twist bends the field, not the geometry: however hard the stroma is wound,
// the disc must stay a circle of exactly the drawn radius.
func TestTwistDoesNotMoveTheRim(t *testing.T) {
	s := configured(t, "--twist", "3.5")
	p := planFor(t, s, testCtx(t, 3))
	for _, angle := range []float64{0, 0.7, 2.4, 3.9, 5.6} {
		inside := p.set.limbus * 0.985
		outside := p.set.limbus * 1.02
		cos, sin := math.Cos(angle), math.Sin(angle)
		if got := p.At(p.centreU+cos*outside, p.centreV+sin*outside); got != p.groundColor {
			t.Errorf("angle %v: point outside the limbus is %v, want ground %v", angle, got, p.groundColor)
		}
		if got := p.At(p.centreU+cos*inside, p.centreV+sin*inside); got == p.groundColor {
			t.Errorf("angle %v: point inside the limbus reads as ground", angle)
		}
	}
}

// The plan holds no pixel dimensions, so two equal-aspect canvases must sample
// the same field at the same normalized coordinate.
func TestEqualAspectRendersSampleTheSameField(t *testing.T) {
	small := planFor(t, New(), testCtx(t, 5))
	large := testCtx(t, 5)
	large.Width, large.Height = 1024, 1024
	big := planFor(t, New(), large)
	for _, point := range [][2]float64{{0.5, 0.5}, {0.37, 0.61}, {0.28, 0.44}, {0.72, 0.31}} {
		if got, want := big.At(point[0], point[1]), small.At(point[0], point[1]); got != want {
			t.Errorf("at %v: 1024px render gives %v, 64px gives %v", point, got, want)
		}
	}
}

// Pinning one number must replace exactly that number. If a pin also shifted
// the RNG, steering a composition would silently redraw the whole iris.
func TestPinningOneFlagLeavesTheRestOfTheDrawAlone(t *testing.T) {
	ctx := testCtx(t, 11)
	base := planFor(t, New(), ctx).set
	pinned := planFor(t, configured(t, "--twist", "2.75"), ctx).set

	if pinned.twist != 2.75 {
		t.Fatalf("twist %v, want the pinned 2.75", pinned.twist)
	}
	base.twist = pinned.twist
	if pinned != base {
		t.Errorf("pinning twist also changed the recipe:\n got %+v\nwant %+v", pinned, base)
	}
}

// Traits come from their own stream, so a trait override must not move the
// numbers either.
func TestOverridingATraitKeepsTheOtherDimensions(t *testing.T) {
	ctx := testCtx(t, 4)
	base := New().Traits(ctx)
	forced := configured(t, "--structure", "crypt").Traits(ctx)
	if forced.Get(dimStructure) != "crypt" {
		t.Fatalf("structure %q, want crypt", forced.Get(dimStructure))
	}
	for _, dim := range []string{dimWeave, dimGrain, dimReach, dimAperture, dimTint, dimGround} {
		if forced.Get(dim) != base.Get(dim) {
			t.Errorf("%s changed from %q to %q when structure was pinned",
				dim, base.Get(dim), forced.Get(dim))
		}
	}
}

// The value ramp is what keeps the stroma mid-key on a palette whose colours
// bunch at one end: reading it upward must never step backward in luminance.
func TestTheValueRampRisesWithLuminance(t *testing.T) {
	for _, name := range []string{"vermeer-pearl-earring", "hokusai-great-wave", "klimt-kiss"} {
		pal, ok := palette.ByName(name)
		if !ok {
			t.Fatalf("palette %q missing", name)
		}
		ramp := newValueRamp(palette.ByLuminance(pal.Colors))
		previous := -1.0
		for i := range 41 {
			got := ramp.At(float64(i) / 40).Luminance()
			if got < previous-1e-9 {
				t.Errorf("%s: ramp falls from %v to %v at t=%v", name, previous, got, float64(i)/40)
			}
			previous = got
		}
	}
}

// Every structure must fill the disc: an empty or single-valued stroma means
// its reading collapsed.
func TestEveryStructureFillsTheDisc(t *testing.T) {
	for _, name := range []string{"fiber", "filament", "strata", "crypt"} {
		p := planFor(t, configured(t, "--structure", name), testCtx(t, 9))
		seen := map[palette.Color]bool{}
		for i := range 64 {
			angle := float64(i) * 0.31
			for _, at := range []float64{0.15, 0.45, 0.75, 0.95} {
				radius := p.set.limbus * (p.set.pupil + at*(1-p.set.pupil))
				seen[p.At(p.centreU+math.Cos(angle)*radius, p.centreV+math.Sin(angle)*radius)] = true
			}
		}
		if len(seen) < 32 {
			t.Errorf("%s: only %d distinct colours over 256 stroma samples", name, len(seen))
		}
	}
}

func TestPaletteTooSmallIsRejected(t *testing.T) {
	ctx := testCtx(t, 1)
	ctx.Palette = palette.Palette{Slug: "pair", Colors: ctx.Palette.Colors[:3]}
	if _, err := New().plan(ctx); err == nil {
		t.Fatal("a three-colour palette was accepted")
	}
}

func TestGolden(t *testing.T) {
	ctx := testCtx(t, 3)
	ctx.Width, ctx.Height = 200, 200
	got := sketchtest.RenderNRGBA(t, New(), ctx)
	sketchtest.Golden(t, got, "testdata/iris_seed3.png", *update)
}

func BenchmarkSample(b *testing.B) {
	p := planFor(b, New(), testCtx(b, 3))
	for i := 0; b.Loop(); i++ {
		u := 0.2 + float64(i%97)/160
		p.At(u, 0.5)
	}
}
