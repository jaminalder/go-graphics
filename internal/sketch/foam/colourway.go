package foam

import (
	"fmt"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The colours are part of the output space, not a setting beside it — the
// argument is set out in sketch 008's colourway.go. Sweeping seeds has to
// give different pictures in different colours; with the palette outside
// the output space every seed of a sweep comes out in whatever `--palette`
// said.
//
// What this sketch asks of a palette is not what 008 asks. There the
// pigments have to survive being laid transparently on a tinted ground, so
// the list is weighted toward the dark and the muted. Here every cell is an
// opaque patch of colour walled off from its neighbours by graphite, and
// the reference is frankly polychrome: what it needs is *hue count*. A
// four-colour palette divides a forty-cell sheet into four zones; a
// palette with distinct warms and cools in it gives every cell a chance to
// differ from the one next to it.

const dimColourway = "colourway"

// fromFlag hands colour duty back to whatever --palette names. Weight 0, so
// a seed never lands on it: using this sketch with a palette from outside
// the curated list is always a deliberate act.
const fromFlag = "from-flag"

// colourways are the palettes a seed may draw: broad, many-hued sets with a
// light member that can serve as paper and a dark one that can pass for
// graphite.
var colourways = []trait.Value{
	{Name: "tchelitchew-hide-and-seek", Weight: 3},
	{Name: "kandinsky-apple-tree", Weight: 3},
	{Name: "matisse-collioure", Weight: 3},
	{Name: "klee-fire-evening", Weight: 2},
	{Name: "chagall-mariee", Weight: 2},
	{Name: "miro-woman-dog-moon", Weight: 2},
	{Name: "hockney-bigger-splash", Weight: 2},
	{Name: "delaunay-bleriot", Weight: 2},
	{Name: "gauguin-siesta", Weight: 2},
	{Name: "redon-green-vase", Weight: 2},
	{Name: "varo-harmony", Weight: 2},
	{Name: "seurat-grande-jatte", Weight: 2},
	{Name: "bruegel-icarus", Weight: 1},
	{Name: "cezanne-bathers", Weight: 1},
	{Name: "diebenkorn-seawall", Weight: 1},
	{Name: "hopper-night-windows", Weight: 1},
	{Name: "avery-bicycle-rider", Weight: 1},
	{Name: "sargent-carnation-lily", Weight: 1},
	{Name: "monet-water-lilies", Weight: 1},
	{Name: "vangogh-arles", Weight: 1},
	{Name: fromFlag, Weight: 0},
}

// colours resolves the colourway trait to the palette a render works from.
func colours(name string, fromCLI palette.Palette) (palette.Palette, error) {
	if name == fromFlag {
		return fromCLI, nil
	}
	p, ok := palette.ByName(name)
	if !ok {
		return palette.Palette{}, fmt.Errorf("foam: no palette %q", name)
	}
	return p, nil
}
