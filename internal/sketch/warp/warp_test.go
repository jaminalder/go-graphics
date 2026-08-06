package warp

import (
	"flag"
	"image"
	"io"
	"math"
	"sort"
	"testing"

	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var fixedSeeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34}

var update = flag.Bool("update", false, "regenerate golden files")

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
			p, err := s.plan(testCtx(t, seed))
			if err != nil {
				t.Fatal(err)
			}
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
	p, err := New().plan(testCtx(t, 13))
	if err != nil {
		t.Fatal(err)
	}
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
		p, err := s.plan(testCtx(t, 13))
		if err != nil {
			t.Fatal(err)
		}
		plans[fieldMode] = p
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

// Shallower modes must not evaluate fields that cannot affect their output.
// Zero components are the fieldSample contract for work deliberately skipped.
func TestModesLeaveUnusedFieldsZero(t *testing.T) {
	for _, tc := range []struct {
		mode mode
		want func(fieldSample) bool
	}{
		{modePlain, func(got fieldSample) bool {
			return got.activity == 0 && got.qx == 0 && got.qy == 0 && got.rx == 0 && got.ry == 0
		}},
		{modeSingle, func(got fieldSample) bool {
			return got.activity != 0 && (got.qx != 0 || got.qy != 0) && got.rx == 0 && got.ry == 0
		}},
		{modeNested, func(got fieldSample) bool {
			return got.activity != 0 && (got.qx != 0 || got.qy != 0) && (got.rx != 0 || got.ry != 0)
		}},
	} {
		s := New()
		s.fieldMode = tc.mode
		p, err := s.plan(testCtx(t, 13))
		if err != nil {
			t.Fatal(err)
		}
		got := p.sample(0.37, 0.61)
		if !tc.want(got) {
			t.Errorf("mode %d evaluated unused fields: %+v", tc.mode, got)
		}
	}
}

func axisAlignedFBM(p plan, field *noise.Perlin, x, y float64) float64 {
	sum, weight := 0.0, 0.0
	frequency, amplitude := 1.0, 1.0
	for range p.set.octaves {
		sum += field.At(x*frequency, y*frequency) * amplitude
		weight += amplitude
		frequency *= p.set.lacunarity
		amplitude *= p.set.gain
	}
	return sum / weight
}

// Octave rotation must preserve coordinate length before lacunarity scaling,
// while producing a different finite evaluation than axis-aligned stepping.
func TestOctaveRotationPreservesScaledLengthAndChangesEvaluation(t *testing.T) {
	p, err := New().plan(testCtx(t, 13))
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.37, 0.61}, {1.13, -0.42}, {-0.73, 1.29}} {
		rx, ry := rotateScale(point[0], point[1], p.set.lacunarity)
		wantLength := math.Hypot(point[0], point[1]) * p.set.lacunarity
		if gotLength := math.Hypot(rx, ry); math.Abs(gotLength-wantLength) > 1e-14 {
			t.Errorf("rotated/scaled %v length %v, want %v", point, gotLength, wantLength)
		}
		got := p.fbm(p.value, point[0], point[1])
		old := axisAlignedFBM(p, p.value, point[0], point[1])
		if got == old {
			t.Errorf("rotated fBM at %v equals axis-aligned value %v", point, got)
		}
		if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got) > 0.75 {
			t.Errorf("rotated fBM at %v is outside practical bounds: %v", point, got)
		}
	}
}

// The nested roles deliberately move from broad bends toward finer local
// structure. Aggregate variation avoids pinning independent fields pointwise.
func TestNestedRolesUseDifferentSpatialScales(t *testing.T) {
	const delta = 0.006
	qVariation, rVariation, valueVariation := 0.0, 0.0, 0.0
	for _, seed := range []uint64{1, 2, 5, 8} {
		p, err := New().plan(testCtx(t, seed))
		if err != nil {
			t.Fatal(err)
		}
		for y := range 12 {
			for x := range 12 {
				u, v := (float64(x)+0.37)/12, (float64(y)+0.61)/12
				a := p.sample(u, v)
				b := p.sample(u+delta, v-delta*0.7)
				for _, value := range [...]float64{a.qx, a.qy, a.rx, a.ry, a.value} {
					if math.IsNaN(value) || math.IsInf(value, 0) {
						t.Fatalf("seed %d sample (%v,%v) is not finite: %+v", seed, u, v, a)
					}
				}
				qVariation += math.Hypot(a.qx-b.qx, a.qy-b.qy)
				rVariation += math.Hypot(a.rx-b.rx, a.ry-b.ry)
				valueVariation += math.Abs(a.value - b.value)
			}
		}
	}
	if rVariation <= qVariation*1.08 {
		t.Errorf("r variation %v is not finer than q variation %v", rVariation, qVariation)
	}
	if valueVariation <= qVariation*0.6 {
		t.Errorf("final variation %v does not carry enough local detail relative to q %v", valueVariation, qVariation)
	}
}

