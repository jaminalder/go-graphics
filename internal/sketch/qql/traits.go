package qql

import "github.com/jaminalder/go-graphics/internal/trait"

// Dimension names. They double as CLI flag names, so they share one flat
// namespace with the generic render flags — hence qql-palette rather than
// the shorter name, which --palette already owns.
const (
	dimFlowField     = "flow-field"
	dimTurbulence    = "turbulence"
	dimMargin        = "margin"
	dimColorVariety  = "color-variety"
	dimColorMode     = "color-mode"
	dimStructure     = "structure"
	dimBullseye      = "bullseye"
	dimRingThickness = "ring-thickness"
	dimRingSize      = "ring-size"
	dimSizeVariety   = "size-variety"
	dimPalette       = "qql-palette"
	dimSpacing       = "spacing"
	dimMedium        = "medium"
)

// mediumWash is the medium value that paints every band as a pool of
// watercolour instead of a stack of opaque strokes. It carries weight 0,
// so a seed never lands on it: QQL is an ink piece unless asked otherwise,
// and the medium is a choice about the material, not a trait of the work.
const mediumWash = "wash"

// paletteExternal is the qql-palette value that hands colour duty to
// whatever --palette names. It carries weight 0, so a seed never lands on
// it: driving QQL from a ColorLisa palette is always a deliberate act.
const paletteExternal = "colorlisa"

// schema is QQL's output space: twelve orthogonal dimensions, each adding
// an axis rather than restricting the others. The weights are QQL's own
// rarity table — spiral fields and small rings common, explosive fields and
// large rings rare — which is what gives the space a quality floor with
// genuine outliers instead of uniform noise.
//
// The order is stable and fixes the draw order; append, never insert.
var schema = trait.Schema{
	{
		Name: dimFlowField, Key: "ff",
		Doc: "shape of the field the dots follow",
		Values: []trait.Value{
			{Name: "horizontal", Weight: 3},
			{Name: "diagonal", Weight: 1},
			{Name: "vertical", Weight: 3},
			{Name: "random-linear", Weight: 1},
			{Name: "explosive", Weight: 1},
			{Name: "spiral", Weight: 4},
			{Name: "circular", Weight: 2},
			{Name: "random-radial", Weight: 1},
		},
	},
	{
		Name: dimTurbulence, Key: "tb",
		Doc: "how much the field is disturbed",
		// The source carries weight 0 for "none" because the mint derives
		// traits by masking seed bits and ignores the weights entirely —
		// which in practice produced "none" about half the time. Honouring
		// the weights here needs a real number, so "none" gets 2: common
		// enough to keep the clean fields QQL is known for, damped so that
		// "low" stays the default character. (Decision log #17.)
		Values: []trait.Value{
			{Name: "none", Weight: 2},
			{Name: "low", Weight: 3},
			{Name: "high", Weight: 1},
		},
	},
	{
		Name: dimMargin, Key: "mg",
		Doc: "how much ground is left around the marks",
		Values: []trait.Value{
			{Name: "none", Weight: 1},
			{Name: "crisp", Weight: 1},
			{Name: "wide", Weight: 2},
		},
	},
	{
		Name: dimColorVariety, Key: "cv",
		Doc: "how many colours are in play",
		Values: []trait.Value{
			{Name: "low", Weight: 2},
			{Name: "medium", Weight: 4},
			{Name: "high", Weight: 3},
		},
	},
	{
		Name: dimColorMode, Key: "cm",
		Doc: "how the two colours of a dot are used",
		Values: []trait.Value{
			{Name: "simple", Weight: 2},
			{Name: "stacked", Weight: 3},
			{Name: "zebra", Weight: 1},
		},
	},
	{
		Name: dimStructure, Key: "str",
		Doc: "how the flow lines are seeded",
		Values: []trait.Value{
			{Name: "orbital", Weight: 1},
			{Name: "formation", Weight: 1},
			{Name: "shadows", Weight: 1},
		},
	},
	{
		Name: dimBullseye, Key: "be",
		Doc: "which ring counts a dot may be built from",
		// The source draws three independent coin flips (may a dot have 1,
		// 3 or 7 rings?); the eight equally likely outcomes are spelled out
		// here so the dimension reads as one choice on the command line.
		Values: []trait.Value{
			{Name: "none", Weight: 1},
			{Name: "1", Weight: 1},
			{Name: "3", Weight: 1},
			{Name: "7", Weight: 1},
			{Name: "1-3", Weight: 1},
			{Name: "1-7", Weight: 1},
			{Name: "3-7", Weight: 1},
			{Name: "1-3-7", Weight: 1},
		},
	},
	{
		Name: dimRingThickness, Key: "rt",
		Doc: "how much of a dot is ink rather than gap",
		Values: []trait.Value{
			{Name: "thin", Weight: 1},
			{Name: "thick", Weight: 2},
			{Name: "mixed", Weight: 2},
		},
	},
	{
		Name: dimRingSize, Key: "rs",
		Doc: "how big the dots are",
		Values: []trait.Value{
			{Name: "small", Weight: 4},
			{Name: "medium", Weight: 3},
			{Name: "large", Weight: 1},
		},
	},
	{
		Name: dimSizeVariety, Key: "sv",
		Doc: "how much dot size changes across the piece",
		Values: []trait.Value{
			{Name: "constant", Weight: 1},
			{Name: "variable", Weight: 3},
			{Name: "wild", Weight: 1},
		},
	},
	{
		Name: dimPalette, Key: "p", InName: true,
		Doc: "colour set",
		Values: []trait.Value{
			{Name: "austin", Weight: 1},
			{Name: "berlin", Weight: 1},
			{Name: "edinburgh", Weight: 2},
			{Name: "fidenza", Weight: 2},
			{Name: "miami", Weight: 1},
			{Name: "seattle", Weight: 1},
			{Name: "seoul", Weight: 2},
			{Name: paletteExternal, Weight: 0},
		},
	},
	{
		Name: dimSpacing, Key: "sp",
		Doc: "how tightly the dots pack",
		Values: []trait.Value{
			{Name: "dense", Weight: 2},
			{Name: "medium", Weight: 1},
			{Name: "sparse", Weight: 1},
		},
	},
	// Appended last, and deliberately so: the twelve dimensions above are
	// QQL's own output space, and inserting into it would move every
	// existing seed onto a different piece. A thirteenth is safe at the
	// end, and with the wash at weight 0 no seed's draw changes at all —
	// the medium is reachable only by asking for it, and shows up in the
	// filename when you do because it was overridden.
	{
		Name: dimMedium, Key: "md",
		Doc: "what the bands are painted with",
		Values: []trait.Value{
			{Name: "ink", Weight: 1},
			{Name: mediumWash, Weight: 0},
		},
	},
}

// bullseyeRings turns a bullseye trait value into the ring counts a dot may
// be built from. An empty selection falls back to two rings, as the source
// does, so no combination can produce dots with no rings at all.
func bullseyeRings(value string) []int {
	var out []int
	for _, r := range []struct {
		n     int
		mark  string
		inAll bool
	}{{1, "1", true}, {3, "3", true}, {7, "7", true}} {
		if hasRing(value, r.mark) {
			out = append(out, r.n)
		}
	}
	if len(out) == 0 {
		return []int{2}
	}
	return out
}

func hasRing(value, mark string) bool {
	for i := 0; i < len(value); i++ {
		if string(value[i]) != mark {
			continue
		}
		// Only a whole segment counts: "1-3" holds 1 and 3, not 13.
		if (i == 0 || value[i-1] == '-') && (i == len(value)-1 || value[i+1] == '-') {
			return true
		}
	}
	return false
}
