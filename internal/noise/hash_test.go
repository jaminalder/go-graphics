package noise

import "testing"

func TestHash01Range(t *testing.T) {
	for i := int64(-100); i < 100; i++ {
		for j := int64(-100); j < 100; j += 7 {
			v := Hash01(42, i, j)
			if v < 0 || v >= 1 {
				t.Fatalf("Hash01(42, %d, %d) = %v out of [0,1)", i, j, v)
			}
		}
	}
}

func TestHash01Deterministic(t *testing.T) {
	if Hash01(7, 13, -5) != Hash01(7, 13, -5) {
		t.Error("same inputs must hash identically")
	}
}

func TestHash01Varies(t *testing.T) {
	seen := map[float64]bool{}
	for i := int64(0); i < 100; i++ {
		seen[Hash01(1, i, 0)] = true
	}
	if len(seen) < 95 {
		t.Errorf("expected ~100 distinct values, got %d", len(seen))
	}
	if Hash01(1, 3, 4) == Hash01(2, 3, 4) {
		t.Error("different seeds should differ")
	}
	if Hash01(1, 3, 4) == Hash01(1, 4, 3) {
		t.Error("coordinates should not be symmetric")
	}
}

func TestHash01Uniformish(t *testing.T) {
	sum := 0.0
	const n = 10000
	for i := int64(0); i < n; i++ {
		sum += Hash01(9, i, i*31)
	}
	mean := sum / n
	if mean < 0.47 || mean > 0.53 {
		t.Errorf("mean = %v, expected ≈0.5", mean)
	}
}
