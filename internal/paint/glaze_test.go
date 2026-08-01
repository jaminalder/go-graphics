package paint

import (
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

var (
	glazePaper = palette.Color{R: 0.93, G: 0.92, B: 0.88}
	glazeRed   = palette.Color{R: 0.72, G: 0.18, B: 0.16}
	glazeBlue  = palette.Color{R: 0.16, G: 0.30, B: 0.55}
)

func closeColor(t *testing.T, got, want palette.Color, tol float64, what string) {
	t.Helper()
	if math.Abs(got.R-want.R) > tol || math.Abs(got.G-want.G) > tol || math.Abs(got.B-want.B) > tol {
		t.Errorf("%s: got %.6f/%.6f/%.6f want %.6f/%.6f/%.6f",
			what, got.R, got.G, got.B, want.R, want.G, want.B)
	}
}

// TestTwoGlazesEqualOneOfTheirCombinedLoad is the property that makes load a
// *quantity of pigment* rather than an opacity. If it fails, a wash split
// into a body and a rim no longer agrees with the same pigment laid once,
// and every effect built by adding loads together (edge darkening, a bloom
// ridge, a second pigment dropped in) drifts from what it asks for.
func TestTwoGlazesEqualOneOfTheirCombinedLoad(t *testing.T) {
	for _, load := range [][2]float64{{0.4, 0.9}, {1.5, 1.5}, {0.05, 4}} {
		twice := Glaze(Glaze(glazePaper, glazeRed, load[0], 0), glazeRed, load[1], 0)
		once := Glaze(glazePaper, glazeRed, load[0]+load[1], 0)
		closeColor(t, twice, once, 1e-9, "stacked glazes")
	}
}

// TestGlazingTwoPigmentsGivesTheSameColourEitherWayRound. Absorption
// commutes, which is what lets a fill lay its own pigment and a neighbour's
// bleed in whatever order is convenient without the picture depending on
// that order.
func TestGlazingTwoPigmentsGivesTheSameColourEitherWayRound(t *testing.T) {
	ab := Glaze(Glaze(glazePaper, glazeRed, 1.1, 0), glazeBlue, 0.7, 0)
	ba := Glaze(Glaze(glazePaper, glazeBlue, 0.7, 0), glazeRed, 1.1, 0)
	closeColor(t, ab, ba, 1e-9, "pigment order")
}

// TestZeroLoadLeavesThePaperExactly — a cell whose wash stopped short of the
// wall has to show bare paper, not a faint tint of its pigment.
func TestZeroLoadLeavesThePaperExactly(t *testing.T) {
	if got := Glaze(glazePaper, glazeRed, 0, 0.2); got != glazePaper {
		t.Errorf("a zero load changed the paper to %v", got)
	}
}

// TestAHeavyGlazeDriesToItsOwnMasstone, not to black. Pure absorption
// marches every pigment to black as the load grows, which is the complaint
// painters have about mixing on the palette rather than glazing; the
// scattering floor is what keeps a dense passage the colour it was painted.
func TestAHeavyGlazeDriesToItsOwnMasstone(t *testing.T) {
	dead := Glaze(glazePaper, glazeRed, 40, 0)
	if dead.Luminance() > 0.02 {
		t.Errorf("without scatter a huge load left luminance %.3f, expected near black", dead.Luminance())
	}
	live := Glaze(glazePaper, glazeRed, 40, 1)
	closeColor(t, live, glazeRed, 1e-6, "fully scattering masstone")
	part := Glaze(glazePaper, glazeRed, 40, 0.25)
	if part.Luminance() <= dead.Luminance() || part.Luminance() >= live.Luminance() {
		t.Errorf("a partial scatter floor gave luminance %.3f, outside (%.3f, %.3f)",
			part.Luminance(), dead.Luminance(), live.Luminance())
	}
}

// TestGlazeIsMonotoneInLoad. Every watercolour cue here — the rim, the
// granulation, the pale interior of a backrun — is a modulation of the load,
// so "more pigment reached this point" must always mean "darker here".
func TestGlazeIsMonotoneInLoad(t *testing.T) {
	prev := glazePaper.Luminance()
	for load := 0.1; load < 6; load += 0.1 {
		lum := Glaze(glazePaper, glazeRed, load, 0.18).Luminance()
		if lum > prev+1e-12 {
			t.Fatalf("load %.1f is lighter (%.4f) than the load before it (%.4f)", load, lum, prev)
		}
		prev = lum
	}
}
