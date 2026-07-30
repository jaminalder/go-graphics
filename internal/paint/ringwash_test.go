package paint

import (
	"math"
	"testing"
)

// ringWash is a wash with the interior variation switched off, so tests
// about the shape of a ring are not decided by granulation or pooling.
func ringWash(seed uint64) Wash {
	w := DefaultWash(seed)
	w.Grain, w.Mottle = 0, 0
	w.Ragged = 0.05
	return w
}

// TestRingLeavesBarePaper is the difference between an annulus and a pool
// with the ground repainted over it: nothing may be deposited inside the
// hole, at any strength, so the paper there is untouched rather than
// covered up.
func TestRingLeavesBarePaper(t *testing.T) {
	const px = 600
	c := NewCanvas(px, px, white)
	ringWash(21).Ring(c, washRNG(2), 0.5, 0.5, 0.25, 0.1, cyan, 0.9)

	// Anywhere comfortably inside the hole — the inner boundary wobbles,
	// so stay clear of it rather than probing right up against it.
	for _, rr := range []float64{0, 0.05, 0.12} {
		for i := range 16 {
			a := float64(i) * 2 * math.Pi / 16
			if got := at(c, 0.5+rr*math.Cos(a), 0.5+rr*math.Sin(a)); got != white {
				t.Fatalf("pigment at radius %v (angle %.2f): %v — the hole is not bare", rr, a, got)
			}
		}
	}
	// And the band itself did get painted.
	if at(c, 0.5, 0.25) == white {
		t.Fatal("nothing painted on the band")
	}
}

// TestRingRimsBothBoundaries pins the reason Ring exists as its own
// primitive. Both edges of a brushed circle of water are wet edges, so the
// pigment banks at both of them and the middle of the band dries lighter.
// A ring drawn as a stroke — or as a pool with a hole punched in it — is
// flat across the band or dark only on the outside.
func TestRingRimsBothBoundaries(t *testing.T) {
	const px, rays = 800, 96
	const rOut, rIn = 0.30, 0.20
	c := NewCanvas(px, px, white)
	ringWash(23).Ring(c, washRNG(6), 0.5, 0.5, (rOut+rIn)/2, rOut-rIn, cyan, 0.7)

	// Measure the whole radial profile rather than probing fixed radii:
	// both boundaries wobble, so where each rim lands is not known in
	// advance — only that the profile across the band must run
	// dark, light, dark.
	const steps = 80
	radius := func(k int) float64 { return 0.15 + 0.22*float64(k)/(steps-1) }
	profile := make([]float64, steps)
	for k := range profile {
		rr, sum := radius(k), 0.0
		for i := range rays {
			a := float64(i) * 2 * math.Pi / rays
			sum += at(c, 0.5+rr*math.Cos(a), 0.5+rr*math.Sin(a)).Luminance()
		}
		profile[k] = sum / rays
	}
	darkest := func(from, to int) (at int) {
		at = from
		for k := from; k < to; k++ {
			if profile[k] < profile[at] {
				at = k
			}
		}
		return at
	}
	in := darkest(0, steps/2)
	out := darkest(steps/2, steps)
	lightest := in
	for k := in; k <= out; k++ {
		if profile[k] > profile[lightest] {
			lightest = k
		}
	}
	if lightest == in || lightest == out {
		t.Fatalf("the band has no lighter middle: profile bottoms at r=%.3f and r=%.3f with nothing between",
			radius(in), radius(out))
	}
	// A rim the eye can see, not one only arithmetic can.
	for _, rim := range []struct {
		name string
		at   int
	}{{"inner", in}, {"outer", out}} {
		if d := profile[lightest] - profile[rim.at]; d < 0.02 {
			t.Errorf("%s rim at r=%.3f is only %.4f darker than the band middle at r=%.3f — no rim there",
				rim.name, radius(rim.at), d, radius(lightest))
		}
	}
}

// TestFatRingClosesIntoAPool guards the boundary between the two
// primitives: as the band grows past the radius there is no hole left to
// have, and the ring has to become an ordinary pool rather than inverting
// or dropping out.
func TestFatRingClosesIntoAPool(t *testing.T) {
	c := NewCanvas(300, 300, white)
	ringWash(4).Ring(c, washRNG(8), 0.5, 0.5, 0.1, 0.5, cyan, 0.8)
	if at(c, 0.5, 0.5) == white {
		t.Fatal("a ring thicker than its radius left a hole")
	}
}

func TestRingIsDeterministicAndBounded(t *testing.T) {
	build := func() *Canvas {
		c := NewCanvas(240, 240, white)
		ringWash(9).Ring(c, washRNG(4), 0.5, 0.5, 0.2, 0.06, cyan, 0.8)
		return c
	}
	a, b := build(), build()
	for i := range a.pix {
		if a.pix[i] != b.pix[i] {
			t.Fatal("same seed must give the same ring")
		}
	}
	lim := (0.2 + 0.03) * 1.3 // outer radius plus the raggedness budget
	for y := range 240 {
		for x := range 240 {
			if a.pix[y*240+x] == white {
				continue
			}
			dx := (float64(x)+0.5)/240 - 0.5
			dy := (float64(y)+0.5)/240 - 0.5
			if d := math.Hypot(dx, dy); d > lim {
				t.Fatalf("pigment at distance %v, beyond the %v budget", d, lim)
			}
		}
	}
}

func TestWashRingIgnoresDegenerateInput(t *testing.T) {
	c := NewCanvas(64, 64, white)
	blank := NewCanvas(64, 64, white)
	w := ringWash(2)
	w.Ring(c, washRNG(1), 0.5, 0.5, 0.2, 0, cyan, 0.8)       // no thickness
	w.Ring(c, washRNG(1), 0.5, 0.5, 0.2, -0.1, cyan, 0.8)    // negative
	w.Ring(c, washRNG(1), 0.5, 0.5, 0.2, 0.05, cyan, 0)      // no pigment
	w.Ring(c, washRNG(1), 0.5, 0.5, 0.0001, 0.0001, cyan, 1) // sub-pixel
	for i := range c.pix {
		if c.pix[i] != blank.pix[i] {
			t.Fatal("a degenerate ring painted something")
		}
	}
}
