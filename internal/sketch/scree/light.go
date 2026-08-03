package scree

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// Giving the bed a surface, and lighting it.
//
// Everything here rests on one fact about the structure: `Hit.Wall` is a real
// signed distance, not a mask. A distance can be turned into a height, a
// height can be differenced into a slope, and a slope is a normal that can be
// lit. So the stones get a surface without anything ever being modelled —
// the height is a function of how deep inside its own stone a point is, and
// the boundaries of the bed become the creases of the surface for free.
//
// The height is in *canvas units of rise*, not an abstract 0..1, which is
// what keeps the lighting resolution-independent: a slope is rise over run
// and both are canvas lengths, so preview and print light the same picture.
//
// One light, one direction, for the whole sheet. Inconsistent lighting is
// what makes fake relief look fake: the eye forgives an impossible surface
// long before it forgives two shadows pointing different ways. The bearing is
// drawn from a narrow band around up-and-left, the convention every embossed
// thing on a page has trained the eye on — a stone lit from below reads as a
// hole in the bed rather than as a stone.

// lightLevels is the lamp and the surface it falls on, resolved per sheet.
type lightLevels struct {
	// bearing and elev are the lamp as given: a compass bearing in radians
	// and how high it stands. lx, ly, lz are those two normalised into a
	// direction, and are what the shading actually reads. Keeping both means
	// --light and --elevation can each be pinned without the other having to
	// know it was, and aim() is idempotent.
	bearing, elev float64
	lx, ly, lz    float64

	// The half-vector between the lamp and the viewer, who is always straight
	// overhead. A highlight sits where a face bisects those two, not where it
	// points at the lamp — and on a bed seen from above the difference is the
	// whole of it, since a face pointing at a low lamp is a face pointing away
	// from the camera and its gleam would never be seen.
	hx, hy, hz float64

	// gain rescales the diffuse so a *flat* face reads as nearly lit. Without
	// it a low lamp costs the whole picture its top end: a horizontal face
	// receives only lz of the light, so on a raking sheet nothing anywhere is
	// brighter than half and the bed comes out as a dark, low-contrast slab
	// whatever the stones are painted. Faces tilted into the lamp clamp at
	// full, which is what gives the lit shoulders their edge.
	gain float64

	amb    float64 // how much light reaches a face turned away
	rise   float64 // how proud a stone stands, ×its own inradius
	gloss  float64 // strength of the specular
	sharp  float64 // how tight it is
	warmth float64 // how far the lit side leans toward the lamp's colour
	cool   float64 // how far the shadowed side leans toward the sky's

	step     float64 // the finite-difference step, canvas units
	maxSlope float64 // the cap on that difference
}

// flatLit is how bright a horizontal face comes out, before the ambient, and
// it is what gain is solved for. It has to leave real headroom above a flat
// face: the top of a dome is nearly flat over a good part of its width, so at
// 0.9 every facet up there clamps to the same value and the stone comes out
// with a bald plateau on it. At 0.7 the flat faces are still the picture's
// light and the shoulders turned into the lamp have somewhere to go.
const flatLit = 0.70

// aim turns the bearing and the elevation into a unit direction, and settles
// everything derived from it. Screen coordinates grow downward, so a bearing
// read the way a compass rose is read has to be flipped in v: 135° is up and
// to the left.
func (l *lightLevels) aim() {
	lx, ly := math.Cos(l.bearing), -math.Sin(l.bearing)
	n := math.Sqrt(lx*lx + ly*ly + l.elev*l.elev)
	if n < 1e-9 {
		l.lx, l.ly, l.lz = 0, 0, 1
	} else {
		l.lx, l.ly, l.lz = lx/n, ly/n, l.elev/n
	}
	// Halfway between the lamp and straight up, which is where the viewer is.
	hn := math.Sqrt(l.lx*l.lx + l.ly*l.ly + (l.lz+1)*(l.lz+1))
	l.hx, l.hy, l.hz = l.lx/hn, l.ly/hn, (l.lz+1)/hn
	l.gain = flatLit / math.Max(l.lz, 1e-3)
}

