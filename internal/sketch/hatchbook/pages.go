package hatchbook

import (
	"fmt"
	"math"
	"strings"

	"github.com/jaminalder/go-graphics/internal/hatch"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// tile is one square of a sheet: what it shows, and how to paint it.
type tile struct {
	note  string     // what varies in this square, in words
	spec  hatch.Spec // the primary spec, printed in full into the manifest
	extra string     // anything the spec cannot say (a second family, a rule)
	paint func(hatch.Sample, ink) palette.Color
}

// page is one sheet of squares.
type page struct {
	name  string
	cols  int
	about string
	build func() []tile
}

var pages = []page{
	{"structures", 4, "one square per arranging rule, all at the same spacing", structuresPage},
	{"parameters", 6, "angle, spacing, thickness and curvature swept a row each", parametersPage},
	{"variation", 6, "waveform, continuity, jitter and tone swept a row each", variationPage},
	{"colour", 4, "the same marks under different colouring rules", colourPage},
	{"shapes", 4, "the structures filling a lobed region instead of a square", shapesPage},
}

// PageNames lists the sheets, in the order they are declared.
func PageNames() []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = p.name
	}
	return out
}

func pageByName(name string) (page, error) {
	for _, p := range pages {
		if p.name == name {
			return p, nil
		}
	}
	return page{}, fmt.Errorf("hatchbook: unknown page %q (have %s)", name, strings.Join(PageNames(), "|"))
}

// base is the spec every square starts from. Lengths are in *tile* units —
// each square is its own unit square — so one table of numbers reads the
// same on every sheet whatever size the squares came out at.
func base() hatch.Spec {
	s := hatch.Defaults()
	s.Spacing = 0.09
	s.Thickness = 0.3
	s.Softness = 0.25
	s.Wavelength = 0.3
	s.Dash = 0.14
	s.Seed = 1
	return s
}

func vary(f func(*hatch.Spec)) hatch.Spec { return base().With(f) }

// draw is the plainest colouring: one ink on the tile's paper. Most squares
// use it, because a catalogue of structures should differ only in structure.
func draw(note string, sp hatch.Spec) tile {
	f := hatch.Of(sp)
	return tile{note: note, spec: sp, paint: func(s hatch.Sample, k ink) palette.Color {
		return palette.Lerp(k.paper, k.line, f(s))
	}}
}

// --- structures ----------------------------------------------------------

