package iris

import "github.com/jaminalder/go-graphics/internal/trait"

// Config holds concrete numeric overrides and trait choices. Nil means omitted;
// an explicit value equal to a default still overrides seed-derived settings.
// Edition defaults and deterministic material streams belong to this sketch.
type Config struct {
	Traits     trait.Set `json:"traits"`
	Scale      *float64  `json:"scale,omitempty"`
	Octaves    *int      `json:"octaves,omitempty"`
	Gain       *float64  `json:"gain,omitempty"`
	Lacunarity *float64  `json:"lacunarity,omitempty"`
	Stretch    *float64  `json:"stretch,omitempty"`
	Twist      *float64  `json:"twist,omitempty"`
	Fiber      *float64  `json:"fiber,omitempty"`
	Radial     *float64  `json:"radial,omitempty"`
	Nested     *float64  `json:"nested,omitempty"`
	Depth      *float64  `json:"depth,omitempty"`
	Gleam      *float64  `json:"gleam,omitempty"`
	Warmth     *float64  `json:"warmth,omitempty"`
	Bands      *float64  `json:"bands,omitempty"`
	Cells      *float64  `json:"cells,omitempty"`
	RimWidth   *float64  `json:"rim-width,omitempty"`
	Limbus     *float64  `json:"limbus,omitempty"`
	Pupil      *float64  `json:"pupil,omitempty"`
}

// FromConfig validates configuration and returns a fresh independently owned sketch.
// CLI Configure and this adapter use the same knob and trait validators.
func FromConfig(c Config) (*Sketch, error) {
	s := New()
	if err := s.traits.SetOverrides(c.Traits); err != nil {
		return nil, err
	}
	var names []string
	if c.Scale != nil {
		s.Scale = *c.Scale
		names = append(names, "scale")
	}
	if c.Octaves != nil {
		s.Octaves = *c.Octaves
		names = append(names, "octaves")
	}
	if c.Gain != nil {
		s.Gain = *c.Gain
		names = append(names, "gain")
	}
	if c.Lacunarity != nil {
		s.Lacunarity = *c.Lacunarity
		names = append(names, "lacunarity")
	}
	if c.Stretch != nil {
		s.Stretch = *c.Stretch
		names = append(names, "stretch")
	}
	if c.Twist != nil {
		s.Twist = *c.Twist
		names = append(names, "twist")
	}
	if c.Fiber != nil {
		s.Fiber = *c.Fiber
		names = append(names, "fiber")
	}
	if c.Radial != nil {
		s.Radial = *c.Radial
		names = append(names, "radial")
	}
	if c.Nested != nil {
		s.Nested = *c.Nested
		names = append(names, "nested")
	}
	if c.Depth != nil {
		s.Depth = *c.Depth
		names = append(names, "depth")
	}
	if c.Gleam != nil {
		s.Gleam = *c.Gleam
		names = append(names, "gleam")
	}
	if c.Warmth != nil {
		s.Warmth = *c.Warmth
		names = append(names, "warmth")
	}
	if c.Bands != nil {
		s.Bands = *c.Bands
		names = append(names, "bands")
	}
	if c.Cells != nil {
		s.Cells = *c.Cells
		names = append(names, "cells")
	}
	if c.RimWidth != nil {
		s.RimWidth = *c.RimWidth
		names = append(names, "rim-width")
	}
	if c.Limbus != nil {
		s.Limbus = *c.Limbus
		names = append(names, "limbus")
	}
	if c.Pupil != nil {
		s.Pupil = *c.Pupil
		names = append(names, "pupil")
	}
	if _, err := s.knobs.Apply(names...); err != nil {
		return nil, err
	}
	return s, nil
}
