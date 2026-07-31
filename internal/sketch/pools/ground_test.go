package pools

import (
	"image"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

// bareGround renders just the ground, with no marks on it.
func bareGround(t *testing.T, seed uint64, args ...string) *image.NRGBA {
	t.Helper()
	s := configured(t, append([]string{"--count", "0"}, args...)...)
	return sketchtest.RenderNRGBA(t, s, testCtx(t, seed))
}

// lumSpread is the standard deviation of luminance over an image, which is
// how much the ground varies across the sheet.
func lumSpread(img *image.NRGBA) float64 {
	b := img.Bounds()
	var vals []float64
	mean := 0.0
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			c := img.NRGBAAt(x, y)
			l := (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
			vals = append(vals, l)
			mean += l
		}
	}
	mean /= float64(len(vals))
	v := 0.0
	for _, l := range vals {
		v += (l - mean) * (l - mean)
	}
	return math.Sqrt(v / float64(len(vals)))
}

// TestGroundIsPaintedNotFilled is the whole point of the ground wash: a
// flat wash is the hardest thing in watercolour to make even, and it dries
// with the unevenness of the process in it. A ground with no variation is
// a fill, and it gives the marks standing on it away as computed.
func TestGroundIsPaintedNotFilled(t *testing.T) {
	painted := lumSpread(bareGround(t, 42))

	// The floor is not zero: the canvas is dithered on its way to 8 bits,
	// so even a flat fill carries about one least-significant bit of noise.
	// Measuring it rather than assuming it keeps this honest if the
	// quantisation ever changes.
	dither := lumSpread(bareGround(t, 42, "--ground", "0"))

	if painted < 4*dither {
		t.Errorf("the painted ground varies by %.5f against a dither floor of %.5f — it is a fill with a name",
			painted, dither)
	}
	// And not so much that it competes with what is standing on it.
	if painted > 0.06 {
		t.Errorf("the painted ground varies by %.5f — that is weather, not paper", painted)
	}
}

// TestGroundCoversTheWholeSheet guards the bleed. The pools are laid on a
// grid that runs off every edge precisely so no pool boundary lands inside
// the frame; if the grid stopped at the canvas, the corners would be bare
// paper and the ground would read as a stain rather than as a wash.
func TestGroundCoversTheWholeSheet(t *testing.T) {
	s := configured(t)
	paper, _, _ := s.inks(palette.ByLuminance(paletteFor(t, s, testCtx(t, 42)).Colors))
	img := bareGround(t, 42)
	b := img.Bounds()

	for _, p := range []struct {
		name string
		x, y int
	}{
		{"top-left", b.Min.X + 1, b.Min.Y + 1},
		{"top-right", b.Max.X - 2, b.Min.Y + 1},
		{"bottom-left", b.Min.X + 1, b.Max.Y - 2},
		{"bottom-right", b.Max.X - 2, b.Max.Y - 2},
		{"centre", (b.Min.X + b.Max.X) / 2, (b.Min.Y + b.Max.Y) / 2},
	} {
		c := img.NRGBAAt(p.x, p.y)
		d := math.Abs(float64(c.R)/255-paper.R) +
			math.Abs(float64(c.G)/255-paper.G) +
			math.Abs(float64(c.B)/255-paper.B)
		if d < 0.02 {
			t.Errorf("%s is still bare paper (%v) — the wash did not reach it", p.name, c)
		}
	}
}
