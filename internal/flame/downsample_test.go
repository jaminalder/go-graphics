package flame

import (
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func TestDownsampleSS1IsIdentity(t *testing.T) {
	src := []palette.Color{
		{R: 0.2, G: 0.4, B: 0.6},
		{R: 0.8, G: 0.1, B: 0.3},
		{R: 0.0, G: 1.0, B: 0.5},
		{R: 0.5, G: 0.5, B: 0.5},
	}
	got, err := Downsample(src, 2, 2, 2, 2, 1, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for i := range src {
		if got[i] != src[i] {
			t.Fatalf("pixel %d: %v vs %v", i, got[i], src[i])
		}
	}
}

func TestDownsampleAveragesCheckerboardToMidGrey(t *testing.T) {
	// 4×4 checker of black/white, ss=2 → 2×2. A Gaussian covering the
	// 2×2 block must land near mid-grey in linear light, not stay black
	// or white (that would be point sampling).
	const ss, outW, outH = 2, 2, 2
	srcW, srcH := outW*ss, outH*ss
	src := make([]palette.Color, srcW*srcH)
	for y := 0; y < srcH; y++ {
		for x := 0; x < srcW; x++ {
			if (x+y)%2 == 0 {
				src[y*srcW+x] = palette.Color{R: 1, G: 1, B: 1}
			}
		}
	}
	got, err := Downsample(src, srcW, srcH, outW, outH, ss, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range got {
		if c.R < 0.35 || c.R > 0.75 {
			t.Fatalf("pixel %d R=%g; want mid-grey from checkerboard", i, c.R)
		}
		if math.Abs(c.R-c.G) > 1e-9 || math.Abs(c.G-c.B) > 1e-9 {
			t.Fatalf("pixel %d not neutral: %v", i, c)
		}
	}
}

func TestDownsampleIsDeterministic(t *testing.T) {
	src := make([]palette.Color, 16)
	for i := range src {
		src[i] = palette.Color{R: float64(i) / 15, G: 0.2, B: 0.7}
	}
	a, err := Downsample(src, 4, 4, 2, 2, 2, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Downsample(src, 4, 4, 2, 2, 2, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("pixel %d moved", i)
		}
	}
}

func TestDownsampleRejectsShapeMismatch(t *testing.T) {
	src := make([]palette.Color, 4)
	if _, err := Downsample(src, 2, 2, 2, 2, 2, 0.5); err == nil {
		t.Fatal("accepted src 2×2 for ss=2 output 2×2 (want 4×4)")
	}
}

func TestSpatialKernelIsNormalisedAndSameParityAsSS(t *testing.T) {
	for _, ss := range []int{1, 2, 3} {
		k, fw := spatialKernel(ss, 0.5)
		if (fw^ss)&1 != 0 {
			t.Fatalf("ss=%d fw=%d; flam3 requires same parity", ss, fw)
		}
		var sum float64
		for _, w := range k {
			sum += w
		}
		if math.Abs(sum-1) > 1e-12 {
			t.Fatalf("ss=%d kernel sum %g", ss, sum)
		}
	}
}
