// Package geom holds shared 2D geometry: circles and a uniform spatial
// index for containment and collision queries. Stdlib-only leaf.
package geom

import "math"

// Circle in canvas units.
type Circle struct {
	X, Y, R float64
}

// Index is a uniform grid over circles supporting point-containment and
// collision queries.
type Index struct {
	originX    float64
	originY    float64
	cell       float64
	cols, rows int
	cells      [][]int32
	circles    []Circle
}

// NewIndex creates an index covering [0, maxX]×[0, maxY] with cells sized
// by the largest expected circle radius.
func NewIndex(maxX, maxY, maxR float64) *Index {
	return NewIndexIn(0, 0, maxX, maxY, math.Max(maxR, 0.02))
}

// NewIndexIn creates an index covering [minX, maxX]×[minY, maxY] — use it
// when circles sit outside the canvas, as packing with an overscan does.
//
// Correctness does not depend on cell: it is purely a performance knob, so
// size it to the typical radius rather than the largest one. Two exceptions
// are worth knowing: At only inspects the cell containing the point, so it
// needs cell ≥ the largest radius; and a circle lying outside the bounds is
// pinned to the nearest edge cells, which costs extra distance checks but
// never hides a neighbour from a circle that is inside the bounds.
func NewIndexIn(minX, minY, maxX, maxY, cell float64) *Index {
	if cell <= 0 {
		cell = 0.02
	}
	g := &Index{originX: minX, originY: minY, cell: cell}
	g.cols = int(math.Ceil((maxX-minX)/cell)) + 1
	g.rows = int(math.Ceil((maxY-minY)/cell)) + 1
	g.cols = max(g.cols, 1)
	g.rows = max(g.rows, 1)
	g.cells = make([][]int32, g.cols*g.rows)
	return g
}

// col and row map a coordinate to a cell index, clamped to the grid.
func (g *Index) col(x float64) int {
	return min(max(int(math.Floor((x-g.originX)/g.cell)), 0), g.cols-1)
}

func (g *Index) row(y float64) int {
	return min(max(int(math.Floor((y-g.originY)/g.cell)), 0), g.rows-1)
}

// Insert adds c and returns its index.
func (g *Index) Insert(c Circle) int {
	i := int32(len(g.circles)) //nolint:gosec // circle counts are small
	g.circles = append(g.circles, c)
	x0, x1 := g.col(c.X-c.R), g.col(c.X+c.R)
	y0, y1 := g.row(c.Y-c.R), g.row(c.Y+c.R)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			idx := y*g.cols + x
			g.cells[idx] = append(g.cells[idx], i)
		}
	}
	return int(i)
}

// Circles returns the inserted circles in insertion order.
func (g *Index) Circles() []Circle { return g.circles }

// FitsWithGap reports whether c keeps at least gap distance to every
// inserted circle. A negative gap lets circles crowd into each other.
func (g *Index) FitsWithGap(c Circle, gap float64) bool {
	reach := c.R + gap + g.cell // any collider's center is within this + its own radius
	x0, x1 := g.col(c.X-reach)-1, g.col(c.X+reach)+1
	y0, y1 := g.row(c.Y-reach)-1, g.row(c.Y+reach)+1
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
	if x < g.originX || y < g.originY {
		return -1
	}
	cx, cy := int((x-g.originX)/g.cell), int((y-g.originY)/g.cell)
	if cx >= g.cols || cy >= g.rows {
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
