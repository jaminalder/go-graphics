// Package cells divides the canvas into curved-walled cells and makes each
// one addressable: which cell a point is in, how far it is from the nearest
// wall, and how close it is to a junction — plus, per cell, its area,
// centroid and inscribed radius.
//
// That is the difference between this and a distance field like
// noise.Worley. A field answers "how far to the nearest site", which is
// enough to draw a crack network and not enough to *fill* anything. A
// partition answers "which region am I in, and what does that region look
// like", which is what a sketch needs before it can put a wash in one cell
// and hatching in the next.
//
// The metric is additively weighted (Apollonius): d_i = |p − c_i| − w_i.
// Its walls are hyperbolic arcs rather than straight bisectors, so a foam
// built on it curves the way a bubble cluster does; with every weight equal
// it degenerates to an ordinary Voronoi diagram.
//
// Sites may be merged into one cell, in which case the wall between them
// never exists and the union — typically concave — is a single region as
// far as everything downstream is concerned.
//
// Stdlib-only leaf.
package cells

import (
	"math"
	"math/rand/v2"
)

// Site is a seed point with an additive weight. A larger weight claims more
// ground, so weight is the lever on relative cell size.
type Site struct {
	X, Y, W float64
}

// Cell is one region of the foam, measured over the canvas. A cell whose
// sites all sit outside the frame, or that a heavier neighbour swallowed,
// has Area 0 and should be skipped rather than drawn.
type Cell struct {
	ID       int
	Sites    int     // sites merged into it; more than one means a lobe
	Area     float64 // share of the canvas, in [0,1]
	CX, CY   float64 // centroid of the part inside the frame
	Inradius float64 // largest wall distance found inside it
}

// Hit is what the foam knows about one point.
type Hit struct {
	// Cell is the id of the region containing the point.
	Cell int
	// Next is the id of the cell across the nearest wall — the region the
	// point would belong to if its own were not there, and the one it would
	// fall into if it stepped over that wall. −1 when only one cell is in
	// range, so an absent neighbour has to be handled rather than silently
	// read as the point's own cell.
	//
	// Wall says how far the boundary is; Next says whose boundary it is.
	// That is what a fill needs before it can reach across a wall: bleeding
	// pigment into a neighbour, failing to register with the drawn line, or
	// simply asking whose company it is keeping.
	Next int
	// Wall is the distance to the nearest wall, in canvas units. Every fill
	// uses it to know how deep inside its own cell it is; it is +Inf when
	// only one cell is in range, and negative just inside a rounded corner.
	//
	// It is measured against a *soft* minimum over the other cells, so a
	// cell's corners are rounded off over Params.Round. A hard minimum
	// leaves every corner sharp — three cells meeting at 120° give a
	// three-cornered cell, and a sheet of those reads as a cracked pane
	// whatever the walls between the corners do.
	Wall float64
	// Node is how crowded the point is with cells beyond the two whose wall
	// it is on: near 0 halfway along a wall, and rising toward 1 at a
	// junction — further at a junction where four cells meet than at one
	// where three do. It is what swells the ink at the junctions and tapers
	// it mid-wall.
	//
	// It is a *smooth* count rather than "how close is the third nearest".
	// Ranking is not a smooth function of position: the identity of the
	// third cell swaps along rays running out of every junction, and a
	// measure built on it creases along those rays. Fed into the ink width
	// the creases come out as sharp spikes radiating from every node — a
	// defect invisible at preview size and unmissable at print.
	Node float64
}

// Params tune the wall geometry and how carefully the cells are measured.
type Params struct {
	// Node is the distance over which a further cell stops counting as near.
	Node float64
	// Round is the radius a cell's corners are rounded over. 0 leaves them
	// sharp; a value approaching the cell size turns the cells into discs
	// with gaps between them.
	Round float64
	// Stat is the number of measuring samples along the canvas height.
	Stat int
}

// DefaultParams are sensible for a canvas of a few dozen cells.
func DefaultParams() Params { return Params{Node: 0.05, Round: 0.02, Stat: 320} }

