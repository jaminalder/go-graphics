package riffle

import (
	"testing"
)

func TestDefaultSurfaceConfigPreservesTheShallowsSurface(t *testing.T) {
	cfg := DefaultSurfaceConfig(42)
	if cfg != (SurfaceConfig{
		Seed: 42, Reach: "pool", Channel: "bend", Boulders: "field",
		Water: "clear", Light: "dappled",
	}) {
		t.Fatalf("default surface config = %+v", cfg)
	}
	s, err := NewSurface(testCtx(t, 7), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.5, 0.5}, {0.9, 0.8}} {
		a := s.At(point[0], point[1])
		b := s.At(point[0], point[1])
		if a != b {
			t.Fatalf("surface sample at %v changed from %+v to %+v", point, a, b)
		}
	}
}

func TestSurfaceConfigRejectsUnknownLevels(t *testing.T) {
	tests := []SurfaceConfig{
		{Seed: 1, Reach: "flood", Channel: "bend", Boulders: "field", Water: "clear", Light: "dappled"},
		{Seed: 1, Reach: "pool", Channel: "canal", Boulders: "field", Water: "clear", Light: "dappled"},
		{Seed: 1, Reach: "pool", Channel: "bend", Boulders: "mountains", Water: "clear", Light: "dappled"},
		{Seed: 1, Reach: "pool", Channel: "bend", Boulders: "field", Water: "opaque", Light: "dappled"},
		{Seed: 1, Reach: "pool", Channel: "bend", Boulders: "field", Water: "clear", Light: "night"},
	}
	for _, cfg := range tests {
		if _, err := NewSurface(testCtx(t, 1), cfg); err == nil {
			t.Errorf("accepted invalid surface config %+v", cfg)
		}
	}
}

func TestSurfaceSpansLightAndShadowWithoutFineStriping(t *testing.T) {
	s, err := NewSurface(testCtx(t, 42), DefaultSurfaceConfig(42))
	if err != nil {
		t.Fatal(err)
	}
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
