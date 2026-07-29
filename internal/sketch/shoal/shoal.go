// Package shoal implements sketch 006: an all-over field of small
// brush-painted dots strung along the streamlines of a noise flow field
// (spec: docs/sketches/006-shoal.md).
//
// Dots are laid down in chains, each dot painted with the bristle brush
// from internal/paint, so a large dot shows the concentric smear of the
// brush that made it and a small one reads as a flat spot.
package shoal

import (
	"image"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamLayout = 1 // streamline walking, dot placement, colour choice
	streamPaint  = 2 // per-dot brush jitter
)

// Noise seed salts, so the three fields are independent of one another.
const (
	saltField = 0x666c64 // "fld"
	saltSize  = 0x73697a // "siz"
	saltColor = 0x636f6c // "col"
)

// Field selects how a flow direction is read out of the noise.
type Field uint8

const (
	// FieldCurl walks the divergence-free curl of the noise: closed
	// eddies, evenly spread.
	FieldCurl Field = iota
	// FieldFlow reads an angle straight from fBm: broad sweeping
	// currents that drain into sinks, leaving open ground.
	FieldFlow
	// FieldRidge steers by ridged noise: chains bunch into filaments
	// along the folds.
	FieldRidge
)

// Grade selects how dot radius varies over the canvas.
type Grade uint8

const (
	// GradePatches takes radius from a low-frequency noise field alone:
	// islands of coarse and fine stipple.
	GradePatches Grade = iota
	// GradeVortex additionally shrinks dots toward a focal point, so the
	// field tightens into a dense eye.
	GradeVortex
)

// Sketch holds the structural knobs. Per-seed variation happens in place.
type Sketch struct {
	MinR, MaxR float64 // dot radius range, canvas units
	Gap        float64 // edge distance between dots, as a fraction of radius
	FieldFreq  float64 // flow field frequency, cycles per canvas unit
	Swing      float64 // radians of direction swing (flow and ridge only)
	SizeFreq   float64 // size field frequency
	ColorFreq  float64 // colour field frequency
	Warp       float64 // domain-warp strength applied to the flow field
	Starts     int     // streamline seeds per axis
	MaxSteps   int     // dots attempted per streamline, each direction
	MaxMiss    int     // consecutive blocked steps before a chain gives up
	MinChain   int     // chains shorter than this are dropped as stubs
	Margin     float64 // clear ground left around the field
	Detail     float64 // share of dots that get concentric ring detail
	Open       float64 // share of dots painted as rings rather than discs
	Confetti   float64 // share of chains whose colour ignores the field
	ColorFlip  float64 // chance a chain changes colour at each dot
	Field      Field
	Grade      Grade

	opts cliOptions
}

// New returns the sketch with its defaults.
func New() *Sketch {
	return &Sketch{
		MinR:      0.0035,
		MaxR:      0.0135,
		Gap:       0.2,
		FieldFreq: 1.8,
		Swing:     6.5,
		SizeFreq:  1.6,
		ColorFreq: 1.9,
		Warp:      0.7,
		Starts:    88,
		MaxSteps:  90,
		MaxMiss:   34,
		MinChain:  3,
		Margin:    0.045,
		Detail:    0.09,
		Open:      0.05,
		Confetti:  0.4,
		ColorFlip: 0.06,
		Field:     FieldFlow,
		Grade:     GradeVortex,
	}
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "shoal" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "all-over field of brush-painted dots strung along a noise flow field"
}

// dot is one placed, coloured dot of the scene.
type dot struct {
	geom.Circle
	main   palette.Color
	ink    palette.Color
	ringed bool
	open   bool
}