func structuresPage() []tile {
	deg := math.Pi / 180
	cross := vary(func(c *hatch.Spec) { c.Angle = 45 * deg })
	triple := vary(func(c *hatch.Spec) { c.Angle = 0; c.Spacing = 0.11; c.Thickness = 0.2 })
	weaveA := vary(func(c *hatch.Spec) { c.Angle = 0; c.Spacing = 0.12; c.Thickness = 0.55 })

	crossF := hatch.Cross(cross, 135*deg)
	tripleF := hatch.Cross(triple, 60*deg, 120*deg)
	weaveF := hatch.Weave(weaveA, weaveA.Rotated(math.Pi/2))
	nestedF := quarters()

	return []tile{
		draw("parallel lines", vary(func(c *hatch.Spec) { c.Angle = 45 * deg })),
		{
			note: "cross-hatching: two families at 45° and 135°", spec: cross,
			extra: "second family: angle=135°, seed rotated",
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.paper, k.line, crossF(s))
			},
		},
		{
			note: "cross-hatching: three families at 0°, 60°, 120°", spec: triple,
			extra: "further families: angle=60° and 120°",
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.paper, k.line, tripleF(s))
			},
		},
		draw("contour: marks follow the region's boundary (Sample.Wall)",
			vary(func(c *hatch.Spec) { c.Structure = hatch.Contour })),
		draw("concentric: nested rings about the region's centre",
			vary(func(c *hatch.Spec) { c.Structure = hatch.Concentric })),
		draw("radial: rays from the region's centre",
			vary(func(c *hatch.Spec) { c.Structure = hatch.Radial })),
		draw("fan: arcs spread between two poles one Reach apart",
			vary(func(c *hatch.Spec) { c.Structure = hatch.Fan; c.Angle = 0 })),
		draw("flow field: marks are streamlines of a noise stream function",
			vary(func(c *hatch.Spec) {
				c.Structure, c.Amplitude, c.Wavelength = hatch.Flow, 2.2, 0.34
			})),
		draw("scribble: the same field with the mean direction removed",
			vary(func(c *hatch.Spec) {
				c.Structure, c.Amplitude, c.Wavelength = hatch.Scribble, 5, 0.42
				c.Thickness, c.Spacing = 0.3, 0.075
			})),
		draw("stipple: dots on the hatch lattice",
			vary(func(c *hatch.Spec) {
				c.Structure, c.Spacing, c.Thickness, c.Jitter = hatch.Stipple, 0.055, 0.45, 0.3
			})),
		draw("chord: every mark runs from boundary to boundary",
			vary(func(c *hatch.Spec) {
				c.Structure, c.Angle, c.Spacing, c.Thickness = hatch.Chord, 2.2, 0.05, 0.14
			})),
		draw("wave: sinusoidal parallel lines",
			vary(func(c *hatch.Spec) {
				c.Angle, c.Waveform, c.Amplitude, c.Wavelength = 0, hatch.Sine, 0.7, 0.3
			})),
		draw("zigzag: the same displacement as a sawtooth",
			vary(func(c *hatch.Spec) {
				c.Angle, c.Waveform, c.Amplitude, c.Wavelength = 0, hatch.Zigzag, 0.7, 0.22
			})),
		draw("broken: each mark cut into dashes with its own phase",
			vary(func(c *hatch.Spec) { c.Angle = 45 * deg; c.Continuity = 0.45 })),
		{
			note: "weave: two families with an over-under rule", spec: weaveA,
			extra: "second family: angle=90°; the thread on top alternates with the parity of the two mark indices",
			paint: func(s hatch.Sample, k ink) palette.Color {
				ca, cb := weaveF(s)
				col := palette.Lerp(k.paper, k.line, ca)
				return palette.Lerp(col, k.second, cb)
			},
		},
		{
			note:  "nested: the square subdivided into four, each quarter its own hatch",
			spec:  base(),
			extra: "quarters: parallel 45° / contour / stipple / radial, each fitted to its own quarter",
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.paper, k.line, nestedF(s))
			},
		},
	}
}

// quarters is a nested hatch: the tile is divided into four sub-regions and
// each is re-described to the inner hatch as a region in its own right, so
// a fitted inner hatch really does fit the quarter and not the tile.
func quarters() hatch.Func {
	q := base().With(func(c *hatch.Spec) { c.Align = hatch.AlignRegion; c.Fit = 5 })
	inner := []hatch.Func{
		hatch.Of(q.With(func(c *hatch.Spec) { c.Angle = math.Pi / 4 })),
		hatch.Of(q.With(func(c *hatch.Spec) { c.Structure = hatch.Contour })),
		hatch.Of(q.With(func(c *hatch.Spec) {
			c.Structure, c.Fit, c.Thickness, c.Jitter = hatch.Stipple, 8, 0.45, 0.3
		})),
		hatch.Of(q.With(func(c *hatch.Spec) { c.Structure = hatch.Radial; c.Fit = 18 })),
	}
	return hatch.Nested(func(s hatch.Sample) (hatch.Sample, int) {
		cx, cy, i := 0.25, 0.25, 0
		if s.U >= 0.5 {
			cx, i = 0.75, i+1
		}
		if s.V >= 0.5 {
			cy, i = 0.75, i+2
		}
		s.CX, s.CY, s.Reach = cx, cy, 0.25
		s.Wall = math.Min(0.25-math.Abs(s.U-cx), 0.25-math.Abs(s.V-cy))
		return s, i
	}, inner...)
}

// --- parameters ----------------------------------------------------------

