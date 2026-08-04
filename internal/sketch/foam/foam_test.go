package foam

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

// densities is every level of the density axis, so tests sweep the whole
// axis rather than whichever one seed 42 happens to draw.
var densities = []string{"sparse", "open", "medium", "busy", "packed", "fine"}

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
// override from what the seed's traits drew.
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

func planned(t *testing.T, s *Sketch, seed uint64) *sheet {
	t.Helper()
	sh, err := s.plan(testCtx(t, seed))
	if err != nil {
		t.Fatal(err)
	}
	return sh
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 42), testCtx(t, 43))
}

func TestPlannedSheetSamplesPurely(t *testing.T) {
	s := configured(t, "--mosaic", "family", "--relief", "cushion")
	sh := planned(t, s, 7)
	cellsBefore := sh.foam.Len()
	tilesBefore := sh.tiles.foam.Len()
	for _, point := range [][2]float64{{0.1, 0.2}, {0.5, 0.5}, {0.9, 0.8}} {
		a := s.sample(sh, 7, point[0], point[1])
		b := s.sample(sh, 7, point[0], point[1])
		if a != b {
			t.Fatalf("sample at %v changed from %v to %v", point, a, b)
		}
	}
	if sh.foam.Len() != cellsBefore || sh.tiles.foam.Len() != tilesBefore {
		t.Fatal("sampling changed the planned partitions")
	}
}

func TestPlannedSheetIgnoresPixelDimensions(t *testing.T) {
	s := configured(t, "--mosaic", "family", "--relief", "cushion")
	small := testCtx(t, 9)
	large := small
	large.Width, large.Height = 512, 512
	a, err := s.plan(small)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.plan(large)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.5, 0.5}, {0.9, 0.8}} {
		if ca, cb := s.sample(a, 9, point[0], point[1]), s.sample(b, 9, point[0], point[1]); ca != cb {
			t.Fatalf("sample at %v changed from %v to %v with pixel dimensions", point, ca, cb)
		}
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/foam_seed42_96.png", *update)
}

// TestEveryDensityDividesTheSheet is the guard the pack needs and the one
// the first version did not have. Sites are placed over an *overscan*, so a
// count that looks generous can still leave the frame nearly undivided —
// the difference never shows in the count, only in how many cells have any
// area on the paper.
func TestEveryDensityDividesTheSheet(t *testing.T) {
	want := map[string][2]int{
		"sparse": {8, 30},
		"open":   {14, 45},
		"medium": {22, 70},
		"busy":   {40, 120},
		"packed": {65, 220},
		"fine":   {130, 380},
	}
	for _, d := range densities {
		for seed := uint64(1); seed <= 4; seed++ {
			// Lobes pinned tidy: merging legitimately lowers the cell count,
			// and this test is about what the *pack* delivers.
			s := configured(t, "--density", d, "--lobes", "tidy")
			n := 0
			for _, c := range planned(t, s, seed).foam.Cells() {
				if c.Area > 0 {
					n++
				}
			}
			lo, hi := want[d][0], want[d][1]
			if n < lo || n > hi {
				t.Errorf("%s seed %d: %d cells on the paper, want %d..%d", d, seed, n, lo, hi)
			}
		}
	}
}

// TestCellsAreSizedInAHierarchy pins what separates this from a honeycomb.
// A foam of uniformly sized cells has nothing to read; the reference's
// character is a quarter-canvas lobe sitting directly against a sliver, so
// the largest cell has to be an order of magnitude above the smallest.
func TestCellsAreSizedInAHierarchy(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		s := configured(t, "--density", "busy")
		small, large := math.Inf(1), 0.0
		for _, c := range planned(t, s, seed).foam.Cells() {
			if c.Area <= 0 {
				continue
			}
			small, large = math.Min(small, c.Area), math.Max(large, c.Area)
		}
		if ratio := large / small; ratio < 10 {
			t.Errorf("seed %d: largest cell is only %.1fx the smallest — no size hierarchy", seed, ratio)
		}
	}
}

// TestLobesActuallyForm — merging is what makes a cell concave, and a sheet
// with no lobes reads as a shattered pane however well the walls curve. The
// share is checked against the level asked for, not against zero: "most"
// that quietly produces three lobes is the failure worth catching.
func TestLobesActuallyForm(t *testing.T) {
	for _, tc := range []struct {
		level string
		least float64
	}{
		{"tidy", 0},
		{"few", 0.06},
		{"many", 0.16},
		{"most", 0.28},
	} {
		lobes, total := 0, 0
		for seed := uint64(1); seed <= 6; seed++ {
			s := configured(t, "--lobes", tc.level, "--density", "medium")
			for _, c := range planned(t, s, seed).foam.Cells() {
				total++
				if c.Sites > 1 {
					lobes++
				}
			}
		}
		if share := float64(lobes) / float64(total); share < tc.least {
			t.Errorf("--lobes %s made %.0f%% of cells lobes, want at least %.0f%%",
				tc.level, share*100, tc.least*100)
		}
	}
}