// Every mode must produce a finite continuous material signal before colour
// and lighting consume it.
func TestMaterialSamplesStayFinite(t *testing.T) {
	for _, seed := range fixedSeeds {
		for _, fieldMode := range []mode{modePlain, modeSingle, modeNested} {
			s := New()
			s.fieldMode = fieldMode
			p, err := s.plan(testCtx(t, seed))
			if err != nil {
				t.Fatal(err)
			}
			for y := range 13 {
				for x := range 13 {
					got := p.sample(float64(x)/12, float64(y)/12)
					for _, value := range [...]float64{got.value, got.fine, got.height, got.ridge} {
						if math.IsNaN(value) || math.IsInf(value, 0) {
							t.Fatalf("seed %d mode %d at (%d,%d) produced %+v", seed, fieldMode, x, y, got)
						}
					}
					if got.height < 0 || got.height > 1 || got.ridge < 0 || got.ridge > 1 {
						t.Fatalf("seed %d mode %d material outside [0,1]: %+v", seed, fieldMode, got)
					}
				}
			}
		}
	}
}

// Preferred seeds must retain cavities, a broad body, and sparse raised
// material instead of collapsing most samples into one clipped tone.
func TestMaterialHeightCreatesCavitiesBodiesAndRidges(t *testing.T) {
	var heights []float64
	for _, seed := range []uint64{1, 2, 5, 8} {
		p, err := New().plan(testCtx(t, seed))
		if err != nil {
			t.Fatal(err)
		}
		for y := range 40 {
			for x := range 40 {
				heights = append(heights, p.sample((float64(x)+0.5)/40, (float64(y)+0.5)/40).height)
			}
		}
	}
	sort.Float64s(heights)
	q10 := heights[len(heights)/10]
	q50 := heights[len(heights)/2]
	q90 := heights[len(heights)*9/10]
	if q50-q10 < 0.18 || q90-q50 < 0.18 {
		t.Fatalf("material quantile spread is too flat: q10=%v q50=%v q90=%v", q10, q50, q90)
	}
	if q10 <= 0.01 || q90 >= 0.99 {
		t.Fatalf("material quantiles are mostly clipped: q10=%v q90=%v", q10, q90)
	}
}

// The broad activity envelope must reserve fine ridges for turbulent zones.
func TestQuietActivitySuppressesFineRidges(t *testing.T) {
	lowSum, highSum := 0.0, 0.0
	lowCount, highCount := 0, 0
	for _, seed := range fixedSeeds {
		p, err := New().plan(testCtx(t, seed))
		if err != nil {
			t.Fatal(err)
		}
		for y := range 40 {
			for x := range 40 {
				got := p.sample((float64(x)+0.5)/40, (float64(y)+0.5)/40)
				switch {
				case got.activity < 0.35:
					lowSum += got.ridge
					lowCount++
				case got.activity > 1.05:
					highSum += got.ridge
					highCount++
				}
			}
		}
	}
	if lowCount == 0 || highCount == 0 {
		t.Fatalf("activity sample groups missing: low=%d high=%d", lowCount, highCount)
	}
	lowMean, highMean := lowSum/float64(lowCount), highSum/float64(highCount)
	if highMean <= lowMean*1.4 {
		t.Fatalf("high activity ridge mean %v is not above quiet mean %v", highMean, lowMean)
	}
}