func parametersPage() []tile {
	deg := math.Pi / 180
	var out []tile
	for _, a := range []float64{0, 30, 60, 90, 120, 150} {
		out = append(out, draw(fmt.Sprintf("angle %g°", a),
			vary(func(c *hatch.Spec) { c.Angle = a * deg })))
	}
	for _, sp := range []float64{0.30, 0.20, 0.135, 0.09, 0.06, 0.04} {
		out = append(out, draw(fmt.Sprintf("spacing %g of the square", sp),
			vary(func(c *hatch.Spec) { c.Spacing = sp })))
	}
	for _, th := range []float64{0.06, 0.15, 0.3, 0.5, 0.75, 1.0} {
		out = append(out, draw(fmt.Sprintf("thickness %g of the spacing", th),
			vary(func(c *hatch.Spec) { c.Thickness = th })))
	}
	for _, k := range []float64{0, 0.5, 1, 2, 4, 8} {
		out = append(out, draw(fmt.Sprintf("curvature %g (arc radius %s of a square, centred on the region)", k, radius(k)),
			vary(func(c *hatch.Spec) { c.Angle, c.Curvature, c.Align = 0, k, hatch.AlignRegion })))
	}
	return out
}

func radius(k float64) string {
	if k == 0 {
		return "infinite"
	}
	return fmt.Sprintf("%.3g", 1/k)
}

// --- variation -----------------------------------------------------------

func variationPage() []tile {
	var out []tile

	waves := []struct {
		note string
		f    func(*hatch.Spec)
	}{
		{"straight", func(c *hatch.Spec) {}},
		{"sine, amplitude 0.3 spacings, wavelength 0.3", func(c *hatch.Spec) {
			c.Waveform, c.Amplitude, c.Wavelength = hatch.Sine, 0.3, 0.3
		}},
		{"sine, amplitude 0.9 spacings, wavelength 0.3", func(c *hatch.Spec) {
			c.Waveform, c.Amplitude, c.Wavelength = hatch.Sine, 0.9, 0.3
		}},
		{"sine, amplitude 0.9 spacings, wavelength 0.12", func(c *hatch.Spec) {
			c.Waveform, c.Amplitude, c.Wavelength = hatch.Sine, 0.9, 0.12
		}},
		{"zigzag, amplitude 0.6 spacings, wavelength 0.2", func(c *hatch.Spec) {
			c.Waveform, c.Amplitude, c.Wavelength = hatch.Zigzag, 0.6, 0.2
		}},
		{"zigzag, amplitude 1.4 spacings, wavelength 0.14", func(c *hatch.Spec) {
			c.Waveform, c.Amplitude, c.Wavelength = hatch.Zigzag, 1.4, 0.14
		}},
	}
	for _, w := range waves {
		out = append(out, draw("waveform: "+w.note, vary(func(c *hatch.Spec) {
			c.Angle = 0
			w.f(c)
		})))
	}

	for _, tc := range []struct{ cont, dash float64 }{
		{1, 0.14}, {0.8, 0.14}, {0.6, 0.14}, {0.4, 0.14}, {0.25, 0.14}, {0.4, 0.05},
	} {
		out = append(out, draw(fmt.Sprintf("continuity %g, dash period %g", tc.cont, tc.dash),
			vary(func(c *hatch.Spec) { c.Continuity, c.Dash = tc.cont, tc.dash })))
	}

	for _, j := range []float64{0, 0.1, 0.25, 0.4} {
		out = append(out, draw(fmt.Sprintf("jitter %g of the spacing", j),
			vary(func(c *hatch.Spec) { c.Jitter = j })))
	}
	for _, sf := range []float64{0, 1} {
		out = append(out, draw(fmt.Sprintf("softness %g of the half-width", sf),
			vary(func(c *hatch.Spec) { c.Softness = sf })))
	}

	for _, d := range []float64{1, 2, 3} {
		out = append(out, draw(fmt.Sprintf("density gradient: tone halves the hatch up to %g times (tone runs left to right)", d),
			vary(func(c *hatch.Spec) { c.ToneDensity = d })))
	}
	for _, w := range []float64{0.6, 1} {
		out = append(out, draw(fmt.Sprintf("variable width: tone drives thickness ±%g (tone runs left to right)", w),
			vary(func(c *hatch.Spec) { c.ToneWidth = w })))
	}
	out = append(out, draw("fit 7: seven marks across the region whatever its size, phase at its centre",
		vary(func(c *hatch.Spec) { c.Fit, c.Align = 7, hatch.AlignRegion })))
	return out
}

