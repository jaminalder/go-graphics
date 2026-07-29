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

// QQL-style packing runs over an area larger than the canvas, so circles at
// negative coordinates must be indexed rather than silently dropped.
func TestNegativeCoordinatesAreIndexed(t *testing.T) {
	idx := NewIndexIn(-0.2, -0.2, 1.2, 1.2, 0.05)
	idx.Insert(Circle{X: -0.15, Y: -0.15, R: 0.02})
	if idx.FitsWithGap(Circle{X: -0.15, Y: -0.14, R: 0.02}, 0) {
		t.Error("a circle overlapping one at negative coordinates reported as fitting")
	}
	if !idx.FitsWithGap(Circle{X: 0.5, Y: 0.5, R: 0.02}, 0) {
		t.Error("a far-away circle reported as colliding")
	}
}

// Cell size is a performance knob only: results must not depend on it.
func TestCellSizeDoesNotChangeResults(t *testing.T) {
	rng := rand.New(rand.NewPCG(21, 0))
	var all []Circle
	for range 200 {
		all = append(all, Circle{
			X: -0.1 + rng.Float64()*1.2,
			Y: -0.1 + rng.Float64()*1.2,
			R: 0.002 + rng.Float64()*0.06,
		})
	}
	queries := make([]Circle, 0, 300)
	qrng := rand.New(rand.NewPCG(22, 0))
	for range 300 {
		queries = append(queries, Circle{
			X: -0.1 + qrng.Float64()*1.2,
			Y: -0.1 + qrng.Float64()*1.2,
			R: 0.002 + qrng.Float64()*0.06,
		})
	}

	var want []bool
	for ci, cell := range []float64{0.005, 0.02, 0.07, 0.3} {
		idx := NewIndexIn(-0.1, -0.1, 1.1, 1.1, cell)
		for _, c := range all {
			idx.Insert(c)
		}
		got := make([]bool, len(queries))
		for i, q := range queries {
			got[i] = idx.FitsWithGap(q, 0.001)
		}
		if ci == 0 {
			want = got
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("cell %v: query %d = %v, want %v", cell, i, got[i], want[i])
			}
		}
	}

	// Sanity: the queries must not be trivially all-true.
	fits := 0
	for _, v := range want {
		if v {
			fits++
		}
	}
	if fits == 0 || fits == len(want) {
		t.Fatalf("degenerate test: %d of %d queries fit", fits, len(want))
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
