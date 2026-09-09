// Package publish is the explicit public edition and resource allowlist.
package publish

import (
	"bytes"
	"errors"
	"strconv"

	"github.com/jaminalder/go-graphics/internal/artwork"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/foam"
	"github.com/jaminalder/go-graphics/internal/sketch/iris"
	"github.com/jaminalder/go-graphics/internal/sketch/pools"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// Choice is a visual direction with explicit artistic pins.
type (
	Choice struct {
		ID, Name, Description string
		Pins                  trait.Set
	}
	// Colour is a curated palette interpreted by the artwork.
	Colour struct{ ID, Name string }
	// Entry describes one locally reviewable publication candidate.
	Entry struct {
		ID, Name, Description string
		Styles                []Choice
		Colours               []Colour
	}
)

// All returns independent catalogue values, never the local developer registry.
func All() []Entry {
	return []Entry{
		{ID: "pools", Name: "Pools", Description: "Transparent circles drift and overlap like pigment on paper.", Styles: []Choice{{"spacious", "Spacious circles", "Air and quiet between the circles.", trait.Set{"fill": "open", "arrange": "scatter"}}, {"flowing", "Flowing strands", "Circles follow a wandering current.", trait.Set{"fill": "busy", "arrange": "orbital"}}, {"gathered", "Gathered colour", "A fuller sheet of overlapping pools.", trait.Set{"fill": "packed"}}}, Colours: colours()},
		{ID: "foam", Name: "Foam", Description: "A field of inked cells, each carrying a different trace of colour.", Styles: []Choice{{"ink", "Ink and paper", "Fine lines and drawn marks.", trait.Set{"fills": "drawn", "line": "fine"}}, {"painted", "Painted cells", "Soft pigment settles inside each cell.", trait.Set{"fills": "watercolour", "line": "fine"}}, {"airy", "Open cells", "Quiet paper between passages of colour.", trait.Set{"fills": "airy", "density": "open"}}}, Colours: colours()},
		{ID: "iris", Name: "Iris", Description: "Fine fibres gather around a dark centre, winding out into light.", Styles: []Choice{{"fine", "Fine fibres", "Long threads radiate from the centre.", trait.Set{"structure": "fiber", "grain": "fine", "weave": "radial"}}, {"winding", "Winding fibres", "Threads turn softly around the pupil.", trait.Set{"structure": "fiber", "weave": "swirl"}}, {"layered", "Layered rings", "Bands of colour reveal a terraced iris.", trait.Set{"structure": "strata"}}}, Colours: colours()},
	}
}

func colours() []Colour {
	return []Colour{{"diebenkorn-seawall", "Tide"}, {"tchelitchew-hide-and-seek", "Earth"}, {"hopper-night-windows", "After dark"}, {"cezanne-bathers", "Meadow"}}
}

// Get looks up an explicitly admitted art form.
func Get(id string) (Entry, bool) {
	for _, e := range All() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Space returns the bounded public trait space; heavy experimental materials stay private.
func Space(id string) (trait.Schema, error) {
	e, ok := Get(id)
	if !ok {
		return nil, errors.New("artwork unavailable")
	}
	s, _ := artwork.Registry().Get(id)
	schema := s.(sketch.Traited).Schema().Boost(nil, 0)
	for i, d := range schema {
		var values []trait.Value
		for _, v := range d.Values {
			allow := v.Weight > 0
			if d.Name == "colourway" {
				allow = false
				for _, c := range e.Colours {
					if c.ID == v.Name {
						allow = true
					}
				}
				if allow {
					v.Weight = 1
				}
			}
			if id == "foam" && d.Name == "density" && (v.Name == "packed" || v.Name == "fine") {
				allow = false
			}
			if id == "foam" && d.Name == "fills" && v.Name == "watercolour" {
				allow = true
			}
			if allow {
				values = append(values, v)
			}
		}
		schema[i].Values = values
	}
	return schema, nil
}

// Pins resolves visual choices before recipe production.
func Pins(id, style, colour string) (trait.Set, string, error) {
	e, ok := Get(id)
	if !ok {
		return nil, "", errors.New("artwork unavailable")
	}
	pins := trait.Set{}
	if style != "" {
		found := false
		for _, s := range e.Styles {
			if s.ID == style {
				found = true
				for k, v := range s.Pins {
					pins[k] = v
				}
			}
		}
		if !found {
			return nil, "", errors.New("unknown style")
		}
	}
	if colour != "" {
		found := false
		for _, c := range e.Colours {
			if c.ID == colour {
				found = true
			}
		}
		if !found {
			return nil, "", errors.New("unknown colour")
		}
		if id != "iris" {
			pins["colourway"] = colour
		}
	}
	return pins, colour, nil
}

// Complete turns permitted traits into an edition recipe with no public numeric overrides.
func Complete(id string, seed uint64, pal string, set trait.Set) (artwork.Recipe, error) {
	if pal == "" {
		pal = "diebenkorn-seawall"
	}
	if c := set["colourway"]; c != "" {
		pal = c
	}
	switch id {
	case "pools":
		return artwork.New(id, seed, pal, pools.Config{Traits: set})
	case "foam":
		return artwork.New(id, seed, pal, foam.Config{Traits: set})
	case "iris":
		return artwork.New(id, seed, pal, iris.Config{Traits: set})
	}
	return artwork.Recipe{}, errors.New("artwork unavailable")
}

// Validate applies the same strict public policy to restore, HTTP and renderer input.
func Validate(raw []byte) (artwork.Recipe, error) {
	r, err := artwork.Decode(raw)
	if err != nil {
		return r, err
	}
	s, err := Space(r.ID())
	if err != nil {
		return r, err
	}
	set := r.Traits()
	for k, v := range set {
		d, ok := s.Dim(k)
		if !ok || !d.Has(v) {
			return r, errors.New("choice unavailable")
		}
	}
	e, _ := Get(r.ID())
	allowed := false
	for _, c := range e.Colours {
		if c.ID == r.Palette() {
			allowed = true
		}
	}
	if !allowed {
		return r, errors.New("colour unavailable")
	}
	seed, _ := strconv.ParseUint(r.Seed(), 10, 64)
	clean, err := Complete(r.ID(), seed, r.Palette(), set)
	if err != nil {
		return r, err
	}
	if !bytes.Equal(clean.Bytes(), r.Bytes()) {
		return r, errors.New("numeric overrides are private")
	}
	return r, nil
}

// Tier returns fixed per-artwork pixel and sampling limits.
func Tier(id, name string) (artwork.Rendition, error) {
	if _, ok := Get(id); !ok {
		return artwork.Rendition{}, errors.New("artwork unavailable")
	}
	size := 600
	if name == "download" {
		size = 1200
	} else if name != "preview" {
		return artwork.Rendition{}, errors.New("rendition unavailable")
	}
	aa := 1
	return artwork.Rendition{Width: size, Height: size, AA: aa, Format: "png"}, nil
}

// Check validates all catalogue choices at startup without rendering.
func Check() error {
	for _, e := range All() {
		s, err := Space(e.ID)
		if err != nil {
			return err
		}
		if err := s.Validate(); err != nil {
			return err
		}
		for _, style := range e.Styles {
			for k, v := range style.Pins {
				d, ok := s.Dim(k)
				if !ok || !d.Has(v) {
					return errors.New("invalid catalogue style " + e.ID + "/" + style.ID)
				}
			}
		}
	}
	return nil
}
