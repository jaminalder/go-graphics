package flame

import (
	"flag"
	"io"
	"testing"

	fl "github.com/jaminalder/go-graphics/internal/flame"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t testing.TB, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("zander-spindle")
	if !ok {
		t.Fatal("zander-spindle palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal, AA: 1}
}

func configured(t testing.TB, args ...string) *Sketch {
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

func TestSchemaIsValid(t *testing.T) {
	if err := New().Schema().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDeterminism(t *testing.T) {
	s := configured(t, "--quality", "8", "--estimator", "3", "--oversample", "2", "--structure", "spindle")
	sketchtest.AssertDeterministic(t, s, testCtx(t, 12), testCtx(t, 13))
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t, "--quality", "8", "--estimator", "0", "--oversample", "1", "--structure", "spindle", "--tint", "split", "--ground", "void"), testCtx(t, 12))
	sketchtest.Golden(t, got, "testdata/flame_seed12_64.png", *update)
}

func TestGenomeDoesNotDependOnPixelSize(t *testing.T) {
	s := configured(t, "--structure", "spindle")
	ctxA := testCtx(t, 21)
	ctxB := ctxA
	ctxB.Width, ctxB.Height = 128, 96
	setA := s.Traits(ctxA)
	setB := s.Traits(ctxB)
	if setA.Get(dimStructure) != setB.Get(dimStructure) {
		t.Fatal("traits moved with canvas size")
	}
	recA := s.pin(draw(setA, ctxA.RNG(streamRecipe)))
	recB := s.pin(draw(setB, ctxB.RNG(streamRecipe)))
	sysA := compose(recA, ctxA.RNG(streamGenome))
	sysB := compose(recB, ctxB.RNG(streamGenome))
	if len(sysA.X) != len(sysB.X) {
		t.Fatalf("xform count %d vs %d", len(sysA.X), len(sysB.X))
	}
	if len(sysA.Xaos) != len(sysB.Xaos) {
		t.Fatalf("xaos %d vs %d", len(sysA.Xaos), len(sysB.Xaos))
	}
	for i := range sysA.X {
		a, b := sysA.X[i], sysB.X[i]
		if a.A != b.A || a.B != b.B || a.C != b.C || a.D != b.D || a.E != b.E || a.F != b.F ||
			a.Color != b.Color || a.Weight != b.Weight || len(a.Vars) != len(b.Vars) {
			t.Fatalf("xform %d changed with canvas size", i)
		}
	}
}

func TestSplitTintIsCoolThenWarm(t *testing.T) {
	pal, _ := palette.ByName("zander-spindle")
	g := colourMap(pal, tintSplit)
	cool, warm := g.At(0), g.At(1)
	if warmth(cool) >= warmth(warm) {
		t.Fatalf("split gradient is not cool→warm: %v then %v", cool, warm)
	}
}

func TestZanderSplitRampIsCyanGoldNotMagenta(t *testing.T) {
	pal, ok := palette.ByName("zander-spindle")
	if !ok {
		t.Fatal("zander-spindle palette missing")
	}
	g := colourMap(pal, tintSplit)
	cool := g.At(0)
	if cool.B <= cool.R {
		t.Fatalf("cool end %v is not cyan", cool)
	}
	warm := g.At(1)
	if warm.R <= warm.B {
		t.Fatalf("warm end %v is not orange", warm)
	}
	for i := 0; i <= 20; i++ {
		c := g.At(float64(i) / 20)
		if c.R > 0.45 && c.B > 0.45 && c.G < 0.35 {
			t.Fatalf("t=%g is magenta %v", float64(i)/20, c)
		}
		if c.G > c.R+0.08 && c.G > c.B+0.08 && c.G > 0.4 {
			t.Fatalf("t=%g is green %v", float64(i)/20, c)
		}
	}
}

func TestSpindleXaosForbidsJulianAfterJulian(t *testing.T) {
	s := configured(t, "--structure", "spindle")
	ctx := testCtx(t, 12)
	rec := s.pin(draw(s.Traits(ctx), ctx.RNG(streamRecipe)))
	sys := compose(rec, ctx.RNG(streamGenome))
	var nest []int
	for i, x := range sys.X {
		for _, v := range x.Vars {
			if v.Kind == fl.JulianN {
				nest = append(nest, i)
				break
			}
		}
	}
	if len(nest) == 0 {
		t.Fatal("spindle has no JulianN")
	}
	if sys.Xaos == nil {
		t.Fatal("spindle xaos missing")
	}
	for _, i := range nest {
		for _, j := range nest {
			if sys.Xaos[i][j] > 0.1 {
				t.Fatalf("julian %d→%d xaos %g; want near-zero so copies reprint the nest", i, j, sys.Xaos[i][j])
			}
		}
	}
}

func TestOversampleTwoWritesOutputSize(t *testing.T) {
	s := configured(t, "--quality", "4", "--estimator", "0", "--oversample", "2", "--structure", "spindle")
	ctx := testCtx(t, 7)
	img, err := s.Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != ctx.Width || img.Bounds().Dy() != ctx.Height {
		t.Fatalf("got %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), ctx.Width, ctx.Height)
	}
}

func TestEveryStructureBuildsASystem(t *testing.T) {
	ctx := testCtx(t, 5)
	for _, name := range []string{"spindle", "filament", "bloom", "spiral", "fold", "julia"} {
		cfg := configured(t, "--structure", name, "--quality", "4")
		if _, err := cfg.Render(ctx); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
