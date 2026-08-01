package foam

// The watercolour layer.
//
// A wash is not a colour, it is a *quantity of pigment* that dried
// somewhere. Everything that makes one read as watercolour — the dark ring
// at the edge, the pale hole a backrun leaves, the grit sitting in the
// paper's tooth, a second pigment bleeding into the first — is a variation
// in how much pigment reached a point, composited as absorption in linear
// light (paint.Glaze). So this file computes one number, the *load*, and
// hands it to the pigment model. Nothing here mixes colours by lerping
// them; two pigments in one cell are two loads, and their meeting is a
// third colour because absorption stacks.
//
// Why this and not paint.Wash
// ---------------------------
// paint.Wash is the repo's existing watercolour, built for sketch 008, and
// it is not reusable here for two reasons that are both about *shape*.
//
//   - It is stamp-based: it writes pixels into a paint.Canvas, in pixel
//     coordinates, sequentially. Sketch 009 is a pure per-pixel function
//     (sketch.Raster), which is what makes it resolution-independent by
//     construction — preview and print of a seed are the same picture. Using
//     Wash would mean giving that up for the whole sketch.
//   - It is radial: a pool is a star-shaped blob described by one radius per
//     angle. A foam cell is an arbitrary curved region and frequently a
//     concave lobe, which no radius table can express. The silhouette is
//     most of what Wash *is*, so what would be left to reuse is the pigment
//     maths.
//
// And the thing Wash has to synthesise — where the wet edge is — a foam cell
// already has, exactly, for free: cells.Hit.Wall is a real signed distance
// to the cell's own boundary, whatever its shape. So the rim, the overshoot,
// the bleed and the backrun front are all one-line functions of Wall here,
// and they work in a crescent because Wall does.
//
// The pigment maths *is* reused, as paint.Glaze: the continuous limit of
// Wash's deposit stack, so a foam wash and a pools wash of the same pigment
// agree about what that pigment looks like.

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// Salts keep the granulation lattice and the wet-pair table independent of
// the seed's other uses.
const (
	saltTooth = 0x746f6f74 // "toot"
	saltFine  = 0x67726974 // "grit"
	saltWet   = 0x77657421 // "wet!"
)

// The manners: what happened in one cell while it was drying. Each is a
// modulation of the load, so they all obey the same edge, the same rim and
// the same granulation — a cell is not a different medium because it
// bloomed.
const (
	mannerFlat    = iota // one pigment laid once and left to dry
	mannerCharged        // a second pigment dropped into the wet first
	mannerBloom          // a backrun: water ran back in and pushed the pigment into a ridge
	mannerGlaze          // a second transparent layer over part of the cell
	nmanners
)

// waterLevels is what the water trait resolves to. Ranges, not numbers, for
// the reason set out in the spec: a level is a kind of sheet.
type waterLevels struct {
	manners [nmanners]float64 // relative weight of each manner
	load    float64           // centre of the pigment load
	spread  float64           // how far the load swings with a cell's tone
	over    float64           // registration error of the paint against the line, ×ink
	grain   float64           // granulation
	bleed   float64           // share of walls whose two cells were wet together
	seep    float64           // how far a bleed carries, ×the cell's own size
}

// The water levels. Each is one of the things watercolour does.
const (
	waterPlain  = "plain"       // flat washes, done properly: rim, registration, pooling
	waterCharge = "charged"     // wet-in-wet: two pigments meeting on a fingered front
	waterBloom  = "blooms"      // backruns — the cauliflower edge of a wash that dried twice
	waterGlaze  = "glazed"      // a second transparent layer deepening part of a cell
	waterSed    = "sedimentary" // heavy pigment settling into the paper's tooth
	waterBled   = "bled"        // neighbouring cells painted wet, mixing across the wall
	waterStudio = "studio"      // all of it at once, at working strength
)

