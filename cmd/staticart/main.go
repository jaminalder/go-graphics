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
	"runtime/debug"
	"strings"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/circles"
	"github.com/jaminalder/go-graphics/internal/sketch/contour"
	"github.com/jaminalder/go-graphics/internal/sketch/drift"
	"github.com/jaminalder/go-graphics/internal/sketch/rounds"
	"github.com/jaminalder/go-graphics/internal/sketch/shoal"
	"github.com/jaminalder/go-graphics/internal/sketch/tapestry"
)

func registry() *sketch.Registry {
	return sketch.NewRegistry(
		circles.New(),
		contour.New(),
		drift.New(),
		rounds.New(),
		shoal.New(),
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
  staticart list                      list sketches
  staticart palettes                  list palette slugs
  staticart render <sketch> [flags]   render a sketch

common render flags:
  --profile preview|web|print   size profile (or --width N --height N)
  --seed N                      composition seed
  --palette slug                see: staticart palettes
  --aa N                        anti-aliasing (default 2; use 3 for print)
  --deep                        16-bit PNG master (png only)
  --format png|jpg  --out dir

sketch-specific flags (e.g. tapestry's --relief, --crackle, --terrace-seed)
are listed by: staticart render <sketch> --help`)
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
	aa := fs.Int("aa", 2, "anti-aliasing: supersamples per axis (1 = off; use 3 for print)")
	deep := fs.Bool("deep", false, "render a 16-bit PNG master (archival/print; png only)")
	paletteName := fs.String("palette", "kandinsky-soft-pressure", "palette slug (see: staticart palettes)")
	format := fs.String("format", "png", "output format: png|jpg")
	outDir := fs.String("out", "out", "output directory")

	s, ok := registry().Get(name)
	if !ok {
		return fmt.Errorf("unknown sketch %q (try: staticart list)", name)
	}
	// Sketch-specific options are owned by the sketch itself.
	if c, ok := s.(sketch.Configurable); ok {
		c.Flags(fs)
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	fileName := s.Name()
	if c, ok := s.(sketch.Configurable); ok {
		suffix, err := c.Configure()
		if err != nil {
			return err
		}
		fileName += suffix
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

	if *deep && *format != "png" {
		return fmt.Errorf("--deep requires --format png")
	}

	ctx := sketch.Context{Width: w, Height: h, Seed: *seed, Palette: pal, AA: *aa, Deep: *deep}
	img, err := s.Render(ctx)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	file := fmt.Sprintf("%s_%s_%d_%dx%d.%s", fileName, pal.Slug, *seed, w, h, *format)
	path := filepath.Join(*outDir, file)

	meta := render.Meta{
		DPI:      300,
		Software: "staticart " + buildRevision(),
		Comment:  recipe(name, fs),
	}
	switch *format {
	case "png":
		err = render.WritePNGMeta(path, img, meta)
	case "jpg", "jpeg":
		err = render.WriteJPEGMeta(path, img, meta)
	default:
		return fmt.Errorf("unknown format %q (png|jpg)", *format)
	}
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

// recipe reconstructs the full canonical render command — every flag at
// its effective value — so an image file stays reproducible after any
// rename (embedded via render.Meta).
func recipe(sketchName string, fs *flag.FlagSet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "staticart render %s", sketchName)
	always := map[string]bool{"seed": true, "palette": true, "profile": true, "aa": true}
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == "out" {
			return // output location is not part of the artwork
		}
		if f.Value.String() != f.DefValue || always[f.Name] {
			fmt.Fprintf(&b, " --%s %s", f.Name, f.Value.String())
		}
	})
	return b.String()
}

// buildRevision returns the VCS revision baked into the binary, if any.
func buildRevision() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return s.Value[:12]
			}
		}
	}
	return "dev"
}
