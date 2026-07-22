// Command reliefexp renders one fixed tapestry composition (hopper seed 66,
// no stripes) under every named relief preset, for side-by-side comparison.
// Run from the repo root:
//
//	go run ./tools/reliefexp [outdir]
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/tapestry"
)

func main() {
	outDir := "out/relief-exp"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	pal, ok := palette.ByName("hopper-night-windows")
	if !ok {
		log.Fatal("palette missing")
	}
	ctx := sketch.Context{Width: 2000, Height: 2000, Seed: 66, Palette: pal}

	for i, name := range tapestry.ReliefPresetNames() {
		params, _ := tapestry.ReliefPreset(name)
		s := tapestry.New()
		s.DisableStripes = true
		s.Relief = true
		s.ReliefParams = params

		img, err := s.Render(ctx)
		if err != nil {
			log.Fatal(err)
		}
		path := filepath.Join(outDir, fmt.Sprintf("relief_%02d-%s.png", i+1, name))
		if err := render.WritePNG(path, img); err != nil {
			log.Fatal(err)
		}
		fmt.Println(path)
	}
}
