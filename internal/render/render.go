// Package render executes per-pixel sketch functions into images and
// encodes them. It owns the normalized-coordinate convention (invariant 2
// in docs/ARCHITECTURE.md): v ∈ [0,1] spans the height, u spans [0, aspect].
package render

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
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
// (anti-aliasing). Subsamples are averaged in linear light — averaging
// sRGB-encoded values would render dark fringes along high-contrast
// edges. Output is 8-bit with deterministic dithered quantization (IGN),
// which prevents banding in slow gradients. samples ≤ 1 means plain
// center sampling.
func RasterSS(w, h, samples int, f PixelFunc) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	rasterLoop(w, h, samples, f, func(x, y int, c palette.Color) {
		d := ign(x, y)
		i := img.PixOffset(x, y)
		img.Pix[i] = quant8(c.R, d)
		img.Pix[i+1] = quant8(c.G, d)
		img.Pix[i+2] = quant8(c.B, d)
		img.Pix[i+3] = 255
	})
	return img
}

// RasterDeep is RasterSS with 16-bit output (no dithering needed) for
// archival/print masters; png.Encode writes it as a 16-bit PNG.
func RasterDeep(w, h, samples int, f PixelFunc) *image.NRGBA64 {
	img := image.NewNRGBA64(image.Rect(0, 0, w, h))
	rasterLoop(w, h, samples, f, func(x, y int, c palette.Color) {
		i := img.PixOffset(x, y)
		putU16 := func(off int, v float64) {
			q := uint16(math.Round(math.Min(1, math.Max(0, v)) * 65535))
			img.Pix[off] = uint8(q >> 8)
			img.Pix[off+1] = uint8(q)
		}
		putU16(i, c.R)
		putU16(i+2, c.G)
		putU16(i+4, c.B)
		img.Pix[i+6] = 255
		img.Pix[i+7] = 255
	})
	return img
}

// rasterLoop runs the parallel sampling loop and hands each pixel's final
// (still-float) color to put. With samples > 1 the subsample average is
// computed in linear light.
func rasterLoop(w, h, samples int, f PixelFunc, put func(x, y int, c palette.Color)) {
	if samples < 1 {
		samples = 1
	}
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
				for x := 0; x < w; x++ {
					if samples == 1 {
						u := (float64(x) + 0.5) * scale
						v := (float64(y) + 0.5) * scale
						put(x, y, f(u, v))
						continue
					}
					var lr, lg, lb float64
					for sj := 0; sj < samples; sj++ {
						v := (float64(y) + (float64(sj)+0.5)*sub) * scale
						for si := 0; si < samples; si++ {
							u := (float64(x) + (float64(si)+0.5)*sub) * scale
							c := f(u, v)
							lr += palette.SRGBToLinear(c.R)
							lg += palette.SRGBToLinear(c.G)
							lb += palette.SRGBToLinear(c.B)
						}
					}
					put(x, y, palette.Color{
						R: palette.LinearToSRGB(lr * norm),
						G: palette.LinearToSRGB(lg * norm),
						B: palette.LinearToSRGB(lb * norm),
					})
				}
			}
		}(start, end)
	}
	wg.Wait()
}

// ImageFromColors quantizes a w×h float color buffer (row-major) to a
// dithered 8-bit image — the same output treatment as RasterSS, for
// renderers that build their own buffer (e.g. paint.Canvas).
func ImageFromColors(w, h int, pix []palette.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := pix[y*w+x]
			d := ign(x, y)
			i := img.PixOffset(x, y)
			img.Pix[i] = quant8(c.R, d)
			img.Pix[i+1] = quant8(c.G, d)
			img.Pix[i+2] = quant8(c.B, d)
			img.Pix[i+3] = 255
		}
	}
	return img
}

// ign is interleaved gradient noise in [0,1) — a cheap, deterministic,
// well-distributed dither pattern.
func ign(x, y int) float64 {
	return math.Mod(52.9829189*math.Mod(0.06711056*float64(x)+0.00583715*float64(y), 1), 1)
}

// quant8 quantizes v to 8 bits with dither offset d in [0,1).
func quant8(v, d float64) uint8 {
	q := math.Floor(math.Min(1, math.Max(0, v))*255 + d)
	if q > 255 {
		q = 255
	}
	return uint8(q)
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