// newWater draws the watercolour behaviour for one level.
func newWater(level string, rng *rand.Rand, l *levels) {
	w := waterLevels{
		load:   rnd.Uniform(rng, 1.5, 2.2),
		spread: rnd.Uniform(rng, 0.45, 0.7),
		over:   rnd.Uniform(rng, 0.9, 1.9),
		grain:  rnd.Uniform(rng, 0.25, 0.5),
		bleed:  0.06,
		seep:   rnd.Uniform(rng, 0.2, 0.4),
	}
	switch level {
	case waterCharge:
		w.manners = [nmanners]float64{mannerFlat: 3, mannerCharged: 10}
	case waterBloom:
		w.manners = [nmanners]float64{mannerFlat: 4, mannerCharged: 2, mannerBloom: 9}
	case waterGlaze:
		w.manners = [nmanners]float64{mannerFlat: 4, mannerGlaze: 9}
	case waterSed:
		w.manners = [nmanners]float64{mannerFlat: 7, mannerCharged: 2}
		// A sedimentary pigment is a coarse one, and it needs to be laid
		// thickly enough for the grains to have somewhere to fall.
		w.grain = rnd.Uniform(rng, 1.0, 1.5)
		w.load = rnd.Uniform(rng, 2.0, 2.9)
	case waterBled:
		w.manners = [nmanners]float64{mannerFlat: 6, mannerCharged: 3}
		w.bleed = rnd.Uniform(rng, 0.5, 0.75)
		w.seep = rnd.Uniform(rng, 0.45, 0.85)
	case waterStudio:
		w.manners = [nmanners]float64{mannerFlat: 5, mannerCharged: 4, mannerBloom: 3, mannerGlaze: 3}
		w.bleed = rnd.Uniform(rng, 0.15, 0.35)
	default: // plain
		w.manners = [nmanners]float64{mannerFlat: 1}
	}
	l.water = w
}

// waterDress is one cell's paint: settled before a pixel is drawn, so the
// per-pixel work is arithmetic on a struct.
type waterDress struct {
	manner  int
	second  palette.Color // the charged-in or glazed-over pigment
	load    float64       // pigment laid over the body of the cell
	glaze   float64       // load of the second layer, where there is one
	reach   float64       // signed offset of the paint's edge from the ink line
	wander  float64       // amplitude of that edge's wander, canvas units
	soft    float64       // how softly the paint's edge fades, canvas units
	rim     float64       // strength of the dried rim
	rimWide float64       // how far in from the paint's edge the rim reaches
	grain   float64       // granulation
	seep    float64       // how far this cell's paint carries into a wet neighbour
	scale   float64       // the cell's own size — the wavelength of its detail
	cx, cy  float64       // the cell's centroid, for the directed fronts
	cos     float64       // direction of the wet-in-wet front and the glaze edge
	sin     float64
	front   float64 // where along that direction the front crosses
	blevel  float64 // wall distance the backrun's front stopped at
	bamp    float64 // how badly that front is broken up
	tilt    float64 // how far the backrun leans off the cell's middle
}

// dressWater settles one cell's paint.
func (s *Sketch) dressWater(c cells.Cell, l levels, rng *rand.Rand, second palette.Color, tone float64) waterDress {
	wl := l.water
	// A cell's own size is the wavelength everything inside it is measured
	// against: a bloom the size of the sheet in a sliver is a smudge, and a
	// front that crosses in a tenth of a sliver never shows in a big lobe.
	scale := math.Max(c.Inradius, l.base*0.4)

	w := waterDress{
		manner: rnd.PickIndex(rng, wl.manners[:]),
		second: second,
		scale:  scale,
		cx:     c.CX,
		cy:     c.CY,
	}
	// Tone is the colour scheme's say in how heavily this cell was loaded —
	// which is what lets a scheme build a value structure rather than only a
	// hue arrangement.
	w.load = math.Max(wl.load*(1+wl.spread*(tone-0.5)*2)*rnd.Uniform(rng, 0.88, 1.12), 0.08)
	w.glaze = w.load * rnd.Uniform(rng, 0.65, 1.15)

	// Registration. A hand-painted cell almost never meets its line exactly:
	// the wash runs a little past it, or stops short and leaves a rind of
	// white paper. Centred slightly positive, because paint under a drawn
	// line is the commoner failure, and capped so a bad draw cannot shrink a
	// small cell to nothing.
	w.reach = math.Max(l.ink*wl.over*rnd.Gauss(rng, 0.2, 1.0), -0.45*scale)
	w.wander = l.ink * rnd.Uniform(rng, 0.4, 1.3)
	w.soft = math.Max(l.ink*rnd.Uniform(rng, 0.3, 0.9), 0.0003)

	w.rim = s.Rim * rnd.Uniform(rng, 0.55, 1.5)
	// The rim is a property of the drying pool, so it is an absolute width —
	// but it can never be more than a fraction of the cell, or a sliver is a
	// solid dark chip and the cue that reads as watercolour on a big cell
	// reads as ink on a small one. This is the whole of what was wrong with
	// the first draft's rim.
	w.rimWide = math.Min(s.RimWide, scale*0.45) * rnd.Uniform(rng, 0.7, 1.35)

	w.grain = wl.grain * rnd.Uniform(rng, 0.5, 1.5)
	w.seep = scale * wl.seep * rnd.Uniform(rng, 0.55, 1.5)

	a := rng.Float64() * 2 * math.Pi
	w.cos, w.sin = math.Cos(a), math.Sin(a)
	w.front = rnd.Uniform(rng, -0.4, 0.4) * scale
	w.blevel = scale * rnd.Uniform(rng, 0.30, 0.58)
	w.bamp = scale * rnd.Uniform(rng, 0.18, 0.38)
	w.tilt = rnd.Uniform(rng, 0.25, 0.65)
	return w
}