// Foam is a built partition: sites bucketed into a uniform grid, the
// site → cell mapping, and the measured cells.
type Foam struct {
	sites []Site
	group []int
	cells []Cell
	node  float64
	round float64

	// Uniform grid over the sites. Sites may lie outside the canvas, so the
	// grid covers the sites' own bounds rather than the frame.
	minX, minY float64
	cell       float64
	cols, rows int
	buckets    [][]int32
	maxW       float64
}

// New builds a foam over the canvas [0,aspect]×[0,1]. group maps each site
// to a cell id; pass Identity for one cell per site. Cell ids must be dense
// (0..n-1) — Merge guarantees that.
func New(sites []Site, group []int, aspect float64, p Params) *Foam {
	if len(sites) == 0 {
		panic("cells: no sites")
	}
	if len(group) != len(sites) {
		panic("cells: group length does not match sites")
	}
	f := &Foam{
		sites: sites, group: group,
		node:  math.Max(p.Node, 1e-9),
		round: math.Max(p.Round, 0),
	}
	f.build(aspect)
	f.measure(aspect, p.Stat)
	return f
}

// Identity is the trivial grouping: every site is its own cell.
func Identity(n int) []int {
	g := make([]int, n)
	for i := range g {
		g[i] = i
	}
	return g
}

// Cells returns the measured cells, indexed by id.
func (f *Foam) Cells() []Cell { return f.cells }

// Len is the number of cells.
func (f *Foam) Len() int { return len(f.cells) }

// build buckets the sites into a grid sized for a couple of sites per cell.
//
// The grid always covers the canvas as well as the sites. Sizing it to the
// sites alone collapses when they are few or collinear — the bucket size
// goes to nothing, and the ring walk then crosses hundreds of empty buckets
// before it finds a second site.
func (f *Foam) build(aspect float64) {
	minX, minY := 0.0, 0.0
	maxX, maxY := aspect, 1.0
	ncell := 0
	for i, s := range f.sites {
		minX, minY = math.Min(minX, s.X), math.Min(minY, s.Y)
		maxX, maxY = math.Max(maxX, s.X), math.Max(maxY, s.Y)
		f.maxW = math.Max(f.maxW, s.W)
		ncell = max(ncell, f.group[i]+1)
	}
	f.cells = make([]Cell, ncell)
	for i := range f.cells {
		f.cells[i].ID = i
	}
	for _, g := range f.group {
		f.cells[g].Sites++
	}

	// A couple of sites per bucket: fine enough that a query touches few
	// sites, coarse enough that the ring walk terminates in a step or two.
	w, h := math.Max(maxX-minX, 1e-6), math.Max(maxY-minY, 1e-6)
	f.cell = math.Max(math.Sqrt(w*h/float64(len(f.sites))), 1e-6)
	f.minX, f.minY = minX, minY
	f.cols = max(int(w/f.cell)+1, 1)
	f.rows = max(int(h/f.cell)+1, 1)
	f.buckets = make([][]int32, f.cols*f.rows)
	for i, s := range f.sites {
		b := f.row(s.Y)*f.cols + f.col(s.X)
		f.buckets[b] = append(f.buckets[b], int32(i))
	}
}

func (f *Foam) col(x float64) int { return min(max(int((x-f.minX)/f.cell), 0), f.cols-1) }
func (f *Foam) row(y float64) int { return min(max(int((y-f.minY)/f.cell), 0), f.rows-1) }

