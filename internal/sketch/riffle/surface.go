package riffle

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// What a pixel is made of, bottom up: gravel, the caustic net lighting it,
// the water column swallowing both, the surface reflecting the sky, and foam
// over everything.

// foamSource is where white water is born. Two terms and no more: the Froude
// number, which puts foam on the riffle crest and nowhere else without a
// threshold on depth or speed alone, and a plume behind each rock, which is
// the one thing a surface field cannot know about because it comes from a
// jet plunging under it.
//
// The source is multiplied by a patchy field and by a sparse bubble lattice
// *before* it is advected, which is what turns it into lines and trails
// rather than an even white haze.
func (p *plan) foamSource(u, v float64, r reading, speed float64) float64 {
	wet := wetness(r.depth)
	if wet <= 0 {
		return 0
	}
	s := mathx.Smoothstep(p.foamLo, p.foamHi, froude(speed, r.depth))
	for i := range p.rocks {
		s += p.rocks[i].white(u, v)
	}
	if s <= 0 {
		return 0
	}
	patch := mathx.Smoothstep(-0.30, 0.30, p.nFoam.FBM(u/foamPatch, v/foamPatch, 2))
	// A lace rather than a lattice. The bubbles were a Worley dot field, and
	// advected through the shear behind a rock a regular lattice comes out as
	// a set of nested arcs — a comb, not a bubble trail. Noise has no lattice
	// to repeat.
	lace := p.nFoam.FBM(u/p.set.bubbles+61, v/p.set.bubbles-23, 1)
	bubble := 0.55 + 0.6*mathx.Smoothstep(-0.15, 0.45, lace)
	return wet * s * patch * bubble
}

// foamPatch is the wavelength of the patchiness, canvas units. Without it
// the source is even and the advected result is a wash rather than a set of
// lines; the lines are most of what says "river" from across the room.
const foamPatch = 0.075

// white is the plume a rock sheds: a pillow of broken water on the upstream
// face and a tail streaming off the back.
func (r rock) white(u, v float64) float64 {
	if r.Wake <= 0 {
		return 0
	}
	a := r.area()
	dx, dy := u-r.X, v-r.Y
	s := dx*r.DU + dy*r.DV  // downstream of the rock
	l := -dx*r.DV + dy*r.DU // across
	if s < -1.4*a || s > 4*a {
		return 0
	}
	// Narrow and short. The first plume was as wide as the rock and seven
	// radii long, and once advected it filled a tenth of the frame with a
	// white fan — a rock trailing a comet. What a boulder actually leaves is
	// a tongue about half its width, torn up within a couple of radii.
	lat := l / (0.55 * a)
	lat = math.Exp(-lat * lat)
	if s < 0 {
		d := (s + 0.7*a) / (0.45 * a)
		return r.Wake * 0.7 * lat * math.Exp(-d*d)
	}
	return r.Wake * lat * math.Exp(-s/(1.15*a))
}

// caustics is the net of light on the bed.
//
// A Worley f2−f1 field is a cell diagram, which on its own is a honeycomb
// and reads as nothing. Evaluating it at a *warped* coordinate is what turns
// it into a caustic: the fold is what makes the cusps, the pinches and the
// long bright filaments that light actually draws. Cell size grows with
// depth and contrast falls with it, so the net is sharp in the shallows and
// gone in the pool without being masked anywhere.
func (p *plan) caustics(u, v, depth float64) float64 {
	if p.set.caustic <= 0 {
		return 0
	}
	scale := p.set.causticScale * (0.65 + 0.8*depth)
	k := p.set.causticWarp
	wu := u/scale + k*p.nCaus.FBM(u/causWarp, v/causWarp, 2)
	wv := v/scale + k*p.nCaus.FBM(u/causWarp+7.3, v/causWarp-2.9, 2)

	// Thin. The threshold on f2−f1 is the net's line width, and the first
	// version's was wide enough to light a third of the sheet — a pale
	// reticulation over everything, at one contrast, which read as tooled
	// leather and buried every other cue. A caustic is a *filament*: bright,
	// narrow, and mostly absent.
	f1a, f2a := noise.Worley(p.causticSeed, wu, wv)
	net := mathx.Smoothstep(0.055, 0.004, f2a-f1a)

	f1b, f2b := noise.Worley(p.causticSeed^0x9e3779b9, wu*2.1+3.1, wv*2.1-1.7)
	net += 0.3 * mathx.Smoothstep(0.035, 0.004, f2b-f1b)

	// Gone by half a metre, twice over: the contrast decays with depth, and
	// a gate closes it entirely below the shallows. Left to the decay alone
	// the net is still faintly legible in a pool, and a caustic visible in
	// deep water is the tell that it was painted on rather than cast.
	fade := math.Exp(-depth*causticFade) * mathx.Smoothstep(0.95, 0.30, depth)
	return net * p.set.caustic * fade
}

