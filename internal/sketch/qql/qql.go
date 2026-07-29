// Package qql renders ring-dots packed along a flow field — a port of
// Tyler Hobbs' and Dandelion Mané's QQL, brought onto this project's terms.
//
// The piece is built in three movements. A seed first resolves to twelve
// orthogonal traits: what shape the field takes, how the lines are seeded,
// how big the dots are, how many colours are in play. Those traits then
// resolve to probabilistic parameters — "large rings" becomes a
// distribution of large sizes, not one size — and only then is anything
// drawn. Dots are laid along flow lines and rejected wherever they would
// crowd a neighbour, so the field proposes and the spacing rule disposes.
//
// Everything about the result is negotiable except the ring-dot itself:
// sizes, colours, density and structure all range widely, while the motif
// is preserved at every scale, small dots quietly shedding rings rather
// than turning to mush. That trade — wide variety, one unmistakable
// vocabulary — is what makes the output space worth exploring rather than
// merely large.
//
// Spec: docs/sketches/007-qql.md
package qql

import (
	"fmt"
	"image"

	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// RNG streams. Each stage draws from its own, so adding a consumer to one
// never disturbs the values another sees.
const (
	streamTraits = 1
	streamSpecs  = 2
	streamField  = 3
	streamLayout = 4
	streamLines  = 5
	streamColor  = 6
	streamPack   = 7
	streamPaint  = 8
)

// Sketch renders one QQL piece.
type Sketch struct {
	opts *trait.Options
}

// New creates the sketch with its default tunables.
func New() *Sketch {
	return &Sketch{opts: trait.NewOptions(schema)}
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "qql" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "ring-dots packed along a flow field (QQL); 12 traits, best at 4:5 — try --profile preview-tall"
}

// Schema implements sketch.Traited.
func (s *Sketch) Schema() trait.Schema { return schema }

// Traits implements sketch.Traited: the traits this seed resolves to, with
// any pinned by the CLI applied on top.
func (s *Sketch) Traits(ctx sketch.Context) trait.Set {
	return s.opts.Resolve(ctx.RNG(streamTraits))
}

// plan is everything a seed resolves to before a pixel is painted. Keeping
// it separate from the painting is what lets the tests assert on the
// composition rather than on an image.
type plan struct {
	traits trait.Set
	frame  frame
	scheme scheme
	stack  stackOffset
	dots   []dot
}

// Render implements sketch.Sketch. Stamp-painted: Context.AA/Deep unused.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	p, err := s.plan(ctx)
	if err != nil {
		return nil, err
	}
	cv := paint.NewCanvas(ctx.Width, ctx.Height, p.scheme.Background)
	paintDots(cv, p.frame, p.traits, p.scheme, p.stack, p.dots, ctx.RNG(streamPaint))
	return cv.Image(), nil
}

func (s *Sketch) plan(ctx sketch.Context) (plan, error) {
	if ctx.Width <= 0 || ctx.Height <= 0 {
		return plan{}, fmt.Errorf("qql: canvas must have a positive size")
	}
	tr := s.Traits(ctx)
	f := frame{aspect: float64(ctx.Width) / float64(ctx.Height)}

	set, err := s.colorSet(ctx, tr)
	if err != nil {
		return plan{}, err
	}

	// Specs: the traits made numeric, all from one stream in a fixed order.
	specRNG := ctx.RNG(streamSpecs)
	fieldSpec := newFlowFieldSpec(tr, specRNG)
	spacing := newSpacingSpec(tr, f, specRNG)
	changeOdds := newColorChangeOdds(tr, specRNG)
	scales := newScaleGenerator(tr, f, specRNG)
	rings := newBullseyeGenerator(tr, specRNG)
	ignoreOdds := newIgnoreFlowFieldOdds(specRNG)
	stack := newStackOffset(tr, f, specRNG)

	field := newFlowField(tr, fieldSpec, f, ctx.RNG(streamField))
	groups := startPoints(tr, f, ctx.RNG(streamLayout))

	// Whether a group runs straight instead of following the field is
	// settled up front, one draw per group.
	lineRNG := ctx.RNG(streamLines)
	ignore := make([]bool, len(groups))
	for i := range ignore {
		ignore[i] = odds(lineRNG, ignoreOdds)
	}

	sch := newScheme(tr, set, f, ctx.RNG(streamColor))

	pk := &packer{
		traits: tr, frame: f, spacing: spacing, odds: changeOdds,
		scales: scales, rings: rings, scheme: sch,
		margins: newMarginChecker(tr, f),
		index:   newPackIndex(f),
		tracer:  newTracer(field, f, lineRNG),
		spec:    fieldSpec,
		rng:     ctx.RNG(streamPack),
	}
	return plan{traits: tr, frame: f, scheme: sch, stack: stack, dots: pk.run(groups, ignore)}, nil
}

// colorSet picks the palette: one of QQL's own seven cities, or — only ever
// by explicit request — the ColorLisa palette named by --palette, adapted
// into the same swatch-and-sequence shape.
func (s *Sketch) colorSet(ctx sketch.Context, tr trait.Set) (colorSet, error) {
	name := tr.Get(dimPalette)
	if name == paletteExternal {
		return colorLisaSet(ctx.Palette)
	}
	set, ok := nativeColorSet(name)
	if !ok {
		return colorSet{}, fmt.Errorf("qql: no colour data for palette %q", name)
	}
	return set, nil
}
