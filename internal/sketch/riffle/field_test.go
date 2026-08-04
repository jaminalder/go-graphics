package riffle

import "testing"

func TestSurfaceSpansLightAndShadowWithoutFineStriping(t *testing.T) {
	s := NewSurface(testCtx(t, 42), 42)
	lo, hi := 1.0, -1.0
	crossings := 0
	previous := s.At(0.5, 0.5/600).Ripple
	for y := 1; y < 600; y++ {
		got := s.At(0.5, (float64(y)+0.5)/600).Ripple
		lo, hi = min(lo, got), max(hi, got)
		if (got < 0) != (previous < 0) {
			crossings++
		}
		previous = got
	}
	if lo > -0.35 || hi < 0.35 {
		t.Fatalf("surface ripple range %.3f..%.3f, want visible light and shadow", lo, hi)
	}
	if crossings > 36 {
		t.Fatalf("surface crosses zero %d times down the frame, want at most 36", crossings)
	}
}
