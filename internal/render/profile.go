package render

import (
	"fmt"
	"sort"
)

// Profile is a named output size.
type Profile struct {
	Name          string
	Width, Height int
}

// Square default profiles (see docs/ARCHITECTURE.md §5).
var profiles = map[string]Profile{
	"preview": {Name: "preview", Width: 600, Height: 600},
	"web":     {Name: "web", Width: 2000, Height: 2000},
	"print":   {Name: "print", Width: 6000, Height: 6000}, // ≈50×50 cm @ 300 DPI

	// Portrait companions: a 4:5 frame reads very differently from a
	// square for all-over compositions, so it is worth having as a size
	// rather than as a per-sketch knob.
	"preview-tall": {Name: "preview-tall", Width: 480, Height: 600},
	"web-tall":     {Name: "web-tall", Width: 1600, Height: 2000},
	"print-tall":   {Name: "print-tall", Width: 4800, Height: 6000},
}

// ProfileByName looks up a size profile.
func ProfileByName(name string) (Profile, error) {
	p, ok := profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("render: unknown profile %q (have %v)", name, ProfileNames())
	}
	return p, nil
}

// ProfileNames returns the available profile names, sorted.
func ProfileNames() []string {
	names := make([]string, 0, len(profiles))
	for n := range profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