const (
	// causWarp is the wavelength of the fold, canvas units — a couple of
	// cells, so a run of cells folds together rather than each one
	// independently.
	causWarp = 0.055
	// causticFade is how fast the net loses contrast with depth: light
	// scattered on the way down spreads the caustic out until there is no
	// net left, well before the bed itself has gone.
	causticFade = 1.6
)

// gravel is the bed: Worley pebbles, each taking a tone from a hash of its
// own cell, with the joint between them darkened.
//
// The pebble scale grows sharply with the rock field, so a boulder comes out
// as one big stone rather than as a patch of gravel that happens to be
// raised. It is the cheapest way to make a boulder read as an object.
func (p *plan) gravel(u, v float64, r reading) palette.Color {
	s := p.set.pebble * (1 + 5*r.rock*r.rock)
	cx, cy, f1, f2 := noise.WorleyCell(p.pebbleSeed, u/s, v/s)
	tone := noise.Hash01(p.pebbleSeed^0x5eed5eed, cx, cy)
	joint := mathx.Smoothstep(0, 0.18, f2-f1)
	dome := 1 - mathx.Clamp01(f1)
	mottle := 0.5 + 0.5*p.nBed.FBM(u/0.24+31, v/0.24-17, 2)

	// The weights are low on everything cell-shaped and high on the broad
	// mottle, and that is a correction, not a taste. Gravel is genuinely
	// cellular, but a Worley diagram with its walls drawn and its cells
	// shaded is *very* cellular, and seen through half a metre of water it
	// came out as tooled leather over the whole sheet — the strongest texture
	// in the picture, at a scale that belongs to nothing.
	t := mathx.Clamp01(0.28*tone + 0.56*mottle + 0.16*dome)
	c := palette.Lerp(p.ink.gravelB, p.ink.gravelA, t)
	return scale(c, 0.95+0.05*joint)
}

// land is gravel out of the water: paler and greyer than the same stone wet,
// with a dark damp band at the waterline. That band is most of what makes an
// exposed bar read as an exposed bar rather than as a pale patch.
func (p *plan) land(g palette.Color, depth float64) palette.Color {
	dry := mathx.Smoothstep(0, -0.11, depth)
	damp := scale(g, 0.78)
	// Only half way to the flat dry colour: taken further, the bar loses the
	// gravel it is made of and reads as a cut-out.
	pale := palette.Lerp(g, p.ink.dry, 0.5)
	return palette.Lerp(damp, pale, dry)
}

// column composites the bed under a depth of water: per-channel
// Beer–Lambert in linear light, toward the water's own body colour.
//
// Per channel is the point. One extinction coefficient dims the bed evenly
// and the water has no colour of its own; three make the red go first, which
// is the actual reason water looks the way it does, and they give the
// shallows a warm cast and the deeps a cold one out of one expression.
func (p *plan) column(bed palette.Color, depth float64) palette.Color {
	k := p.ink.k
	b := [3]float64{
		palette.SRGBToLinear(bed.R),
		palette.SRGBToLinear(bed.G),
		palette.SRGBToLinear(bed.B),
	}
	var out [3]float64
	for i := range out {
		t := math.Exp(-k[i] * depth)
		out[i] = b[i]*t + p.ink.bodyLin[i]*(1-t)
	}
	return palette.Color{
		R: palette.LinearToSRGB(out[0]),
		G: palette.LinearToSRGB(out[1]),
		B: palette.LinearToSRGB(out[2]),
	}
}

// surface reflects the sky and catches the sun.
//
// The normal is (−slope·direction, 1): only the along-flow component of the
// surface gradient, because a riffle's standing waves have their crests
// across the flow and nearly all the slope is in that one direction. The
// walk produced it as the difference of two samples it had already taken.
func (p *plan) surface(c palette.Color, w walk, sun float64) palette.Color {
	nx, ny := -w.slope*w.dirU, -w.slope*w.dirV
	ninv := 1 / math.Sqrt(nx*nx+ny*ny+1)

	// The ripples, shaded. A facet tilted toward the sun is lighter and one
	// tilted away is darker — a smooth, signed term, and the reason the
	// surface reads as a *surface* rather than as a texture. The first
	// version had only the specular below it, which is a threshold, and a
	// threshold on a noise field is salt and pepper.
	lam := nx*p.lightX + ny*p.lightY
	c = scale(c, 1+rippleGain*lam)

	// The streaks, as a tonal veil along the flow. This is the convolution's
	// own output used for what it is good for: it has no structure except
	// the field's, so it cannot read as anything but current. It is the cue
	// that survives being squinted at, and without it the surface is a
	// dimpled texture with no direction in it.
	//
	// Normalised, because averaging twenty correlated samples of a field
	// that spans ±1 gives something that spans ±0.3, and at that amplitude
	// the streaks are there in the numbers and invisible on the sheet.
	c = scale(c, 1+streakGain*clampUnit(w.streak*streakNorm))

	// Reflection. Seen from straight above, water reflects almost nothing —
	// Fresnel at normal incidence is a couple of percent — and the first
	// version's flat 20% lerp toward a pale sky is what turned every pool
	// into milk. What is left is small and *tilt-dependent*.
	refl := p.set.sheen * (0.35 + 5.0*(1-ninv))
	c = palette.Lerp(c, p.ink.sky, mathx.Clamp01(refl))

	// A glint is an angular window, not a power of a cosine. Raised to a
	// power, a *flat* surface under a sun 60° up still returns a third of
	// full brightness — the half vector is only 15° off vertical — so the
	// whole river lit up. A gaussian in the angle between the normal and the
	// half vector fires only where a facet actually tips far enough to throw
	// the sun into the lens, which is what sun glitter is.
	nh := mathx.Clamp01((nx*p.halfX + ny*p.halfY + p.halfZ) * ninv)
	ang2 := 2 * (1 - nh)
	g := math.Exp(-ang2/(p.set.glint*p.set.glint)) * p.sunPower * sun
	return palette.Lerp(c, p.ink.glint, mathx.Clamp01(g)*glintDepth)
}