// --- colour --------------------------------------------------------------

func colourPage() []tile {
	deg := math.Pi / 180
	plain := vary(func(c *hatch.Spec) { c.Angle = 45 * deg })
	warp := vary(func(c *hatch.Spec) { c.Angle = 0; c.Spacing = 0.12; c.Thickness = 0.55 })

	a := hatch.New(plain)
	b := hatch.New(plain.Rotated(135 * deg).With(func(c *hatch.Spec) { c.Seed = 2 }))
	flow := hatch.New(vary(func(c *hatch.Spec) {
		c.Structure, c.Amplitude, c.Wavelength = hatch.Flow, 2.2, 0.34
	}))
	contour := hatch.New(vary(func(c *hatch.Spec) { c.Structure = hatch.Contour }))
	stip := hatch.New(vary(func(c *hatch.Spec) {
		c.Structure, c.Spacing, c.Thickness, c.Jitter = hatch.Stipple, 0.05, 0.5, 0.3
		c.ToneWidth = 1
	}))
	dense := hatch.New(vary(func(c *hatch.Spec) { c.Angle = 45 * deg; c.ToneDensity = 3 }))
	wide := hatch.New(vary(func(c *hatch.Spec) { c.Angle = 45 * deg; c.ToneWidth = 1 }))
	both := hatch.New(vary(func(c *hatch.Spec) {
		c.Angle, c.ToneDensity, c.ToneWidth = 45*deg, 2, 0.8
	}))
	weave := hatch.Weave(warp, warp.Rotated(math.Pi/2))
	triple := []*hatch.Hatch{
		hatch.New(vary(func(c *hatch.Spec) { c.Angle = 0; c.Spacing = 0.11 })),
		hatch.New(vary(func(c *hatch.Spec) { c.Angle = 60 * deg; c.Spacing = 0.11; c.Seed = 2 })),
		hatch.New(vary(func(c *hatch.Spec) { c.Angle = 120 * deg; c.Spacing = 0.11; c.Seed = 3 })),
	}

	return []tile{
		{
			note: "single ink on paper", spec: plain,
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.paper, k.line, a.Cover(s))
			},
		},
		{
			note: "two colours in a cross-hatch: a family each", spec: plain,
			extra: "second family: angle=135°",
			paint: func(s hatch.Sample, k ink) palette.Color {
				col := palette.Lerp(k.paper, k.line, a.Cover(s))
				return palette.Lerp(col, k.second, b.Cover(s))
			},
		},
		{
			note: "cross-hatch where the crossings take a third colour", spec: plain,
			extra: "second family: angle=135°; the overprint is painted where both cover",
			paint: func(s hatch.Sample, k ink) palette.Color {
				ca, cb := a.Cover(s), b.Cover(s)
				col := palette.Lerp(k.paper, k.line, ca)
				col = palette.Lerp(col, k.second, cb)
				return palette.Lerp(col, k.third, ca*cb)
			},
		},
		{
			note: "density encodes tone: the hatch halves itself as the tone falls (tone runs left to right)",
			spec: dense.Spec(),
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.paper, k.line, dense.Cover(s))
			},
		},
		{
			note: "variable width encodes tone (tone runs left to right)", spec: wide.Spec(),
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.paper, k.line, wide.Cover(s))
			},
		},
		{
			note: "density and width together", spec: both.Spec(),
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.paper, k.line, both.Cover(s))
			},
		},
		{
			note: "hatch against a filled ground", spec: plain,
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.tint, k.line, a.Cover(s))
			},
		},
		{
			note: "light ink on a dark ground", spec: plain,
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.line, k.paper, a.Cover(s))
			},
		},
		{
			note:  "colour shifts along the hatch direction",
			spec:  plain,
			extra: "the ink is lerped by Coords' along coordinate",
			paint: func(s hatch.Sample, k ink) palette.Color {
				_, along, ok := a.Coords(s)
				t := 0.5
				if ok {
					t = mathx.Clamp01(along/1.4 + 0.5)
				}
				return palette.Lerp(k.paper, palette.LerpHSL(k.line, k.second, t), a.Cover(s))
			},
		},
		{
			note:  "colour shifts along a flow field's own streamlines",
			spec:  flow.Spec(),
			extra: "the ink is lerped by Coords' along coordinate",
			paint: func(s hatch.Sample, k ink) palette.Color {
				_, along, ok := flow.Coords(s)
				t := 0.5
				if ok {
					t = mathx.Clamp01(along/1.4 + 0.5)
				}
				return palette.Lerp(k.paper, palette.LerpHSL(k.third, k.line, t), flow.Cover(s))
			},
		},
		{
			note:  "colour bands across the family: every third mark in a second ink",
			spec:  plain,
			extra: "the ink is chosen by CoverLine's mark index",
			paint: func(s hatch.Sample, k ink) palette.Color {
				c, line := a.CoverLine(s)
				col := k.line
				if ((line%3)+3)%3 == 0 {
					col = k.second
				}
				return palette.Lerp(k.paper, col, c)
			},
		},
		{
			note:  "ground graded under a constant hatch",
			spec:  contour.Spec(),
			extra: "the paper is lerped toward the tint by the tone; the ink is not",
			paint: func(s hatch.Sample, k ink) palette.Color {
				ground := palette.LerpHSL(k.paper, k.tint, s.Tone)
				return palette.Lerp(ground, k.line, contour.Cover(s))
			},
		},
		{
			note: "tonal stipple: tone drives the dot size (tone runs left to right)", spec: stip.Spec(),
			paint: func(s hatch.Sample, k ink) palette.Color {
				return palette.Lerp(k.tint, k.line, stip.Cover(s))
			},
		},
		{
			note: "duotone weave: a colour per thread, over and under", spec: warp,
			extra: "second family: angle=90°",
			paint: func(s hatch.Sample, k ink) palette.Color {
				ca, cb := weave(s)
				col := palette.Lerp(k.paper, k.line, ca)
				return palette.Lerp(col, k.second, cb)
			},
		},
		{
			note: "three families, a colour each", spec: triple[0].Spec(),
			extra: "further families: angle=60° and 120°",
			paint: func(s hatch.Sample, k ink) palette.Color {
				col := palette.Lerp(k.paper, k.line, triple[0].Cover(s))
				col = palette.Lerp(col, k.second, triple[1].Cover(s))
				return palette.Lerp(col, k.third, triple[2].Cover(s))
			},
		},
		{
			note:  "the hatch as a mask, ink graded across the square",
			spec:  plain,
			extra: "coverage picks nothing but the amount; the colour comes from position",
			paint: func(s hatch.Sample, k ink) palette.Color {
				ink3 := palette.LerpHSL(k.line, k.third, mathx.Clamp01(s.V))
				return palette.Lerp(k.paper, ink3, a.Cover(s))
			},
		},
	}
}

