package artwork

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"strconv"
	"strings"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/foam"
	"github.com/jaminalder/go-graphics/internal/sketch/iris"
	"github.com/jaminalder/go-graphics/internal/sketch/pools"
	"github.com/jaminalder/go-graphics/internal/trait"
)

const maxRecipe = 16384

// Recipe is an immutable validated edition record. Its encoding owns all data.
type (
	Recipe struct {
		canonical     string
		id, seed, pal string
		traits        trait.Set
	}
	envelope struct {
		Version int             `json:"recipe_version"`
		ID      string          `json:"artwork_id"`
		Edition string          `json:"edition"`
		Seed    string          `json:"seed"`
		Palette string          `json:"palette"`
		Config  json.RawMessage `json:"config"`
	}
)

// StrictJSON rejects ambiguous keys, trailing data, excessive nesting and unknown fields.
func StrictJSON(data []byte, dst any) error {
	if len(data) > 65536 {
		return errors.New("payload too large")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := scanJSON(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(dst)
}

func scanJSON(d *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("JSON nesting too deep")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return errors.New("unexpected JSON delimiter")
	}
	seen := map[string]bool{}
	n := 0
	for d.More() {
		n++
		if n > 256 {
			return errors.New("too many JSON members")
		}
		if delim == '{' {
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || key != strings.ToLower(key) || seen[key] {
				return errors.New("duplicate JSON key")
			}
			seen[key] = true
		}
		if err := scanJSON(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

// New completes an edition recipe from concrete sketch configuration.
func New(id string, seed uint64, pal string, config any) (Recipe, error) {
	c, err := json.Marshal(config)
	if err != nil {
		return Recipe{}, err
	}
	b, err := json.Marshal(envelope{1, id, "1", strconv.FormatUint(seed, 10), pal, c})
	if err != nil {
		return Recipe{}, err
	}
	return Decode(b)
}

// Decode validates and canonicalizes one supported edition; unknown editions fail closed.
func Decode(data []byte) (Recipe, error) {
	if len(data) > maxRecipe {
		return Recipe{}, errors.New("recipe too large")
	}
	var e envelope
	if err := StrictJSON(data, &e); err != nil {
		return Recipe{}, err
	}
	if e.Version != 1 || e.Edition != "1" {
		return Recipe{}, errors.New("unsupported recipe edition")
	}
	seed, err := strconv.ParseUint(e.Seed, 10, 64)
	if err != nil {
		return Recipe{}, errors.New("seed must be an unsigned decimal string")
	}
	e.Seed = strconv.FormatUint(seed, 10)
	if _, ok := palette.ByName(e.Palette); !ok {
		return Recipe{}, errors.New("unknown palette")
	}
	s, c, err := configured(e.ID, e.Config, seed)
	if err != nil {
		return Recipe{}, err
	}
	if colour := s.Traits(sketch.Context{Seed: seed})["colourway"]; colour != "" && colour != "from-flag" {
		e.Palette = colour
	}
	e.Config, err = json.Marshal(c)
	if err != nil {
		return Recipe{}, err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return Recipe{}, err
	}
	return Recipe{string(b), e.ID, e.Seed, e.Palette, s.Traits(sketch.Context{Seed: seed})}, nil
}

func configured(id string, raw []byte, seed uint64) (sketch.Traited, any, error) {
	ctx := sketch.Context{Seed: seed}
	switch id {
	case "pools":
		var c pools.Config
		if err := StrictJSON(raw, &c); err != nil {
			return nil, nil, err
		}
		s, err := pools.FromConfig(c)
		if err != nil {
			return nil, nil, err
		}
		c.Traits = s.Traits(ctx)
		return s, c, nil
	case "foam":
		var c foam.Config
		if err := StrictJSON(raw, &c); err != nil {
			return nil, nil, err
		}
		s, err := foam.FromConfig(c)
		if err != nil {
			return nil, nil, err
		}
		c.Traits = s.Traits(ctx)
		return s, c, nil
	case "iris":
		var c iris.Config
		if err := StrictJSON(raw, &c); err != nil {
			return nil, nil, err
		}
		s, err := iris.FromConfig(c)
		if err != nil {
			return nil, nil, err
		}
		c.Traits = s.Traits(ctx)
		return s, c, nil
	default:
		return nil, nil, errors.New("unsupported artwork")
	}
}

// Bytes returns an owned copy of the canonical record.
func (r Recipe) Bytes() []byte { return []byte(r.canonical) }

// ID returns the artwork identity.
func (r Recipe) ID() string { return r.id }

// Seed returns the full precision seed string.
func (r Recipe) Seed() string { return r.seed }

// Palette returns the actual palette identity.
func (r Recipe) Palette() string { return r.pal }

// Traits returns a copy of the complete resolved choices.
func (r Recipe) Traits() trait.Set {
	out := trait.Set{}
	for k, v := range r.traits {
		out[k] = v
	}
	return out
}

// Digest returns the recipe identity.
func (r Recipe) Digest() string { return digest(r.Bytes()) }
func digest(b []byte) string    { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Rendition is server-owned image encoding and sampling policy.
type Rendition struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	AA     int    `json:"aa"`
	Format string `json:"format"`
}

// Key binds the recipe, rendition and immutable renderer release.
func (r Recipe) Key(t Rendition, build string) string {
	b, _ := json.Marshal(t)
	return digest([]byte(r.canonical + string(b) + build))
}

// Render creates a fresh sketch and encodes it with deterministic recipe metadata.
func (r Recipe) Render(w io.Writer, t Rendition, build string) error {
	if t.Width < 1 || t.Height < 1 || t.Width > 1200 || t.Height > 1200 || t.AA < 1 || t.AA > 2 || t.Format != "png" {
		return errors.New("invalid rendition")
	}
	var e envelope
	if err := StrictJSON(r.Bytes(), &e); err != nil {
		return err
	}
	seed, _ := strconv.ParseUint(e.Seed, 10, 64)
	s, _, err := configured(e.ID, e.Config, seed)
	if err != nil {
		return err
	}
	pal, ok := palette.ByName(e.Palette)
	if !ok {
		return errors.New("unknown palette")
	}
	var img image.Image
	img, err = s.Render(sketch.Context{Width: t.Width, Height: t.Height, Seed: seed, Palette: pal, AA: t.AA})
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return render.EncodePNGMeta(w, img, render.Meta{Software: "artwork " + build, Comment: r.canonical})
}
