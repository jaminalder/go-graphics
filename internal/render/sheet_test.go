package render

import (
	"image"
	"image/color"
	"testing"
)

// TestContactSheetAveragesRatherThanSamples is the reason a sheet is worth
// having at all: these sketches are made of fine rings and dithered
// gradients, which alias into moiré under point sampling. A thumbnail that
// lies about the image it stands for is worse than no thumbnail.
func TestContactSheetAveragesRatherThanSamples(t *testing.T) {
	// A one-pixel checkerboard: any point sample gives pure black or pure
	// white, the average gives grey.
	src := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			v := uint8(0)
			if (x+y)%2 == 0 {
				v = 255
			}
			src.SetNRGBA(x, y, color.NRGBA{v, v, v, 255})
		}
	}
	sheet := ContactSheet([]image.Image{src}, 1, 8, 0)
	c := sheet.NRGBAAt(4, 4)
	if c.R < 100 || c.R > 155 {
		t.Errorf("checkerboard downsampled to %d, want mid grey — it is point sampling", c.R)
	}
}

func TestContactSheetLaysOutAGrid(t *testing.T) {
	sq := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	imgs := make([]image.Image, 5)
	for i := range imgs {
		imgs[i] = sq
	}
	// Five tiles at three columns is two rows, the second one short.
	sheet := ContactSheet(imgs, 3, 20, 2)
	if w := sheet.Rect.Dx(); w != 3*20+4*2 {
		t.Errorf("sheet width %d, want %d", w, 3*20+4*2)
	}
	if h := sheet.Rect.Dy(); h != 2*20+3*2 {
		t.Errorf("sheet height %d, want %d", h, 2*20+3*2)
	}
}

func TestContactSheetHandlesNothing(t *testing.T) {
	if got := ContactSheet(nil, 3, 20, 2); got.Rect.Empty() {
		t.Error("an empty sweep produced an empty image rather than a placeholder")
	}
}