// Normals derived from the visible material must be valid unit vectors over
// the complete fixed-seed and mode space.
func TestMaterialNormalsAreFiniteAndNormalized(t *testing.T) {
	for _, seed := range fixedSeeds {
		for _, fieldMode := range []mode{modePlain, modeSingle, modeNested} {
			s := New()
			s.fieldMode = fieldMode
			p, err := s.plan(testCtx(t, seed))
			if err != nil {
				t.Fatal(err)
			}
			for y := range 7 {
				for x := range 7 {
					n := p.materialNormal((float64(x)+0.3)/7, (float64(y)+0.7)/7)
					length := math.Sqrt(n.x*n.x + n.y*n.y + n.z*n.z)
					if math.IsNaN(length) || math.IsInf(length, 0) || math.Abs(length-1) > 1e-12 {
						t.Fatalf("seed %d mode %d normal %+v has length %v", seed, fieldMode, n, length)
					}
				}
			}
		}
	}
}

// Increasing height along either canvas axis must tilt the corresponding
// normal component toward the negative axis used by the lighting convention.
func TestMaterialNormalDerivativeAxesAndSigns(t *testing.T) {
	flat := normalFromHeights(0.4, 0.4, 0.4, 0.4)
	xSlope := normalFromHeights(0.2, 0.6, 0.4, 0.4)
	ySlope := normalFromHeights(0.4, 0.4, 0.2, 0.6)

	if flat != (vector3{0, 0, 1}) {
		t.Fatalf("flat heights produced normal %+v", flat)
	}
	if !(xSlope.x < 0 && xSlope.y == 0 && xSlope.z > 0) {
		t.Fatalf("increasing x height produced normal %+v", xSlope)
	}
	if !(ySlope.x == 0 && ySlope.y < 0 && ySlope.z > 0) {
		t.Fatalf("increasing y height produced normal %+v", ySlope)
	}
}

// One oblique light must order opposite slopes consistently and retain an
// ambient floor even when a face turns away.
func TestLightOrdersOppositeSlopes(t *testing.T) {
	toward := normalize3(vector3{0.48, -0.36, 0.8})
	flat := vector3{0, 0, 1}
	away := normalize3(vector3{-0.48, 0.36, 0.8})
	a, b, c := lightFactor(toward), lightFactor(flat), lightFactor(away)
	if !(a > b && b > c && c > 0) {
		t.Fatalf("light order toward=%v flat=%v away=%v", a, b, c)
	}
}

// The normal step is a canvas length, so pixel dimensions cannot change the
// surface or folded color at one normalized coordinate.
func TestNormalStepUsesCanvasUnits(t *testing.T) {
	smallCtx := testCtx(t, 13)
	smallCtx.Width, smallCtx.Height = 96, 64
	largeCtx := smallCtx
	largeCtx.Width, largeCtx.Height = 960, 640
	s := New()
	s.appearance = appearanceFolded
	small, err := s.plan(smallCtx)
	if err != nil {
		t.Fatal(err)
	}
	large, err := s.plan(largeCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.75, 0.5}, {1.4, 0.8}} {
		if a, b := small.materialNormal(point[0], point[1]), large.materialNormal(point[0], point[1]); a != b {
			t.Fatalf("normal at %v changed from %+v to %+v", point, a, b)
		}
		if a, b := small.At(point[0], point[1]), large.At(point[0], point[1]); a != b {
			t.Fatalf("folded color at %v changed from %+v to %+v", point, a, b)
		}
	}
}

// Illumination is applied in linear light and must preserve finite sRGB while
// ordering the resulting physical luminance by light factor.
func TestFoldedShadingPreservesColorBounds(t *testing.T) {
	base := palette.Color{R: 0.46, G: 0.31, B: 0.62}
	previous := -1.0
	for _, factor := range []float64{0.35, 0.7, 1.0} {
		got := shadeLinear(base, factor)
		want := palette.Color{
			R: palette.LinearToSRGB(palette.SRGBToLinear(base.R) * factor),
			G: palette.LinearToSRGB(palette.SRGBToLinear(base.G) * factor),
			B: palette.LinearToSRGB(palette.SRGBToLinear(base.B) * factor),
		}
		for i, component := range [...]float64{got.R, got.G, got.B} {
			if math.IsNaN(component) || math.IsInf(component, 0) || component < 0 || component > 1 {
				t.Fatalf("factor %v produced invalid color %+v", factor, got)
			}
			wantComponent := [...]float64{want.R, want.G, want.B}[i]
			if math.Abs(component-wantComponent) > 1e-15 {
				t.Errorf("factor %v channel %d = %v, want linear-light conversion %v", factor, i, component, wantComponent)
			}
		}
		linearLuminance := 0.2126*palette.SRGBToLinear(got.R) +
			0.7152*palette.SRGBToLinear(got.G) +
			0.0722*palette.SRGBToLinear(got.B)
		if linearLuminance <= previous {
			t.Fatalf("factor %v luminance %v did not exceed %v", factor, linearLuminance, previous)
		}
		previous = linearLuminance
	}
}

