package render

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func TestRasterCoordinates(t *testing.T) {
	// 4 wide × 2 high: v spans [0,1] over height, u spans [0,2] over width.
	img := Raster(4, 2, func(u, v float64) palette.Color {
		return palette.Color{R: u / 2, G: v}
	})
	// Pixel (0,0) center: u=0.25, v=0.25 → R=0.125·255≈32, G=64.
	c := img.NRGBAAt(0, 0)
	if c.R != 32 || c.G != 64 {
		t.Errorf("pixel (0,0) = %v, want R≈32 G≈64", c)
	}
	// Pixel (3,1) center: u=1.75, v=0.75 → R=0.875·255≈223, G=191.
	c = img.NRGBAAt(3, 1)
	if c.R != 223 || c.G != 191 {
		t.Errorf("pixel (3,1) = %v, want R≈223 G≈191", c)
	}
}

func TestRasterDeterministicUnderParallelism(t *testing.T) {
	f := func(u, v float64) palette.Color {
		return palette.Color{R: u * v, G: u, B: v}
	}
	a := Raster(131, 97, f) // odd sizes to exercise chunk boundaries
	b := Raster(131, 97, f)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("two renders of the same pure function differ")
	}
}

func TestRasterSS(t *testing.T) {
	// On a linear ramp, supersampling averages to the same value as
	// center sampling — AA must not shift smooth content.
	f := func(u, v float64) palette.Color { return palette.Color{R: u / 2, G: v, B: 0.25} }
	plain := Raster(16, 16, f)
	ss := RasterSS(16, 16, 3, f)
	for i := 0; i < len(plain.Pix); i++ {
		d := int(plain.Pix[i]) - int(ss.Pix[i])
		if d < -1 || d > 1 {
			t.Fatalf("pixel byte %d differs by more than rounding: %d vs %d", i, plain.Pix[i], ss.Pix[i])
		}
	}

	// On a hard edge, supersampling must produce intermediate values.
	step := func(u, v float64) palette.Color {
		if u > 1.03 { // splits pixel x=16's subsamples at h=16
			return palette.Color{R: 1, G: 1, B: 1}
		}
		return palette.Color{}
	}
	ssEdge := RasterSS(32, 16, 4, step)
	found := false
	for x := 0; x < 32 && !found; x++ {
		px := ssEdge.NRGBAAt(x, 8).R
		if px > 20 && px < 235 {
			found = true
		}
	}
	if !found {
		t.Error("no anti-aliased intermediate pixels found along the edge")
	}
}

func TestWritePNGRoundTrip(t *testing.T) {
	img := Raster(8, 8, func(u, v float64) palette.Color {
		return palette.Color{R: u, G: v, B: 0.5}
	})
	path := filepath.Join(t.TempDir(), "x.png")
	if err := WritePNG(path, img); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v vs %v", decoded.Bounds(), img.Bounds())
	}
}

func TestProfileByName(t *testing.T) {
	p, err := ProfileByName("preview")
	if err != nil || p.Width != 600 || p.Height != 600 {
		t.Errorf("preview profile = %+v, err %v", p, err)
	}
	if _, err := ProfileByName("poster"); err == nil {
		t.Error("expected error for unknown profile")
	}
}
