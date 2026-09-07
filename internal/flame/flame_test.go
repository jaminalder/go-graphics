package flame

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func TestPolarUsesThePapersTheta(t *testing.T) {
	// θ = atan2(x, y). At (1, 0) that is π/2, so polar → (0.5, 0).
	// atan2(y, x) would give 0 and this test would fail.
	x, y := applyVar(Var{Kind: Polar}, 1, 0, Xform{}, rand.New(rand.NewPCG(1, 1)))
	if math.Abs(x-0.5) > 1e-9 || math.Abs(y) > 1e-9 {
		t.Fatalf("polar(1,0) = (%g,%g), want (0.5, 0)", x, y)
	}
}

func TestSphericalInvertsTheUnitCircle(t *testing.T) {
	x, y := applyVar(Var{Kind: Spherical}, 1, 0, Xform{}, nil)
	if math.Abs(x-1) > 1e-9 || math.Abs(y) > 1e-9 {
		t.Fatalf("spherical(1,0) = (%g,%g), want (1, 0)", x, y)
	}
	x, y = applyVar(Var{Kind: Spherical}, 2, 0, Xform{}, nil)
	if math.Abs(x-0.5) > 1e-9 || math.Abs(y) > 1e-9 {
		t.Fatalf("spherical(2,0) = (%g,%g), want (0.5, 0)", x, y)
	}
}

func TestSinusoidalAtOriginIsOrigin(t *testing.T) {
	x, y := applyVar(Var{Kind: Sinusoidal}, 0, 0, Xform{}, nil)
	if x != 0 || y != 0 {
		t.Fatalf("sinusoidal(0,0) = (%g,%g)", x, y)
	}
}

func TestHeartIsPointedAlongPlusX(t *testing.T) {
	x, y := applyVar(Var{Kind: Heart}, 1, 0, Xform{}, nil)
	if math.Abs(x-1) > 1e-9 || math.Abs(y) > 1e-9 {
		t.Fatalf("heart(1,0) = (%g,%g), want (1, 0)", x, y)
	}
}

func TestJulianNPowerOneIsIdentity(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 1))
	x, y := applyVar(Var{Kind: JulianN, P: [4]float64{1, 1}}, 0.6, -0.3, Xform{}, rng)
	if math.Abs(x-0.6) > 1e-9 || math.Abs(y+0.3) > 1e-9 {
		t.Fatalf("julianN power=1 (0.6,-0.3)=(%g,%g)", x, y)
	}
}

func TestJulianNDistScalesRadius(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 1))
	x, y := applyVar(Var{Kind: JulianN, P: [4]float64{1, 2}}, 0, 0.5, Xform{}, rng)
	if math.Abs(x) > 1e-9 || math.Abs(y-0.25) > 1e-9 {
		t.Fatalf("got (%g,%g) want (0, 0.25)", x, y)
	}
}

func TestJulianNHasSeveralBranches(t *testing.T) {
	// Power 4 must pick among 4 angles; a single branch cannot make filigree.
	seen := map[[2]int]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		rng := rand.New(rand.NewPCG(seed, 1))
		x, y := applyVar(Var{Kind: JulianN, P: [4]float64{4, 1}}, 1, 0, Xform{}, rng)
		key := [2]int{int(math.Round(x * 8)), int(math.Round(y * 8))}
		seen[key]++
	}
	if len(seen) < 3 {
		t.Fatalf("power-4 julianN produced %d distinct images of (1,0); want several branches", len(seen))
	}
}

func TestLogDensityKeepsThinFilamentsVisible(t *testing.T) {
	// A linear map of 10 vs 1000 hits would crush the thin bin to ~1%
	// brightness. Log-density must leave it clearly above the floor.
	h := &Hist{
		w: 2, h: 1,
		r: []uint64{colorScale * 10, colorScale * 1000},
		g: []uint64{colorScale * 10, colorScale * 1000},
		b: []uint64{colorScale * 10, colorScale * 1000},
		a: []uint64{colorScale * 10, colorScale * 1000},
	}
	pix := h.Develop(Tone{Gamma: 1, Vibrancy: 1, Brightness: 1})
	thin := pix[0].R
	core := pix[1].R
	if thin < 0.15 {
		t.Fatalf("thin filament brightness %g, want enough to read", thin)
	}
	if core <= thin {
		t.Fatalf("core %g should exceed filament %g", core, thin)
	}
	if thin/core < 0.2 {
		t.Fatalf("filament/core = %g, log map has collapsed the thin visits", thin/core)
	}
}

