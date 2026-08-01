package hatchbook

import (
	"bytes"
	"flag"
	"image"
	"io"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, w, h int) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("hopper-night-windows")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: w, Height: h, Seed: 42, Palette: pal}
}

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

// TestASheetIsIdenticalAtAnyResolution is the reason the specimen is a
// sketch at all. Nothing in a hatch may be measured in pixels, so a sheet
// rendered small and the same sheet rendered large and averaged down must
// agree — through the real pixel loop, not through a bespoke harness.
//
// If this fails, some length in internal/hatch or in this page has become a
// function of the output size, and preview no longer predicts print.
func TestASheetIsIdenticalAtAnyResolution(t *testing.T) {
	const small, large, common = 240, 720, 120
	for _, name := range PageNames() {
		s := configured(t, "--page", name)
		// The small render is supersampled to the density the large one
		// has natively, so the comparison is about where the marks are and
		// not about how a one-pixel line happens to alias.
		lo := testCtx(t, small, small)
		lo.AA = large / small
		a := boxDown(sketchtest.RenderNRGBA(t, s, lo), small/common)
		b := boxDown(sketchtest.RenderNRGBA(t, s, testCtx(t, large, large)), large/common)

		// Both are averaged down to the same coarse grid first. Comparing
		// the raw renders would measure how differently the two samplings
		// alias a mark a pixel wide, which is a fact about sampling and not
		// about the hatch; averaging them to a common grid leaves the thing
		// actually at stake — is the pattern in the same place, at the same
		// scale, with the same weight.
		var sum float64
		for i := 0; i < len(a.Pix); i += 4 {
			for c := range 3 {
				sum += math.Abs(float64(a.Pix[i+c]) - float64(b.Pix[i+c]))
			}
		}
		mean := sum / float64(len(a.Pix)/4*3)
		// Measured at 0.15–0.22 of a level, which is the dithering and
		// nothing else. A hatch that had picked up a pixel-dependent length
		// would move a mark by a fraction of its own spacing, and that is
		// tens of levels over the marks it moved.
		if mean > 0.6 {
			t.Errorf("page %q: mean channel difference %.2f between %d and %d px",
				name, mean, small, large)
		}
	}
}

// TestTheSheetDoesNotDependOnTheSeed: a catalogue is not an output space.
// Every square is pinned by construction, and a specimen that redrew itself
// per seed could not be cited. This is the opposite of what the other
// sketches promise, so it is worth a test of its own.
func TestTheSheetDoesNotDependOnTheSeed(t *testing.T) {
	s := configured(t, "--page", "structures")
	a := sketchtest.RenderNRGBA(t, s, testCtx(t, 96, 96))
	other := testCtx(t, 96, 96)
	other.Seed = 7777
	b := sketchtest.RenderNRGBA(t, s, other)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("two seeds drew different specimen sheets")
	}
}

// TestEverySquareIsDrawn guards the tables against a nil paint function and
// the grid against dropping a tile: the manifest promises rows × columns
// entries and the sheet has to have that many squares in it.
func TestEverySquareIsDrawn(t *testing.T) {
	for _, p := range pages {
		tiles := p.build()
		if len(tiles) == 0 {
			t.Fatalf("page %q has no squares", p.name)
		}
		if len(tiles)%p.cols != 0 {
			t.Errorf("page %q: %d squares in %d columns leaves a ragged last row",
				p.name, len(tiles), p.cols)
		}
		for i, tl := range tiles {
			if tl.paint == nil {
				t.Errorf("page %q square %d has no paint function", p.name, i)
			}
			if tl.note == "" {
				t.Errorf("page %q square %d has no manifest note", p.name, i)
			}
		}
	}
}

// TestTheManifestNamesEverySquare: the sheets carry no labels, so the
// manifest is the only way to read one. Every square must appear in it, at
// its own row and column.
func TestTheManifestNamesEverySquare(t *testing.T) {
	m := Manifest()
	for _, p := range pages {
		tiles := p.build()
		for i := range tiles {
			key := "\n" + coord(i/p.cols+1, i%p.cols+1) + " "
			if !strings.Contains(m, key) {
				t.Errorf("page %q: no manifest entry for %s", p.name, strings.TrimSpace(key))
			}
		}
	}
	if !strings.Contains(m, "left to right") {
		t.Error("the manifest does not state the reading order")
	}
}

func coord(r, c int) string {
	return "r" + itoa(r) + "c" + itoa(c)
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// TestAnUnknownPageIsRejected: the page is part of the filename, so a typo
// must fail at configure time rather than quietly render the default sheet
// under the wrong name.
func TestAnUnknownPageIsRejected(t *testing.T) {
	s := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse([]string{"--page", "spirals"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Configure(); err == nil {
		t.Error("an unknown page was accepted")
	}
}

// TestTheFilenameNamesThePage: five sheets that all land on out/hatchbook…
// would overwrite each other, so the page is always in the suffix even when
// it is the default one.
func TestTheFilenameNamesThePage(t *testing.T) {
	for _, name := range PageNames() {
		s := New()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		s.Flags(fs)
		if err := fs.Parse([]string{"--page", name}); err != nil {
			t.Fatal(err)
		}
		suffix, err := s.Configure()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(suffix, name) {
			t.Errorf("page %q gave filename suffix %q", name, suffix)
		}
	}
}

func TestGolden(t *testing.T) {
	for _, name := range PageNames() {
		s := configured(t, "--page", name)
		got := sketchtest.RenderNRGBA(t, s, testCtx(t, 96, 96))
		sketchtest.Golden(t, got, filepath.Join("testdata", "hatchbook_"+name+"_96.png"), *update)
	}
}

// boxDown averages n×n blocks of src into one pixel, in linear light —
// the same space the renderer's own supersampling averages in, so that
// downsampling a large render and supersampling a small one are the same
// operation and any difference left over is the hatch's.
func boxDown(src *image.NRGBA, n int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx()/n, b.Dy()/n
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			var sum [3]float64
			for j := range n {
				for i := range n {
					o := src.PixOffset(x*n+i, y*n+j)
					for c := range 3 {
						sum[c] += palette.SRGBToLinear(float64(src.Pix[o+c]) / 255)
					}
				}
			}
			o := out.PixOffset(x, y)
			for c := range 3 {
				out.Pix[o+c] = uint8(math.Round(255 * palette.LinearToSRGB(sum[c]/float64(n*n))))
			}
			out.Pix[o+3] = 255
		}
	}
	return out
}
