package gradient

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func colorsClose(a, b palette.Color) bool {
	const eps = 1e-9
	return math.Abs(a.R-b.R) < eps && math.Abs(a.G-b.G) < eps && math.Abs(a.B-b.B) < eps
}

func TestCosineBetweenEndpoints(t *testing.T) {
	c1 := palette.MustHex("#ED6A5A")
	c2 := palette.MustHex("#9BC1BC")
	g := CosineBetween(c1, c2)
	if got := g.At(0); !colorsClose(got, c1) {
		t.Errorf("At(0) = %v, want %v", got, c1)
	}
	if got := g.At(1); !colorsClose(got, c2) {
		t.Errorf("At(1) = %v, want %v", got, c2)
	}
	// Midpoint of a half-cosine ease is the average of the endpoints.
	mid := palette.Lerp(c1, c2, 0.5)
	if got := g.At(0.5); !colorsClose(got, mid) {
		t.Errorf("At(0.5) = %v, want %v", got, mid)
	}
}

func TestCosineAtClampsT(t *testing.T) {
	g := CosineBetween(palette.MustHex("#000000"), palette.MustHex("#FFFFFF"))
	if g.At(-5) != g.At(0) || g.At(5) != g.At(1) {
		t.Error("At should clamp t to [0,1]")
	}
}

func TestSample(t *testing.T) {
	c1, c2 := palette.MustHex("#000000"), palette.MustHex("#FFFFFF")
	d := Sample(CosineBetween(c1, c2), 50)
	if len(d) != 50 {
		t.Fatalf("len = %d, want 50", len(d))
	}
	if !colorsClose(d[0], c1) || !colorsClose(d[49], c2) {
		t.Error("sampled endpoints should match gradient endpoints")
	}
}

func TestDiscreteAt(t *testing.T) {
	d := Discrete{palette.Color{R: 1}, palette.Color{G: 1}, palette.Color{B: 1}}
	tests := []struct {
		t    float64
		want palette.Color
	}{
		{0, d[0]},
		{0.32, d[0]},
		{0.34, d[1]},
		{0.99, d[2]},
		{1, d[2]},
		{-1, d[0]},
		{2, d[2]}, // clamped
	}
	for _, tt := range tests {
		if got := d.At(tt.t); got != tt.want {
			t.Errorf("At(%v) = %v, want %v", tt.t, got, tt.want)
		}
	}
}

func TestShuffledDeterministicAndComplete(t *testing.T) {
	d := Sample(CosineBetween(palette.MustHex("#ED6A5A"), palette.MustHex("#F4F1BB")), 50)

	s1 := d.Shuffled(rand.New(rand.NewPCG(42, 0)))
	s2 := d.Shuffled(rand.New(rand.NewPCG(42, 0)))
	for i := range s1 {
		if s1[i] != s2[i] {
			t.Fatal("same seed must give the same shuffle")
		}
	}

	// Same multiset of colors, and source not mutated.
	counts := map[palette.Color]int{}
	for i := range d {
		counts[d[i]]++
		counts[s1[i]]--
	}
	for _, v := range counts {
		if v != 0 {
			t.Fatal("shuffle changed the color multiset")
		}
	}

	s3 := d.Shuffled(rand.New(rand.NewPCG(7, 0)))
	same := true
	for i := range s1 {
		if s1[i] != s3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different seeds should give different shuffles")
	}
}
