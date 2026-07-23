package render

import (
	"bytes"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func TestRasterCoordinates(t *testing.T) {
	near := func(got uint8, want int) bool {
		d := int(got) - want
		return d >= -1 && d <= 1 // dithered quantization may shift by 1
	}
	// 4 wide × 2 high: v spans [0,1] over height, u spans [0,2] over width.
	img := Raster(4, 2, func(u, v float64) palette.Color {
		return palette.Color{R: u / 2, G: v}
	})
	// Pixel (0,0) center: u=0.25, v=0.25 → R=0.125·255≈32, G=64.
	c := img.NRGBAAt(0, 0)
	if !near(c.R, 32) || !near(c.G, 64) {
		t.Errorf("pixel (0,0) = %v, want R≈32 G≈64", c)
	}
	// Pixel (3,1) center: u=1.75, v=0.75 → R=0.875·255≈223, G=191.
	c = img.NRGBAAt(3, 1)
	if !near(c.R, 223) || !near(c.G, 191) {
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
		if d < -2 || d > 2 {
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

func TestLinearAveraging(t *testing.T) {
	// A pixel whose subsamples are half black / half white must average in
	// linear light: sRGB(0.5 linear) ≈ 0.735 → ~187, not the sRGB-space
	// average 128.
	f := func(u, v float64) palette.Color {
		if v < 0.5 {
			return palette.Color{R: 1, G: 1, B: 1}
		}
		return palette.Color{}
	}
	img := RasterSS(1, 1, 2, f) // 1×1 canvas: subsamples straddle v=0.5
	got := img.NRGBAAt(0, 0).R
	if got < 183 || got > 191 {
		t.Errorf("half black/white pixel = %d, want ≈187 (linear-light average)", got)
	}
}

func TestRasterDeep(t *testing.T) {
	f := func(u, v float64) palette.Color { return palette.Color{R: v, G: 0.5, B: 0.25} }
	a := RasterDeep(8, 8, 2, f)
	b := RasterDeep(8, 8, 2, f)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("deep render not deterministic")
	}
	// 16-bit must resolve steps that 8-bit cannot: a v-ramp over 8 rows in
	// [0,1] has distinct 16-bit values per row.
	prev := a.NRGBA64At(0, 0).R
	for y := 1; y < 8; y++ {
		cur := a.NRGBA64At(0, y).R
		if cur == prev {
			t.Fatalf("rows %d and %d have identical 16-bit values", y-1, y)
		}
		prev = cur
	}
}

func TestWriteMeta(t *testing.T) {
	img := Raster(8, 8, func(u, v float64) palette.Color { return palette.Color{R: u, G: v} })
	m := Meta{DPI: 300, Software: "staticart test", Comment: "staticart render tapestry --seed 1"}

	pngPath := filepath.Join(t.TempDir(), "m.png")
	if err := WritePNGMeta(pngPath, img, m); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sRGB", "pHYs", "Software\x00staticart test", "Comment\x00staticart render"} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Errorf("PNG missing %q", want)
		}
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Errorf("PNG with metadata no longer decodes: %v", err)
	}

	jpgPath := filepath.Join(t.TempDir(), "m.jpg")
	if err := WriteJPEGMeta(jpgPath, img, m); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(jpgPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"JFIF", "staticart render"} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Errorf("JPEG missing %q", want)
		}
	}
	if _, err := jpeg.Decode(bytes.NewReader(raw)); err != nil {
		t.Errorf("JPEG with metadata no longer decodes: %v", err)
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