// watercolour paints one point of a wash cell over whatever ground it finds.
//
// The whole function is a load: how much pigment reached this point. The
// manner, the rim, the pooling and the tooth all multiply it, and only at
// the very end does it become a colour.
func (s *Sketch) watercolour(d dress, wall float64, field *noise.Perlin, seed uint64, u, v float64, ground palette.Color) palette.Color {
	w := d.water
	cov, edge := s.wetEdge(w, wall, field, u, v)
	if cov <= 0.001 {
		return ground
	}
	load := w.load * cov

	// Pooling, at two scales. The broad one is the sheet's — the paper was
	// wetter here than there, and a whole passage of cells shares it, which
	// is what stops the unevenness reading as per-cell texture. The fine one
	// is the cell's own puddle.
	load *= 1 + s.Blotch*(1.7*field.FBM(u/s.Mottle, v/s.Mottle, 3)+
		2.1*field.FBM(u/(w.scale*1.4)+13.7, v/(w.scale*1.4)-7.1, 2))

	// The rim: pigment carried to the wet edge as the water retreated and
	// left there. Measured from the *paint's* edge, not from the ink line —
	// a wash that overshot carries its rim out past the line with it, which
	// is exactly the registration failure that reads as hand-painted.
	//
	// A ridge peaking a little way inside, not a ramp climbing to the edge:
	// coverage is already falling away there, so a rim that only rises
	// toward the boundary multiplies a number on its way to zero.
	if w.rim > 0 && !math.IsInf(edge, -1) {
		t := (wall - edge) / w.rimWide
		// Not evenly all the way round. Where the paper was wetter the
		// pigment diffused instead of being carried to a boundary, and a
		// wash that rims evenly reads as an outlined shape rather than a
		// dried pool. This is the cue the first draft got most wrong: its
		// rim was a plain smoothstep on the wall distance, which is a drawn
		// outline in everything but name.
		crisp := mathx.Clamp01(0.5 + 2.4*field.At(u/(w.scale*2.3)+31.3, v/(w.scale*2.3)+17.9))
		load *= 1 + w.rim*(0.25+0.9*crisp)*math.Exp(-6*(t-0.55)*(t-0.55))
	}

	pig, second := d.pigment, w.second
	var glaze float64
	switch w.manner {
	case mannerCharged:
		// Wet-in-wet. Two pigments dropped into one puddle do not meet on a
		// line: the front between them is fingered at every scale, and where
		// they overlap both are present. So this is two loads, not a lerp,
		// and the middle is a genuine third colour because the absorptions
		// stack.
		f, _ := s.frontAt(w, field, u, v, w.scale*0.75)
		// A little extra in the meeting zone: pigment collects where two wet
		// edges push against each other.
		both := load * 0.22 * 4 * f * (1 - f)
		glaze = load*f + both/2
		load = load*(1-f) + both/2
	case mannerBloom:
		load *= bloom(w, wall, field, u, v)
	case mannerGlaze:
		// A second transparent layer over part of the cell, with its own wet
		// edge and therefore its own rim. Where it lies over the first the
		// two loads add, which is what deepens the overlap without changing
		// its hue the way a lerp would.
		g, sd := s.frontAt(w, field, u, v, w.scale*0.06)
		glaze = w.glaze * g * cov
		if glaze > 0 {
			gw := w.rimWide
			t := (sd - 0.5*gw) / gw
			glaze *= 1 + 1.2*w.rim*math.Exp(-t*t)
		}
	}

	// Granulation: heavy pigment settling into the paper's tooth. Gated on
	// how much pigment is there, because grains need pigment to be grains
	// of — ungated it covers bare paper and dense wash alike and reads as
	// film grain laid over the picture rather than as grit inside it.
	if w.grain > 0 {
		// In patches, not evenly. A granulating pigment settles where the
		// water stood longest, so the grain comes and goes across a cell;
		// applied at one strength everywhere it reads as sandpaper laid over
		// the picture, which is what the first version looked like.
		patch := 0.3 + 0.7*mathx.Clamp01(0.5+2.2*field.FBM(u/(w.scale*2.2)+61.3, v/(w.scale*2.2)-44.1, 2))
		g := 0.55 * w.grain * patch * math.Pow(mathx.Clamp01(load/math.Max(w.load, 1e-6)), 0.8)
		t := 2*toothAt(seed, u, v, s.Tooth) - 1
		load *= 1 + g*t
		glaze *= 1 + g*t
	}

	col := paint.Glaze(ground, pig, math.Max(load, 0), s.Scatter)
	if glaze > 0 {
		col = paint.Glaze(col, second, glaze, s.Scatter)
	}
	return col
}

