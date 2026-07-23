package geom

import (
	"math"
	"math/rand/v2"
	"testing"
)

func randomIndex(n int) (*Index, []Circle) {
	rng := rand.New(rand.NewPCG(7, 0))
	idx := NewIndex(1, 1, 0.1)
	var all []Circle
	for range n {
		c := Circle{X: rng.Float64(), Y: rng.Float64(), R: 0.005 + rng.Float64()*0.09}
		idx.Insert(c)
		all = append(all, c)
	}
	return idx, all
}

func TestAtMatchesBruteForce(t *testing.T) {
	idx, all := randomIndex(60)
	for i := range 800 {
		x := float64(i%40) / 39
		y := float64(i/40) / 19.95
		want := -1
		for j, c := range all {
			dx, dy := x-c.X, y-c.Y
			if dx*dx+dy*dy <= c.R*c.R {
				want = j
				break
			}
		}
		if got := idx.At(x, y); got != want {
			t.Fatalf("At(%v, %v) = %d, want %d", x, y, got, want)
		}
	}
}

func TestFitsMatchesBruteForce(t *testing.T) {
	idx, all := randomIndex(60)
	rng := rand.New(rand.NewPCG(9, 0))
	const gap = 0.004
	for range 500 {
		c := Circle{X: rng.Float64(), Y: rng.Float64(), R: 0.005 + rng.Float64()*0.08}
		want := true
		for _, o := range all {
			dx, dy := o.X-c.X, o.Y-c.Y
			if math.Sqrt(dx*dx+dy*dy) < o.R+c.R+gap {
				want = false
				break
			}
		}
		if got := idx.FitsWithGap(c, gap); got != want {
			t.Fatalf("FitsWithGap(%+v) = %v, want %v", c, got, want)
		}
	}
}

func TestInsertReturnsOrder(t *testing.T) {
	idx := NewIndex(1, 1, 0.1)
	if i := idx.Insert(Circle{0.5, 0.5, 0.05}); i != 0 {
		t.Fatalf("first insert index = %d", i)
	}
	if i := idx.Insert(Circle{0.2, 0.2, 0.05}); i != 1 {
		t.Fatalf("second insert index = %d", i)
	}
	if len(idx.Circles()) != 2 {
		t.Fatal("Circles() length mismatch")
	}
}
