package render

import (
	"image"
	"image/color"
)

// ContactSheet tiles images into one grid, box-downsampling each to cellW
// pixels wide.
//
// It exists because the unit of work here is not an image, it is an output
// space: a change to a sketch is judged by sweeping twenty seeds, not by
// looking at one render, and twenty full-size files are not something you
// can hold in your head at once. A sheet is.
//
// Box-averaging rather than nearest-neighbour is the whole point at this
// size: these sketches are made of fine rings, granulation and dithered
// gradients, all of which alias into moiré under point sampling — a
// thumbnail that lies about the image it stands for is worse than none.
func ContactSheet(imgs []image.Image, cols, cellW, pad int) *image.NRGBA {
	if len(imgs) == 0 || cols < 1 || cellW < 1 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	if pad < 0 {
		pad = 0
	}

	tiles := make([]*image.NRGBA, len(imgs))
	cellH := 0
	for i, src := range imgs {
		b := src.Bounds()
		h := max(cellW*b.Dy()/max(b.Dx(), 1), 1)
		tiles[i] = boxDown(src, cellW, h)
		cellH = max(cellH, h)
	}

	rows := (len(tiles) + cols - 1) / cols
	sheet := image.NewNRGBA(image.Rect(0, 0,
		cols*cellW+(cols+1)*pad, rows*cellH+(rows+1)*pad))
	// A dark surround, so a pale sheet and its neighbour do not run together.
	surround := color.NRGBA{28, 28, 30, 255}
	for y := range sheet.Rect.Dy() {
		for x := range sheet.Rect.Dx() {
			sheet.SetNRGBA(x, y, surround)
		}
	}
	for i, t := range tiles {
		ox := pad + (i%cols)*(cellW+pad)
		oy := pad + (i/cols)*(cellH+pad)
		for y := range t.Rect.Dy() {
			for x := range t.Rect.Dx() {
				sheet.SetNRGBA(ox+x, oy+y, t.NRGBAAt(x, y))
			}
		}
	}
	return sheet
}

// boxDown averages each destination pixel over its whole source footprint.
func boxDown(src image.Image, w, h int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		y0, y1 := y*sh/h, (y+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := range w {
			x0, x1 := x*sw/w, (x+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bb, n uint32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, _ := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					r += cr >> 8
					g += cg >> 8
					bb += cb >> 8
					n++
				}
			}
			dst.SetNRGBA(x, y, color.NRGBA{uint8(r / n), uint8(g / n), uint8(bb / n), 255})
		}
	}
	return dst
}
