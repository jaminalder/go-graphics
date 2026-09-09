package pools

import "github.com/jaminalder/go-graphics/internal/trait"

// Config holds concrete numeric overrides and trait choices. Nil means omitted;
// an explicit value equal to a default still overrides seed-derived settings.
// Edition defaults and deterministic material streams belong to this sketch.
type Config struct {
	Traits       trait.Set `json:"traits"`
	Count        *int      `json:"count,omitempty"`
	Rungs        *int      `json:"rungs,omitempty"`
	Base         *float64  `json:"base,omitempty"`
	Ratio        *float64  `json:"ratio,omitempty"`
	Satellites   *float64  `json:"satellites,omitempty"`
	Gap          *float64  `json:"gap,omitempty"`
	Margin       *float64  `json:"margin,omitempty"`
	Ragged       *float64  `json:"ragged,omitempty"`
	Rings        *float64  `json:"rings,omitempty"`
	Open         *float64  `json:"open,omitempty"`
	Glaze        *float64  `json:"glaze,omitempty"`
	Banded       *float64  `json:"banded,omitempty"`
	BandWidth    *float64  `json:"band-width,omitempty"`
	BandOverlap  *float64  `json:"band-overlap,omitempty"`
	MaxBands     *int      `json:"max-bands,omitempty"`
	Alpha        *float64  `json:"alpha,omitempty"`
	Pigments     *int      `json:"pigments,omitempty"`
	Ground       *float64  `json:"ground,omitempty"`
	GroundBlotch *float64  `json:"ground-blotch,omitempty"`
}

// FromConfig validates configuration and returns a fresh independently owned sketch.
// CLI Configure and this adapter use the same knob and trait validators.
func FromConfig(c Config) (*Sketch, error) {
	s := New()
	if err := s.traits.SetOverrides(c.Traits); err != nil {
		return nil, err
	}
	var names []string
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
	if c.Satellites != nil {
		s.pin.satellites = *c.Satellites
		names = append(names, "satellites")
	}
	if c.Gap != nil {
		s.pin.gap = *c.Gap
		names = append(names, "gap")
	}
	if c.Margin != nil {
		s.pin.margin = *c.Margin
		names = append(names, "margin")
	}
	if c.Ragged != nil {
		s.Ragged = *c.Ragged
		names = append(names, "ragged")
	}
	if c.Rings != nil {
		s.Rings = *c.Rings
		names = append(names, "rings")
	}
	if c.Open != nil {
		s.Open = *c.Open
		names = append(names, "open")
	}
	if c.Glaze != nil {
		s.Glaze = *c.Glaze
		names = append(names, "glaze")
	}
	if c.Banded != nil {
		s.Banded = *c.Banded
		names = append(names, "banded")
	}
	if c.BandWidth != nil {
		s.BandWidth = *c.BandWidth
		names = append(names, "band-width")
	}
	if c.BandOverlap != nil {
		s.BandOverlap = *c.BandOverlap
		names = append(names, "band-overlap")
	}
	if c.MaxBands != nil {
		s.MaxBands = *c.MaxBands
		names = append(names, "max-bands")
	}
	if c.Alpha != nil {
		s.Alpha = *c.Alpha
		names = append(names, "alpha")
	}
	if c.Pigments != nil {
		s.Pigments = *c.Pigments
		names = append(names, "pigments")
	}
	if c.Ground != nil {
		s.Ground = *c.Ground
		names = append(names, "ground")
	}
	if c.GroundBlotch != nil {
		s.GroundBlotch = *c.GroundBlotch
		names = append(names, "ground-blotch")
	}
	if _, err := s.knobs.Apply(names...); err != nil {
		return nil, err
	}
	return s, nil
}