// wetEdge is the paint's own boundary: how much of the pigment reached this
// point, and where — in wall-distance — that boundary sits.
//
// It is deliberately *not* the ink line. reach offsets it (positive runs the
// paint past the line, negative leaves a rind of white paper inside it) and
// a noise field makes the offset wander, so the paint and the drawing fail
// to register the way a hand's do.
func (s *Sketch) wetEdge(w waterDress, wall float64, field *noise.Perlin, u, v float64) (cover, edge float64) {
	if math.IsInf(wall, 1) {
		return 1, math.Inf(-1)
	}
	edge = -w.reach + 3*w.wander*field.FBM(u/(w.scale*0.8)+3.1, v/(w.scale*0.8)+11.7, 3)
	return mathx.Smoothstep(edge-w.soft, edge+w.soft, wall), edge
}

// frontAt is a directed boundary crossing the cell, fingered by noise: the
// wet-in-wet front and the glaze's edge are both one. ramp is how sharply it
// turns over, which is the difference between two pigments diffusing into
// each other and a second layer laid with a brush.
//
// It returns the crossing as a fraction in [0,1] and as a signed distance in
// canvas units, because a glaze needs the distance to put a rim on its own
// edge.
func (s *Sketch) frontAt(w waterDress, field *noise.Perlin, u, v, ramp float64) (t, signed float64) {
	signed = (u-w.cx)*w.cos + (v-w.cy)*w.sin - w.front
	signed += 3 * w.scale * 0.32 * field.FBM(u/(w.scale*1.1)+5.3, v/(w.scale*1.1)-2.7, 3)
	return mathx.Clamp01(0.5 + signed/math.Max(ramp*2, 1e-6)), signed
}

// bloom is a backrun — the "cauliflower" a wash gets when water runs back
// into it while it is drying. It is the most recognisable watercolour
// accident there is and almost never simulated.
//
// The physics is that the returning water dissolves the settled pigment,
// carries it outward and abandons it at the front where it finally stops.
// So the shape has two parts, and both matter: a *pale* interior, and a hard
// scalloped ridge at the boundary where all that pigment ended up. A pale
// blob alone reads as a lifted highlight; the ridge is the cue.
//
// The front is a level set of the cell's own wall distance, tilted off
// centre and badly broken up. That is what makes it work in a lobe: the
// backrun spreads from the wet edge inward, so its front follows the shape
// of the cell it is in, whatever that shape is.
func bloom(w waterDress, wall float64, field *noise.Perlin, u, v float64) float64 {
	if math.IsInf(wall, 1) {
		return 1
	}
	q := wall - w.blevel
	// The front is warped at two well-separated scales, and that separation
	// is the whole difference between a backrun and lichen. One broad term
	// makes the front *lobed* — it reaches into part of the cell and not the
	// rest — and one fine term scallops the edge of those lobes. Warped at
	// one middling scale, which is what this did first, the front fragments
	// into a spatter of small patches and the cell reads as mould.
	q += 3 * w.bamp * field.FBM(u/(w.scale*1.6)+21.1, v/(w.scale*1.6)+9.4, 2)
	q += 3 * w.bamp * 0.3 * field.FBM(u/(w.scale*0.34)+4.7, v/(w.scale*0.34)-8.2, 2)
	q += w.tilt * ((u-w.cx)*w.cos + (v-w.cy)*w.sin)

	edge := math.Max(w.scale*0.07, 1e-5)
	// Out of the middle: the returning water dissolved what had settled
	// there and carried it away, so a backrun's interior is paler than the
	// wash around it.
	f := 1 - 0.62*mathx.Smoothstep(0, edge*2.2, q)
	// ...and into a ridge just outside the front it stopped at. Narrow, and
	// hard on its outer side: a backrun's edge is the one crisp thing a wash
	// does by accident, and softening it loses the whole effect.
	x := (q + edge*0.85) / edge
	return f * (1 + 2.1*math.Exp(-x*x))
}

