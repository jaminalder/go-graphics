package pools

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// Where the marks go, borrowed from QQL.
//
// QQL's own note on its structure trait is the useful part: the structure
// decides where the flow lines start, "and that turns out to matter more
// than the field itself — the same field seeded in concentric bands, in
// soft blobs, or in a grid of rectangles gives three unmistakably different
// pieces". So what is taken here is the seeding, not the flow: each
// arrangement produces candidate positions in a laying order, and the
// spacing rule decides which of them survive. The field proposes and the
// packing disposes, which is QQL's mechanism with one moving part removed.
//
// The order matters as much as the positions. Candidates are walked in the
// order the structure produced them — round a ring, along a row, outward
// through a blob — and the colour walks with them (see colorWalk), so a
// piece comes out in passages rather than as confetti. That is the other
// half of what makes a QQL piece read as composed.

const dimArrange = "arrange"

// arrangements are the structures a seed can draw. Scatter is the original
// blue-noise placement and stays reachable: it is the only one that leaves
// the sheet with no direction at all, which some pieces want.
var arrangements = []trait.Value{
	{Name: "scatter", Weight: 2},
	{Name: "orbital", Weight: 3},
	{Name: "formation", Weight: 3},
	{Name: "shadows", Weight: 2},
}

// pt is a candidate position.
type pt struct{ x, y float64 }

// candidateCap bounds a structure's offer. A fine step over a bled canvas
// can ask for a hundred thousand positions, almost all of which the first
// few marks invalidate; this keeps a pathological fill from spending its
// render on arithmetic nobody sees.
const candidateCap = 20000

// candidates lays out the positions one arrangement offers, in the order
// they should be tried. step is the spacing to aim for, which the caller
// sets from the size ladder so the same structure works at every fill.
func candidates(name string, rng *rand.Rand, aspect, step float64) []pt {
	step = math.Max(step, 0.004)
	switch name {
	case "orbital":
		return capped(orbital(rng, aspect, step))
	case "formation":
		return capped(formation(rng, aspect, step))
	case "shadows":
		return capped(shadows(rng, aspect, step))
	default:
		return nil // scatter has no structure; the planner throws darts
	}
}

// capped trims an offer that ran away.
func capped(p []pt) []pt {
	if len(p) > candidateCap {
		return p[:candidateCap]
	}
	return p
}

// bleed is how far past the canvas a structure reaches, so a composition
// runs off the edge instead of stopping politely at it.
const bleed = 0.15

// orbital seeds concentric rings around a centre that is often off-canvas,
// so the sheet shows an arc of something larger rather than a target.
func orbital(rng *rand.Rand, aspect, step float64) []pt {
	cx := aspect * rnd.Pick(rng, []rnd.Weighted[float64]{
		{V: 0.5, W: 3}, {V: 0.2, W: 2}, {V: 0.8, W: 2}, {V: -0.4, W: 1}, {V: 1.4, W: 1},
	})
	cy := rnd.Pick(rng, []rnd.Weighted[float64]{
		{V: 0.5, W: 3}, {V: 0.2, W: 2}, {V: 0.8, W: 2}, {V: -0.4, W: 1}, {V: 1.4, W: 1},
	})
	// Rings are spaced a little wider than the marks, so the bands read as
	// bands; packed exactly at step they merge into a texture.
	ringStep := step * rnd.Uniform(rng, 1.0, 1.5)
	phase := rnd.Uniform(rng, 0, 2*math.Pi)

	reach := 0.0
	for _, c := range [][2]float64{{0, 0}, {aspect, 0}, {0, 1}, {aspect, 1}} {
		reach = math.Max(reach, math.Hypot(c[0]-cx, c[1]-cy))
	}

	var out []pt
	for r := ringStep; r < reach+ringStep; r += ringStep {
		// One mark every `step` of arc, so rings stay evenly dense however
		// large they get.
		n := max(int(2*math.Pi*r/step), 1)
		for i := range n {
			th := phase + float64(i)*2*math.Pi/float64(n)
			p := pt{cx + r*math.Cos(th), cy + r*math.Sin(th)}
			if inBleed(p, aspect) {
				out = append(out, p)
			}
		}
	}
	return out
}

