// Package sketch defines the contract every artwork implements and the
// context it renders with. One subpackage per sketch.
package sketch

import (
	"fmt"
	"image"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
)

// Context carries everything a sketch may depend on. All randomness must
// come from RNG(stream) so output is fully determined by Seed (invariant 1
// in docs/ARCHITECTURE.md).
type Context struct {
	Width, Height int
	Seed          uint64
	Palette       palette.Palette

	// AA is the supersampling factor per axis (0 or 1 = off). Purely a
	// quality setting: it never changes the composition.
	AA int

	// Deep renders to 16-bit (archival/print master, PNG only).
	Deep bool
}

// Samples returns the effective supersampling factor (≥ 1).
func (c Context) Samples() int {
	if c.AA < 1 {
		return 1
	}
	return c.AA
}

// RNG returns a generator derived from the seed and a caller-chosen stream
// number. Distinct streams are independent, so adding a new consumer never
// disturbs the values an existing stream produces.
func (c Context) RNG(stream uint64) *rand.Rand {
	return rand.New(rand.NewPCG(c.Seed, stream))
}

// Raster renders f according to the Context's quality settings (AA
// supersampling, 8- vs 16-bit). Sketches should use this instead of
// calling the render package directly.
func Raster(ctx Context, f render.PixelFunc) image.Image {
	if ctx.Deep {
		return render.RasterDeep(ctx.Width, ctx.Height, ctx.Samples(), f)
	}
	return render.RasterSS(ctx.Width, ctx.Height, ctx.Samples(), f)
}

// Sketch is one artwork algorithm: a deterministic function of
// (parameters, Context) to an image.
type Sketch interface {
	Name() string     // CLI id, kebab-case
	Describe() string // one line for `staticart list`
	Render(ctx Context) (image.Image, error)
}

// Registry is an explicit, immutable-after-construction set of sketches.
type Registry struct {
	byName map[string]Sketch
}

// NewRegistry builds a registry; duplicate names are a programming error.
func NewRegistry(sketches ...Sketch) *Registry {
	r := &Registry{byName: make(map[string]Sketch, len(sketches))}
	for _, s := range sketches {
		if _, dup := r.byName[s.Name()]; dup {
			panic(fmt.Sprintf("sketch: duplicate name %q", s.Name()))
		}
		r.byName[s.Name()] = s
	}
	return r
}

// Get looks a sketch up by name.
func (r *Registry) Get(name string) (Sketch, bool) {
	s, ok := r.byName[name]
	return s, ok
}

// All returns the sketches sorted by name.
func (r *Registry) All() []Sketch {
	out := make([]Sketch, 0, len(r.byName))
	for _, s := range r.byName {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}