func TestAccumulateIsDeterministic(t *testing.T) {
	s := System{
		X: []Xform{
			{A: 0.6, E: 0.6, Vars: []Var{{Kind: Spherical, Weight: 1}}, Color: 0.2, ColorSpeed: 0.5, Weight: 1},
			{A: 0.5, E: 0.5, C: 0.3, Vars: []Var{{Kind: Swirl, Weight: 1}}, Color: 0.8, ColorSpeed: 0.5, Weight: 1},
		},
		Cam: Camera{Scale: 3},
	}
	color := func(t float64) palette.Color { return palette.Color{R: t, G: 0.4, B: 1 - t} }
	a := Accumulate(s, color, 24, 24, 8000, 7)
	b := Accumulate(s, color, 24, 24, 8000, 7)
	for i := range a.a {
		if a.a[i] != b.a[i] || a.r[i] != b.r[i] {
			t.Fatalf("histogram bin %d changed across runs", i)
		}
	}
}

func TestFrameIsDeterministic(t *testing.T) {
	sys := System{
		X: []Xform{
			{A: 0.7, E: 0.9, Vars: []Var{{Kind: Heart, Weight: 0.8}, {Kind: Spherical, Weight: 0.2}}, Weight: 1, ColorSpeed: 0.5},
			{A: 0.5, B: -0.2, D: 0.2, E: 0.5, Vars: []Var{{Kind: Swirl, Weight: 1}}, Weight: 1, ColorSpeed: 0.5},
		},
	}
	rng := func() *rand.Rand { return rand.New(rand.NewPCG(11, 3)) }
	c1 := Frame(sys, rng(), 4000)
	c2 := Frame(sys, rng(), 4000)
	if c1 != c2 {
		t.Fatalf("camera %v vs %v", c1, c2)
	}
	if c1.Scale <= 0 {
		t.Fatal("camera scale must be positive")
	}
}

func TestSymmetryMapsDoNotBlendColour(t *testing.T) {
	x := Xform{Color: 1, ColorSpeed: 0.5, Symmetry: true}
	if got := x.blendColor(0.2); got != 0.2 {
		t.Fatalf("symmetry blend %g, want 0.2", got)
	}
}

func TestXaosZeroForbidsATransition(t *testing.T) {
	s := System{
		X: []Xform{
			{A: 0.5, E: 0.5, Weight: 1},
			{A: 0.5, E: 0.5, C: 0.4, Weight: 1},
			{A: 0.5, E: 0.5, F: 0.4, Weight: 1},
		},
	}
	s.InitXaos()
	s.SetXaos(0, 1, 0) // 0 may not go to 1
	s.prepare()
	rng := rand.New(rand.NewPCG(3, 9))
	for i := 0; i < 2000; i++ {
		if got := s.pick(rng, 0); got == 1 {
			t.Fatalf("xaos forbade 0→1, pick returned 1 on draw %d", i)
		}
	}
}

func TestXaosAllOnesMatchesRawWeights(t *testing.T) {
	s := System{
		X: []Xform{
			{Weight: 1},
			{Weight: 3},
		},
	}
	s.InitXaos()
	s.prepare()
	rng := rand.New(rand.NewPCG(1, 2))
	var n0, n1 int
	for i := 0; i < 8000; i++ {
		if s.pick(rng, 0) == 0 {
			n0++
		} else {
			n1++
		}
	}
	// 1:3, with room for Monte Carlo noise.
	if n1 < 2*n0 || n1 > 5*n0 {
		t.Fatalf("all-ones xaos from 0: n0=%d n1=%d, want ~1:3", n0, n1)
	}
}
