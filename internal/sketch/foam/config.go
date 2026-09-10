package foam

import "github.com/jaminalder/go-graphics/internal/trait"

// Config holds concrete numeric overrides and trait choices. Nil means omitted;
// an explicit value equal to a default still overrides seed-derived settings.
// Edition defaults and deterministic material streams belong to this sketch.
type Config struct {
	Traits      trait.Set `json:"traits"`
	Tile        *float64  `json:"tile,omitempty"`
	Tiled       *float64  `json:"tiled,omitempty"`
	Fine        *float64  `json:"fine,omitempty"`
	Spread      *float64  `json:"spread,omitempty"`
	Depth       *float64  `json:"depth,omitempty"`
	Bevel       *float64  `json:"bevel,omitempty"`
	Light       *float64  `json:"light,omitempty"`
	Count       *int      `json:"count,omitempty"`
	Rungs       *int      `json:"rungs,omitempty"`
	Base        *float64  `json:"base,omitempty"`
	Ratio       *float64  `json:"ratio,omitempty"`
	Gap         *float64  `json:"gap,omitempty"`
	Over        *float64  `json:"over,omitempty"`
	Merge       *float64  `json:"merge,omitempty"`
	MaxLobe     *int      `json:"max-lobe,omitempty"`
	Warp        *float64  `json:"warp,omitempty"`
	Swirl       *float64  `json:"swirl,omitempty"`
	Ink         *float64  `json:"ink,omitempty"`
	Swell       *float64  `json:"swell,omitempty"`
	Node        *float64  `json:"node,omitempty"`
	Round       *float64  `json:"round,omitempty"`
	Wash        *float64  `json:"wash,omitempty"`
	Pencil      *float64  `json:"pencil,omitempty"`
	Bands       *float64  `json:"bands,omitempty"`
	Hatch       *float64  `json:"hatch,omitempty"`
	Empty       *float64  `json:"empty,omitempty"`
	Load        *float64  `json:"load,omitempty"`
	Pool        *float64  `json:"pool,omitempty"`
	Uneven      *float64  `json:"uneven,omitempty"`
	Dry         *float64  `json:"dry,omitempty"`
	Hatching    *string   `json:"hatching,omitempty"`
	HatchPress  *float64  `json:"hatch-press,omitempty"`
	HatchFit    *int      `json:"hatch-fit,omitempty"`
	HatchPitch  *float64  `json:"hatch-pitch,omitempty"`
	HatchVary   *float64  `json:"hatch-vary,omitempty"`
	HatchWeight *float64  `json:"hatch-weight,omitempty"`
	Weight      *float64  `json:"weight,omitempty"`
	Wobble      *float64  `json:"wobble,omitempty"`
	Rim         *float64  `json:"rim,omitempty"`
	RimWidth    *float64  `json:"rim-width,omitempty"`
	Mottle      *float64  `json:"mottle,omitempty"`
	Blotch      *float64  `json:"blotch,omitempty"`
	Grain       *float64  `json:"grain,omitempty"`
	Accent      *float64  `json:"accent,omitempty"`
	Passage     *float64  `json:"passage,omitempty"`
	Saturate    *float64  `json:"saturate,omitempty"`
	Shades      *float64  `json:"shades,omitempty"`
	Flat        *float64  `json:"flat,omitempty"`
	Stroke      *float64  `json:"stroke,omitempty"`
}

