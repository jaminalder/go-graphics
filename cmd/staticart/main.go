// Command staticart renders generative sketches to image files.
//
//	staticart list                      list sketches
//	staticart palettes                  list palette slugs
//	staticart render <sketch> [flags]   render a sketch
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/contour"
	"github.com/jaminalder/go-graphics/internal/sketch/tapestry"
)

func registry() *sketch.Registry {
	return sketch.NewRegistry(
		contour.New(),
		tapestry.New(),
	)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "staticart:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "list":
		for _, s := range registry().All() {
			fmt.Printf("%-12s %s\n", s.Name(), s.Describe())
		}
		return nil
	case "palettes":
		for _, name := range palette.Names() {
			p, _ := palette.ByName(name)
			fmt.Printf("%-30s %s — %s\n", p.Slug, p.Artist, p.Work)
		}
		return nil
	case "render":
		return runRender(args[1:])
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (try: staticart help)", args[0])
	}
}

func usage() {
	fmt.Println(`usage:
  staticart list
  staticart palettes
  staticart render <sketch> [--profile preview|web|print] [--width N --height N]
                   [--seed N] [--palette slug] [--format png|jpg] [--out dir]`)
}

func runRender(args []string) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("render needs a sketch name (try: staticart list)")
	}
	name := args[0]

	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	profileName := fs.String("profile", "preview", "size profile: "+strings.Join(render.ProfileNames(), "|"))
	width := fs.Int("width", 0, "override width in px (requires --height)")
	height := fs.Int("height", 0, "override height in px (requires --width)")
	seed := fs.Uint64("seed", 42, "random seed (same seed → same image)")
	paletteName := fs.String("palette", "kandinsky-soft-pressure", "palette slug (see: staticart palettes)")
	format := fs.String("format", "png", "output format: png|jpg")
	outDir := fs.String("out", "out", "output directory")
	noStripes := fs.Bool("no-stripes", false, "tapestry only: render without the vertical stripe layer")
	relief := fs.Bool("relief", false, "tapestry only: 3D relief shading (hillshade + paper-cut edges)")
	reliefPreset := fs.String("relief-preset", "", "tapestry only: named relief look (implies --relief): "+strings.Join(tapestry.ReliefPresetNames(), "|"))
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	s, ok := registry().Get(name)
	if !ok {
		return fmt.Errorf("unknown sketch %q (try: staticart list)", name)
	}
	fileName := s.Name()
	if *relief || *reliefPreset != "" {
		ts, ok := s.(*tapestry.Sketch)
		if !ok {
			return fmt.Errorf("--relief/--relief-preset only apply to the tapestry sketch")
		}
		ts.Relief = true
		fileName += "-relief"
		if *reliefPreset != "" && *reliefPreset != "baseline" {
			params, ok := tapestry.ReliefPreset(*reliefPreset)
			if !ok {
				return fmt.Errorf("unknown relief preset %q (have %v)", *reliefPreset, tapestry.ReliefPresetNames())
			}
			ts.ReliefParams = params
			fileName += "-" + *reliefPreset
		}
	}
	if *noStripes {
		ts, ok := s.(*tapestry.Sketch)
		if !ok {
			return fmt.Errorf("--no-stripes only applies to the tapestry sketch")
		}
		ts.DisableStripes = true
		fileName += "-nostripes"
	}
	pal, ok := palette.ByName(*paletteName)
	if !ok {
		return fmt.Errorf("unknown palette %q (try: staticart palettes)", *paletteName)
	}

	w, h := *width, *height
	if w == 0 || h == 0 {
		if w != 0 || h != 0 {
			return fmt.Errorf("--width and --height must be given together")
		}
		p, err := render.ProfileByName(*profileName)
		if err != nil {
			return err
		}
		w, h = p.Width, p.Height
	}

	ctx := sketch.Context{Width: w, Height: h, Seed: *seed, Palette: pal}
	img, err := s.Render(ctx)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	file := fmt.Sprintf("%s_%s_%d_%dx%d.%s", fileName, pal.Slug, *seed, w, h, *format)
	path := filepath.Join(*outDir, file)
	switch *format {
	case "png":
		err = render.WritePNG(path, img)
	case "jpg", "jpeg":
		err = render.WriteJPEG(path, img)
	default:
		return fmt.Errorf("unknown format %q (png|jpg)", *format)
	}
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
