package palette

import "sort"

// Palette is a named, ordered set of colors with provenance.
type Palette struct {
	Slug   string // canonical id, kebab-case (CLI name)
	Artist string
	Work   string
	Colors []Color
}

// Color returns the i-th color, wrapping around so any index is valid.
func (p Palette) Color(i int) Color {
	return p.Colors[((i%len(p.Colors))+len(p.Colors))%len(p.Colors)]
}

// Desaturated returns a derived palette with all colors desaturated by amount.
// The original palette data is never mutated.
func (p Palette) Desaturated(amount float64) Palette {
	d := Palette{
		Slug: p.Slug, Artist: p.Artist, Work: p.Work,
		Colors: make([]Color, len(p.Colors)),
	}
	for i, c := range p.Colors {
		d.Colors[i] = c.Desaturate(amount)
	}
	return d
}

// ByName looks up a palette by slug.
func ByName(slug string) (Palette, bool) {
	for _, p := range palettes {
		if p.Slug == slug {
			return p, true
		}
	}
	return Palette{}, false
}

// Names returns all palette slugs, sorted.
func Names() []string {
	names := make([]string, len(palettes))
	for i, p := range palettes {
		names[i] = p.Slug
	}
	sort.Strings(names)
	return names
}

// All returns all palettes in dataset order.
func All() []Palette {
	out := make([]Palette, len(palettes))
	copy(out, palettes)
	return out
}
