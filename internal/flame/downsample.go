package flame

import (
	"fmt"
	"math"

	"github.com/jaminalder/go-graphics/internal/palette"
)

const (
	// spatialSupport is flam3's Gaussian spatial-filter support
	// (flam3_spatial_support[gaussian] = 1.5).
	spatialSupport = 1.5
	// DefaultFilter is flam3's usual spatial_filter_radius in output pixels.
	DefaultFilter = 0.5
)

// Downsample reduces an oversampled colour buffer to outW×outH with
// flam3's separable Gaussian spatial filter. src must be row-major
// srcW×srcH with srcW = outW*ss and srcH = outH*ss. radius is the
// filter radius in output pixels; 0 falls back to DefaultFilter.
// Averaging is in linear light. ss ≤ 1 returns a copy of src.
//
// The filter is applied sequentially in row-major order so the result
// cannot depend on GOMAXPROCS.
func Downsample(src []palette.Color, srcW, srcH, outW, outH, ss int, radius float64) ([]palette.Color, error) {
	if ss < 1 {
		ss = 1
	}
	if outW < 1 {
		outW = 1
	}
	if outH < 1 {
		outH = 1
	}
	if ss == 1 {
		out := make([]palette.Color, outW*outH)
		n := len(src)
		if n > len(out) {
			n = len(out)
		}
		copy(out, src[:n])
		for i := n; i < len(out); i++ {
			out[i] = palette.Color{}
		}
		return out, nil
	}
	if srcW != outW*ss || srcH != outH*ss {
		return nil, fmt.Errorf("flame: downsample shape %dx%d with ss=%d, want %dx%d",
			srcW, srcH, ss, outW*ss, outH*ss)
	}
	if len(src) < srcW*srcH {
		return nil, fmt.Errorf("flame: downsample src len %d < %d", len(src), srcW*srcH)
	}
	if radius <= 0 {
		radius = DefaultFilter
	}
	kernel, fw := spatialKernel(ss, radius)
	gutter := (fw - ss) / 2
	out := make([]palette.Color, outW*outH)
	for y := 0; y < outH; y++ {
		baseY := y*ss - gutter
		for x := 0; x < outW; x++ {
			baseX := x*ss - gutter
			var lr, lg, lb float64
			for jj := 0; jj < fw; jj++ {
				sy := baseY + jj
				if sy < 0 {
					sy = 0
				} else if sy >= srcH {
					sy = srcH - 1
				}
				row := sy * srcW
				for ii := 0; ii < fw; ii++ {
					sx := baseX + ii
					if sx < 0 {
						sx = 0
					} else if sx >= srcW {
						sx = srcW - 1
					}
					k := kernel[ii+jj*fw]
					c := src[row+sx]
					lr += palette.SRGBToLinear(c.R) * k
					lg += palette.SRGBToLinear(c.G) * k
					lb += palette.SRGBToLinear(c.B) * k
				}
			}
			out[y*outW+x] = palette.Color{
				R: palette.LinearToSRGB(lr),
				G: palette.LinearToSRGB(lg),
				B: palette.LinearToSRGB(lb),
			}.Clamp()
		}
	}
	return out, nil
}

// spatialKernel builds flam3's normalised separable Gaussian of width
// fw for the given oversample and filter radius (output pixels).
func spatialKernel(ss int, radius float64) ([]float64, int) {
	fw := 2.0 * spatialSupport * float64(ss) * radius
	fwidth := int(fw) + 1
	if (fwidth^ss)&1 != 0 {
		fwidth++
	}
	if fwidth < ss {
		fwidth = ss
		if (fwidth^ss)&1 != 0 {
			fwidth++
		}
	}
	adjust := 1.0
	if fw > 0 {
		adjust = spatialSupport * float64(fwidth) / fw
	}
	k := make([]float64, fwidth*fwidth)
	var sum float64
	for j := 0; j < fwidth; j++ {
		jj := ((2.0*float64(j)+1.0)/float64(fwidth) - 1.0) * adjust
		wj := math.Exp(-2 * jj * jj)
		for i := 0; i < fwidth; i++ {
			ii := ((2.0*float64(i)+1.0)/float64(fwidth) - 1.0) * adjust
			wi := math.Exp(-2 * ii * ii)
			w := wi * wj
			k[i+j*fwidth] = w
			sum += w
		}
	}
	if sum <= 0 {
		k[0] = 1
		return k, fwidth
	}
	inv := 1 / sum
	for i := range k {
		k[i] *= inv
	}
	return k, fwidth
}
