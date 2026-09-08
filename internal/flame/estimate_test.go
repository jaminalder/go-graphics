package flame

import (
	"testing"
)

func TestEstimateRadiusZeroMatchesDirectDevelop(t *testing.T) {
	h := &Hist{
		w: 3, h: 1,
		r: []uint64{colorScale * 10, colorScale * 100, 0},
		g: []uint64{colorScale * 4, colorScale * 20, 0},
		b: []uint64{colorScale * 2, colorScale * 10, 0},
		a: []uint64{colorScale * 10, colorScale * 100, 0},
	}
	tone := Tone{Gamma: 2.2, Vibrancy: 1, Brightness: 1}
	direct := h.Develop(tone)
	tone.Estimate = Estimate{Radius: 0, Curve: 0.4}
	got := h.Develop(tone)
	for i := range direct {
		if direct[i] != got[i] {
			t.Fatalf("pixel %d: estimator 0 changed %v to %v", i, direct[i], got[i])
		}
	}
}

func TestEstimateSpreadsAThinHitAndNotADenseCore(t *testing.T) {
	// Two isolated bins on a 21-wide row: 10 hits at x=2, 4000 hits at x=18.
	const w = 21
	h := &Hist{
		w: w, h: 1,
		r: make([]uint64, w),
		g: make([]uint64, w),
		b: make([]uint64, w),
		a: make([]uint64, w),
	}
	thin, dense := 2, 18
	h.a[thin], h.r[thin] = colorScale*10, colorScale*10
	h.g[thin], h.b[thin] = colorScale*10, colorScale*10
	h.a[dense], h.r[dense] = colorScale*4000, colorScale*4000
	h.g[dense], h.b[dense] = colorScale*4000, colorScale*4000

	pix := h.Develop(Tone{
		Gamma: 1, Vibrancy: 1, Brightness: 1,
		Estimate: Estimate{Radius: 4, Min: 0, Curve: 0.4},
	})
	if pix[thin+1].R < 0.02 {
		t.Fatalf("thin filament did not spread: neighbour %g", pix[thin+1].R)
	}
	// The dense core's 1px neighbour must be dimmer, relative to its
	// centre, than the thin hit's — otherwise DE is a uniform blur.
	thinSpread := pix[thin+1].R / (pix[thin].R + 1e-9)
	denseSpread := pix[dense-1].R / (pix[dense].R + 1e-9)
	if denseSpread >= thinSpread {
		t.Fatalf("core spread %g ≥ filament spread %g; DE is not density-dependent", denseSpread, thinSpread)
	}
}

func TestEstimateIsDeterministic(t *testing.T) {
	h := &Hist{
		w: 5, h: 5,
		r: make([]uint64, 25),
		g: make([]uint64, 25),
		b: make([]uint64, 25),
		a: make([]uint64, 25),
	}
	h.a[12], h.r[12] = colorScale*30, colorScale*20
	h.g[12], h.b[12] = colorScale*10, colorScale*5
	tone := Tone{Gamma: 2.5, Vibrancy: 0.9, Brightness: 1.1, Gleam: 0.3, Estimate: Estimate{Radius: 3, Curve: 0.4}}
	a := h.Develop(tone)
	b := h.Develop(tone)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("pixel %d moved %v vs %v", i, a[i], b[i])
		}
	}
}