// Folded appearance must create stronger value hierarchy than the diagnostic
// ramp without achieving it by clipping most samples to black or white.
func TestFoldedAppearanceHasGreaterLuminanceRange(t *testing.T) {
	var gradientValues, foldedValues []float64
	clipped := 0
	for _, seed := range []uint64{1, 2, 5, 8} {
		gradientPlan, err := configured(t, "--appearance", "gradient").plan(testCtx(t, seed))
		if err != nil {
			t.Fatal(err)
		}
		foldedPlan, err := configured(t, "--appearance", "folded").plan(testCtx(t, seed))
		if err != nil {
			t.Fatal(err)
		}
		for y := range 30 {
			for x := range 30 {
				u, v := (float64(x)+0.5)/30, (float64(y)+0.5)/30
				gradientValues = append(gradientValues, gradientPlan.At(u, v).Luminance())
				luminance := foldedPlan.At(u, v).Luminance()
				foldedValues = append(foldedValues, luminance)
				if luminance < 0.015 || luminance > 0.985 {
					clipped++
				}
			}
		}
	}
	sort.Float64s(gradientValues)
	sort.Float64s(foldedValues)
	robustRange := func(values []float64) float64 {
		return values[len(values)*99/100] - values[len(values)/100]
	}
	gradientRange, foldedRange := robustRange(gradientValues), robustRange(foldedValues)
	if foldedRange <= gradientRange*1.08 {
		t.Fatalf("folded luminance range %v is not materially above gradient %v", foldedRange, gradientRange)
	}
	if float64(clipped)/float64(len(foldedValues)) > 0.08 {
		t.Fatalf("folded appearance clips %d/%d samples", clipped, len(foldedValues))
	}
}

func configured(t testing.TB, args ...string) *Sketch {
	t.Helper()
	s := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Configure(); err != nil {
		t.Fatal(err)
	}
	return s
}

// A render is a deterministic function of its recipe and must vary by seed.
func TestRenderIsDeterministic(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 13), testCtx(t, 21))
}

// Pixel dimensions are output quality, not field configuration.
func TestPlanIgnoresPixelDimensions(t *testing.T) {
	smallCtx := testCtx(t, 13)
	smallCtx.Width, smallCtx.Height = 96, 64
	largeCtx := smallCtx
	largeCtx.Width, largeCtx.Height = 960, 640

	s := configured(t, "--appearance", "folded")
	small, err := s.plan(smallCtx)
	if err != nil {
		t.Fatal(err)
	}
	large, err := s.plan(largeCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.75, 0.5}, {1.4, 0.8}} {
		if a, b := small.sample(point[0], point[1]), large.sample(point[0], point[1]); a != b {
			t.Fatalf("field sample at %v changed from %v to %v", point, a, b)
		}
		if a, b := small.At(point[0], point[1]), large.At(point[0], point[1]); a != b {
			t.Fatalf("color at %v changed from %v to %v", point, a, b)
		}
	}
}

// Both mappings must stay inside finite sRGB for every field mode.
func TestBothAppearancesProduceFiniteColors(t *testing.T) {
	for _, seed := range fixedSeeds {
		for _, appearance := range []string{"gradient", "folded"} {
			for _, fieldMode := range []string{"plain", "single", "nested"} {
				p, err := configured(t, "--appearance", appearance, "--warp", fieldMode).plan(testCtx(t, seed))
				if err != nil {
					t.Fatal(err)
				}
				for y := range 9 {
					for x := range 9 {
						color := p.At(float64(x)/8, float64(y)/8)
						for _, component := range [...]float64{color.R, color.G, color.B} {
							if math.IsNaN(component) || math.IsInf(component, 0) || component < 0 || component > 1 {
								t.Fatalf("seed %d %s/%s at (%d,%d) produced %v", seed, appearance, fieldMode, x, y, color)
							}
						}
					}
				}
			}
		}
	}
}

