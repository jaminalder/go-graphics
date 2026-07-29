package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/trait"
)

// The structure trait decides where flow lines start, and that turns out to
// matter more than the field itself: the same field seeded in concentric
// bands, in soft blobs, or in a grid of rectangles gives three unmistakably
// different pieces. Start points come in groups, and a group is the unit
// that shares a colour and a dot size — so the layout also decides how the
// piece is divided into passages.

func startPoints(tr trait.Set, f frame, rng *rand.Rand) [][]pt {
	switch tr.Get(dimStructure) {
	case "orbital":
		return orbital(f, rng)
	case "shadows":
		return shadows(f, rng)
	default:
		return formation(f, rng)
	}
}

// orbital seeds concentric bands around a centre, each band cut into one to
// three arcs. Lines started this way sweep round and round, which is what
// gives orbital pieces their record-groove reading.
func orbital(f frame, rng *rand.Rand) [][]pt {
	baseStep := wc(rng, []weighted[float64]{
		{f.w(0.01), 3},
		{f.w(0.02), 2},
		{f.w(0.04), 1},
		{f.w(0.06), 1},
		{f.w(0.08), 1},
		{f.w(0.16), 0.5},
	})
	radialStep := baseStep * 0.5
	groupStep := wc(rng, []weighted[float64]{{f.w(0.07), 0.333}, {f.w(0.15), 0.333}, {f.w(0.3), 0.333}})

	cx := wc(rng, []weighted[float64]{
		{f.w(0.5), 0.3},
		{f.w(0.333), 0.2},
		{f.w(0.666), 0.2},
		{f.w(-0.333), 0.1},
		{f.w(1.333), 0.1},
		{f.w(-1.6), 0.05},
		{f.w(1.6), 0.05},
	})
	cy := wc(rng, []weighted[float64]{
		{f.h(0.5), 0.3},
		{f.h(0.333), 0.2},
		{f.h(0.666), 0.2},
		{f.h(-0.333), 0.1},
		{f.h(1.333), 0.1},
		{f.h(-1.6), 0.05},
		{f.h(1.6), 0.05},
	})

	x0, x1 := f.w(-1.0/3.0), f.w(4.0/3.0)
	y0, y1 := f.h(-1.0/3.0), f.h(4.0/3.0)

	// Reach past the furthest corner, so the bands cover the canvas even
	// when the centre sits well outside it.
	maxRadius := f.w(0.05) + math.Max(
		math.Max(dist(cx, cy, 0, 0), dist(cx, cy, f.w(1), 0)),
		math.Max(dist(cx, cy, f.w(1), f.h(1)), dist(cx, cy, 0, f.h(1))))
	splitOffset := uniform(rng, 0, pi(2))

	var groups [][]pt
	for groupRadius := f.w(0.001); groupRadius < maxRadius; groupRadius += groupStep {
		splits := choice(rng, []int{1, 2, 3})
		splitLen := pi(2) / float64(splits)

		for theta := splitOffset; theta < splitOffset+pi(2); theta += splitLen {
			var group []pt
			for radius := groupRadius; radius < groupRadius+groupStep; radius += radialStep {
				stepsWanted := radius * pi(2) / baseStep
				thetaStep := math.Max(pi(0.005), pi(2)/stepsWanted)
				for inner := theta; inner < theta+splitLen; inner += thetaStep {
					x := cx + radius*math.Cos(inner)
					y := cy + radius*math.Sin(inner)
					if x > x0 && x < x1 && y > y0 && y < y1 {
						group = append(group, pt{x, y})
					}
				}
			}
			groups = append(groups, group)
		}
	}
	return groups
}

