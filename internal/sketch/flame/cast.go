package flame

import (
	"fmt"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/trait"
)

const (
	dimCast  = "cast"
	fromFlag = "from-flag"
)

// Casts curated for flame: each must hold a cool end and a warm end under
// split tint without collapsing into magenta or green trenches. from-flag
// hands colour duty back to --palette and is never drawn from a seed.
var casts = []trait.Value{
	{Name: "zander-spindle", Weight: 6},
	{Name: "diebenkorn-seawall", Weight: 2},
	{Name: "hopper-night-windows", Weight: 1.5},
	{Name: "cezanne-bathers", Weight: 1.5},
	{Name: "davis-anthracite-minuet", Weight: 1},
	{Name: "bruegel-icarus", Weight: 1},
	{Name: fromFlag, Weight: 0},
}

func castPalette(name string, fromCLI palette.Palette) (palette.Palette, error) {
	if name == "" || name == fromFlag {
		return fromCLI, nil
	}
	p, ok := palette.ByName(name)
	if !ok {
		return palette.Palette{}, fmt.Errorf("flame: no palette %q", name)
	}
	return p, nil
}
