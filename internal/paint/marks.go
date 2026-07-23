package paint

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// wobbler perturbs a radius as a function of angle using a few random
// harmonics plus a slow center drift — the "hand" in every mark.
type wobbler struct {
	amp   [3]float64
	phase [3]float64
	drift float64
	dPh   float64
}

func newWobbler(rng *rand.Rand, amount float64) wobbler {
	var w wobbler
	for i := range w.amp {
		w.amp[i] = amount * (0.4 + rng.Float64()) / float64(i+1)
		w.phase[i] = rng.Float64() * 2 * math.Pi
	}
	w.drift = amount * (0.5 + rng.Float64())
	w.dPh = rng.Float64() * 2 * math.Pi
	return w
}

func (w wobbler) at(theta float64) (dr, dx, dy float64) {
	for i := range w.amp {
		dr += w.amp[i] * math.Sin(float64(i+2)*theta+w.phase[i])
	}
	dx = w.drift * math.Cos(theta*0.5+w.dPh)
	dy = w.drift * math.Sin(theta*0.7+w.dPh)
	return dr, dx, dy
}

// Spiral returns a hand-drawn spiral path around (cx, cy) from radius r0
// to r1 over the given number of turns, with wobble in canvas units.
func Spiral(rng *rand.Rand, cx, cy, r0, r1, turns, wobble float64) []Pt {
	w := newWobbler(rng, wobble)
	n := int(turns*72) + 2
	pts := make([]Pt, n)
	for i := range pts {
		t := float64(i) / float64(n-1)
		theta := t * turns * 2 * math.Pi
		dr, dx, dy := w.at(theta)
		r := r0 + (r1-r0)*t + dr
		pts[i] = Pt{
			X: cx + dx + r*math.Cos(theta),
			Y: cy + dy + r*math.Sin(theta),
		}
	}
	return pts
}

// Loop returns a slightly-more-than-closed wobbly ring (1.1 turns, so the
// stroke overlaps its own start like a drawn circle).
func Loop(rng *rand.Rand, cx, cy, r, wobble float64) []Pt {
	start := rng.Float64() * 2 * math.Pi
	pts := Spiral(rng, cx, cy, r, r, 1.1, wobble)
	sin, cos := math.Sin(start), math.Cos(start)
	for i, p := range pts {
		dx, dy := p.X-cx, p.Y-cy
		pts[i] = Pt{X: cx + dx*cos - dy*sin, Y: cy + dx*sin + dy*cos}
	}
	return pts
}

// FillDisc paints a near-solid disc with a softly irregular edge by
// stroking a tight spiral with a wide brush.
func FillDisc(c *Canvas, rng *rand.Rand, cx, cy, r float64, col palette.Color, alpha float64) {
	w := r * 0.55
	turns := math.Max((r-w/2)/(w*0.45), 1)
	path := Spiral(rng, cx, cy, 0, r-w/2, turns, r*0.03)
	c.Dab(cx, cy, w*0.6, col, alpha)
	c.Stroke(path, w, col, alpha)
}

// RingsDisc paints the classic ringed dot: a solid under-disc slightly
// offset beneath a hand-drawn spiral of concentric ink rings and a center
// dot.
func RingsDisc(c *Canvas, rng *rand.Rand, cx, cy, r float64, ink, under palette.Color) {
	ox := (rng.Float64() - 0.5) * 0.18 * r
	oy := (rng.Float64() - 0.5) * 0.18 * r
	FillDisc(c, rng, cx+ox, cy+oy, r*0.97, under, 0.95)

	spacing := r / (5.5 + rng.Float64()*3.5)
	turns := (r*0.92 - r*0.15) / spacing
	path := Spiral(rng, cx, cy, r*0.15, r*0.92, turns, spacing*0.4)
	c.Stroke(path, r*0.055, ink, 0.9)
	c.Dab(cx, cy, r*0.07, ink, 0.9)
}

// ScribbleDisc paints an under-disc beneath many overlapping translucent
// loops at random radii — strokes build up where they cross.
func ScribbleDisc(c *Canvas, rng *rand.Rand, cx, cy, r float64, ink, under palette.Color) {
	ox := (rng.Float64() - 0.5) * 0.14 * r
	oy := (rng.Float64() - 0.5) * 0.14 * r
	FillDisc(c, rng, cx+ox, cy+oy, r*0.95, under, 0.9)

	loops := 9 + rng.IntN(6)
	for i := range loops {
		// Radii sweep outward with jitter, so loops stack roughly
		// concentric instead of tangling.
		lr := r * (0.25 + 0.68*(float64(i)+rng.Float64())/float64(loops))
		jx := (rng.Float64() - 0.5) * (r - lr) * 0.6
		jy := (rng.Float64() - 0.5) * (r - lr) * 0.6
		path := Loop(rng, cx+jx, cy+jy, lr, lr*0.12)
		c.Stroke(path, r*0.075, ink, 0.55)
	}
}

// GouacheDisc paints a flat opaque blob with an irregular edge over a
// misregistered shadow blob.
func GouacheDisc(c *Canvas, rng *rand.Rand, cx, cy, r float64, col, shadow palette.Color) {
	ox := (rng.Float64() - 0.5) * 0.22 * r
	oy := (rng.Float64() - 0.5) * 0.22 * r
	FillDisc(c, rng, cx+ox, cy+oy, r*0.96, shadow, 0.85)
	FillDisc(c, rng, cx, cy, r*0.94, col, 0.96)
}