// shadows scatters non-overlapping discs and fills each one, either in
// concentric rings or as a raster of rows. Each disc becomes its own group,
// so they read as separate objects casting into the same field.
func shadows(f frame, rng *rand.Rand) [][]pt {
	count := choice(rng, []int{5, 7, 10, 20, 30, 60})
	pSquare := choice(rng, []float64{0, 0.5, 1})
	columnar := odds(rng, 0.5)
	outward := odds(rng, 0.5)

	type disc struct{ x, y, r float64 }
	var discs []disc
	for iter := 0; len(discs) < count && iter < 1000; iter++ {
		c := disc{
			x: uniform(rng, f.w(0), f.w(1)),
			y: uniform(rng, f.h(0), f.h(1)),
			r: uniform(rng, f.w(0.05), f.w(0.5)),
		}
		clear := true
		for _, o := range discs {
			if dist(c.x, c.y, o.x, o.y) < c.r+o.r {
				clear = false
				break
			}
		}
		if clear {
			discs = append(discs, c)
		}
	}

	radialFill := func(c disc) []pt {
		radiusStep, circStep := f.w(0.02), f.w(0.01)
		var group []pt
		for radius := c.r; radius > 0; radius -= radiusStep {
			thetaStep := pi(2) / (radius * pi(2) / circStep)
			for theta := 0.0; theta < pi(2.01); theta += thetaStep {
				group = append(group, pt{c.x + radius*math.Cos(theta), c.y + radius*math.Sin(theta)})
			}
		}
		if outward {
			for i, j := 0, len(group)-1; i < j; i, j = i+1, j-1 {
				group[i], group[j] = group[j], group[i]
			}
		}
		// Rarely, scramble the order so the disc fills in at random rather
		// than sweeping — the dots land the same, the colour runs do not.
		if odds(rng, 0.05) {
			group = shuffled(rng, group)
		}
		return group
	}

	squareFill := func(c disc) []pt {
		step := wc(rng, []weighted[float64]{
			{f.w(0.0075), 0.37},
			{f.w(0.01), 0.35},
			{f.w(0.02), 0.25},
			{f.w(0.04), 0.02},
			{f.w(0.08), 0.01},
		})
		var group []pt
		r2 := c.r * c.r
		for o1 := -c.r; o1 < c.r; o1 += step {
			for o2 := -c.r; o2 < c.r; o2 += step {
				x, y := c.x+o2, c.y+o1
				if columnar {
					x, y = c.x+o1, c.y+o2
				}
				dx, dy := c.x-x, c.y-y
				if dx*dx+dy*dy < r2 {
					group = append(group, pt{x, y})
				}
			}
		}
		return group
	}

	groups := make([][]pt, 0, len(discs))
	for _, c := range discs {
		if odds(rng, pSquare) {
			groups = append(groups, squareFill(c))
		} else {
			groups = append(groups, radialFill(c))
		}
	}
	return groups
}

// formation divides the canvas into a grid of rectangles and rasters each
// one, dropping some at random. The dropped chunks are what keep a
// formation piece from reading as wallpaper.
func formation(f frame, rng *rand.Rand) [][]pt {
	step := wc(rng, []weighted[float64]{
		{f.w(0.0075), 0.37},
		{f.w(0.01), 0.35},
		{f.w(0.02), 0.25},
		{f.w(0.04), 0.02},
		{f.w(0.08), 0.01},
	})
	across := wc(rng, []weighted[int]{{1, 0.7}, {2, 0.35}, {3, 0.25}, {4, 0.1}, {5, 0.05}, {7, 0.05}})
	down := wc(rng, []weighted[int]{{1, 0.4}, {2, 0.35}, {3, 0.25}, {4, 0.1}, {5, 0.05}, {7, 0.05}})

	chunkW := f.w(1.2) / float64(across)
	chunkH := f.h(1.2) / float64(down)
	skipOdds := wc(rng, []weighted[float64]{{0, 0.5}, {0.1, 0.3}, {0.2, 0.15}, {0.5, 0.05}})

	// Exactly across×down chunks tile the area. The source accumulates the
	// chunk origin in a float loop, which rounding lets overshoot by one
	// column or row lying wholly off-canvas — and since the first chunk
	// after the shuffle is the one that can never be skipped, an overshot
	// chunk landing there can empty the piece outright.
	chunks := make([]pt, 0, across*down)
	for i := range across {
		for j := range down {
			chunks = append(chunks, pt{f.w(-0.1) + float64(i)*chunkW, f.h(-0.1) + float64(j)*chunkH})
		}
	}
	chunks = shuffled(rng, chunks)

	var groups [][]pt
	for i, c := range chunks {
		// The first chunk always survives, so no seed comes out empty.
		if i > 0 && odds(rng, skipOdds) {
			continue
		}
		var group []pt
		for y := c.Y; y < c.Y+chunkH; y += step {
			for x := c.X; x < c.X+chunkW; x += step {
				group = append(group, pt{x, y})
			}
		}
		groups = append(groups, group)
	}
	return groups
}
