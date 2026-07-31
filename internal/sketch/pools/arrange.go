package pools

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// Where the marks go, borrowed from QQL — all three moving parts of it.
//
// A structure seeds start points; a flow field says which way to walk from
// them; and marks are laid along that walk, shoulder to shoulder, until
// something already on the sheet is in the way. The field proposes and the
// spacing rule disposes.
//
// The first version of this took only the structure, on the reasoning —
// QQL's own — that the seeding matters more than the field. That is true of
// the *composition*, and false of the surface. What makes a QQL piece
// recognisable at a glance is that its marks touch: they run in contiguous
// strands that curve, and a strand holds one size and one colour for its
// whole length. Scattering marks over a structured grid gives the same
// large-scale arrangement with none of that, and reads as a scatter.
//
// So three things are load-bearing here, and all three come from the walk:
//
//   - Marks advance by their own diameter, so consecutive marks touch. A
//     fixed grid of candidate positions cannot do this: the step has to
//     follow the size of the mark being laid.
//   - A run holds one size and one colour. QQL sets both per group, not per
//     dot; per dot they come out as salt and pepper and no strand reads as
//     a strand.
//   - Runs are laid along the field, so the strands curve with it.

const (
	dimArrange = "arrange"
	dimFlow    = "flow"
)

// arrangements are the structures a seed can draw: where the walks begin.
// Orbital and shadows carry the weight: the sweeping arcs of one and the
// filled-in fields of the other are where this vocabulary is at its best.
// Scatter stays reachable but rare — it is the only one with no direction
// in it at all, which a piece occasionally wants and usually does not.
var arrangements = []trait.Value{
	{Name: "orbital", Weight: 3},
	{Name: "formation", Weight: 2},
	{Name: "shadows", Weight: 3},
	{Name: "scatter", Weight: 1},
}

// flows are the fields a walk follows. The linear ones give strands that
// run straight across the sheet, the radial ones strands that curve; spiral
// sits between circular and explosive and is the most useful of them, which
// is why QQL weights it highest too.
var flows = []trait.Value{
	{Name: "horizontal", Weight: 3},
	{Name: "vertical", Weight: 2},
	{Name: "diagonal", Weight: 2},
	{Name: "spiral", Weight: 4},
	{Name: "circular", Weight: 2},
	{Name: "explosive", Weight: 1},
}

// pt is a start point.
type pt struct{ x, y float64 }

// bleed is how far past the canvas a structure reaches and a walk may
// wander, so a composition runs off the edge instead of stopping at it.
const bleed = 0.2

// field is the direction a walk takes at any point.
type field struct {
	radial      bool
	theta       float64 // linear heading
	cx, cy      float64 // radial centre
	circularity float64 // 0 = straight out of the centre, 1 = round it
	clockwise   bool
}

func newField(name string, rng *rand.Rand, aspect float64) field {
	f := field{
		cx: aspect * rnd.Uniform(rng, 0.15, 0.85),
		cy: rnd.Uniform(rng, 0.15, 0.85),
		// A little off true, always: a field at exactly 0 or 90 degrees
		// draws strands that line up with the frame and read as ruled.
		clockwise: rnd.Odds(rng, 0.5),
	}
	switch name {
	case "horizontal":
		f.theta = rnd.Gauss(rng, 0, 0.12)
	case "vertical":
		f.theta = math.Pi/2 + rnd.Gauss(rng, 0, 0.12)
	case "diagonal":
		f.theta = math.Pi/4 + rnd.Gauss(rng, 0, 0.12)
		if rnd.Odds(rng, 0.5) {
			f.theta = -f.theta
		}
	case "circular":
		f.radial, f.circularity = true, rnd.Uniform(rng, 0.88, 1)
	case "explosive":
		f.radial, f.circularity = true, rnd.Uniform(rng, 0.1, 0.35)
	default: // spiral
		f.radial, f.circularity = true, rnd.Uniform(rng, 0.45, 0.8)
	}
	return f
}

