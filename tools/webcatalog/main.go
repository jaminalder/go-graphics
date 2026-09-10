// Command webcatalog builds deliberate catalogue assets and their reproducible manifest.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jaminalder/go-graphics/internal/artwork"
	"github.com/jaminalder/go-graphics/internal/explore"
	"github.com/jaminalder/go-graphics/internal/publish"
)

type example struct {
	File       string          `json:"file"`
	Recipe     json.RawMessage `json:"recipe"`
	SHA256     string          `json:"sha256"`
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Alt        string          `json:"alt"`
	Provenance string          `json:"provenance"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := os.MkdirAll("out/catalog", 0o755); err != nil {
		return err
	}
	var manifest []example
	for _, entry := range publish.All() {
		space, err := publish.Space(entry.ID)
		if err != nil {
			return err
		}
		type choice struct{ name, style, colour string }
		heroColour := map[string]string{"pools": "diebenkorn-seawall", "foam": "hopper-night-windows", "iris": "tchelitchew-hide-and-seek"}[entry.ID]
		choices := []choice{{"hero", entry.Styles[0].ID, heroColour}}
		for _, s := range entry.Styles {
			choices = append(choices, choice{s.ID, s.ID, entry.Colours[0].ID})
		}
		for _, c := range entry.Colours {
			choices = append(choices, choice{c.ID, entry.Styles[0].ID, c.ID})
		}
		for _, c := range choices {
			pins, pal, err := publish.Pins(entry.ID, c.style, c.colour)
			if err != nil {
				return err
			}
			batch, err := explore.Public(space, pins, nil, 42, 0, nil)
			if err != nil {
				return err
			}
			r, err := publish.Complete(entry.ID, batch[0].Seed, pal, batch[0].Traits)
			if err != nil {
				return err
			}
			size := 300
			if c.name == "hero" {
				size = 600
			}
			tier := artwork.Rendition{Width: size, Height: size, AA: 1, Format: "png"}
			var b bytes.Buffer
			if err := r.Render(&b, tier, "catalogue-edition-1"); err != nil {
				return err
			}
			name := entry.ID + "-" + c.name + ".png"
			if err := os.WriteFile(filepath.Join("out/catalog", name), b.Bytes(), 0o644); err != nil {
				return err
			}
			h := sha256.Sum256(b.Bytes())
			manifest = append(manifest, example{name, r.Bytes(), hex.EncodeToString(h[:]), size, size, entry.Description, "go-graphics edition 1; repository ColorLisa palette " + r.Palette() + "; provisional owner review"})
			fmt.Println(name)
		}
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("out/catalog/manifest.json", b, 0o644)
}