// Render implements sketch.Sketch. Painting is stamp-based, so
// Context.AA and Deep are unused — the soft brush edges anti-alias.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	aspect := float64(ctx.Width) / float64(ctx.Height)

	byLum := append([]palette.Color(nil), ctx.Palette.Colors...)
	sort.SliceStable(byLum, func(i, j int) bool {
		return byLum[i].Luminance() < byLum[j].Luminance()
	})
	ground := byLum[len(byLum)-1].Lighten(0.62).Desaturate(0.45)
	inks := dotColors(byLum)

	dots := newPlanner(s, ctx, aspect, inks, byLum[0]).run()

	cv := paint.NewCanvas(ctx.Width, ctx.Height, ground)
	brng := ctx.RNG(streamPaint)
	for _, d := range dots {
		s.paintDot(cv, brng, d)
	}
	return cv.Image(), nil
}

// dotColors widens a five-colour palette into a set with enough range to
// carry a whole field. Each palette colour contributes itself and a
// lightened tint, which is what lets one hue appear as both a deep and a
// pale dot the way a hand-mixed set of paints would.
func dotColors(byLum []palette.Color) []palette.Color {
	base := byLum[:len(byLum)-1] // lightest is the ground
	out := make([]palette.Color, 0, len(base)*2)
	for _, c := range base {
		out = append(out, c, c.Lighten(0.3).Desaturate(0.12))
	}
	return out
}

// direction is the flow angle at (x, y) for the selected field.
//
// The plain angle field is the default rather than the curl even though
// the curl is the "correct" divergence-free one: an angle field has
// sources, sinks and saddles, and those are exactly what gather chains
// into dense drifts and open up quiet ground elsewhere. A curl field
// spreads dots perfectly evenly, which is the same problem as wallpaper.
func (s *Sketch) direction(n *noise.Perlin, x, y float64) float64 {
	fx, fy := x*s.FieldFreq, y*s.FieldFreq
	if s.Warp > 0 {
		// One level of domain warp: enough to fold the broad sweeps into
		// eddies, where a second level would just be turbulence.
		qx := n.FBM(fx, fy, 2)
		qy := n.FBM(fx+5.2, fy+1.3, 2)
		fx += s.Warp * qx
		fy += s.Warp * qy
	}
	switch s.Field {
	case FieldCurl:
		return n.CurlAngle(fx, fy, 2)
	case FieldRidge:
		return s.Swing * (2*n.Ridged(fx, fy, 3) - 1)
	default:
		return s.Swing * n.FBM(fx, fy, 2)
	}
}

// planner is the per-render state the layout walk needs: the three noise
// fields, the layout RNG, the ink set and its weighted draw pile, and the
// collision index. It exists so the walk can stay a method with a short
// signature instead of threading a dozen arguments through every call.
type planner struct {
	s              *Sketch
	fieldN, sizeN  *noise.Perlin
	colorN         *noise.Perlin
	rng            *rand.Rand
	inks, bag      []palette.Color
	darkest        palette.Color
	index          *geom.Index
	aspect         float64
	focusX, focusY float64
}

// newPlanner sets up the layout state for one render.
func newPlanner(s *Sketch, ctx sketch.Context, aspect float64, inks []palette.Color, darkest palette.Color) *planner {
	rng := ctx.RNG(streamLayout)
	return &planner{
		s:       s,
		fieldN:  noise.New(ctx.Seed ^ saltField),
		sizeN:   noise.New(ctx.Seed ^ saltSize),
		colorN:  noise.New(ctx.Seed ^ saltColor),
		rng:     rng,
		inks:    inks,
		bag:     weightedBag(rng, inks),
		darkest: darkest,
		index:   geom.NewIndex(aspect, 1, s.MaxR),
		aspect:  aspect,
		// The eye of a vortex sits off-centre; dead centre reads as a target.
		focusX: aspect * (0.32 + 0.36*rng.Float64()),
		focusY: 0.32 + 0.36*rng.Float64(),
	}
}