// At locates a point: which cell, how far from the wall, how close to a
// junction.
//
// Buckets are visited in expanding square rings. A site in ring r is at
// least (r-1)·cell away geometrically, so its weighted distance is at least
// (r-1)·cell − maxW; once that floor passes the second-nearest cell plus
// the node range, nothing further out can change the answer. Sites outside
// the grid are pinned to a boundary bucket, which only ever makes them be
// visited *earlier* than their true position warrants — safe, never a
// missed neighbour.
func (f *Foam) At(u, v float64) Hit {
	var nb near
	cu, cv := f.col(u), f.row(v)
	maxR := max(f.cols, f.rows)
	for r := 0; r <= maxR; r++ {
		// Six node-lengths, not one. Whether a far cell falls inside the
		// searched rings flips as the query point moves, so whatever that
		// cell contributes to the crowding measure is a step in it — the
		// exact defect the smooth measure exists to avoid. At six lengths
		// the step is e⁻⁶, a quarter of a percent of a line width, and the
		// extra ring costs one bucket row.
		if nb.n >= 2 && float64(r-1)*f.cell-f.maxW > nb.dist[1]+6*math.Max(f.node, f.round) {
			break
		}
		f.ring(cu, cv, r, u, v, &nb)
	}
	h := Hit{Cell: nb.cell[0], Next: -1, Wall: math.Inf(1)}
	if nb.n < 2 {
		return h
	}
	h.Next = nb.cell[1]
	// Halved because d₂ − d₁ closes at twice the rate the point approaches
	// the wall: both distances move, in opposite directions.
	//
	// The second term rounds the corners. Replacing the hard minimum over
	// the other cells with a soft one — −σ·log Σ exp(−dₖ/σ) — pulls the
	// wall in wherever two of them are close at once, which is exactly at a
	// corner and nowhere else: mid-wall the sum has one term and the soft
	// minimum equals the hard one, so a straight run of wall is untouched
	// while the corner it runs into is rounded over σ.
	h.Wall = (nb.dist[1] - nb.dist[0]) / 2
	if f.round > 0 {
		crowd := 1.0
		for k := 2; k < nb.n; k++ {
			crowd += math.Exp(-(nb.dist[k] - nb.dist[1]) / f.round)
		}
		h.Wall -= f.round * math.Log(crowd) / 2
	}
	// Every cell past the second contributes, weighted by how far behind
	// the second it is. The sum is smooth in the position because nothing
	// in it depends on the *order* of the terms.
	soft := 0.0
	for k := 2; k < nb.n; k++ {
		soft += math.Exp(-(nb.dist[k] - nb.dist[1]) / f.node)
	}
	h.Node = 1 - math.Exp(-soft)
	return h
}

// ring visits every bucket at Chebyshev distance r from (cu, cv).
func (f *Foam) ring(cu, cv, r int, u, v float64, nb *near) {
	lo, hi := cv-r, cv+r
	for row := lo; row <= hi; row++ {
		if row < 0 || row >= f.rows {
			continue
		}
		step := 1
		if r > 0 && row != lo && row != hi {
			step = 2 * r // only the two edge columns on an interior row
		}
		for col := cu - r; col <= cu+r; col += step {
			if col < 0 || col >= f.cols {
				continue
			}
			for _, i := range f.buckets[row*f.cols+col] {
				s := f.sites[i]
				nb.add(f.group[i], math.Hypot(s.X-u, s.Y-v)-s.W)
			}
		}
	}
}

// slots is how many distinct cells a lookup keeps. Two are needed for the
// wall; the rest feed the smooth crowding measure, where anything past the
// sixth nearest is contributing less than a thousandth of a vote.
const slots = 6

// near keeps the nearest distinct cells, ascending. Distinct is the whole
// point: two sites of one merged lobe must not occupy two of the slots, or
// the wall to the genuine neighbour is never found.
type near struct {
	cell [slots]int
	dist [slots]float64
	n    int
}

func (nb *near) add(cell int, d float64) {
	for i := range nb.n {
		if nb.cell[i] == cell {
			if d < nb.dist[i] {
				nb.dist[i] = d
				nb.up(i)
			}
			return
		}
	}
	switch {
	case nb.n < slots:
		nb.cell[nb.n], nb.dist[nb.n] = cell, d
		nb.n++
		nb.up(nb.n - 1)
	case d < nb.dist[slots-1]:
		nb.cell[slots-1], nb.dist[slots-1] = cell, d
		nb.up(slots - 1)
	}
}

func (nb *near) up(i int) {
	for ; i > 0 && nb.dist[i] < nb.dist[i-1]; i-- {
		nb.dist[i], nb.dist[i-1] = nb.dist[i-1], nb.dist[i]
		nb.cell[i], nb.cell[i-1] = nb.cell[i-1], nb.cell[i]
	}
}