// --- shapes --------------------------------------------------------------

// lobes are two wavy discs of quite different size. Two rather than one on
// purpose: half of what alignment means only shows when the same rule fills
// regions that differ, and a single specimen cannot show whether a hatch
// belongs to the canvas or to the shape.
var lobes = []struct{ cx, cy, r, twist float64 }{
	{0.34, 0.38, 0.27, 0},
	{0.74, 0.76, 0.17, 1.3},
}

// lobe re-describes a tile point as a point in whichever lobe it falls in.
// It is the closest thing here to a real caller: a foam cell hands over
// exactly this much — a centre, an inscribed radius, a wall distance — and
// nothing at all about its outline.
func lobe(s hatch.Sample) (hatch.Sample, float64) {
	best, wall, pick := math.Inf(-1), 0.0, 0
	for i, l := range lobes {
		dx, dy := s.U-l.cx, s.V-l.cy
		edge := l.r * (1 + 0.17*math.Cos(3*math.Atan2(dy, dx)+l.twist) + 0.07*math.Cos(5*math.Atan2(dy, dx)-0.6))
		w := edge - math.Hypot(dx, dy)
		// Relative depth, so the small lobe is not swallowed by the large one.
		if w/l.r > best {
			best, wall, pick = w/l.r, w, i
		}
	}
	l := lobes[pick]
	s.CX, s.CY, s.Reach, s.Wall = l.cx, l.cy, l.r, wall
	s.Tone = mathx.Clamp01(wall / l.r)
	// The mask is the region's own outline, which the hatch never sees.
	return s, mathx.Smoothstep(-0.003, 0.003, wall)
}

