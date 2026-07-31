package pools

import (
	"fmt"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The colours are part of the output space, not a setting beside it.
//
// Without this a seed cannot choose a colourway: every seed of a sweep
// comes out in whatever `--palette` said, and sweeping the palette instead
// gives the cartesian product — the same composition repeated once per
// colour, which is one picture shown five ways rather than five pictures.
// QQL has always had this (`--qql-palette`); 008 was the odd one out.
//
// The list is curated rather than the whole ColorLisa set. A transparent
// wash on tinted paper asks something specific of a palette — pigments
// dark enough to read against their own ground, and a lightest colour that
// can tint the sheet — and most of the collection was chosen by painters
// for other purposes.

const dimColourway = "colourway"

// fromFlag hands colour duty back to whatever --palette names. It carries
// weight 0, so a seed never lands on it: using this sketch with a palette
// from outside the curated list is always a deliberate act.
const fromFlag = "from-flag"

// colourways are the palettes a seed may draw, all verified against the
// wash: they hold up as pigment on their own tinted ground.
var colourways = []trait.Value{
	{Name: "tchelitchew-hide-and-seek", Weight: 3},
	{Name: "botero-seated-nude", Weight: 2},
	{Name: "diebenkorn-seawall", Weight: 2},
	{Name: "davis-anthracite-minuet", Weight: 2},
	{Name: "bruegel-icarus", Weight: 2},
	{Name: "hopper-night-windows", Weight: 2},
	{Name: "cezanne-bathers", Weight: 2},
	{Name: "beckmann-dancing-bar", Weight: 2},
	{Name: "avery-bicycle-rider", Weight: 1},
	{Name: "munch-scream-oil", Weight: 1},
	{Name: "dix-nun", Weight: 1},
	{Name: "dechirico-red-tower", Weight: 1},
	{Name: "chagall-mariee", Weight: 1},
	{Name: "klee-destruction-hope", Weight: 1},
	{Name: "botticelli-venus", Weight: 1},
	{Name: "cassatt-letter", Weight: 1},
	{Name: "albers-tehuana", Weight: 1},
	{Name: "duchamp-landscape", Weight: 1},
	{Name: fromFlag, Weight: 0},
}

// colours resolves the colourway trait to the palette a render works from.
func colours(name string, fromCLI palette.Palette) (palette.Palette, error) {
	if name == fromFlag {
		return fromCLI, nil
	}
	p, ok := palette.ByName(name)
	if !ok {
		return palette.Palette{}, fmt.Errorf("pools: no palette %q", name)
	}
	return p, nil
}
