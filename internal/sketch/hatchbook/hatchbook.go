// Package hatchbook is a specimen sheet for internal/hatch: a grid of
// squares, each one structure of hatching at one setting, so the whole
// family can be judged in a single look.
//
// It is a catalogue rather than an artwork, and it is a sketch anyway. A
// sketch gets the size profiles, the palettes, the supersampling and the
// embedded recipe for free, and — the reason that matters — it exercises
// the hatch package through exactly the pipeline a real sketch would, so a
// specimen that reads correctly at preview and at print is evidence about
// the invariants rather than about a bespoke renderer.
//
// The squares carry no labels: there are no fonts in this repo and no
// third-party dependencies to bring one in. `go run ./tools/hatchbook`
// prints a manifest naming every tile from the same tables the sketch
// draws from, so the two cannot drift.
package hatchbook

import (
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/hatch"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// Sketch renders one page of the specimen sheet.
type Sketch struct {
	// Page selects which sheet to draw; see PageNames.
	Page string
	// Margin is the border around the grid, in canvas units.
	Margin float64
	// Gutter is the space between squares, in canvas units.
	Gutter float64

	knobs *opt.Set
}

// New returns the specimen sheet with its defaults.
func New() *Sketch {
	s := &Sketch{Page: "structures", Margin: 0.03, Gutter: 0.014}
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "hatchbook" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "specimen sheet for internal/hatch: one square per structure, parameter and colouring"
}

// ink is the small set of roles a page paints with, pulled out of whatever
// palette was asked for. Naming the roles rather than indexing the palette
// is what lets every tile keep its meaning under any palette.
type ink struct {
	paper  palette.Color // the ground the marks are drawn on
	line   palette.Color // the main ink
	second palette.Color // the second family of a cross-hatch or weave
	third  palette.Color // the overprint where they meet
	tint   palette.Color // a filled ground to hatch against
}

func inks(p palette.Palette) ink {
	c := palette.ByLuminance(p.Colors) // darkest first
	n := len(c)
	k := ink{
		paper:  c[n-1].Lighten(0.72),
		line:   c[0],
		second: c[min(1, n-1)],
		third:  c[min(2, n-1)],
		tint:   c[n-1],
	}
	// The second ink has to be tellable from the first at a glance or a
	// two-colour cross-hatch reads as a one-colour one.
	if math.Abs(k.second.Luminance()-k.line.Luminance()) < 0.08 {
		k.second = c[min(n-2, n-1)]
	}
	return k
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	pg, err := pageByName(s.Page)
	if err != nil {
		return nil, err
	}
	k := inks(ctx.Palette)
	tiles := pg.build()
	g := s.grid(ctx, pg.cols, len(tiles))

	// A sheet ground a shade off the paper, so a tile whose own paper is
	// pale still reads as an object on a page.
	sheet := palette.Lerp(k.paper, k.line, 0.13)
	edge := palette.Lerp(k.paper, k.line, 0.35)

	return sketch.Raster(ctx, func(u, v float64) palette.Color {
		i, lu, lv, ok := g.locate(u, v)
		if !ok || i >= len(tiles) {
			return sheet
		}
		// A hairline frame: the squares butt onto a ground of similar value
		// and without it a pale tile has no edge.
		if b := g.frame(lu, lv); b > 0 {
			return palette.Lerp(tiles[i].paint(g.sample(lu, lv), k), edge, b)
		}
		return tiles[i].paint(g.sample(lu, lv), k)
	}), nil
}

// grid is the sheet's geometry: where each square sits and how a point
// inside one is described to a hatch.
type grid struct {
	cols, rows int
	side       float64 // square side, canvas units
	pitch      float64 // side + gutter
	ox, oy     float64 // top-left of the block of squares
	rim        float64 // frame thickness, canvas units
}

func (s *Sketch) grid(ctx sketch.Context, cols, n int) grid {
	rows := (n + cols - 1) / cols
	aspect := float64(ctx.Width) / float64(ctx.Height)
	// The square side is whichever of the two axes runs out first, so the
	// sheet fills a portrait frame as happily as a landscape one.
	w := (aspect - 2*s.Margin - float64(cols-1)*s.Gutter) / float64(cols)
	h := (1 - 2*s.Margin - float64(rows-1)*s.Gutter) / float64(rows)
	side := math.Max(math.Min(w, h), 1e-4)
	pitch := side + s.Gutter
	return grid{
		cols: cols, rows: rows, side: side, pitch: pitch,
		ox:  (aspect - (float64(cols)*side + float64(cols-1)*s.Gutter)) / 2,
		oy:  (1 - (float64(rows)*side + float64(rows-1)*s.Gutter)) / 2,
		rim: math.Min(0.0025, side*0.02),
	}
}

// locate maps a canvas point to the tile containing it and to that tile's
// own [0,1]² coordinates. Reading order is left to right, top to bottom.
func (g grid) locate(u, v float64) (index int, lu, lv float64, ok bool) {
	x, y := u-g.ox, v-g.oy
	if x < 0 || y < 0 {
		return 0, 0, 0, false
	}
	c, r := int(x/g.pitch), int(y/g.pitch)
	if c >= g.cols || r >= g.rows {
		return 0, 0, 0, false
	}
	lu = (x - float64(c)*g.pitch) / g.side
	lv = (y - float64(r)*g.pitch) / g.side
	if lu > 1 || lv > 1 {
		return 0, 0, 0, false // in the gutter
	}
	return r*g.cols + c, lu, lv, true
}

// sample describes a point of a tile to a hatch: the square is the region,
// so its centre, its half-width and the distance to its edge are all known,
// and the tone runs left to right across it.
//
// The tile is its own unit square rather than a window onto the canvas, so
// a spacing of 0.1 means a tenth of a square whatever size the square came
// out at. That is what makes one table of specs legible on every page.
func (g grid) sample(lu, lv float64) hatch.Sample {
	return hatch.Sample{
		U: lu, V: lv,
		CX: 0.5, CY: 0.5, Reach: 0.5,
		Wall: math.Min(math.Min(lu, 1-lu), math.Min(lv, 1-lv)),
		Tone: lu,
	}
}

// frame is how much of the tile's own border falls at this point.
func (g grid) frame(lu, lv float64) float64 {
	d := math.Min(math.Min(lu, 1-lu), math.Min(lv, 1-lv)) * g.side
	return 1 - mathx.Smoothstep(g.rim*0.6, g.rim, d)
}
