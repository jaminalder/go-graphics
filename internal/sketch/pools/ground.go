package pools

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// The ground is laid, not filled. A flat wash is the first thing a
// watercolourist learns and the hardest to make even: it goes on in
// overlapping strokes while the paper is wet, and it dries with the
// unevenness of that process in it — heavier where strokes crossed,
// granulated wherever the paper's tooth caught pigment, never quite the
// same colour twice across a sheet.
//
// So the canvas starts as bare paper and the ground colour is glazed over
// it in a few large pools that run off every edge. What makes it read as
// painted is that the paper still shows through: the variation is the
// paper, not a texture laid over a flat fill.

// groundPitch is the spacing of the pools, in canvas units. They are laid
// on a jittered grid at rather more than this radius, so every point is
// covered by two or three and the crossings do the work.
const groundPitch = 0.55

// groundBleed is how far past the canvas the grid extends. A pool boundary
// inside the frame would read as a circle on the ground rather than as the
// ground, and this sketch is already full of circles.
const groundBleed = 0.4

// groundWash is the wash the ground is laid with: broad, soft-edged and
// almost rimless. A flat wash keeps a hard edge only where it stops, which
// on a full sheet is off the paper entirely.
func groundWash(seed uint64) paint.Wash {
	w := paint.DefaultWash(seed)
	// Deposits are what the softness costs, and a pool the size of the
	// canvas pays it on every pixel it covers. A ground has no fine
	// structure to resolve — it is the one wash in the picture that is
	// supposed to be featureless — so it runs on a quarter of the stack,
	// which is the difference between the ground being most of the render
	// and being a fraction of it.
	w.Layers = 6
	// Very irregular, so that where a pool's edge does fall inside the
	// frame it reads as the far side of a stroke rather than as a circle.
	w.Ragged = 0.5
	// A rim is the mark of a boundary that dried in place. These have none
	// inside the frame, and any rim here would draw a ring on the sky.
	w.Edge = 0.08
	// The two cues that survive from a real flat wash, both turned up: the
	// pigment pooling unevenly and the paper's tooth holding it.
	w.Mottle = 0.7
	w.Grain = 0.26
	return w
}

// paintGround glazes col over the bare paper the canvas starts as.
// strength is how far the wash carries; at 0 the caller should not have
// called at all, and at 1 the paper barely shows.
func paintGround(cv *paint.Canvas, rng *rand.Rand, w paint.Wash, aspect, strength float64, col palette.Color) {
	cols := int(math.Ceil((aspect+2*groundBleed)/groundPitch)) + 1
	rows := int(math.Ceil((1+2*groundBleed)/groundPitch)) + 1

	// Each pool is laid weakly and the coverage is what builds the colour,
	// so the ground gains its depth the way the real thing does — from how
	// many strokes happened to cross a spot — rather than from one flat
	// value with noise added to it.
	alpha := 0.16 + 0.3*strength
	for row := range rows {
		for c := range cols {
			x := -groundBleed + float64(c)*groundPitch + groundPitch*0.6*(rng.Float64()-0.5)
			y := -groundBleed + float64(row)*groundPitch + groundPitch*0.6*(rng.Float64()-0.5)
			// Just wide enough to close over the grid's corners. Wider
			// costs coverage that nobody sees: every extra pool over a
			// pixel is another stack to walk there.
			r := groundPitch * (0.72 + 0.22*rng.Float64())
			w.Pool(cv, rng, x, y, r, col, alpha*(0.8+0.4*rng.Float64()))
		}
	}
}