// newLight draws the weather.
//
// The lamp's height and the ambient are one decision, not two numbers. A low
// sun with a high ambient is a contradiction — it renders as a flat sheet
// with long shadows on it — and a high sun with a low one is a photograph
// taken on the moon. The gloss goes with them for the same reason: an
// overcast sky is a large soft source, so its highlight is broad and weak,
// and a low sun is a small hard one.
func newLight(level string, rng *rand.Rand, l *levels) {
	switch level {
	case "raking":
		l.lit.elev = rnd.Uniform(rng, 0.22, 0.32)
		l.lit.amb = rnd.Uniform(rng, 0.34, 0.41)
		l.lit.gloss, l.lit.sharp = rnd.Uniform(rng, 0.18, 0.26), rnd.Uniform(rng, 30, 44)
		l.lit.warmth, l.lit.cool = rnd.Uniform(rng, 0.34, 0.44), rnd.Uniform(rng, 0.36, 0.46)
	case "noon":
		l.lit.elev = rnd.Uniform(rng, 0.62, 0.80)
		l.lit.amb = rnd.Uniform(rng, 0.46, 0.53)
		l.lit.gloss, l.lit.sharp = rnd.Uniform(rng, 0.12, 0.18), rnd.Uniform(rng, 18, 26)
		l.lit.warmth, l.lit.cool = rnd.Uniform(rng, 0.20, 0.30), rnd.Uniform(rng, 0.24, 0.34)
	case "overcast":
		l.lit.elev = rnd.Uniform(rng, 0.85, 0.98)
		l.lit.amb = rnd.Uniform(rng, 0.60, 0.68)
		l.lit.gloss, l.lit.sharp = rnd.Uniform(rng, 0.04, 0.08), rnd.Uniform(rng, 8, 14)
		l.lit.warmth, l.lit.cool = rnd.Uniform(rng, 0.05, 0.12), rnd.Uniform(rng, 0.12, 0.22)
	default: // morning
		l.lit.elev = rnd.Uniform(rng, 0.40, 0.54)
		l.lit.amb = rnd.Uniform(rng, 0.40, 0.47)
		l.lit.gloss, l.lit.sharp = rnd.Uniform(rng, 0.15, 0.22), rnd.Uniform(rng, 24, 34)
		l.lit.warmth, l.lit.cool = rnd.Uniform(rng, 0.28, 0.38), rnd.Uniform(rng, 0.30, 0.40)
	}
	// Up and to the left, within twenty degrees of a bearing of 135°.
	l.lit.bearing = 3*math.Pi/4 + rnd.Uniform(rng, -0.35, 0.35)
}

// height is the surface of the bed at a warped point, in canvas units.
//
// A stone is a dome over its own wall distance, and the rise is a multiple of
// the stone's *own* inradius rather than a length on the page. Pebbles are
// roughly self-similar: a big one stands proportionally as proud as a small
// one. Fixed in canvas units instead, the small stones come out as domes and
// the large ones as puddles, and a bed graded over an order of magnitude —
// which is the whole character of a river bed — reads as two materials.
func height(st *cells.Foam, l levels, u, v float64) float64 {
	h := st.At(u, v)
	span := math.Max(st.Cells()[h.Cell].Inradius, l.base)
	if math.IsInf(h.Wall, 1) {
		return l.lit.rise * span
	}
	return l.lit.rise * span * dome(h.Wall, span)
}

// dome is the profile of a rolled stone: 0 at the wall, 1 at the deepest
// point, with the steep shoulder near the edge that makes a rolled thing look
// rolled. A smoothstep here gives a pillow with flat sides, which reads as a
// button sewn to the paper.
func dome(wall, span float64) float64 {
	t := mathx.Clamp01(wall / math.Max(span, 1e-9))
	return math.Sqrt(t * (2 - t))
}

// slope is the surface's gradient at a warped point, capped.
//
// Both the cap and the reason for it are geometric. The dome's shoulder is
// vertical where it meets the wall, and a difference taken *across* a wall
// compares two different stones' domes, so the raw gradient is unbounded
// exactly at the boundary — which is where the joint is about to be drawn
// anyway. Capped, a rim comes out steep and the terminator stays a surface
// instead of collapsing to a black edge round every stone.
func slope(st *cells.Foam, l levels, u, v float64) (hx, hy float64) {
	e := l.lit.step
	hx = (height(st, l, u+e, v) - height(st, l, u-e, v)) / (2 * e)
	hy = (height(st, l, u, v+e) - height(st, l, u, v-e)) / (2 * e)
	if m := math.Hypot(hx, hy); m > l.lit.maxSlope {
		k := l.lit.maxSlope / m
		hx, hy = hx*k, hy*k
	}
	return hx, hy
}

// litFor is how much light a face at that slope receives, and how much of it
// comes straight back at the viewer.
//
// The normal of a height field is (−∂h/∂x, −∂h/∂y, 1) normalised. Nothing
// more is needed: the creases of the partition are already in the height, so
// they come out as edges of the surface without being drawn.
func (l lightLevels) litFor(gx, gy float64) (diffuse, spec float64) {
	n := math.Sqrt(gx*gx + gy*gy + 1)
	lambert := (-gx*l.lx - gy*l.ly + l.lz) / n
	diffuse = l.amb + (1-l.amb)*mathx.Clamp01(lambert*l.gain)
	// A narrow specular where the face bisects the lamp and the viewer. It is
	// the one cue that says the surface is wet and smooth rather than matte
	// paper, which is why the water axis moves it.
	blinn := (-gx*l.hx - gy*l.hy + l.hz) / n
	spec = math.Pow(mathx.Clamp01(blinn), l.sharp) * l.gloss
	return diffuse, spec
}

