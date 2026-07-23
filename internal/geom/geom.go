// Package geom holds shared 2D geometry: circles and a uniform spatial
// index for containment and collision queries. Stdlib-only leaf.
package geom

import "math"

// Circle in canvas units.
type Circle struct {
	X, Y, R float64
}

// Index is a uniform grid over circles supporting point-containment and
// collision queries. Cells are sized for the largest expected radius.
type Index struct {
	cell       float64
	cols, rows int
	cells      [][]int32
	circles    []Circle
}

// NewIndex creates an index covering [0, maxX]×[0, maxY] with cells sized
// by the largest expected circle radius.
func NewIndex(maxX, maxY, maxR float64) *Index {
	g := &Index{cell: math.Max(maxR, 0.02)}
	g.cols = int(math.Ceil(maxX/g.cell)) + 1
	g.rows = int(math.Ceil(maxY/g.cell)) + 1
	g.cells = make([][]int32, g.cols*g.rows)
	return g
}

// Insert adds c and returns its index.
func (g *Index) Insert(c Circle) int {
	i := int32(len(g.circles)) //nolint:gosec // circle counts are small
	g.circles = append(g.circles, c)
	x0, x1 := int((c.X-c.R)/g.cell), int((c.X+c.R)/g.cell)
	y0, y1 := int((c.Y-c.R)/g.cell), int((c.Y+c.R)/g.cell)
	for y := max(y0, 0); y <= min(y1, g.rows-1); y++ {
		for x := max(x0, 0); x <= min(x1, g.cols-1); x++ {
			idx := y*g.cols + x
			g.cells[idx] = append(g.cells[idx], i)
		}
	}
	return int(i)
}

// Circles returns the inserted circles in insertion order.
func (g *Index) Circles() []Circle { return g.circles }

// FitsWithGap reports whether c keeps at least gap distance to every
// inserted circle.
func (g *Index) FitsWithGap(c Circle, gap float64) bool {
	reach := c.R + gap + g.cell // any collider's center is within this + its own radius
	x0, x1 := int((c.X-reach)/g.cell)-1, int((c.X+reach)/g.cell)+1
	y0, y1 := int((c.Y-reach)/g.cell)-1, int((c.Y+reach)/g.cell)+1
	// Circles listed in several cells are checked more than once — the
	// redundant distance checks are cheaper than deduplication.
	for y := max(y0, 0); y <= min(y1, g.rows-1); y++ {
		for x := max(x0, 0); x <= min(x1, g.cols-1); x++ {
			for _, i := range g.cells[y*g.cols+x] {
				o := g.circles[i]
				dx, dy := o.X-c.X, o.Y-c.Y
				minD := o.R + c.R + gap
				if dx*dx+dy*dy < minD*minD {
					return false
				}
			}
		}
	}
	return true
}

// At returns the index of the circle containing (x, y), or -1. When
// circles overlap, the earliest inserted wins.
func (g *Index) At(x, y float64) int {
	cx, cy := int(x/g.cell), int(y/g.cell)
	if cx < 0 || cy < 0 || cx >= g.cols || cy >= g.rows {
		return -1
	}
	best := int32(-1)
	for _, i := range g.cells[cy*g.cols+cx] {
		c := g.circles[i]
		dx, dy := x-c.X, y-c.Y
		if dx*dx+dy*dy <= c.R*c.R && (best < 0 || i < best) {
			best = i
		}
	}
	return int(best)
}