// measure walks a fixed sample grid to give every cell its area, centroid
// and inscribed radius.
//
// The grid is fixed rather than tied to the output size: a cell's centroid
// decides its colour and its inradius decides whether a band pattern can
// resolve, so both have to be the same at preview and at print (invariant 2
// in docs/ARCHITECTURE.md).
func (f *Foam) measure(aspect float64, res int) {
	res = max(res, 16)
	step := 1 / float64(res)
	cols := max(int(aspect*float64(res)), 1)
	sumX := make([]float64, len(f.cells))
	sumY := make([]float64, len(f.cells))
	count := make([]float64, len(f.cells))
	total := 0.0
	for j := range res {
		v := (float64(j) + 0.5) * step
		for i := range cols {
			u := (float64(i) + 0.5) * step
			h := f.At(u, v)
			c := &f.cells[h.Cell]
			sumX[h.Cell] += u
			sumY[h.Cell] += v
			count[h.Cell]++
			total++
			if !math.IsInf(h.Wall, 1) && h.Wall > c.Inradius {
				c.Inradius = h.Wall
			}
		}
	}
	for i := range f.cells {
		if count[i] == 0 {
			continue
		}
		f.cells[i].Area = count[i] / total
		f.cells[i].CX = sumX[i] / count[i]
		f.cells[i].CY = sumY[i] / count[i]
	}
}

// reach is how far down a site's neighbour list a merge may look. Small on
// purpose: past the third or fourth nearest, "neighbour" stops being true.
const reach = 4

// nearest returns the reach nearest sites to i, closest first.
func nearest(sites []Site, i, reach int) []int {
	idx := make([]int, 0, reach)
	dist := make([]float64, 0, reach)
	for j := range sites {
		if j == i {
			continue
		}
		d := math.Hypot(sites[i].X-sites[j].X, sites[i].Y-sites[j].Y)
		if len(idx) == reach && d >= dist[len(dist)-1] {
			continue
		}
		if len(idx) < reach {
			idx, dist = append(idx, j), append(dist, d)
		} else {
			idx[len(idx)-1], dist[len(dist)-1] = j, d
		}
		for k := len(idx) - 1; k > 0 && dist[k] < dist[k-1]; k-- {
			dist[k], dist[k-1] = dist[k-1], dist[k]
			idx[k], idx[k-1] = idx[k-1], idx[k]
		}
	}
	return idx
}

// Merge groups sites into cells, joining a share of neighbouring pairs so
// that some cells come out as concave lobes instead of convex bubbles. It
// returns a dense site → cell mapping.
//
// share is the probability a site reaches for a neighbour; maxSites caps
// how many sites one lobe may absorb. Merging the *ids* rather than erasing
// a drawn line matters: a lobe has no interior wall at all, so its fill,
// its rim and its band spacing all treat it as the single region it now is.
func Merge(rng *rand.Rand, sites []Site, share float64, maxSites int) []int {
	n := len(sites)
	parent := make([]int, n)
	size := make([]int, n)
	for i := range parent {
		parent[i], size[i] = i, 1
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	if maxSites < 2 {
		share = 0
	}

	// Visited in index order so the result depends only on the rng stream,
	// never on map iteration.
	for i := range n {
		if rng.Float64() >= share {
			continue
		}
		ri := find(i)
		if size[ri] >= maxSites {
			continue
		}
		// Only a genuine neighbour will do — one of the reach nearest sites,
		// whether or not it has room. A lobe is the union of two adjacent
		// bubbles; joining the nearest site that *happens to be free* lets a
		// crowded sheet pair a site with something across the frame, and the
		// result is a cell in two disconnected pieces, which neither the
		// metric nor any fill can express. A site whose neighbours are all
		// spoken for simply stays single.
		best := -1
		for _, j := range nearest(sites, i, reach) {
			rj := find(j)
			if rj != ri && size[ri]+size[rj] <= maxSites {
				best = rj
				break
			}
		}
		if best < 0 {
			continue
		}
		parent[best] = ri
		size[ri] += size[best]
	}

	// Compact the roots to dense ids, in first-appearance order.
	group := make([]int, n)
	id := map[int]int{}
	for i := range n {
		r := find(i)
		if _, ok := id[r]; !ok {
			id[r] = len(id)
		}
		group[i] = id[r]
	}
	return group
}