// TestBandsOnlyWhereTheyCanResolve. The wall-distance field ridges along a
// cell's medial axis; in a long or forked cell the rings trace those ridges
// and the fill comes out as a tangle of pinched loops rather than as
// concentric rings. Both gates — roundness and sheer size — are what keep
// the style off the cells it cannot serve.
func TestBandsOnlyWhereTheyCanResolve(t *testing.T) {
	banded := 0
	for seed := uint64(1); seed <= 10; seed++ {
		s := configured(t, "--bands", "8", "--density", "medium")
		sh := planned(t, s, seed)
		for i, c := range sh.foam.Cells() {
			if sh.skin[i].style != styleBands {
				continue
			}
			banded++
			if r := roundness(c, 1); r > 1.8 {
				t.Errorf("seed %d: cell %d has roundness %.2f and was banded anyway", seed, i, r)
			}
			if c.Inradius < 3*s.Stroke {
				t.Errorf("seed %d: cell %d is %.4f from its wall at the deepest and was banded", seed, i, c.Inradius)
			}
		}
	}
	if banded == 0 {
		t.Error("ten seeds at --bands 8 produced no banded cell at all")
	}
}

// TestSomeCellsAreLeftBare — empty cells are load-bearing. A sheet with
// every cell filled has nowhere to rest, and the reference leaves roughly
// one cell in six as bare paper.
func TestSomeCellsAreLeftBare(t *testing.T) {
	empty, total := 0, 0
	for seed := uint64(1); seed <= 8; seed++ {
		sh := planned(t, configured(t, "--fills", "mixed"), seed)
		for _, d := range sh.skin {
			total++
			if d.style == styleEmpty {
				empty++
			}
		}
	}
	if share := float64(empty) / float64(total); share < 0.08 || share > 0.35 {
		t.Errorf("%.0f%% of cells are bare paper, want roughly a sixth", share*100)
	}
}

// TestTheWarpMovesTheStructureAndNotTheFrame. The warp is a displacement of
// the plane the partition is read through, so it has to bend the cells
// while leaving every point on the canvas inside the picture. A warp that
// pushed lookups far outside the packed region would read cells that were
// never measured.
func TestTheWarpMovesTheStructureAndNotTheFrame(t *testing.T) {
	s := configured(t, "--lobes", "most")
	sh := planned(t, s, 5)
	moved, worst := 0.0, 0.0
	for i := range 40 {
		for j := range 40 {
			u, v := (float64(i)+0.5)/40, (float64(j)+0.5)/40
			wu, wv := s.warp(sh.field, sh.level, u, v)
			d := math.Hypot(wu-u, wv-v)
			moved += d
			worst = math.Max(worst, d)
		}
	}
	if mean := moved / 1600; mean < sh.level.base*0.1 {
		t.Errorf("the warp moves a point by %.4f on average against a cell of %.4f — it is doing nothing", mean, sh.level.base)
	}
	// The overscan is what the warp may eat into; past it there is nothing.
	if worst > sh.level.over {
		t.Errorf("the warp moves a point by up to %.4f, past the %.4f overscan", worst, sh.level.over)
	}
}

// TestTheNetFillLevelLeavesEveryCellBare. `--fills net` is the only way to
// actually look at the structure: no seed draws it, because an unfilled
// sheet is a drawing of the algorithm rather than a picture.
func TestTheNetFillLevelLeavesEveryCellBare(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		for _, d := range densities {
			sh := planned(t, configured(t, "--fills", "net", "--density", d), seed)
			for i, dr := range sh.skin {
				if dr.style != styleEmpty {
					t.Fatalf("%s seed %d: cell %d is filled on a bare net", d, seed, i)
				}
			}
		}
	}
	// And no seed lands on it by itself.
	s := configured(t)
	for seed := uint64(1); seed <= 60; seed++ {
		if got := s.Traits(testCtx(t, seed)).Get(dimFills); got == "net" {
			t.Fatalf("seed %d drew the net fill level, which carries weight 0", seed)
		}
	}
}

func TestConfigureRejectsOutOfRange(t *testing.T) {
	for _, args := range [][]string{
		{"--weight", "9"},
		{"--merge", "1.5"},
		{"--ink", "0"},
		{"--count", "1"},
		{"--swirl", "0.5"},
		{"--density", "enormous"},
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

// TestSeedsChooseTheirColours is why the colourway is a trait and not a
// flag: sweeping seeds has to give different pictures in different colours.
func TestSeedsChooseTheirColours(t *testing.T) {
	s := configured(t)
	seen := map[string]bool{}
	for seed := uint64(1); seed <= 30; seed++ {
		seen[s.Traits(testCtx(t, seed)).Get(dimColourway)] = true
	}
	if len(seen) < 6 {
		t.Errorf("thirty seeds drew only %d colourways", len(seen))
	}
	if seen[fromFlag] {
		t.Error("a seed landed on the from-flag colourway, which carries weight 0")
	}
	if got := configured(t, "--colourway", fromFlag).Traits(testCtx(t, 1)).Get(dimColourway); got != fromFlag {
		t.Errorf("--colourway %s resolved to %q", fromFlag, got)
	}
}

// TestPinningACellSizeMovesTheLineWithIt. The ink width is a fraction of
// the smallest site, so the pack overrides have to land before the line is
// drawn. Applied afterwards, --base gives a hand-set cell size under a line
// scaled for the one the seed drew — cells three times smaller behind the
// same heavy net.
func TestPinningACellSizeMovesTheLineWithIt(t *testing.T) {
	wide := planned(t, configured(t, "--base", "0.09"), 3).level
	tight := planned(t, configured(t, "--base", "0.03"), 3).level
	if wide.ink <= tight.ink {
		t.Errorf("--base 0.09 drew a %.5f line and --base 0.03 a %.5f one", wide.ink, tight.ink)
	}
	if r := wide.ink / tight.ink; r < 2.5 || r > 3.5 {
		t.Errorf("the line scaled by %.2f against a threefold change in cell size", r)
	}
}