// illuminate lights one point.
//
// The whole sketch is the first branch. The shade is computed once per facet,
// at the facet's centroid, and held constant across it, so every facet edge
// is a hard step in value and the stone is a faceted solid. Shading the same
// surface per pixel — one line's difference, and it is the `smooth` level —
// gives soft blobs from the same light and the same colours. Flat shading is
// not an optimisation of smooth shading; it is a different picture, and it is
// the one that reads as rock.
func (s *Sketch) illuminate(sh *sheet, oh cells.Hit, wu, wv float64, col palette.Color) palette.Color {
	l := sh.level
	var diffuse, spec float64

	if f := sh.facets; f != nil && f.on[oh.Cell] {
		ih := f.foam.At(wu, wv)
		if int(f.stone[ih.Cell]) == oh.Cell {
			diffuse, spec = f.diffuse[ih.Cell], f.spec[ih.Cell]
		} else {
			// A facet straddling a joint. Its centroid is in the stone next
			// door, and the rims either side of one wall have nearly opposite
			// normals, so its flat shade would carry a lit sliver into the
			// shadowed rim it is sitting on. Shade this pixel smoothly from
			// its *own* stone's surface instead: continuous where it meets the
			// flat shading, and the same function the `smooth` level uses over
			// the whole sheet. It costs four extra lookups on the small share
			// of the bed that straddles anything.
			diffuse, spec = l.lit.litFor(slope(sh.stones, l, wu, wv))
		}
		diffuse *= f.crease(ih, l)
	} else {
		diffuse, spec = l.lit.litFor(slope(sh.stones, l, wu, wv))
	}
	return s.applyLight(sh, col, diffuse, spec)
}

// applyLight puts the light on the paint rather than mixing it in.
//
// Multiplying is what keeps the colour scheme's value structure: a pigment
// the scheme made dark stays dark on the lit side, so "this stone is dark"
// and "this face is turned away" remain two readable facts. Lerping the
// painted colour toward a light and a dark instead collapses them into one,
// and the sheet comes out as a single colour in relief.
func (s *Sketch) applyLight(sh *sheet, col palette.Color, diffuse, spec float64) palette.Color {
	l := sh.level.lit
	out := shade(col, diffuse)
	span := math.Max(1-l.amb, 1e-6)

	// What the shadow takes away leans toward the sky's colour, because a face
	// lit only by the sky is bluer than the thing turning away from the sun.
	//
	// The sky colour is matched to the value the point already has before it
	// is mixed in, and that is not a nicety. `out` has been multiplied by the
	// diffuse already; lerping it toward a colour dark in its own right takes
	// the light away a second time, and the shadowed half of every stone
	// collapses into the joint it is sitting in. Only the hue is borrowed.
	if k := mathx.Clamp01((1 - diffuse) / span); k > 0 && l.cool > 0 {
		out = palette.Lerp(out, atValue(sh.ink.cool, out), k*l.cool)
	}
	// And what the light adds leans toward the lamp's. Squared, so only the
	// faces genuinely square to it take the colour — linear, every stone
	// drifts warm at once and the sheet reads as a colour cast rather than as
	// a lit surface.
	if k := mathx.Clamp01((diffuse - l.amb) / span); k > 0 && l.warmth > 0 {
		out = palette.Lerp(out, sh.ink.warm, k*k*l.warmth)
	}
	if spec <= 0 {
		return out
	}
	// Added rather than mixed, and in the lamp's own colour: a highlight is
	// light arriving on top of the pigment, so it has to be able to go past
	// the pigment's own value. Mixed in, a wet stone's gleam is capped by how
	// pale the stone already is, and the dark stones — the ones a gleam
	// actually reads on — never get one.
	w := sh.ink.warm
	return palette.Color{
		R: out.R + spec*w.R,
		G: out.G + spec*w.G,
		B: out.B + spec*w.B,
	}.Clamp()
}

// atValue is a colour rescaled to the luminance of another: the first one's
// hue and saturation, the second one's weight. It is how a lean stays a lean.
func atValue(c, ref palette.Color) palette.Color {
	lum := c.Luminance()
	if lum < 1e-4 {
		return ref
	}
	return shade(c, ref.Luminance()/lum)
}
