package tapestry

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("kandinsky-soft-pressure")
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

func TestRejectsTooSmallPalette(t *testing.T) {
	ctx := testCtx(t, 1)
	ctx.Palette = palette.Palette{Slug: "tiny", Colors: []palette.Color{{}, {}, {}}}
	if _, err := New().Render(ctx); err == nil {
		t.Error("expected error for palette with < 4 colors")
	}
}

func TestPlanBounds(t *testing.T) {
	s := New()
	for seed := uint64(0); seed < 200; seed++ {
		p := s.plan(testCtx(t, seed))
		if p.freq < 4.5 || p.freq > 8.5 {
			t.Fatalf("seed %d: freq %v out of range", seed, p.freq)
		}
		if p.highThresh < 0.10 || p.highThresh > 0.18 || p.lowThresh != -p.highThresh {
			t.Fatalf("seed %d: thresholds %v/%v out of range", seed, p.lowThresh, p.highThresh)
		}
		if n := len(p.gradLow); n < 35 || n > 70 {
			t.Fatalf("seed %d: %d bands out of range", seed, n)
		}
		if p.regionStrength < 0.50 || p.regionStrength > 0.85 {
			t.Fatalf("seed %d: region strength %v out of range", seed, p.regionStrength)
		}
		if len(p.stripes) < 8 {
			t.Fatalf("seed %d: only %d stripes on a square canvas", seed, len(p.stripes))
		}
		if last := p.stripes[len(p.stripes)-1].end; last < 1 {
			t.Fatalf("seed %d: stripes end at %v, canvas not covered", seed, last)
		}
		for _, st := range p.stripes {
			if st.amount < 0 || st.amount > 0.30 {
				t.Fatalf("seed %d: stripe amount %v out of range", seed, st.amount)
			}
		}
	}
}

func TestStripeAt(t *testing.T) {
	p := &plan{stripes: []stripe{{end: 0.2, amount: 0.1}, {end: 0.5, amount: 0.2}, {end: 1.1, amount: 0.3}}}
	tests := []struct {
		u    float64
		want float64
	}{{0, 0.1}, {0.19, 0.1}, {0.2, 0.2}, {0.49, 0.2}, {0.5, 0.3}, {1.0, 0.3}}
	for _, tt := range tests {
		if got := p.stripeAt(tt.u).amount; got != tt.want {
			t.Errorf("stripeAt(%v).amount = %v, want %v", tt.u, got, tt.want)
		}
	}
}

func TestGolden(t *testing.T) {
	got := renderNRGBA(t, testCtx(t, 42))
	golden := filepath.Join("testdata", "tapestry_seed42_64.png")

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
