package main

// One-shot: build contact sheet + verify numbering for a flock directory.
import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jaminalder/go-graphics/internal/render"
)

func main() {
	dir := "out/flame-wash-flock-2"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	paths, err := filepath.Glob(filepath.Join(dir, "[0-9][0-9]_*.png"))
	if err != nil {
		fatal(err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		fatal(fmt.Errorf("no numbered pngs in %s", dir))
	}
	imgs := make([]image.Image, 0, len(paths))
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			fatal(err)
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			fatal(err)
		}
		imgs = append(imgs, img)
	}
	cols := 1
	for cols*cols < len(imgs) {
		cols++
	}
	sheet := render.ContactSheet(imgs, cols, 320, 6)
	out := filepath.Join(dir, "sheet.png")
	var man strings.Builder
	for i, p := range paths {
		fmt.Fprintf(&man, "%02d  %s\n", i+1, filepath.Base(p))
	}
	if err := render.WritePNGMeta(out, sheet, render.Meta{
		Software: "staticart flock-sheet",
		Comment:  man.String(),
	}); err != nil {
		fatal(err)
	}
	fmt.Printf("%d tiles → %s\n", len(imgs), out)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