// formation rasters a grid of rectangles and drops some of them. The gaps
// are what keep it from reading as wallpaper.
func formation(rng *rand.Rand, aspect, step float64) []pt {
	across := rnd.Pick(rng, []rnd.Weighted[int]{{V: 1, W: 3}, {V: 2, W: 3}, {V: 3, W: 2}, {V: 4, W: 1}})
	down := rnd.Pick(rng, []rnd.Weighted[int]{{V: 1, W: 3}, {V: 2, W: 3}, {V: 3, W: 2}, {V: 4, W: 1}})
	skip := rnd.Pick(rng, []rnd.Weighted[float64]{{V: 0, W: 3}, {V: 0.15, W: 2}, {V: 0.35, W: 1}})
	// Rows or columns: which way a chunk is rastered decides which way the
	// colour passages run, and that is half of how the piece reads.
	columnar := rnd.Odds(rng, 0.5)
	jitter := step * rnd.Uniform(rng, 0.05, 0.3)

	w := (aspect + 2*bleed) / float64(across)
	h := (1 + 2*bleed) / float64(down)

	var out []pt
	first := true
	for i := range across {
		for j := range down {
			// The first chunk always survives, so no seed comes out empty.
			if !first && rnd.Odds(rng, skip) {
				continue
			}
			first = false
			x0, y0 := -bleed+float64(i)*w, -bleed+float64(j)*h
			outer, inner := h, w
			if columnar {
				outer, inner = w, h
			}
			for a := 0.0; a < outer; a += step {
				for b := 0.0; b < inner; b += step {
					p := pt{x0 + b, y0 + a}
					if columnar {
						p = pt{x0 + a, y0 + b}
					}
					p.x += rnd.Gauss(rng, 0, jitter)
					p.y += rnd.Gauss(rng, 0, jitter)
					if inBleed(p, aspect) {
						out = append(out, p)
					}
				}
			}
		}
	}
	return out
}

// shadows scatters non-overlapping blobs and fills each one from the rim
// inward, so they read as separate objects in the same field.
func shadows(rng *rand.Rand, aspect, step float64) []pt {
	count := rnd.Choice(rng, []int{3, 4, 6, 9, 14})
	type blob struct{ x, y, r float64 }
	var blobs []blob
	for iter := 0; len(blobs) < count && iter < 400; iter++ {
		b := blob{
			x: rnd.Uniform(rng, 0, aspect),
			y: rnd.Uniform(rng, 0, 1),
			r: rnd.Uniform(rng, 4*step, math.Max(0.42, 6*step)),
		}
		clear := true
		for _, o := range blobs {
			if math.Hypot(b.x-o.x, b.y-o.y) < b.r+o.r {
				clear = false
				break
			}
		}
		if clear {
			blobs = append(blobs, b)
		}
	}

	outward := rnd.Odds(rng, 0.5)
	var out []pt
	for _, b := range blobs {
		var group []pt
		for r := b.r; r > 0; r -= step {
			n := max(int(2*math.Pi*r/step), 1)
			phase := rnd.Uniform(rng, 0, 2*math.Pi)
			for i := range n {
				th := phase + float64(i)*2*math.Pi/float64(n)
				p := pt{b.x + r*math.Cos(th), b.y + r*math.Sin(th)}
				if inBleed(p, aspect) {
					group = append(group, p)
				}
			}
		}
		group = append(group, pt{b.x, b.y})
		if outward {
			for i, j := 0, len(group)-1; i < j; i, j = i+1, j-1 {
				group[i], group[j] = group[j], group[i]
			}
		}
		out = append(out, group...)
	}
	return out
}

func inBleed(p pt, aspect float64) bool {
	return p.x > -bleed && p.x < aspect+bleed && p.y > -bleed && p.y < 1+bleed
}

// colorWalk is how a piece gets passages of colour instead of confetti.
//
// It steps an index along the luminance-ordered pigments and mostly stays
// put: neighbouring marks — which, because candidates are walked in the
// structure's own order, are neighbours on the sheet too — come out
// related, and the piece turns over a handful of times rather than at every
// mark. Drawing each mark's colour freely gives every pigment equal
// presence everywhere, which is what reads as confetti.
type colorWalk struct {
	n     int     // pigments to walk over
	at    int     // where the walk stands
	churn float64 // chance of stepping at each mark
}

func newColorWalk(rng *rand.Rand, n int) *colorWalk {
	return &colorWalk{
		n:  n,
		at: rng.IntN(max(n, 1)),
		// Low: a piece with a dozen colour changes reads as composed, one
		// with a hundred reads as noise.
		churn: rnd.Uniform(rng, 0.04, 0.22),
	}
}

// next advances the walk and returns the pigment index for the next mark.
func (w *colorWalk) next(rng *rand.Rand) int {
	if w.n < 2 {
		return 0
	}
	if rnd.Odds(rng, w.churn) {
		step := rnd.Pick(rng, []rnd.Weighted[int]{{V: 1, W: 6}, {V: 2, W: 3}, {V: 3, W: 1}})
		if rnd.Odds(rng, 0.5) {
			step = -step
		}
		w.at = ((w.at+step)%w.n + w.n) % w.n
	}
	return w.at
}
