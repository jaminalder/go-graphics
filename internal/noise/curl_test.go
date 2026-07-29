package noise

import (
	"math"
	"testing"
)

func TestCurlIsDivergenceFree(t *testing.T) {
	// The whole reason to prefer Curl over reading an angle out of the
	// noise is that it has no sources or sinks, so walkers spread evenly
	// instead of draining. Check ∇·v numerically over a grid.
	// Measured on the field's own stencil, where the mixed partials
	// cancel term for term, so the only error is float rounding.
	p := New(12345)
	const h = curlEps
	worst, scale := 0.0, 0.0
	for i := range 12 {
		for j := range 12 {
			x, y := float64(i)*0.37+0.11, float64(j)*0.41+0.07
			vxp, _ := p.Curl(x+h, y, 3)
			vxm, _ := p.Curl(x-h, y, 3)
			_, vyp := p.Curl(x, y+h, 3)
			_, vym := p.Curl(x, y-h, 3)
			div := (vxp-vxm)/(2*h) + (vyp-vym)/(2*h)
			worst = math.Max(worst, math.Abs(div))
			vx, vy := p.Curl(x, y, 3)
			scale = math.Max(scale, math.Hypot(vx, vy)/h)
		}
	}
	// Relative to the size of the derivatives being differenced, a field
	// with real sources would land near 1 rather than near zero.
	if rel := worst / scale; rel > 1e-9 {
		t.Errorf("max |divergence| = %g (relative %g), want ~0", worst, rel)
	}
}

func TestCurlNonZeroAndDeterministic(t *testing.T) {
	a, b := New(7), New(7)
	moved := false
	for i := range 20 {
		x, y := float64(i)*0.29, float64(i)*0.53
		ax, ay := a.Curl(x, y, 2)
		bx, by := b.Curl(x, y, 2)
		if ax != bx || ay != by {
			t.Fatalf("same seed gave different curl at (%v,%v)", x, y)
		}
		if math.Hypot(ax, ay) > 1e-6 {
			moved = true
		}
	}
	if !moved {
		t.Error("curl is identically zero")
	}
}

func TestCurlAngleMatchesCurl(t *testing.T) {
	p := New(99)
	for i := range 10 {
		x, y := float64(i)*0.31+0.2, float64(i)*0.17+0.4
		vx, vy := p.Curl(x, y, 2)
		if got, want := p.CurlAngle(x, y, 2), math.Atan2(vy, vx); math.Abs(got-want) > 1e-12 {
			t.Errorf("CurlAngle = %v, want %v", got, want)
		}
	}
}

func TestRidgedInUnitRangeAndPeaksAtZeroCrossings(t *testing.T) {
	p := New(4242)
	sawHigh := false
	for i := range 40 {
		for j := range 40 {
			x, y := float64(i)*0.13, float64(j)*0.13
			v := p.Ridged(x, y, 3)
			if v < 0 || v > 1 {
				t.Fatalf("Ridged(%v,%v) = %v, outside [0,1]", x, y, v)
			}
			// A ridge sits where the underlying fBm crosses zero.
			if math.Abs(p.FBM(x, y, 3)) < 0.01 && v < 0.98 {
				t.Errorf("at a zero crossing Ridged = %v, want ≈1", v)
			}
			if v > 0.9 {
				sawHigh = true
			}
		}
	}
	if !sawHigh {
		t.Error("Ridged never approached 1; there should be ridges")
	}
}
