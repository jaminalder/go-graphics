package paint

import (
	"bytes"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func newRNG() *rand.Rand { return rand.New(rand.NewPCG(7, 11)) }

// painted reports whether the pixel carries any of the stroke colour.
func painted(c *Canvas, x, y int) bool { return c.pix[y*c.w+x].G < 0.9 }

func TestWetBrushLaysSolidBand(t *testing.T) {
	c := NewCanvas(200, 200, white)
	b := NewBrush(newRNG(), 0.2, 12, 0)
	b.Stroke(c, []Pt{{X: 0.1, Y: 0.5}, {X: 0.9, Y: 0.5}}, red, 1)

	// The core of the band is fully covered: a wet brush must not leave
	// bristle gaps, or every fill built on it reads as grit.
	for x := 60; x < 140; x++ {
		for y := 92; y <= 108; y++ {
			if !painted(c, x, y) {
				t.Fatalf("gap inside wet band at (%d,%d)", x, y)
			}
		}
	}
	// And it stays inside the ferrule, with a little room for the soft
	// dab edge and the outermost bristle's own width.
	for x := 60; x < 140; x++ {
		for _, y := range []int{70, 130} {
			if painted(c, x, y) {
				t.Fatalf("paint at (%d,%d) is outside the ferrule", x, y)
			}
		}
	}
}

// TestDryStreaksRunAlongTheStroke is the load-bearing property of the
// whole brush model: gaps left by a dry brush must be long in the
// direction of travel and narrow across it. Streaks that run along the
// stroke read as a smear; gaps that are round or run across it read as
// grit or as scribble.
func TestDryStreaksRunAlongTheStroke(t *testing.T) {
	c := NewCanvas(400, 400, white)
	b := NewBrush(newRNG(), 0.3, 14, 0.5).Grain(newRNG(), 0.25)
	b.Stroke(c, []Pt{{X: 0.05, Y: 0.5}, {X: 0.95, Y: 0.5}}, red, 1)

	// Measure mean run length of unpainted pixels inside the band, along
	// the stroke and across it.
	const x0, x1, y0, y1 = 40, 360, 145, 255
	along := meanRun(func(i, j int) bool { return !painted(c, x0+i, y0+j) }, x1-x0, y1-y0)
	across := meanRun(func(i, j int) bool { return !painted(c, x0+j, y0+i) }, y1-y0, x1-x0)

	if along == 0 || across == 0 {
		t.Fatalf("expected gaps in a dry stroke (along=%v across=%v)", along, across)
	}
	// Directional: guards the bristle geometry. Tracks that wandered or
	// crossed would even the two out.
	if along < 3*across {
		t.Errorf("gaps are not directional: mean run along=%.1f across=%.1f, want along ≥ 3× across", along, across)
	}
	// Long: guards the grain. The gaps must be smears, not dashes. With a
	// 0.25-unit grain on a 400px canvas a streak runs ~40px; tying the
	// lift-off wave to the ferrule width instead once cut this to ~9px,
	// which is the stitched look this constant exists to catch.
	if along < 25 {
		t.Errorf("mean gap run along the stroke = %.1fpx, want ≥ 25 — streaks have broken into dashes", along)
	}
}

// meanRun averages the length of consecutive true runs over a w×h grid,
// scanning the first index.
func meanRun(hit func(i, j int) bool, w, h int) float64 {
	total, runs, cur := 0, 0, 0
	for j := range h {
		for i := range w {
			if hit(i, j) {
				cur++
				continue
			}
			if cur > 0 {
				total, runs = total+cur, runs+1
				cur = 0
			}
		}
		if cur > 0 {
			total, runs, cur = total+cur, runs+1, 0
		}
	}
	if runs == 0 {
		return 0
	}
	return float64(total) / float64(runs)
}

func TestBrushDeterministicAndGrainMatters(t *testing.T) {
	stroke := func(b Brush) *Canvas {
		c := NewCanvas(120, 120, white)
		b.Stroke(c, []Pt{{X: 0.1, Y: 0.5}, {X: 0.9, Y: 0.5}}, red, 1)
		return c
	}
	a := stroke(NewBrush(newRNG(), 0.15, 8, 0.4))
	b := stroke(NewBrush(newRNG(), 0.15, 8, 0.4))
	if !bytes.Equal(a.Image().Pix, b.Image().Pix) {
		t.Error("same seed must give the same stroke")
	}
	// Grain changes the streak length, so it must change the output.
	g := stroke(NewBrush(newRNG(), 0.15, 8, 0.4).Grain(newRNG(), 2.0))
	if bytes.Equal(a.Image().Pix, g.Image().Pix) {
		t.Error("grain changed nothing")
	}
}

func TestReloadRearrangesBundle(t *testing.T) {
	rng := newRNG()
	b := NewBrush(rng, 0.15, 8, 0.3)
	r := b.Reload(rng)
	if b.Width() != r.Width() {
		t.Errorf("reload changed width: %v vs %v", b.Width(), r.Width())
	}
	same := true
	for i := range b.bristles {
		if b.bristles[i] != r.bristles[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("reload returned an identical bundle")
	}
}

func TestSweepRingCoversBandNotHole(t *testing.T) {
	c := NewCanvas(300, 300, white)
	b := NewBrush(newRNG(), 0.05, 8, 0)
	SweepRing(c, newRNG(), b, 0.5, 0.5, 0.15, 0.35, red, 1, 0)

	mid := func(frac float64) (int, int) {
		return 150, int((0.5 - frac) * 300)
	}
	// Painted through the band...
	for _, f := range []float64{0.18, 0.25, 0.32} {
		if x, y := mid(f); !painted(c, x, y) {
			t.Errorf("band unpainted at radius %v", f)
		}
	}
	// ...and the hole and the outside are left alone.
	if painted(c, 150, 150) {
		t.Error("sweep filled the hole")
	}
	if x, y := mid(0.45); painted(c, x, y) {
		t.Error("sweep painted outside the band")
	}
}

func TestSweepRingFillsDisc(t *testing.T) {
	c := NewCanvas(200, 200, white)
	b := NewBrush(newRNG(), 0.06, 8, 0)
	SweepRing(c, newRNG(), b, 0.5, 0.5, 0, 0.3, palette.Color{}, 1, 0)
	if !painted(c, 100, 100) {
		t.Error("rIn=0 must fill the centre")
	}
}

func TestNormalsArePerpendicularAndUnit(t *testing.T) {
	pts := []Pt{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 1}, {X: 2, Y: 2}}
	nx, ny := normals(pts)
	for i := range pts {
		if l := math.Hypot(nx[i], ny[i]); math.Abs(l-1) > 1e-9 {
			t.Errorf("normal %d has length %v, want 1", i, l)
		}
	}
	// Straight segment start: path runs +x, so the left normal is +y.
	if math.Abs(nx[0]) > 1e-9 || math.Abs(ny[0]-1) > 1e-9 {
		t.Errorf("normal 0 = (%v,%v), want (0,1)", nx[0], ny[0])
	}
}
