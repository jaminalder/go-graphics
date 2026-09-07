package flame

import (
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
)

const (
	colorScale   = 1 << 24
	orbitWorkers = 8
	warmup       = 20
)

// Tone is the log-density display: gamma lifts filaments, vibrancy keeps
// saturated colour in the cores, gleam pulls the densest bins toward white.
type Tone struct {
	Gamma, Vibrancy, Brightness, Gleam float64
	Background                         palette.Color
	Estimate                           Estimate
}

// Estimate is flam3 density estimation: a Gaussian whose radius is
// estimator / n^curve, applied after log-density. Radius 0 disables it.
type Estimate struct {
	Radius, Min, Curve float64
}

// DefaultTone is an Apophysis-like display: high gamma so hairline
// filaments read, high vibrancy so they stay coloured.
func DefaultTone() Tone {
	return Tone{Gamma: 3.0, Vibrancy: 0.88, Brightness: 1.05, Gleam: 0.4}
}

// Hist is a four-channel integer histogram. RGB accumulate scaled colour;
// A accumulates scaled hit weight. Integer bins keep parallel orbits
// deterministic: addition is commutative, so GOMAXPROCS cannot change the
// picture. The worker count itself is a constant for the same reason.
type Hist struct {
	w, h       int
	r, g, b, a []uint64
}

// Accumulate runs the chaos game into a histogram of size w×h. samples is
// the number of plotted points (after warmup). seed keys eight independent
// orbits; it must not be derived from the machine.
func Accumulate(s System, color func(float64) palette.Color, w, h, samples int, seed uint64) *Hist {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if samples < 1 {
		samples = 1
	}
	s.prepare()
	hist := &Hist{
		w: w, h: h,
		r: make([]uint64, w*h),
		g: make([]uint64, w*h),
		b: make([]uint64, w*h),
		a: make([]uint64, w*h),
	}
	base := samples / orbitWorkers
	rem := samples % orbitWorkers
	var wg sync.WaitGroup
	wg.Add(orbitWorkers)
	for worker := 0; worker < orbitWorkers; worker++ {
		n := base
		if worker < rem {
			n++
		}
		go func(worker, n int) {
			defer wg.Done()
			if n <= 0 {
				return
			}
			rng := rand.New(rand.NewPCG(seed, uint64(worker)+1))
			orbit(s, color, hist, rng, n)
		}(worker, n)
	}
	wg.Wait()
	return hist
}

func orbit(s System, color func(float64) palette.Color, hist *Hist, rng *rand.Rand, n int) {
	x, y := rng.Float64()*2-1, rng.Float64()*2-1
	c := rng.Float64()
	last := -1
	for i := 0; i < n+warmup; i++ {
		xi := s.pick(rng, last)
		last = xi
		xf := s.X[xi]
		x, y = xf.apply(x, y, rng)
		c = xf.blendColor(c)
		if bad(x, y) {
			x, y = rng.Float64()*2-1, rng.Float64()*2-1
			c = rng.Float64()
			last = -1
			continue
		}
		px, py := x, y
		cc := c
		if s.Final != nil {
			px, py = s.Final.apply(px, py, rng)
			cc = s.Final.blendColor(cc)
			if bad(px, py) {
				continue
			}
		}
		if i < warmup {
			continue
		}
		col := color(cc)
		sx, sy := s.Cam.project(px, py, hist.w, hist.h)
		hist.splat(sx, sy, col.R, col.G, col.B, xf.opacity())
	}
}

func (h *Hist) splat(x, y, cr, cg, cb, weight float64) {
	if weight <= 0 {
		return
	}
	x0 := math.Floor(x)
	y0 := math.Floor(y)
	fx := x - x0
	fy := y - y0
	ix, iy := int(x0), int(y0)
	h.add(ix, iy, (1-fx)*(1-fy)*weight, cr, cg, cb)
	h.add(ix+1, iy, fx*(1-fy)*weight, cr, cg, cb)
	h.add(ix, iy+1, (1-fx)*fy*weight, cr, cg, cb)
	h.add(ix+1, iy+1, fx*fy*weight, cr, cg, cb)
}

func (h *Hist) add(x, y int, w, cr, cg, cb float64) {
	if w <= 0 || x < 0 || y < 0 || x >= h.w || y >= h.h {
		return
	}
	aw := uint64(w*colorScale + 0.5)
	if aw == 0 {
		return
	}
	i := y*h.w + x
	atomic.AddUint64(&h.a[i], aw)
	atomic.AddUint64(&h.r[i], uint64(cr*float64(aw)+0.5))
	atomic.AddUint64(&h.g[i], uint64(cg*float64(aw)+0.5))
	atomic.AddUint64(&h.b[i], uint64(cb*float64(aw)+0.5))
}

// Develop tone-maps the histogram into a row-major sRGB buffer.
func (h *Hist) Develop(tone Tone) []palette.Color {
	out := make([]palette.Color, h.w*h.h)
	gamma := tone.Gamma
	if gamma < 0.2 {
		gamma = 0.2
	}
	invG := 1 / gamma
	vib := mathx.Clamp01(tone.Vibrancy)
	bright := tone.Brightness
	if bright <= 0 {
		bright = 1
	}
	gleam := mathx.Clamp01(tone.Gleam)
	bg := tone.Background

	var nMax float64
	for i := range h.a {
		n := float64(h.a[i]) / colorScale
		if n > nMax {
			nMax = n
		}
	}
	if nMax <= 0 {
		for i := range out {
			out[i] = bg
		}
		return out
	}
	logMax := math.Log(1 + nMax*bright)
	if tone.Estimate.Radius > 0 {
		return h.developEstimated(tone, out, logMax, invG, vib, bright, gleam, bg)
	}

	for i := range h.a {
		a := h.a[i]
		if a == 0 {
			out[i] = bg
			continue
		}
		n := float64(a) / colorScale
		meanR := float64(h.r[i]) / float64(a)
		meanG := float64(h.g[i]) / float64(a)
		meanB := float64(h.b[i]) / float64(a)
		scale := math.Log(1+n*bright) / logMax
		alpha := math.Pow(scale, invG)
		if gleam > 0 {
			w := mathx.Smoothstep(0.62, 1, scale) * gleam
			meanR += (1 - meanR) * w
			meanG += (1 - meanG) * w
			meanB += (1 - meanB) * w
		}
		ch := func(m float64) float64 {
			ind := math.Pow(mathx.Clamp01(m*scale), invG)
			return vib*(m*alpha) + (1-vib)*ind
		}
		r, g, b := ch(meanR), ch(meanG), ch(meanB)
		out[i] = palette.Color{
			R: bg.R*(1-alpha) + r,
			G: bg.G*(1-alpha) + g,
			B: bg.B*(1-alpha) + b,
		}.Clamp()
	}
	return out
}