// scale multiplies a colour's brightness, which is what a shading term does.
func scale(c palette.Color, k float64) palette.Color {
	return palette.Color{R: c.R * k, G: c.G * k, B: c.B * k}
}

// clampUnit limits to [−1, 1] — the signed companion of mathx.Clamp01.
func clampUnit(v float64) float64 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}

// sunAt is how much sun reaches a point: everywhere, unless the sheet is
// dappled, in which case it comes and goes in soft patches the way it does
// under trees.
func (p *plan) sunAt(u, v float64) float64 {
	if p.set.dapple <= 0 {
		return 1
	}
	n := p.nCaus.FBM(u/dappleWave+101, v/dappleWave-53, 2)
	return 1 - p.set.dapple*mathx.Smoothstep(-0.05, 0.45, n)
}

// dappleWave is the size of a shade patch, canvas units — a large fraction
// of the frame, so it organises the picture instead of texturing it.
const dappleWave = 0.42

// pixel is the whole sketch: one pure function of position.
func (p *plan) pixel(u, v float64) palette.Color {
	r := p.read(u, v)
	wet := wetness(r.depth)
	if wet <= 0 {
		return p.land(p.gravel(u, v, r), r.depth)
	}

	w := p.upstream(u, v, r)

	// Refraction: the bed is sampled at a point displaced along the flow by
	// depth × surface slope. One multiply, and it is the difference between
	// a bed under glass and a bed under water.
	off := refract * r.depth * w.slope
	bed := p.gravel(u-off*w.dirU, v-off*w.dirV, r)

	// And the bed goes soft with depth, because light scattered on the way
	// down and back has been spread sideways. Mixing toward the gravel's own
	// mean rather than blurring costs one lerp and does the same job: crisp
	// stones in the shallows, a smooth wash in the pool, and the crossover
	// between them is a large part of what reads as depth.

	// Caustics light the bed by mixing it toward its own colour lit — not by
	// multiplying it. Multiplied, a warm gravel under a strong net goes to a
	// saturated orange web and the picture reads as reptile skin; mixed, it
	// goes toward white the way a lit stone does.
	bed = palette.Lerp(p.ink.gravelMean, bed, math.Exp(-r.depth*bedBlur))

	sun := p.sunAt(u, v)
	if caus := p.caustics(u, v, r.depth) * sun; caus > 0 {
		bed = palette.Lerp(bed, p.ink.lit, mathx.Clamp01(caus)*causticGain)
	}

	c := p.column(bed, r.depth)
	c = p.surface(c, w, sun)
	c = palette.Lerp(c, p.ink.foam, mathx.Smoothstep(p.foamOn, p.foamFull, w.foam))

	if wet < 1 {
		c = palette.Lerp(p.land(p.gravel(u, v, r), r.depth), c, wet)
	}
	return c
}

const (
	// refract is how far the bed is displaced per unit of depth times slope,
	// in canvas units. A few pebbles' worth at a typical wave.
	refract = 0.06
	// bedBlur is how fast the bed loses its detail with depth.
	bedBlur = 0.75
	// causticGain is how far a pebble goes toward its lit colour under the
	// brightest part of the net.
	causticGain = 0.9
	// streakGain is how far the convolution swings the surface tone, and
	// streakNorm brings its output back to about ±1 first.
	streakGain = 0.22
	streakNorm = 3.0
	// rippleGain is how hard the wave facets are shaded by the sun.
	rippleGain = 0.35
	// glintDepth is the most of the sun a single facet may throw back. At 1
	// the glitter is white paint and the picture loses its shadows to it.
	glintDepth = 0.12
)