func shapesPage() []tile {
	deg := math.Pi / 180
	fitted := base().With(func(c *hatch.Spec) { c.Align = hatch.AlignRegion })

	shape := func(note string, sp hatch.Spec) tile {
		f := hatch.Of(sp)
		return tile{
			note: note, spec: sp, extra: "region: a lobed disc of reach 0.40; the hatch is masked by its outline",
			paint: func(s hatch.Sample, k ink) palette.Color {
				in, m := lobe(s)
				return palette.Lerp(k.paper, k.line, f(in)*m)
			},
		}
	}

	return []tile{
		shape("parallel, canvas-aligned: one screen over both lobes — the marks line up across the gap",
			vary(func(c *hatch.Spec) { c.Angle = 45 * deg })),
		shape("parallel, region-aligned: each lobe carries its own hatch, phased at its own centre",
			fitted.With(func(c *hatch.Spec) { c.Angle = 45 * deg })),
		shape("fit 7: seven marks across each lobe, though one is half the size of the other",
			fitted.With(func(c *hatch.Spec) { c.Angle = 45 * deg; c.Fit = 7 })),
		shape("contour: the marks bend around the lobes",
			fitted.With(func(c *hatch.Spec) { c.Structure = hatch.Contour; c.Fit = 6 })),
		shape("concentric: rings about the centre, ignoring the lobes",
			fitted.With(func(c *hatch.Spec) { c.Structure = hatch.Concentric; c.Fit = 6 })),
		shape("radial: rays from the centre",
			fitted.With(func(c *hatch.Spec) { c.Structure = hatch.Radial; c.Fit = 30 })),
		shape("fan between two poles inside the region",
			fitted.With(func(c *hatch.Spec) { c.Structure = hatch.Fan; c.Angle = 0; c.Fit = 30 })),
		shape("chord: every mark runs from edge to edge",
			fitted.With(func(c *hatch.Spec) {
				c.Structure, c.Angle, c.Fit, c.Thickness = hatch.Chord, 2.2, 46, 0.16
			})),
		shape("flow field, clipped to the region",
			fitted.With(func(c *hatch.Spec) {
				c.Structure, c.Amplitude, c.Wavelength = hatch.Flow, 2.2, 0.34
			})),
		shape("stipple, fitted to the region",
			fitted.With(func(c *hatch.Spec) {
				c.Structure, c.Fit, c.Thickness, c.Jitter = hatch.Stipple, 14, 0.45, 0.3
			})),
		shape("broken contour: dashes following the boundary",
			fitted.With(func(c *hatch.Spec) {
				c.Structure, c.Fit, c.Continuity, c.Dash = hatch.Contour, 7, 0.5, 0.12
			})),
		shape("contour with tone: the hatch thins toward the middle of the region",
			fitted.With(func(c *hatch.Spec) {
				c.Structure, c.Fit, c.ToneDensity = hatch.Contour, 9, 2
			})),
	}
}
