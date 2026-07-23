// Package sketchtest holds the test helpers shared by all sketch
// packages: rendering to NRGBA, determinism assertions, and golden-file
// comparison. Import only from _test files.
package sketchtest

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaminalder/go-graphics/internal/sketch"
)

// RenderNRGBA renders s and fails the test on error.
func RenderNRGBA(t *testing.T, s sketch.Sketch, ctx sketch.Context) *image.NRGBA {
	t.Helper()
	img, err := s.Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return img.(*image.NRGBA)
}

// AssertDeterministic renders ctx twice (must be byte-identical) and
// otherCtx once (must differ from ctx).
func AssertDeterministic(t *testing.T, s sketch.Sketch, ctx, otherCtx sketch.Context) {
	t.Helper()
	a := RenderNRGBA(t, s, ctx)
	b := RenderNRGBA(t, s, ctx)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("same seed produced different images")
	}
	c := RenderNRGBA(t, s, otherCtx)
	if bytes.Equal(a.Pix, c.Pix) {
		t.Error("different seeds produced identical images")
	}
}

// Golden compares got against the PNG at path; with update it (re)writes
// the file instead. Regenerated goldens must be eyeballed before commit.
func Golden(t *testing.T, got *image.NRGBA, path string, update bool) {
	t.Helper()
	if update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(path)
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

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Pix, ToNRGBA(decoded).Pix) {
		t.Error("render differs from golden (intentional change? re-run with -update and eyeball)")
	}
}

// ToNRGBA normalizes any decoded image to NRGBA for pixel comparison.
func ToNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			n.Set(x, y, img.At(x, y))
		}
	}
	return n
}
