package paint

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// Painting an arbitrary shape with a wash.
//
// A Wash pool is *radial* — one radius per angle around a centre — which
// describes a blob exactly and a crescent not at all. That looked like a
// reason to write a different watercolour model for regions that are not
// round. It is not. A painter filling an irregular shape does not have a
// crescent-shaped brush; they lay several round touches that between them
// cover it, and the shape comes from where the touches stop.
//
// So: cover the region with pools, each sized to how deep the region is
// where it sits. Every pool is the same pool the rest of the repo already
// paints with, so a foam cell, a polygon and a circle are all filled by the
// same transparent, granulating, rim-drying model — and because the wash is
// transparent, the places where two touches overlap deepen exactly the way
// a second brush load does.
//
// The radius follows the region's own depth, which is what makes this work
// on a concave shape: pools crowd small into the waist of a crescent and
// run large in its lobes, and their union is the crescent.

// Region is any shape that can say how deep inside itself a point lies:
// positive within, 0 on the boundary, negative outside, in canvas units.
//
// A foam cell answers with its wall distance, a disc with r − |p − c|, a
// polygon with its signed distance. Anything that can answer this can be
// painted, which is the whole point of asking for it rather than for a
// centre and a radius.
type Region interface {
	Depth(u, v float64) float64
}

// Box bounds a region in canvas units. Fill only looks inside it, so a box
// that is too small crops the paint and one that is far too large only
// costs time.
type Box struct {
	MinU, MinV, MaxU, MaxV float64
}

// Fill covers a region with pools of one colour.
//
// alpha is the strength of a single touch, not of the finished region:
// where touches overlap the wash deepens, which is the model working as
// intended rather than an error to correct. reach is how far past the
// boundary a pool is allowed to sit, as a multiple of the local depth —
// slightly over 1 puts the pool's nominal edge on the boundary and lets the
// ragged edge wobble across it, which is the misregistration that stops a
// painted shape looking like a filled one.
func (w Wash) Fill(c *Canvas, rng *rand.Rand, reg Region, box Box, col palette.Color, alpha, reach float64) {
	for _, p := range cover(rng, reg, box) {
		w.Pool(c, rng, p.u, p.v, p.r*reach, col, alpha)
	}
}

// touch is one brush load: where it goes and how big it is.
type touch struct{ u, v, r float64 }

// refine walks a touch uphill to the deepest point near it.
//
// Darts alone will not find it. The deepest point of a region is a single
// spot, and a few dozen darts over a bounding box land near it only by luck,
// so every pool came out a fraction of the size it should have been and a
// cell read as a cluster of small blobs rather than as one brush load. A
// short hill climb costs a couple of dozen depth lookups and finds the
// inscribed radius properly.
func refine(reg Region, t touch, rng *rand.Rand) touch {
	step := t.r
	for range 40 {
		a := rng.Float64() * 2 * math.Pi
		u, v := t.u+step*math.Cos(a), t.v+step*math.Sin(a)
		if d := reg.Depth(u, v); d > t.r {
			t = touch{u, v, d}
			continue
		}
		step *= 0.88
	}
	return t
}

// cover chooses the touches. Deepest first, and a candidate is dropped if
// an accepted touch already reaches it — a greedy covering of the region's
// medial axis, which is where a brush naturally sits.
func cover(rng *rand.Rand, reg Region, box Box) []touch {
	w, h := box.MaxU-box.MinU, box.MaxV-box.MinV
	if w <= 0 || h <= 0 {
		return nil
	}
	// Enough candidates that the deepest points are found, scaled to the
	// area rather than fixed: one number cannot serve a sliver and a
	// quarter-canvas lobe.
	n := min(max(int(48*w*h/(0.01*0.01)/100), 40), 900)

	cand := make([]touch, 0, n)
	for range n {
		u := box.MinU + rng.Float64()*w
		v := box.MinV + rng.Float64()*h
		if d := reg.Depth(u, v); d > 0 {
			cand = append(cand, touch{u, v, d})
		}
	}
	if len(cand) == 0 {
		// A region too thin for the darts to land in still has to be
		// painted, or a packed sheet loses its slivers. Aim at the middle.
		u, v := (box.MinU+box.MaxU)/2, (box.MinV+box.MaxV)/2
		if d := reg.Depth(u, v); d > 0 {
			return []touch{{u, v, d}}
		}
		return nil
	}
	sort.SliceStable(cand, func(i, j int) bool { return cand[i].r > cand[j].r })

	// A touch is worth adding only if its *centre* lies outside every pool
	// already down. Anything closer sits inside one, and adds a dark patch
	// rather than any shape — which is what a round cell looked like when
	// this admitted overlapping touches: a cluster of blobs with the corners
	// left bare, instead of one confident load of the brush.
	// Strictly outside: a candidate closer than this sits *inside* a pool
	// already down and would only add a dark patch. At 0.85 it admitted
	// touches near the rim of a round region and the result was a cluster of
	// blobs instead of one confident load of the brush.
	const spread = 1.0
	// A touch may be much shallower than the deepest one and still be worth
	// making: the horns of a crescent are thin, and with the bar set high
	// they went unpainted — bare paper inside a painted shape. The strict
	// spread above is what makes a low bar safe, because a shallow candidate
	// is only reached at all if nothing already covers it.
	const least = 0.08
	out := make([]touch, 0, 8)
	deepest := cand[0].r
	for _, t := range cand {
		if t.r < deepest*least {
			continue
		}
		keep := true
		for _, o := range out {
			if math.Hypot(o.u-t.u, o.v-t.v) < o.r*spread {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, refine(reg, t, rng))
		}
		if len(out) >= 12 {
			break
		}
	}
	return out
}
