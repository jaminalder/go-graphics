package scheme

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func testPalette(t *testing.T) []palette.Color {
	t.Helper()
	p, ok := palette.ByName("kandinsky-apple-tree")
	if !ok {
		t.Fatal("palette missing")
	}
	return p.Colors
}

// scatter lays regions of varied size over the frame.
func scatter(seed uint64, n int) []Region {
	rng := rand.New(rand.NewPCG(seed, 3))
	out := make([]Region, n)
	for i := range out {
		out[i] = Region{
			X:    rng.Float64() * 1.25,
			Y:    rng.Float64(),
			Size: 0.0005 + 0.02*rng.Float64()*rng.Float64(),
		}
	}
	return out
}

func mix(t *testing.T, name string, seed uint64, regions []Region) *Mixer {
	t.Helper()
	return New(Spec{
		Name: name, Palette: testPalette(t), Seed: seed,
		Aspect: 1.25, Passage: 0.75, Accent: 0.2,
	}, regions)
}

func fills(m *Mixer) []palette.Color {
	out := make([]palette.Color, m.Len())
	for i := range out {
		out[i] = m.At(i).Fill
	}
	return out
}

func distinct(cs []palette.Color) int {
	seen := map[palette.Color]bool{}
	for _, c := range cs {
		seen[c] = true
	}
	return len(seen)
}

// TestEveryStrategyIsDeterministic — invariant 1. A colour arrangement that
// moves between runs makes every golden and every sweep meaningless.
func TestEveryStrategyIsDeterministic(t *testing.T) {
	regions := scatter(1, 120)
	for _, name := range Names() {
		a, b := mix(t, name, 9, regions), mix(t, name, 9, regions)
		for i := range regions {
			if a.At(i) != b.At(i) {
				t.Fatalf("%s: region %d drew %v then %v", name, i, a.At(i), b.At(i))
			}
		}
	}
}

// TestEveryStrategyAnswersValueAsWellAsHue is the claim the whole package is
// built on. An arrangement of hue with no value structure goes flat grey when
// you squint at it, and that is the commonest way a correctly-harmonised
// palette still comes out looking like a swatch card.
func TestEveryStrategyAnswersValueAsWellAsHue(t *testing.T) {
	regions := scatter(2, 150)
	for _, name := range Names() {
		m := mix(t, name, 11, regions)
		lo, hi, sum := math.Inf(1), math.Inf(-1), 0.0
		for i := range regions {
			v := m.At(i).Tone
			if v < 0 || v > 1 {
				t.Fatalf("%s: tone %v out of range", name, v)
			}
			lo, hi, sum = math.Min(lo, v), math.Max(hi, v), sum+v
		}
		if hi-lo < 0.4 {
			t.Errorf("%s: tone spans only %.2f..%.2f — no value structure", name, lo, hi)
		}
		// And it must not be one extreme with a lone outlier at the other.
		if mean := sum / float64(len(regions)); mean < 0.1 || mean > 0.9 {
			t.Errorf("%s: mean tone %.2f — the value structure is all at one end", name, mean)
		}
	}
}

// TestNoTwoStrategiesAreTheSamePicture. Each is meant to be a different
// arrangement of the same palette over the same regions; two that agree
// nearly everywhere are one idea listed twice, which is exactly what
// by-darkness was before it got its own hue field.
func TestNoTwoStrategiesAreTheSamePicture(t *testing.T) {
	regions := scatter(3, 150)
	got := map[string][]palette.Color{}
	for _, name := range Names() {
		got[name] = fills(mix(t, name, 5, regions))
	}
	names := Names()
	for i := range names {
		for j := i + 1; j < len(names); j++ {
			same := 0
			for k := range regions {
				if got[names[i]][k] == got[names[j]][k] {
					same++
				}
			}
			if share := float64(same) / float64(len(regions)); share > 0.75 {
				t.Errorf("%s and %s agree on %.0f%% of regions — one idea shown twice",
					names[i], names[j], share*100)
			}
		}
	}
}

// TestSchemesPaintWithThePaletteTheyWereGiven. The palettes carry an artist
// and an artwork; a strategy that synthesises its own hues — h+120°, say —
// has stopped painting with them, and the provenance in internal/palette
// becomes a decoration. Lightening, darkening and mixing are fair; landing on
// a hue nothing in the palette is anywhere near is not.
func TestSchemesPaintWithThePaletteTheyWereGiven(t *testing.T) {
	pal := testPalette(t)
	regions := scatter(4, 120)
	for _, name := range Names() {
		m := mix(t, name, 13, regions)
		for i := range regions {
			c := m.At(i).Fill
			_, sat, _ := c.HSL()
			if sat < 0.12 {
				continue // a near-neutral has no hue to be wrong about
			}
			h, _, _ := c.HSL()
			near := 360.0
			for _, p := range pal {
				ph, ps, _ := p.HSL()
				if ps < 0.05 {
					continue
				}
				near = math.Min(near, hueGap(h, ph))
			}
			if near > 40 {
				t.Errorf("%s: region %d is hue %.0f°, %.0f° from anything in the palette",
					name, i, h, near)
			}
		}
	}
}

