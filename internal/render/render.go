// Package render executes per-pixel sketch functions into images and
// encodes them. It owns the normalized-coordinate convention (invariant 2
// in docs/ARCHITECTURE.md): v ∈ [0,1] spans the height, u spans [0, aspect].
package render

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"runtime"
	"sync"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// PixelFunc computes the color at normalized coordinates. It must be pure:
// it is called concurrently from multiple goroutines.
type PixelFunc func(u, v float64) palette.Color

// Raster renders f over a w×h canvas, sampling pixel centers:
// u = (x+0.5)/h, v = (y+0.5)/h — dividing both by height keeps the scale
// uniform, so v spans [0,1] and u spans [0, w/h]. Rows are rendered in
// parallel; output is deterministic because f is pure.
func Raster(w, h int, f PixelFunc) *image.NRGBA {
	return RasterSS(w, h, 1, f)
}

// RasterSS is Raster with samples×samples supersampling per pixel
// (anti-aliasing): band and crack boundaries lose their pixel staircase.
// samples ≤ 1 means plain center sampling.
func RasterSS(w, h, samples int, f PixelFunc) *image.NRGBA {
	if samples < 1 {
		samples = 1
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	scale := 1 / float64(h)
	sub := 1 / float64(samples)
	norm := 1 / float64(samples*samples)

	var wg sync.WaitGroup
	workers := runtime.GOMAXPROCS(0)
	rowsPer := (h + workers - 1) / workers
	for start := 0; start < h; start += rowsPer {
		end := min(start+rowsPer, h)
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			for y := y0; y < y1; y++ {
				i := img.PixOffset(0, y)
				for x := 0; x < w; x++ {
					var acc palette.Color
					for sj := 0; sj < samples; sj++ {
						v := (float64(y) + (float64(sj)+0.5)*sub) * scale
						for si := 0; si < samples; si++ {
							u := (float64(x) + (float64(si)+0.5)*sub) * scale
							c := f(u, v)
							acc.R += c.R
							acc.G += c.G
							acc.B += c.B
						}
					}
					c := palette.Color{R: acc.R * norm, G: acc.G * norm, B: acc.B * norm}.NRGBA()
					img.Pix[i] = c.R
					img.Pix[i+1] = c.G
					img.Pix[i+2] = c.B
					img.Pix[i+3] = c.A
					i += 4
				}
			}
		}(start, end)
	}
	wg.Wait()
	return img
}

// WritePNG encodes img to path as PNG.
func WritePNG(path string, img image.Image) error {
	return writeFile(path, func(f *os.File) error { return png.Encode(f, img) })
}

// JPEGQuality is the encoding quality for JPEG output.
const JPEGQuality = 95

// WriteJPEG encodes img to path as JPEG at JPEGQuality.
func WriteJPEG(path string, img image.Image) error {
	return writeFile(path, func(f *os.File) error {
		return jpeg.Encode(f, img, &jpeg.Options{Quality: JPEGQuality})
	})
}

func writeFile(path string, encode func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if err := encode(f); err != nil {
		f.Close()
		return fmt.Errorf("render: encoding %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}
