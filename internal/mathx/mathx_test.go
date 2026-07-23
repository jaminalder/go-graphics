package mathx

import (
	"math"
	"testing"
)

func TestClamp01(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{-1, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {2, 1},
	}
	for _, tt := range tests {
		if got := Clamp01(tt.in); got != tt.want {
			t.Errorf("Clamp01(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestRemap(t *testing.T) {
	if got := Remap(5, 0, 10); got != 0.5 {
		t.Errorf("Remap(5,0,10) = %v, want 0.5", got)
	}
	if got := Remap(-2, 0, 10); got != -0.2 {
		t.Errorf("Remap is unclamped: got %v, want -0.2", got)
	}
}

func TestSmoothstep(t *testing.T) {
	if Smoothstep(0, 1, -1) != 0 || Smoothstep(0, 1, 2) != 1 {
		t.Error("Smoothstep must clamp to [0,1]")
	}
	if got := Smoothstep(0, 1, 0.5); math.Abs(got-0.5) > 1e-12 {
		t.Errorf("Smoothstep midpoint = %v, want 0.5", got)
	}
	// Monotonic.
	prev := -1.0
	for i := 0; i <= 20; i++ {
		v := Smoothstep(0.2, 0.8, float64(i)/20)
		if v < prev {
			t.Fatal("Smoothstep not monotonic")
		}
		prev = v
	}
}
