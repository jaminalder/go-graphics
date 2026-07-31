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

// ByLuminance copies a palette's colours into darkest-first order. Nearly
// every sketch needs this — the darkest colour is the one that reads as
// ink and the lightest as paper — so it lives here rather than being
// re-sorted in each of them.
func ByLuminance(colors []Color) []Color {
	out := append([]Color(nil), colors...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Luminance() < out[j].Luminance() })
	return out
}
