package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// The wash medium swaps QQL's painting model without touching its drawing.
// Every band a dot is built from is still exactly where the ink medium
// would put it, at the same radius and thickness and in the same colour —
// it is laid as one pool of watercolour instead of as a stack of thirty
// opaque strokes. The motif, the layout and the traits are untouched; only
// the material changes.
//
// Two things have to be handled that the ink medium never has to think
// about, and both are properties of transparent paint rather than
// shortcomings of the port.

// washMinBand is the thinnest band, in pixels, still worth laying as a
// pool. Below it a wash has no room for the things that make it one: no
// rim, no granulation, no interior — it is a coloured line, identical to
// what the ink medium would draw, at something like a hundred times the
// cost. Those bands are drawn as ink, which is why a piece of very small
// dots looks much the same in either medium. The wash wants room: pair it
// with --ring-size large or --spacing sparse.
const washMinBand = 1.8

// washBody is the opacity every wash mark carries before the light-on-dark
// correction. It is high on purpose. QQL's ink medium is fully opaque, and
// what the wash medium is for is the paint — the wet edge, the rim, the
// granulation, the pooling — not for turning the piece into a set of
// glazes: laid thin, every mark takes the ground's colour and the whole
// piece desaturates to the ground's hue. This keeps the pigment reading as
// the colour QQL chose, with the ground modulating it rather than
// swallowing it.
const washBody = 0.85

// washAlpha is how strong a band is where every deposit covers it. Below
// 1 so a dot's stacked shadow and any splatter over it still read through
// one another, and so the ground keeps a little say in every mark.
const washAlpha = 0.9

// washMaxLayers caps the deposit stack. A pool's softness comes from how
// many near-transparent deposits happen to reach a pixel, so a large mark
// wants the full stack and a four-pixel band would spend forty tables to
// produce a shape it could have described with eight.
const washMaxLayers = 40

// dotWash is the wash configured for one QQL piece.
type dotWash struct {
	base   paint.Wash
	ground palette.Color
}

func newDotWash(seed uint64, ground palette.Color) *dotWash {
	w := paint.DefaultWash(seed)
	// QQL's marks are circles, and have to stay circles at every size —
	// that is the one thing the whole output space is built around. The
	// default 0.22 is a blob; this is a wet edge on a circle.
	w.Ragged = 0.075
	return &dotWash{base: w, ground: ground}
}

// band lays one of a dot's bands as a pool, reporting false when the band
// is too small to be worth one and the caller should fall back to ink.
func (d *dotWash) band(cv *paint.Canvas, x, y, r, thickness float64, col palette.Color, rng *rand.Rand) bool {
	px := cv.Scale()
	tp, rp := thickness*px, r*px
	if tp < washMinBand || rp < 2 {
		return false
	}

	w := d.base
	w.Layers = min(max(int(4*tp)+6, 8), washMaxLayers)

	// Raggedness is a fraction of the radius, so on a thin band the two
	// boundaries would wander past each other and break the ring into
	// beads. Cap the deviation against the band's own width instead.
	w.Ragged = math.Min(w.Ragged, 0.35*thickness/r)

	// How much the pigment hides its ground. QQL is not a picture painted
	// on white paper: it lays opaque marks on a coloured ground, often a
	// dark one, and a pure glaze cannot do that — transparent pigment
	// cannot put light on dark at all, so every pale mark on a dark piece
	// would simply vanish. Every mark therefore carries some body, and a
	// mark lighter than its ground carries as much as it needs. What is
	// kept from the wash is everything that makes it one — the wet edge,
	// the rim, the granulation, the pooling — which is the part QQL does
	// not have and the reason to want this at all.
	lift := col.Luminance() - d.ground.Luminance()
	w.Body = mathx.Clamp01(washBody + 1.4*math.Max(lift, 0))

	w.Ring(cv, rng, x, y, r-thickness/2, thickness, col, washAlpha)
	return true
}