// Every public boundary is accepted and applies to the resolved sketch.
func TestOptionsAcceptBoundariesAndChoices(t *testing.T) {
	for _, args := range [][]string{
		{"--scale", "0.25"},
		{"--scale", "12"},
		{"--octaves", "1"},
		{"--octaves", "8"},
		{"--gain", "0.2"},
		{"--gain", "0.85"},
		{"--lacunarity", "1.2"},
		{"--lacunarity", "3.5"},
		{"--warp-strength", "0"},
		{"--warp-strength", "8"},
		{"--nested-strength", "0"},
		{"--nested-strength", "8"},
		{"--warp", "plain"},
		{"--warp", "single"},
		{"--warp", "nested"},
		{"--appearance", "gradient"},
		{"--appearance", "folded"},
	} {
		configured(t, args...)
	}
}

// Each option must change only its own immutable setting. This catches
// declaration plumbing that accidentally targets a neighboring field.
func TestOptionsAlterOnlySelectedResolvedSetting(t *testing.T) {
	defaults, err := New().plan(testCtx(t, 13))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		args   []string
		change func(*settings)
	}{
		{"scale", []string{"--scale", "3.25"}, func(s *settings) { s.scale = 3.25 }},
		{"octaves", []string{"--octaves", "7"}, func(s *settings) { s.octaves = 7 }},
		{"gain", []string{"--gain", "0.7"}, func(s *settings) { s.gain = 0.7 }},
		{"lacunarity", []string{"--lacunarity", "2.75"}, func(s *settings) { s.lacunarity = 2.75 }},
		{"warp-strength", []string{"--warp-strength", "6.5"}, func(s *settings) { s.warpStrength = 6.5 }},
		{"nested-strength", []string{"--nested-strength", "7.5"}, func(s *settings) { s.nestedStrength = 7.5 }},
		{"warp", []string{"--warp", "single"}, func(s *settings) { s.mode = modeSingle }},
		{"appearance", []string{"--appearance", "gradient"}, func(s *settings) { s.appearance = appearanceGradient }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := configured(t, tc.args...).plan(testCtx(t, 13))
			if err != nil {
				t.Fatal(err)
			}
			want := defaults.set
			tc.change(&want)
			if p.set != want {
				t.Errorf("resolved settings %+v, want only selected change %+v", p.set, want)
			}
		})
	}
}

// Values outside documented ranges and unknown names must fail Configure.
func TestOptionsRejectInvalidValues(t *testing.T) {
	for _, args := range [][]string{
		{"--scale", "0.24"},
		{"--scale", "12.1"},
		{"--octaves", "0"},
		{"--octaves", "9"},
		{"--gain", "0.19"},
		{"--gain", "0.86"},
		{"--lacunarity", "1.1"},
		{"--lacunarity", "3.6"},
		{"--warp-strength", "-0.1"},
		{"--warp-strength", "8.1"},
		{"--nested-strength", "-0.1"},
		{"--nested-strength", "8.1"},
		{"--warp", "triple"},
		{"--appearance", "structure"},
		{"--appearance", "rainbow"},
	} {
		s := New()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		s.Flags(fs)
		if err := fs.Parse(args); err != nil {
			continue
		}
		if _, err := s.Configure(); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
}

func TestRejectsTooSmallPalette(t *testing.T) {
	ctx := testCtx(t, 13)
	ctx.Palette = palette.Palette{Slug: "tiny", Colors: []palette.Color{{}, {}}}
	if _, err := configured(t).Render(ctx); err == nil {
		t.Error("expected error for palette with fewer than three colors")
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 13))
	sketchtest.Golden(t, got, "testdata/warp_seed13_64.png", *update)
}

var (
	benchmarkSample fieldSample
	benchmarkColor  palette.Color
	benchmarkImage  image.Image
)

func BenchmarkSample(b *testing.B) {
	p, err := configured(b).plan(testCtx(b, 13))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		benchmarkSample = p.sample(float64(i%97)/97, float64(i%89)/89)
	}
}

func BenchmarkAt(b *testing.B) {
	p, err := configured(b).plan(testCtx(b, 13))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		benchmarkColor = p.At(float64(i%97)/97, float64(i%89)/89)
	}
}

func BenchmarkRender64(b *testing.B) {
	s := New()
	ctx := testCtx(b, 13)
	b.ReportAllocs()
	for range b.N {
		var err error
		benchmarkImage, err = s.Render(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
