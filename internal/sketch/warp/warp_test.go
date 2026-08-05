package warp

import (
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var fixedSeeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34}

func testCtx(t testing.TB, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("kandinsky-soft-pressure")
	if !ok {
		t.Fatal("kandinsky-soft-pressure palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

// A non-finite field value would poison both domain displacement and colour.
func TestFieldSamplesStayFinite(t *testing.T) {
	for _, seed := range fixedSeeds {
		for _, fieldMode := range []mode{modePlain, modeSingle, modeNested} {
			s := New()
			s.fieldMode = fieldMode
			p := s.plan(testCtx(t, seed))
			for y := range 9 {
				for x := range 9 {
					got := p.sample(float64(x)/8, float64(y)/8)
					values := [...]float64{
						got.value, got.qx, got.qy, got.rx, got.ry, got.activity,
					}
					for _, value := range values {
						if math.IsNaN(value) || math.IsInf(value, 0) {
							t.Fatalf("seed %d mode %d at (%d,%d) produced %v", seed, fieldMode, x, y, got)
						}
					}
				}
			}
		}
	}
}

// Repeated coordinate queries must not consume state or mutate the plan.
func TestSamplingIsPure(t *testing.T) {
	p := New().plan(testCtx(t, 13))
	for _, point := range [][2]float64{{0.1, 0.2}, {0.5, 0.5}, {0.93, 0.81}} {
		first := p.sample(point[0], point[1])
		if second := p.sample(point[0], point[1]); second != first {
			t.Fatalf("sample at %v changed from %v to %v", point, first, second)
		}
	}
}

// The comparison modes must expose real field development, not aliases.
func TestModesAreDistinct(t *testing.T) {
	var plans [3]plan
	for fieldMode := modePlain; fieldMode <= modeNested; fieldMode++ {
		s := New()
		s.fieldMode = fieldMode
		plans[fieldMode] = s.plan(testCtx(t, 13))
	}

	plainSingleDiffer := false
	singleNestedDiffer := false
	for y := range 9 {
		for x := range 9 {
			u, v := float64(x)/8, float64(y)/8
			plain := plans[modePlain].sample(u, v).value
			single := plans[modeSingle].sample(u, v).value
			nested := plans[modeNested].sample(u, v).value
			plainSingleDiffer = plainSingleDiffer || plain != single
			singleNestedDiffer = singleNestedDiffer || single != nested
		}
	}
	if !plainSingleDiffer || !singleNestedDiffer {
		t.Fatalf("mode differences: plain/single=%t single/nested=%t", plainSingleDiffer, singleNestedDiffer)
	}
}