// TestQuietIsActuallyNearMonochrome — the point of the scheme. Its dilution
// belongs in the Tone, not baked into the Fill: baked in, a near-monochrome
// sheet reports a hundred different pigments and a caller laying a wash has
// no load left to read.
func TestQuietIsActuallyNearMonochrome(t *testing.T) {
	regions := scatter(5, 150)
	quiet := distinct(fills(mix(t, Monochrome, 7, regions)))
	loud := distinct(fills(mix(t, Passage, 7, regions)))
	if quiet > 3 {
		t.Errorf("the quiet sheet uses %d pigments", quiet)
	}
	if quiet >= loud {
		t.Errorf("quiet uses %d pigments against passage's %d", quiet, loud)
	}
}

// TestNotanIsThreeValues. Its carrying power comes from having almost no
// mid-tones; every extra value softens the poster read it exists for.
func TestNotanIsThreeValues(t *testing.T) {
	if n := distinct(fills(mix(t, Notan, 7, scatter(6, 150)))); n > 3 {
		t.Errorf("notan uses %d values, want at most 3", n)
	}
}

// TestDominanceIsUnequal. Proportion is as much of harmony as hue is: a
// uniform draw gives every colour equal presence, which reads as a sampler
// rather than as a picture. Roughly 70/20/10 is the classical shape.
func TestDominanceIsUnequal(t *testing.T) {
	regions := scatter(7, 300)
	m := mix(t, Dominance, 21, regions)
	count := map[palette.Color]int{}
	for i := range regions {
		count[m.At(i).Fill]++
	}
	most := 0
	for _, n := range count {
		most = max(most, n)
	}
	if share := float64(most) / float64(len(regions)); share < 0.5 {
		t.Errorf("the dominant colour holds %.0f%% — nothing dominates", share*100)
	}
	if len(count) < 2 {
		t.Error("dominance used a single colour — there is no hierarchy in one colour")
	}
}

// TestComplementKeepsItsAccentsRare. A complementary scheme is four fifths
// muted dominant and one fifth intense opposite; split evenly it is a flag.
func TestComplementKeepsItsAccentsRare(t *testing.T) {
	regions := scatter(8, 300)
	m := mix(t, Complement, 33, regions)
	hot := 0
	for i := range regions {
		if m.At(i).Tone > 0.8 {
			hot++
		}
	}
	if share := float64(hot) / float64(len(regions)); share > 0.35 {
		t.Errorf("%.0f%% of the sheet is accent — that is a flag, not a complementary scheme", share*100)
	}
}

// TestInheritBuildsChunksRatherThanConfetti. Inheritance exists to make large
// unified areas with organic edges, which a noise field cannot do — a field's
// patches are always field-shaped. Raise the mutation rate and it degenerates
// into exactly the confetti it was meant to avoid.
//
// Measured by colour *distance*, not equality: a child drifts a few degrees
// off its parent on purpose, so that a chunk reads as a family rather than as
// a flat stencil, and no two regions ever hold precisely the same colour.
func TestInheritBuildsChunksRatherThanConfetti(t *testing.T) {
	regions := scatter(9, 200)
	m := mix(t, Inherit, 17, regions)

	gap := func(a, b palette.Color) float64 {
		return math.Abs(a.R-b.R) + math.Abs(a.G-b.G) + math.Abs(a.B-b.B)
	}
	near, far := 0.0, 0.0
	nearN, farN := 0, 0
	for i := range regions {
		for j := i + 1; j < len(regions); j++ {
			d := math.Hypot(regions[i].X-regions[j].X, regions[i].Y-regions[j].Y)
			g := gap(m.At(i).Fill, m.At(j).Fill)
			switch {
			case d < 0.08:
				near, nearN = near+g, nearN+1
			case d > 0.5:
				far, farN = far+g, farN+1
			}
		}
	}
	if nearN == 0 || farN == 0 {
		t.Fatal("not enough pairs to judge")
	}
	n, f := near/float64(nearN), far/float64(farN)
	if n >= f*0.7 {
		t.Errorf("neighbours differ by %.3f and distant regions by %.3f — nothing is being inherited", n, f)
	}
}

// TestAnUnknownStrategyIsPassage, rather than a panic or a blank sheet: a
// scheme name reaches this package from a command line, and a typo should
// cost the caller its arrangement, not its render.
func TestAnUnknownStrategyIsPassage(t *testing.T) {
	regions := scatter(10, 60)
	got := fills(mix(t, "no-such-scheme", 4, regions))
	want := fills(mix(t, Passage, 4, regions))
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("region %d: %v, want passage's %v", i, got[i], want[i])
		}
	}
}

// TestAnEmptySetIsNotACrash — a caller may hand over a partition whose cells
// were all swallowed.
func TestAnEmptySetIsNotACrash(t *testing.T) {
	m := New(Spec{Name: Passage, Palette: testPalette(t), Seed: 1}, nil)
	if m.Len() != 0 {
		t.Errorf("resolved %d regions from none", m.Len())
	}
	if got := m.At(0); got.Tone != 1 {
		t.Errorf("out-of-range lookup gave %v", got)
	}
}
