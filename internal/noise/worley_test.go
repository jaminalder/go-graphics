package noise

import "testing"

func TestWorleyDeterministic(t *testing.T) {
	a1, a2 := Worley(7, 3.7, -1.2)
	b1, b2 := Worley(7, 3.7, -1.2)
	if a1 != b1 || a2 != b2 {
		t.Error("same inputs must give identical distances")
	}
	c1, _ := Worley(8, 3.7, -1.2)
	if a1 == c1 {
		t.Error("different seeds should move the feature points")
	}
}

func TestWorleyOrderingAndRange(t *testing.T) {
	for i := range 200 {
		x := float64(i)*0.37 - 30
		y := float64(i)*0.71 - 20
		f1, f2 := Worley(42, x, y)
		if f1 < 0 || f2 < f1 {
			t.Fatalf("at (%v, %v): f1=%v f2=%v violates 0 <= f1 <= f2", x, y, f1, f2)
		}
		// With one feature point per unit cell and a 3x3 neighborhood, the
		// nearest point is at most ~1.5 cells away.
		if f1 > 1.6 {
			t.Fatalf("at (%v, %v): f1=%v implausibly large", x, y, f1)
		}
	}
}

func TestWorleyCellIdentity(t *testing.T) {
	// Near a feature point, the nearest cell is that point's cell.
	seed := uint64(5)
	for i := int64(-3); i <= 3; i++ {
		for j := int64(-3); j <= 3; j++ {
			px := float64(i) + Hash01(seed^worleySaltX, i, j)
			py := float64(j) + Hash01(seed^worleySaltY, i, j)
			cx, cy, f1, _ := WorleyCell(seed, px, py)
			if cx != i || cy != j {
				t.Fatalf("at feature point of cell (%d,%d): got cell (%d,%d)", i, j, cx, cy)
			}
			if f1 > 1e-12 {
				t.Fatalf("f1 at feature point = %v, want 0", f1)
			}
		}
	}
	// Cell identity varies across space.
	seen := map[[2]int64]bool{}
	for i := range 50 {
		cx, cy, _, _ := WorleyCell(seed, float64(i)*0.9, float64(i)*0.7)
		seen[[2]int64{cx, cy}] = true
	}
	if len(seen) < 20 {
		t.Errorf("expected many distinct cells, got %d", len(seen))
	}
}

func TestWorleyBordersExist(t *testing.T) {
	// Somewhere on a dense sample grid the border metric f2-f1 must come
	// close to zero (cell boundaries exist).
	minDiff := 10.0
	for i := range 100 {
		for j := range 100 {
			f1, f2 := Worley(1, float64(i)*0.1, float64(j)*0.1)
			if d := f2 - f1; d < minDiff {
				minDiff = d
			}
		}
	}
	if minDiff > 0.05 {
		t.Errorf("no near-border samples found; min f2-f1 = %v", minDiff)
	}
}
