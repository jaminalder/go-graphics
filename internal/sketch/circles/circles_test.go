package circles

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
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

func renderNRGBA(t *testing.T, ctx sketch.Context) *image.NRGBA {
	t.Helper()
	img, err := New().Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return img.(*image.NRGBA)
}

func TestDeterminism(t *testing.T) {
	a := renderNRGBA(t, testCtx(t, 42))
	b := renderNRGBA(t, testCtx(t, 42))
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("same seed produced different images")
	}
	c := renderNRGBA(t, testCtx(t, 43))
	if bytes.Equal(a.Pix, c.Pix) {
		t.Error("different seeds produced identical images")
	}
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
			if a.r < s.MinR-1e-12 || a.r > s.MaxR+1e-12 {
				t.Fatalf("seed %d: radius %v out of range", seed, a.r)
			}
			if a.cx-a.r < 0 || a.cx+a.r > 1 || a.cy-a.r < 0 || a.cy+a.r > 1 {
				t.Fatalf("seed %d: circle %d leaves the canvas", seed, i)
			}
			for j := i + 1; j < len(p.circles); j++ {
				b := &p.circles[j]
				dx, dy := a.cx-b.cx, a.cy-b.cy
				if math.Sqrt(dx*dx+dy*dy) < a.r+b.r+s.Gap-1e-9 {
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
			if feature := c.scale * c.r; feature < 0.0079 && c.scale < 1 {
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
		got := p.grid.at(u, v, p.circles)
		want := -1
		for j := range p.circles {
			c := &p.circles[j]
			dx, dy := u-c.cx, v-c.cy
			if dx*dx+dy*dy <= c.r*c.r {
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
	got := renderNRGBA(t, testCtx(t, 42))
	golden := filepath.Join("testdata", "circles_seed42_64.png")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(golden)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, got); err != nil {
			t.Fatal(err)
		}
		t.Log("golden regenerated — eyeball it before committing")
		return
	}

	f, err := os.Open(golden)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := want.Bounds()
	wantN := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			wantN.Set(x, y, want.At(x, y))
		}
	}
	if !bytes.Equal(got.Pix, wantN.Pix) {
		t.Error("render differs from golden (intentional change? re-run with -update and eyeball)")
	}
}
