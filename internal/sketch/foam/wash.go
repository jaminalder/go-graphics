package foam

// Painting a sheet, as opposed to answering it per pixel.
//
// Every other fill here is a function of position: ask it about a point and
// it returns a colour. A wash is not. `internal/paint`'s pool is built from
// a stack of forty near-transparent deposits, and the density variation, the
// broken edge and the concentration toward the rim all fall out of how many
// of them happen to reach a given pixel — which is a thing you *stamp*, not
// a thing you evaluate.
//
// So a sheet with any washed cell on it is painted in two passes: the pools
// go down onto a paint.Canvas first, and the pixel loop then runs over the
// result, answering for every cell that is not washed and laying the ink
// over the top of all of it.
//
// The alternative was to write an analytic watercolour that could be
// evaluated per pixel, which is what this sketch had for a while. It was
// harder-edged and darker than the real thing — the model in paint/water.go
// is luminous because a stack of transparent deposits is luminous, and that
// is not something a closed form recovers.

import (
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// washes reports whether any cell on the sheet is to be painted. A sheet
// with none is cheaper to raster, and keeps the 16-bit and supersampling
// settings a stamped canvas cannot offer.
func (sh *sheet) washes() bool {
	for i, c := range sh.foam.Cells() {
		if c.Area > 0 && sh.skin[i].style == styleWash {
			return true
		}
	}
	return false
}

// cellRegion is one cell seen as a shape to be painted: it answers how deep
// inside itself a point lies, which is all paint.Region asks for.
//
// The lookup goes through the warp, exactly as the pixel loop does, so the
// paint lands on the bent cell the viewer sees rather than on the straight
// one underneath it.
type cellRegion struct {
	s  *Sketch
	sh *sheet
	id int
}

func (r cellRegion) Depth(u, v float64) float64 {
	h := r.sh.foam.At(r.s.warp(r.sh.field, r.sh.level, u, v))
	if h.Cell != r.id {
		return -1
	}
	if math.IsInf(h.Wall, 1) {
		// A lone cell has no wall to be far from; give the brush something
		// finite to size itself against.
		return r.sh.foam.Cells()[r.id].Inradius
	}
	return h.Wall
}

// paintSheet lays the washes, then runs the pixel loop over them.
func (s *Sketch) paintSheet(ctx sketch.Context, sh *sheet) image.Image {
	aspect := float64(ctx.Width) / float64(ctx.Height)

	cv := paint.NewCanvas(ctx.Width, ctx.Height, sh.paper)
	w := paint.DefaultWash(ctx.Seed ^ saltPaper)
	w.Ragged = s.Ragged
	w.GrainScale = tooth
	prng := ctx.RNG(streamPaint)

	// Largest first, so the small marks settle on top the way a later touch
	// of a loaded brush does.
	order := make([]int, 0, sh.foam.Len())
	for i, c := range sh.foam.Cells() {
		if c.Area > 0 && sh.skin[i].style == styleWash {
			order = append(order, i)
		}
	}
	sortByAreaDesc(order, sh.foam.Cells())

	for _, i := range order {
		c := sh.foam.Cells()[i]
		d := sh.skin[i]
		// The scheme's tone is the strength of the touch — how loaded the
		// brush was — which is the reading the wash was built for, and the
		// one a flat fill has to fake by lightening.
		w.Fill(cv, prng, cellRegion{s, sh, i}, paint.Box{
			MinU: c.MinX, MinV: c.MinY, MaxU: c.MaxX, MaxV: c.MaxY,
		}, d.pigment, s.Load*(0.45+0.85*d.tone), s.Reach)
	}
	painted := cv.Image()

	// The pixel loop, over the painted canvas. Supersampled for everything
	// analytic — the ink above all, which is a thin line and shows every
	// stair — while the wash underneath is read at pixel resolution, having
	// already anti-aliased itself with forty soft-edged deposits.
	n := ctx.Samples()
	pix := make([]palette.Color, ctx.Width*ctx.Height)
	for y := range ctx.Height {
		for x := range ctx.Width {
			var acc palette.Color
			for j := range n {
				for i := range n {
					u := (float64(x) + (float64(i)+0.5)/float64(n)) / float64(ctx.Height)
					v := (float64(y) + (float64(j)+0.5)/float64(n)) / float64(ctx.Height)
					c := s.over(sh, ctx.Seed, painted, x, y, u, v)
					acc.R += c.R
					acc.G += c.G
					acc.B += c.B
				}
			}
			k := 1 / float64(n*n)
			pix[y*ctx.Width+x] = palette.Color{R: acc.R * k, G: acc.G * k, B: acc.B * k}
		}
	}
	_ = aspect
	return render.ImageFromColors(ctx.Width, ctx.Height, pix)
}

// over answers one sample: the wash if this cell was painted, the cell's own
// fill otherwise, then the mosaic, the relief and the ink over the top.
func (s *Sketch) over(sh *sheet, seed uint64, painted *image.NRGBA, x, y int, u, v float64) palette.Color {
	wu, wv := s.warp(sh.field, sh.level, u, v)
	h := sh.foam.At(wu, wv)

	var c palette.Color
	if sh.skin[h.Cell].style == styleWash {
		o := painted.PixOffset(x, y)
		c = palette.Color{
			R: float64(painted.Pix[o]) / 255,
			G: float64(painted.Pix[o+1]) / 255,
			B: float64(painted.Pix[o+2]) / 255,
		}
	} else {
		c = s.fill(sh.skin[h.Cell], h, sh.field, seed, u, v, sh.paper)
	}
	c = s.tile(sh, h, u, v, wu, wv, c, seed)
	c = s.relief(sh, h, u, v, c, seed)
	return s.lay(c, h, sh.level, sh.field, seed, sh.ink, u, v)
}

// sortByAreaDesc orders cell ids largest first, by insertion — a sheet holds
// a few hundred cells and this keeps the order stable.
func sortByAreaDesc(ids []int, cs []cells.Cell) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && cs[ids[j]].Area > cs[ids[j-1]].Area; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}