// toothAt is the paper's tooth for granulation: cell noise at two
// incommensurate scales, in canvas units so preview and print granulate
// identically.
//
// Cell noise rather than a value lattice, for the reason water.go gives:
// any noise whose extrema sit on a lattice lines its maxima up into rows,
// and a large cell then reads as a grid of soft pixels rather than as paper.
func toothAt(seed uint64, u, v, cell float64) float64 {
	f1, _ := noise.Worley(seed^saltTooth, u/cell, v/cell)
	g1, _ := noise.Worley(seed^saltFine, u/(cell*2.37), v/(cell*2.37))
	return mathx.Clamp01(0.62*f1/0.9 + 0.38*g1/0.9)
}

// paint is the pixel function for a sheet: the cell's own fill, and then
// whatever its neighbour's paint did across the wall.
//
// Both things that cross a wall are the same mechanism. A wash that
// overshoots its line covers a strip of the next cell; two cells painted
// while both were wet exchange pigment over a much deeper and much softer
// front. So the neighbour is painted with its own dressing, evaluated at the
// *mirrored* wall distance — this point is as far outside the neighbour as
// it is inside its own cell — and the difference between overshoot and bleed
// is only how far the paint reaches and how softly it stops.
//
// It costs a second wash evaluation, and only within the neighbour's reach
// of a wall.
func (s *Sketch) paint(sh *sheet, h cells.Hit, seed uint64, u, v float64) palette.Color {
	col := s.fill(sh.skin[h.Cell], h, sh.field, seed, u, v, sh.paper)
	if h.Next < 0 || math.IsInf(h.Wall, 1) {
		return col
	}
	nb, ok := crossing(sh, seed, h.Cell, h.Next)
	if !ok {
		return col
	}
	return s.watercolour(nb, -h.Wall, sh.field, seed, u, v, col)
}

// crossing is the neighbour's paint as it arrives in this cell: its own
// dressing, with the wall's wetness folded in. It reports false when nothing
// crosses at all.
func crossing(sh *sheet, seed uint64, own, next int) (dress, bool) {
	nb := sh.skin[next]
	if nb.style != styleWash {
		return nb, false
	}
	// Only some pairs were wet together — a sheet where every wall bleeds is
	// a sheet that was painted all at once, which no one does.
	if wetPair(seed, own, next) < sh.level.water.bleed {
		nb.water.reach += nb.water.seep
		// A bleed is not an overshoot with a longer reach: it is pigment
		// diffusing rather than a boundary in the wrong place, so its front
		// is soft and ragged over the whole depth it travels, and it has no
		// rim at all. A rim is what a wash leaves where it *stopped*.
		nb.water.soft = math.Max(nb.water.soft, nb.water.seep*0.55)
		nb.water.wander = math.Max(nb.water.wander, nb.water.seep*0.75)
		nb.water.rim *= 0.2
	}
	return nb, nb.water.reach > 0
}

// depth is how far into the next cell a crossing could possibly carry: the
// offset of the paint's edge, plus everything that makes that edge wander.
// The wander is an unnormalised fBm, whose four octaves can reach 1.875.
func (w waterDress) depth() float64 {
	return w.reach + 3*1.875*w.wander + w.soft
}

// wetPair is a stable draw per wall: the same for both cells, and unrelated
// to how the sheet was dressed. A wall either was painted wet on both sides
// or it was not, and that has to be a property of the pair rather than of
// whichever cell is asking.
func wetPair(seed uint64, a, b int) float64 {
	if a > b {
		a, b = b, a
	}
	return noise.Hash01(seed^saltWet, int64(a), int64(b))
}