// run walks streamlines and drops a dot wherever one fits. Every budget
// is fixed, so the walk is deterministic. Afterwards the returned dots
// and p.index hold exactly the same circles.
func (p *planner) run() []dot {
	s, rng := p.s, p.rng
	var dots []dot
	// Each seed grows a chain in both directions along the flow, so a
	// chain is a whole streamline through its seed rather than a tail
	// hanging off one. Seeds that land in already-crowded ground then
	// still contribute the part of the line that runs into open space.
	for sy := range s.Starts {
		for sx := range s.Starts {
			x := (float64(sx) + 0.15 + 0.7*rng.Float64()) / float64(s.Starts) * p.aspect
			y := (float64(sy) + 0.15 + 0.7*rng.Float64()) / float64(s.Starts)
			col := p.chainColor(x, y)
			var chain []dot
			for _, dir := range [2]float64{1, -1} {
				chain = p.walk(chain, x, y, dir, col)
			}
			// Stubs are visual noise: a chain has to read as a thread.
			// Nothing reaches the collision index until the chain has
			// earned its place — a culled stub that had already been
			// inserted would go on blocking ground it no longer occupies,
			// and enough of those starve the whole field of space.
			if len(chain) < s.MinChain {
				continue
			}
			for _, d := range chain {
				p.index.Insert(d.Circle)
			}
			dots = append(dots, chain...)
		}
	}
	return dots
}

// radiusAt is the dot radius the size field calls for at (x, y).
func (p *planner) radiusAt(x, y float64) float64 {
	s := p.s
	t := mathx.Clamp01(p.sizeN.FBM(x*s.SizeFreq, y*s.SizeFreq, 2)/1.1 + 0.5)
	t *= t // quadratic: many small dots, few large ones
	if s.Grade == GradeVortex {
		// Squeeze only the variable part, so the eye of the vortex fills
		// with dots at MinR rather than shrinking to specks.
		d := math.Hypot(x-p.focusX, y-p.focusY)
		t *= 0.12 + 0.88*mathx.Smoothstep(0, 0.45, d)
	}
	return s.MinR + (s.MaxR-s.MinR)*t
}

// walk grows one chain from (x, y) along dir, returning chain with every
// dot that fits appended. A blocked step is skipped rather than fatal —
// the chain threads on through crowded ground and only gives up after
// MaxMiss consecutive failures. Candidates are tested against both the
// index and the chain so far, since the chain is not in the index yet.
func (p *planner) walk(chain []dot, x, y, dir float64, col palette.Color) []dot {
	s := p.s
	misses := 0
	for step := 0; step < s.MaxSteps && misses < s.MaxMiss; step++ {
		r := p.radiusAt(x, y)
		gap := r * s.Gap
		inside := x > s.Margin+r && x < p.aspect-s.Margin-r &&
			y > s.Margin+r && y < 1-s.Margin-r
		c := geom.Circle{X: x, Y: y, R: r}
		if inside && p.index.FitsWithGap(c, gap) && fitsChain(c, gap, chain) {
			// Colour runs along the chain and only occasionally turns
			// over. Re-rolling per dot would scatter the palette into
			// confetti and destroy the drifts of colour that give the
			// field its large-scale structure.
			if p.rng.Float64() < s.ColorFlip {
				col = p.chainColor(x, y)
			}
			chain = append(chain, dot{
				Circle: c,
				main:   col,
				ink:    p.darkest,
				ringed: p.ringedAt(x, y, r),
				open:   p.rng.Float64() < s.Open,
			})
			misses = 0
		} else {
			misses++
		}
		angle := s.direction(p.fieldN, x, y)
		// Advance by one dot plus the gap, so consecutive dots nearly
		// touch and the chain reads as a thread rather than a scatter.
		adv := 2*r + gap
		x += dir * adv * math.Cos(angle)
		y += dir * adv * math.Sin(angle)
		if x < 0 || x > p.aspect || y < 0 || y > 1 {
			break
		}
	}
	return chain
}