// at is the heading at a point.
func (f field) at(x, y float64) float64 {
	if !f.radial {
		return f.theta
	}
	out := math.Atan2(y-f.cy, x-f.cx)
	turn := math.Pi / 2 * f.circularity
	if f.clockwise {
		turn = -turn
	}
	return out + turn
}

// startGroups seeds the walks. A group is the unit that shares a size and a
// colour, so the structure decides not only where the piece is dense but
// how it is divided into passages.
//
// The spacing here is the distance between *walks*, not between marks: the
// walk fills in along its own length, so seeding at mark spacing would lay
// every strand on top of the last one.
func startGroups(name string, rng *rand.Rand, aspect, spacing float64) [][]pt {
	switch name {
	case "orbital":
		return orbitalStarts(rng, aspect, spacing)
	case "formation":
		return formationStarts(rng, aspect, spacing)
	case "shadows":
		return shadowStarts(rng, aspect, spacing)
	default:
		return nil // scatter has no structure; the planner throws darts
	}
}

// orbitalStarts seeds concentric bands about a centre, each band cut into
// arcs. Walked along a circular field these come out as record grooves;
// walked across one they come out as spokes.
func orbitalStarts(rng *rand.Rand, aspect, spacing float64) [][]pt {
	cx := aspect * rnd.Pick(rng, []rnd.Weighted[float64]{
		{V: 0.5, W: 3}, {V: 0.15, W: 2}, {V: 0.85, W: 2}, {V: -0.3, W: 1}, {V: 1.3, W: 1},
	})
	cy := rnd.Pick(rng, []rnd.Weighted[float64]{
		{V: 0.5, W: 3}, {V: 0.15, W: 2}, {V: 0.85, W: 2}, {V: -0.3, W: 1}, {V: 1.3, W: 1},
	})
	ringStep := spacing * rnd.Uniform(rng, 0.9, 1.6)
	arcs := rnd.Choice(rng, []int{1, 2, 3})
	phase := rnd.Uniform(rng, 0, 2*math.Pi)

	reach := 0.0
	for _, c := range [][2]float64{{0, 0}, {aspect, 0}, {0, 1}, {aspect, 1}} {
		reach = math.Max(reach, math.Hypot(c[0]-cx, c[1]-cy))
	}

	var groups [][]pt
	for r := ringStep; r < reach+ringStep; r += ringStep {
		for a := range arcs {
			var g []pt
			span := 2 * math.Pi / float64(arcs)
			from := phase + float64(a)*span
			// A handful of seeds per arc; the walk joins them up.
			n := max(int(r*span/(spacing*2)), 1)
			for i := range n {
				th := from + float64(i)*span/float64(n)
				p := pt{cx + r*math.Cos(th), cy + r*math.Sin(th)}
				if inBleed(p, aspect) {
					g = append(g, p)
				}
			}
			if len(g) > 0 {
				groups = append(groups, g)
			}
		}
	}
	return groups
}

// formationStarts divides the sheet into rectangles and seeds each with a
// coarse lattice, dropping some chunks. The gaps are what keep a formation
// piece from reading as wallpaper.
func formationStarts(rng *rand.Rand, aspect, spacing float64) [][]pt {
	across := rnd.Pick(rng, []rnd.Weighted[int]{{V: 1, W: 3}, {V: 2, W: 3}, {V: 3, W: 2}, {V: 4, W: 1}})
	down := rnd.Pick(rng, []rnd.Weighted[int]{{V: 1, W: 3}, {V: 2, W: 3}, {V: 3, W: 2}, {V: 4, W: 1}})
	skip := rnd.Pick(rng, []rnd.Weighted[float64]{{V: 0, W: 3}, {V: 0.15, W: 2}, {V: 0.35, W: 1}})

	w := (aspect + 2*bleed) / float64(across)
	h := (1 + 2*bleed) / float64(down)

	var groups [][]pt
	first := true
	for i := range across {
		for j := range down {
			if !first && rnd.Odds(rng, skip) {
				continue
			}
			first = false
			x0, y0 := -bleed+float64(i)*w, -bleed+float64(j)*h
			var g []pt
			for y := y0; y < y0+h; y += spacing {
				for x := x0; x < x0+w; x += spacing {
					p := pt{x, y}
					if inBleed(p, aspect) {
						g = append(g, p)
					}
				}
			}
			if len(g) > 0 {
				groups = append(groups, g)
			}
		}
	}
	return groups
}