// FromConfig validates configuration and returns a fresh independently owned sketch.
// CLI Configure and this adapter use the same knob and trait validators.
func FromConfig(c Config) (*Sketch, error) {
	s := New()
	if err := s.traits.SetOverrides(c.Traits); err != nil {
		return nil, err
	}
	var names []string
	if c.Tile != nil {
		s.pin.sub.tile = *c.Tile
		names = append(names, "tile")
	}
	if c.Tiled != nil {
		s.pin.sub.share = *c.Tiled
		names = append(names, "tiled")
	}
	if c.Fine != nil {
		s.pin.sub.fine = *c.Fine
		names = append(names, "fine")
	}
	if c.Spread != nil {
		s.pin.sub.spread = *c.Spread
		names = append(names, "spread")
	}
	if c.Depth != nil {
		s.Depth = *c.Depth
		names = append(names, "depth")
	}
	if c.Bevel != nil {
		s.Bevel = *c.Bevel
		names = append(names, "bevel")
	}
	if c.Light != nil {
		s.Light = *c.Light
		names = append(names, "light")
	}
	if c.Count != nil {
		s.pin.count = *c.Count
		names = append(names, "count")
	}
	if c.Rungs != nil {
		s.pin.rungs = *c.Rungs
		names = append(names, "rungs")
	}
	if c.Base != nil {
		s.pin.base = *c.Base
		names = append(names, "base")
	}
	if c.Ratio != nil {
		s.pin.ratio = *c.Ratio
		names = append(names, "ratio")
	}
	if c.Gap != nil {
		s.pin.gap = *c.Gap
		names = append(names, "gap")
	}
	if c.Over != nil {
		s.pin.over = *c.Over
		names = append(names, "over")
	}
	if c.Merge != nil {
		s.pin.merge = *c.Merge
		names = append(names, "merge")
	}
	if c.MaxLobe != nil {
		s.pin.maxLobe = *c.MaxLobe
		names = append(names, "max-lobe")
	}
	if c.Warp != nil {
		s.pin.warp = *c.Warp
		names = append(names, "warp")
	}
	if c.Swirl != nil {
		s.pin.swirl = *c.Swirl
		names = append(names, "swirl")
	}
	if c.Ink != nil {
		s.pin.ink = *c.Ink
		names = append(names, "ink")
	}
	if c.Swell != nil {
		s.pin.swell = *c.Swell
		names = append(names, "swell")
	}
	if c.Node != nil {
		s.pin.node = *c.Node
		names = append(names, "node")
	}
	if c.Round != nil {
		s.pin.round = *c.Round
		names = append(names, "round")
	}
	if c.Wash != nil {
		s.pin.styles[styleWash] = *c.Wash
		names = append(names, "wash")
	}
	if c.Pencil != nil {
		s.pin.styles[stylePencil] = *c.Pencil
		names = append(names, "pencil")
	}
	if c.Bands != nil {
		s.pin.styles[styleBands] = *c.Bands
		names = append(names, "bands")
	}
	if c.Hatch != nil {
		s.pin.styles[styleHatch] = *c.Hatch
		names = append(names, "hatch")
	}
	if c.Empty != nil {
		s.pin.styles[styleEmpty] = *c.Empty
		names = append(names, "empty")
	}
	if c.Load != nil {
		s.Load = *c.Load
		names = append(names, "load")
	}
	if c.Pool != nil {
		s.Pool = *c.Pool
		names = append(names, "pool")
	}
	if c.Uneven != nil {
		s.Uneven = *c.Uneven
		names = append(names, "uneven")
	}
	if c.Dry != nil {
		s.Dry = *c.Dry
		names = append(names, "dry")
	}
	if c.Hatching != nil {
		s.Look = *c.Hatching
		names = append(names, "hatching")
	}
	if c.HatchPress != nil {
		s.Hatching = *c.HatchPress
		names = append(names, "hatch-press")
	}
	if c.HatchFit != nil {
		s.HatchFit = *c.HatchFit
		names = append(names, "hatch-fit")
	}
	if c.HatchPitch != nil {
		s.HatchPitch = *c.HatchPitch
		names = append(names, "hatch-pitch")
	}
	if c.HatchVary != nil {
		s.HatchVary = *c.HatchVary
		names = append(names, "hatch-vary")
	}
	if c.HatchWeight != nil {
		s.HatchWeight = *c.HatchWeight
		names = append(names, "hatch-weight")
	}
	if c.Weight != nil {
		s.Weight = *c.Weight
		names = append(names, "weight")
	}
	if c.Wobble != nil {
		s.Wobble = *c.Wobble
		names = append(names, "wobble")
	}
	if c.Rim != nil {
		s.Rim = *c.Rim
		names = append(names, "rim")
	}
	if c.RimWidth != nil {
		s.RimWide = *c.RimWidth
		names = append(names, "rim-width")
	}
	if c.Mottle != nil {
		s.Mottle = *c.Mottle
		names = append(names, "mottle")
	}
	if c.Blotch != nil {
		s.Blotch = *c.Blotch
		names = append(names, "blotch")
	}
	if c.Grain != nil {
		s.Grain = *c.Grain
		names = append(names, "grain")
	}
	if c.Accent != nil {
		s.Accent = *c.Accent
		names = append(names, "accent")
	}
	if c.Passage != nil {
		s.Passage = *c.Passage
		names = append(names, "passage")
	}
	if c.Saturate != nil {
		s.Sat = *c.Saturate
		names = append(names, "saturate")
	}
	if c.Shades != nil {
		s.Shades = *c.Shades
		names = append(names, "shades")
	}
	if c.Flat != nil {
		s.Flat = *c.Flat
		names = append(names, "flat")
	}
	if c.Stroke != nil {
		s.Stroke = *c.Stroke
		names = append(names, "stroke")
	}
	if _, err := s.knobs.Apply(names...); err != nil {
		return nil, err
	}
	return s, nil
}