// fitsChain reports whether c clears every dot of the chain being grown.
// The chain is not in the collision index yet, so it has to be tested
// directly; chains are short enough that the linear scan costs nothing.
func fitsChain(c geom.Circle, gap float64, chain []dot) bool {
	for _, d := range chain {
		if math.Hypot(c.X-d.X, c.Y-d.Y) < c.R+d.R+gap {
			return false
		}
	}
	return true
}

// weightedBag turns the ink set into a draw pile with a long tail: after
// a per-seed shuffle, one colour dominates, one supports it and the rest
// are progressively rarer. Drawing uniformly from the palette instead
// gives every hue equal presence, which reads as confetti; a dominant
// colour with rare accents is what reads as chosen.
func weightedBag(rng *rand.Rand, inks []palette.Color) []palette.Color {
	order := append([]palette.Color(nil), inks...)
	rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
	weights := []int{14, 8, 5, 4, 3, 2, 2, 1}
	var bag []palette.Color
	for i, c := range order {
		w := 1
		if i < len(weights) {
			w = weights[i]
		}
		for range w {
			bag = append(bag, c)
		}
	}
	return bag
}

// chainColor picks the colour a chain starts with: mostly from the
// low-frequency colour field, so neighbouring chains agree and build
// large masses, and a Confetti share from the weighted bag instead,
// which keeps those masses from hardening into flat zones.
func (p *planner) chainColor(x, y float64) palette.Color {
	if p.rng.Float64() < p.s.Confetti {
		return p.bag[p.rng.IntN(len(p.bag))]
	}
	v := mathx.Clamp01(p.colorN.FBM(x*p.s.ColorFreq, y*p.s.ColorFreq, 2)/1.1 + 0.5)
	i := int(v * float64(len(p.inks)))
	if p.rng.Float64() < 0.3 { // soften the banding between field levels
		i += 1 - p.rng.IntN(3)
	}
	return p.inks[((i%len(p.inks))+len(p.inks))%len(p.inks)]
}

// ringedAt decides whether a dot carries interior rings. The detail is
// clustered by its own noise field and gated on size: scattered evenly it
// reads as a rendering fault, and on a dot near MinR the bands cannot
// resolve at all.
func (p *planner) ringedAt(x, y, r float64) bool {
	if r < p.s.MinR*2.2 {
		return false
	}
	// Reuse the colour field at a different offset and scale: cheaper
	// than a fourth generator and just as uncorrelated in practice.
	v := mathx.Clamp01(p.colorN.FBM(x*0.9+11.7, y*0.9+3.1, 2)/1.1 + 0.5)
	return p.rng.Float64() < p.s.Detail*(0.25+2.2*v*v)
}

// paintDot lays one dot with the bristle brush. Bundle size follows the
// dot's radius: a big dot is worth a full brush and shows its smear, a
// small one only ever reads as a spot and does not need the work.
func (s *Sketch) paintDot(cv *paint.Canvas, rng *rand.Rand, d dot) {
	t := mathx.Clamp01((d.R - s.MinR) / math.Max(s.MaxR-s.MinR, 1e-9))
	n := 3 + int(8*t)
	dry := 0.16 + 0.2*rng.Float64()
	const wobble = 0.035

	if d.open {
		b := paint.NewBrush(rng, d.R*0.42, max(n/2, 3), dry)
		paint.SweepRing(cv, rng, b, d.X, d.Y, d.R*0.48, d.R, d.main, 0.97, wobble)
		return
	}
	b := paint.NewBrush(rng, d.R*0.62, n, dry)
	paint.SweepRing(cv, rng, b, d.X, d.Y, 0, d.R, d.main, 0.97, wobble)

	if d.ringed {
		k := 2 + rng.IntN(2)
		fine := paint.NewBrush(rng, d.R*0.1, 2, 0.15).Grain(rng, d.R*2.5)
		for i := 1; i <= k; i++ {
			rr := d.R * (float64(i) - 0.25) / float64(k)
			fine.Stroke(cv, paint.Ring(rng, d.X, d.Y, rr, 1.05, wobble*rr), d.ink, 0.75)
		}
	}
}
