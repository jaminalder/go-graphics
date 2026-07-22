// Command reliefexp renders one fixed tapestry composition (hopper seed 66,
// no stripes) under a set of named relief lighting/shading presets, for
// side-by-side comparison. Run from the repo root:
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

type variant struct {
	name string
	p    tapestry.ReliefParams
}

func variants() []variant {
	base := tapestry.DefaultReliefParams()

	v := func(name string, mod func(*tapestry.ReliefParams)) variant {
		p := base
		mod(&p)
		return variant{name, p}
	}

	return []variant{
		{"01-baseline", base},
		v("02-low-sun-west", func(p *tapestry.ReliefParams) {
			// Late-afternoon raking light from the left: long, dramatic relief.
			p.LightDir = [3]float64{-1, -0.15, 0.35}
			p.Ambient = 0.45
		}),
		v("03-noon-soft", func(p *tapestry.ReliefParams) {
			// High overhead light, almost shadowless; edges do the talking.
			p.LightDir = [3]float64{-0.1, -0.1, 1.5}
			p.Ambient = 0.75
			p.Spec = 0.05
		}),
		v("04-light-from-southeast", func(p *tapestry.ReliefParams) {
			// Light from bottom-right — inverts the perceived height.
			p.LightDir = [3]float64{0.7, 0.7, 0.6}
		}),
		v("05-deep-carve", func(p *tapestry.ReliefParams) {
			// Exaggerated terrain slope, darker shadows.
			p.Slope = 0.12
			p.Ambient = 0.50
		}),
		v("06-gentle-emboss", func(p *tapestry.ReliefParams) {
			// Barely-there relief: soft wax-seal emboss.
			p.Slope = 0.02
			p.Ambient = 0.70
			p.Shadow = 0.15
			p.Rim = 0.05
		}),
		v("07-papercut-max", func(p *tapestry.ReliefParams) {
			// Minimal hillshade, maximal band-edge shadows: layered paper.
			p.Slope = 0.015
			p.Ambient = 0.80
			p.EdgeWidth = 0.70
			p.Shadow = 0.50
			p.Rim = 0.18
			p.Spec = 0
		}),
		v("08-smooth-marble", func(p *tapestry.ReliefParams) {
			// Hillshade only, no engraved edges: polished sculpted stone.
			p.Slope = 0.08
			p.Ambient = 0.50
			p.Shadow = 0
			p.Rim = 0
		}),
		v("09-glossy-enamel", func(p *tapestry.ReliefParams) {
			// Broad wet gloss over the baseline carve.
			p.Spec = 0.35
			p.Shininess = 6
			p.Ambient = 0.65
		}),
		v("10-hard-metallic", func(p *tapestry.ReliefParams) {
			// Low raking light from top-right with a tight bright highlight.
			p.LightDir = [3]float64{0.8, -0.6, 0.4}
			p.Slope = 0.08
			p.Ambient = 0.45
			p.Spec = 0.50
			p.Shininess = 60
		}),
	}
}

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

	for _, vr := range variants() {
		s := tapestry.New()
		s.DisableStripes = true
		s.Relief = true
		s.ReliefParams = vr.p

		img, err := s.Render(ctx)
		if err != nil {
			log.Fatal(err)
		}
		path := filepath.Join(outDir, fmt.Sprintf("relief_%s.png", vr.name))
		if err := render.WritePNG(path, img); err != nil {
			log.Fatal(err)
		}
		fmt.Println(path)
	}
}
