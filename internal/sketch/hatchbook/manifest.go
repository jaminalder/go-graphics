package hatchbook

import (
	"fmt"
	"math"
	"strings"

	"github.com/jaminalder/go-graphics/internal/hatch"
)

// Manifest is the reading key for the sheets: for every page, every square
// by row and column, what it shows and the exact spec that drew it.
//
// It is generated from the same tables the sketch renders from, so a square
// and its entry cannot disagree — which matters more than usual here,
// because the squares carry no labels of their own (no fonts in this repo,
// and no third-party dependency to bring one in).
func Manifest() string {
	var b strings.Builder
	b.WriteString(`hatchbook — specimen sheet for internal/hatch

Reading order: left to right, then top to bottom. Row 1 is the top row,
column 1 the left-hand one. Every square is its own unit square, so a
spacing of 0.09 means nine hundredths of a square's side, whatever size
the squares came out at.

Regenerate every sheet and this file with:

  make hatchbook

The squares are square, so a sheet is given a frame of its own grid's
proportions rather than a square profile — otherwise a 6x4 page of squares
sits in a band of empty paper. The sizes are in the make target.

`)
	for _, p := range pages {
		tiles := p.build()
		rows := (len(tiles) + p.cols - 1) / p.cols
		fmt.Fprintf(&b, "\n=== page %q — %s\n", p.name, p.about)
		fmt.Fprintf(&b, "    %d squares, %d columns, %d rows\n\n", len(tiles), p.cols, rows)
		for i, t := range tiles {
			fmt.Fprintf(&b, "r%dc%d  %s\n", i/p.cols+1, i%p.cols+1, t.note)
			fmt.Fprintf(&b, "      %s\n", specLine(t.spec))
			if t.extra != "" {
				fmt.Fprintf(&b, "      %s\n", t.extra)
			}
		}
	}
	return b.String()
}

// specLine prints a spec in full. A catalogue entry that omits the fields
// that happen to be at their default is not a recipe.
func specLine(s hatch.Spec) string {
	r := hatch.New(s).Spec() // the values actually in force
	parts := []string{
		"structure=" + r.Structure.String(),
		fmt.Sprintf("angle=%.6g°", r.Angle*180/math.Pi),
		fmt.Sprintf("spacing=%.4g", r.Spacing),
		fmt.Sprintf("fit=%d", r.Fit),
		fmt.Sprintf("thickness=%.4g", r.Thickness),
		fmt.Sprintf("softness=%.4g", r.Softness),
		fmt.Sprintf("curvature=%.4g", r.Curvature),
		"waveform=" + r.Waveform.String(),
		fmt.Sprintf("amplitude=%.4g", r.Amplitude),
		fmt.Sprintf("wavelength=%.4g", r.Wavelength),
		fmt.Sprintf("continuity=%.4g", r.Continuity),
		fmt.Sprintf("dash=%.4g", r.Dash),
		fmt.Sprintf("jitter=%.4g", r.Jitter),
		fmt.Sprintf("tone-density=%.4g", r.ToneDensity),
		fmt.Sprintf("tone-width=%.4g", r.ToneWidth),
		"align=" + r.Align.String(),
		fmt.Sprintf("seed=%d", r.Seed),
	}
	return strings.Join(parts, " ")
}