// shadowStarts scatters non-overlapping blobs and seeds each one, so they
// read as separate objects filling in against the same field.
func shadowStarts(rng *rand.Rand, aspect, spacing float64) [][]pt {
	count := rnd.Choice(rng, []int{3, 5, 8, 12})
	type blob struct{ x, y, r float64 }
	var blobs []blob
	for iter := 0; len(blobs) < count && iter < 400; iter++ {
		// Blob size is a fraction of the sheet, not a multiple of the seed
		// spacing: tied to spacing, a sparse fill — whose marks are large,
		// so whose seeds are far apart — asks for blobs bigger than the
		// canvas, one of them fits, and the piece is a single object.
		b := blob{
			x: rnd.Uniform(rng, 0, aspect),
			y: rnd.Uniform(rng, 0, 1),
			r: rnd.Uniform(rng, 0.10, 0.34),
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

	var groups [][]pt
	for _, b := range blobs {
		var g []pt
		for r := b.r; r > 0; r -= spacing {
			n := max(int(2*math.Pi*r/(spacing*1.5)), 1)
			for i := range n {
				th := float64(i) * 2 * math.Pi / float64(n)
				p := pt{b.x + r*math.Cos(th), b.y + r*math.Sin(th)}
				if inBleed(p, aspect) {
					g = append(g, p)
				}
			}
		}
		g = append(g, pt{b.x, b.y})
		if len(g) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}

func inBleed(p pt, aspect float64) bool {
	return p.x > -bleed && p.x < aspect+bleed && p.y > -bleed && p.y < 1+bleed
}

// colorWalk is how a piece gets passages of colour instead of confetti.
//
// It steps an index along the luminance-ordered pigments and mostly stays
// put. Because it is advanced once per *group* rather than once per mark,
// and a group is a run of marks laid along one stretch of the field, the
// piece turns colour where it turns direction — which is what reads as
// composed. Drawing each mark's colour freely gives every pigment equal
// presence everywhere, which reads as confetti.
type colorWalk struct {
	n     int
	at    int
	churn float64
}

func newColorWalk(rng *rand.Rand, n int) *colorWalk {
	return &colorWalk{
		n:     n,
		at:    rng.IntN(max(n, 1)),
		churn: rnd.Uniform(rng, 0.3, 0.8),
	}
}

// scheme is a whole colour decision, held for a run: which pigment, which
// partner it is paired with, and which way a banded mark graduates.
//
// All three, not just the first. Holding the base pigment alone still lets
// the partner and the graduation re-draw at every mark, and a strand whose
// marks share a colour but differ in everything else about their colour
// does not read as one thing — QQL's runs are identical marks repeated,
// and that is what makes them runs.
type scheme struct {
	at     int // pigment, as an index into the ramp
	second int // its partner
	dir    int // which way a banded mark walks the ramp: +1 or -1
}

// next advances the walk and returns the scheme for the next group.
func (w *colorWalk) next(rng *rand.Rand) scheme {
	if w.n < 2 {
		return scheme{dir: 1}
	}
	if rnd.Odds(rng, w.churn) {
		step := rnd.Pick(rng, []rnd.Weighted[int]{{V: 1, W: 6}, {V: 2, W: 3}, {V: 3, W: 1}})
		if rnd.Odds(rng, 0.5) {
			step = -step
		}
		w.at = ((w.at+step)%w.n + w.n) % w.n
	}
	dir := 1
	if rnd.Odds(rng, 0.5) {
		dir = -1
	}
	return scheme{
		at:     w.at,
		second: (w.at + 1 + rng.IntN(max(w.n-1, 1))) % w.n,
		dir:    dir,
	}
}
