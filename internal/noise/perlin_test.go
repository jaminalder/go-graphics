package noise

import (
	"math"
	"testing"
)

func TestDeterministic(t *testing.T) {
	a, b := New(42), New(42)
	for i := range 100 {
		x, y := float64(i)*0.13, float64(i)*0.71
		if a.At(x, y) != b.At(x, y) {
			t.Fatalf("same seed differs at (%v, %v)", x, y)
		}
	}
}

func TestSeedsDiffer(t *testing.T) {
	a, b := New(1), New(2)
	for i := range 100 {
		x, y := float64(i)*0.13, float64(i)*0.71
		if a.At(x, y) != b.At(x, y) {
			return
		}
	}
	t.Fatal("seeds 1 and 2 produced identical noise on all samples")
}

func TestZeroAtLatticePoints(t *testing.T) {
	p := New(7)
	for x := range 8 {
		for y := range 8 {
			if v := p.At(float64(x), float64(y)); v != 0 {
				t.Errorf("At(%d, %d) = %v, want 0", x, y, v)
			}
		}
	}
}

func TestValueRange(t *testing.T) {
	p := New(3)
	limit := math.Sqrt2/2 + 1e-9
	for i := range 200 {
		for j := range 200 {
			x, y := float64(i)*0.083, float64(j)*0.077
			if v := p.At(x, y); math.Abs(v) > limit {
				t.Fatalf("At(%v, %v) = %v exceeds ±√2/2", x, y, v)
			}
		}
	}
}

func TestFBMSingleOctaveEqualsAt(t *testing.T) {
	p := New(11)
	if p.FBM(1.3, 2.7, 0) != p.At(1.3, 2.7) {
		t.Error("FBM with maxOctave 0 should equal At")
	}
}

func TestFBMOctaveSum(t *testing.T) {
	p := New(11)
	x, y := 0.37, 1.91
	want := p.At(x, y) + p.At(2*x, 2*y)/2 + p.At(4*x, 4*y)/4
	if got := p.FBM(x, y, 2); math.Abs(got-want) > 1e-12 {
		t.Errorf("FBM(2 octaves) = %v, want %v", got, want)
	}
}

func TestFBMPersistence(t *testing.T) {
	p := New(11)
	x, y := 0.37, 1.91
	if p.FBMP(x, y, 2, 0.5) != p.FBM(x, y, 2) {
		t.Error("FBMP with persistence 0.5 should equal FBM")
	}
	want := p.At(x, y) + p.At(2*x, 2*y)*0.3 + p.At(4*x, 4*y)*0.09
	if got := p.FBMP(x, y, 2, 0.3); math.Abs(got-want) > 1e-12 {
		t.Errorf("FBMP(persistence 0.3) = %v, want %v", got, want)
	}
}

func TestContinuity(t *testing.T) {
	// Noise must not jump across cell boundaries.
	p := New(5)
	const eps = 1e-6
	for i := range 50 {
		x := float64(i) * 0.4
		below := p.At(x, 1-eps)
		above := p.At(x, 1+eps)
		if math.Abs(below-above) > 1e-3 {
			t.Errorf("discontinuity at (%v, 1): %v vs %v", x, below, above)
		}
	}
}
