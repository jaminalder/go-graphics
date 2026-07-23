package circles

import "math"

// grid is a uniform spatial index mapping a canvas point to the circle
// containing it, so the pixel loop avoids scanning every circle.
type grid struct {
	cell       float64
	cols, rows int
	cells      [][]int32 // circle indices per cell, by bounding box
}

// newGrid buckets circles into cells sized by the largest radius.
func newGrid(circles []circleSpec, maxR float64) *grid {
	g := &grid{cell: math.Max(maxR, 0.02)}
	maxU, maxV := 1.0, 1.0
	for i := range circles {
		maxU = math.Max(maxU, circles[i].cx+circles[i].r)
		maxV = math.Max(maxV, circles[i].cy+circles[i].r)
	}
	g.cols = int(math.Ceil(maxU/g.cell)) + 1
	g.rows = int(math.Ceil(maxV/g.cell)) + 1
	g.cells = make([][]int32, g.cols*g.rows)

	for i := range circles {
		c := &circles[i]
		x0 := int((c.cx - c.r) / g.cell)
		x1 := int((c.cx + c.r) / g.cell)
		y0 := int((c.cy - c.r) / g.cell)
		y1 := int((c.cy + c.r) / g.cell)
		for y := max(y0, 0); y <= min(y1, g.rows-1); y++ {
			for x := max(x0, 0); x <= min(x1, g.cols-1); x++ {
				idx := y*g.cols + x
				g.cells[idx] = append(g.cells[idx], int32(i)) //nolint:gosec // circle count is small
			}
		}
	}
	return g
}

// at returns the index of the circle containing (u, v), or -1.
func (g *grid) at(u, v float64, circles []circleSpec) int {
	x, y := int(u/g.cell), int(v/g.cell)
	if x < 0 || y < 0 || x >= g.cols || y >= g.rows {
		return -1
	}
	for _, i := range g.cells[y*g.cols+x] {
		c := &circles[i]
		dx, dy := u-c.cx, v-c.cy
		if dx*dx+dy*dy <= c.r*c.r {
			return int(i)
		}
	}
	return -1
}
