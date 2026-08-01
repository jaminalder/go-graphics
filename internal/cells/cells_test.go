package cells

import (
	"math"
	"math/rand/v2"
	"testing"
)

func rngFor(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 7)) }

// scatter lays n sites over [0,aspect]×[0,1] with varied weights.
func scatter(rng *rand.Rand, n int, aspect float64) []Site {
	s := make([]Site, n)
	for i := range s {
		s[i] = Site{
			X: rng.Float64() * aspect,
			Y: rng.Float64(),
			W: 0.01 + 0.05*rng.Float64(),
		}
	}
	return s
}

// slowAt is the definition the grid walk has to match: scan every site.
func slowAt(sites []Site, group []int, p Params, u, v float64) Hit {
	var nb near
	for i, s := range sites {
		nb.add(group[i], math.Hypot(s.X-u, s.Y-v)-s.W)
	}
	h := Hit{Cell: nb.cell[0], Wall: math.Inf(1)}
	if nb.n < 2 {
		return h
	}
	h.Wall = (nb.dist[1] - nb.dist[0]) / 2
	crowd, soft := 1.0, 0.0
	for k := 2; k < nb.n; k++ {
		crowd += math.Exp(-(nb.dist[k] - nb.dist[1]) / p.Round)
		soft += math.Exp(-(nb.dist[k] - nb.dist[1]) / p.Node)
	}
	h.Wall -= p.Round * math.Log(crowd) / 2
	h.Node = 1 - math.Exp(-soft)
	return h
}

// TestTheGridWalkFindsWhatABruteForceScanFinds is the load-bearing test of
// the package: At visits buckets in expanding rings and stops early, and a
// stopping rule that is a hair too eager silently returns the wrong cell —
// which shows up as a cell boundary in the wrong place, not as a crash.
func TestTheGridWalkFindsWhatABruteForceScanFinds(t *testing.T) {
	p := DefaultParams()
	for _, n := range []int{2, 3, 12, 80, 300} {
		rng := rngFor(uint64(n))
		sites := scatter(rng, n, 1.25)
		group := Identity(n)
		f := New(sites, group, 1.25, p)
		for range 2000 {
			u, v := rng.Float64()*1.25, rng.Float64()
			got, want := f.At(u, v), slowAt(sites, group, p, u, v)
			if got.Cell != want.Cell {
				t.Fatalf("n=%d at (%.4f,%.4f): cell %d, want %d", n, u, v, got.Cell, want.Cell)
			}
			// Wall carries the same search-cutoff approximation as Node —
			// through the corner rounding, which is a sum over the same
			// cells. The cell, which decides what is drawn where, is exact.
			if math.Abs(got.Wall-want.Wall) > 1e-4 && !(math.IsInf(got.Wall, 1) && math.IsInf(want.Wall, 1)) {
				t.Fatalf("n=%d at (%.4f,%.4f): wall %v, want %v", n, u, v, got.Wall, want.Wall)
			}
			// Node is exact only up to the search cutoff: a cell more than
			// six node-lengths behind the second may or may not have been
			// visited, and contributes at most e⁻⁶ either way. The cell and
			// the wall, which decide what is drawn where, are exact.
			if math.Abs(got.Node-want.Node) > 3e-3 {
				t.Fatalf("n=%d at (%.4f,%.4f): node %v, want %v", n, u, v, got.Node, want.Node)
			}
		}
	}
}

// TestWallDistanceVanishesAtTheBoundary pins the field every fill depends
// on. If Wall does not reach 0 where the cell changes, the ink is drawn off
// the boundary and fills leak past their own edge.
func TestWallDistanceVanishesAtTheBoundary(t *testing.T) {
	sites := []Site{{X: 0.3, Y: 0.5, W: 0.02}, {X: 0.7, Y: 0.5, W: 0.02}}
	f := New(sites, Identity(2), 1, DefaultParams())

	prev := f.At(0.3, 0.5).Cell
	crossed := false
	for x := 0.3; x <= 0.7; x += 0.0005 {
		h := f.At(x, 0.5)
		if h.Cell != prev {
			crossed = true
			if h.Wall > 0.001 {
				t.Fatalf("cell changed at x=%.4f with wall distance %v — the boundary is not where Wall says it is", x, h.Wall)
			}
		}
		prev = h.Cell
	}
	if !crossed {
		t.Fatal("never left the first cell")
	}
}

// TestJunctionsReadAsJunctions is what swells the ink where three cells
// meet. Node must be high at the meeting point and near zero halfway along
// a wall; without that separation the line has a constant width and the
// foam reads as a cracked pane rather than a bubble cluster.
//
// The values are the smooth crowding measure, not a distance ratio: one
// cell sitting exactly alongside the second contributes a full vote, which
// saturates to 1 − e⁻¹.
func TestJunctionsReadAsJunctions(t *testing.T) {
	// Three equal sites on a circle: their meeting point is the centre.
	const r = 0.25
	var sites []Site
	for i := range 3 {
		a := float64(i)*2*math.Pi/3 + 0.3
		sites = append(sites, Site{X: 0.5 + r*math.Cos(a), Y: 0.5 + r*math.Sin(a), W: 0.02})
	}
	f := New(sites, Identity(3), 1, DefaultParams())

	if got := f.At(0.5, 0.5).Node; got < 0.6 {
		t.Errorf("node at the meeting point is %.3f, want ~0.63", got)
	}
	// Halfway between two sites, well away from the centre, only two cells
	// are close.
	mx, my := (sites[0].X+sites[1].X)/2, (sites[0].Y+sites[1].Y)/2
	// Push outward, away from the junction at the centre.
	ox, oy := mx-0.5, my-0.5
	l := math.Hypot(ox, oy)
	mx, my = mx+ox/l*0.22, my+oy/l*0.22
	if got := f.At(mx, my).Node; got > 0.05 {
		t.Errorf("node mid-wall is %.3f, want ~0 — the ink will not taper", got)
	}
}

// TestCornersAreRounded is what stops a cell reading as a polygon. Three
// cells meet at 120°, so a cell of a foam has corners; a hard minimum over
// the neighbours leaves them sharp, and a sheet of sharp-cornered cells
// reads as a cracked pane however well the walls between the corners curve.
//
// The rounding has to be local: a straight run of wall, where only one
// other cell is anywhere near, must come out untouched.
func TestCornersAreRounded(t *testing.T) {
	const r = 0.25
	var sites []Site
	for i := range 3 {
		a := float64(i)*2*math.Pi/3 + 0.3
		sites = append(sites, Site{X: 0.5 + r*math.Cos(a), Y: 0.5 + r*math.Sin(a), W: 0.02})
	}
	sharp := New(sites, Identity(3), 1, Params{Node: 0.05, Round: 0, Stat: 64})
	round := New(sites, Identity(3), 1, Params{Node: 0.05, Round: 0.03, Stat: 64})

	// Just inside a cell, a little way out from the junction at the centre:
	// the corner. Rounding must pull the wall in toward the site.
	cx, cy := 0.5+0.06*math.Cos(0.3), 0.5+0.06*math.Sin(0.3)
	a, b := sharp.At(cx, cy).Wall, round.At(cx, cy).Wall
	if b >= a {
		t.Errorf("at the corner the wall is %.5f rounded against %.5f sharp — nothing was rounded off", b, a)
	}

	// Mid-wall, well away from the junction, where only one other cell is
	// near. The soft minimum has a single term there and must equal the hard
	// one, or the rounding is thinning every wall rather than its corners.
	mx, my := (sites[0].X+sites[1].X)/2, (sites[0].Y+sites[1].Y)/2
	ox, oy := mx-0.5, my-0.5
	l := math.Hypot(ox, oy)
	mx, my = mx+ox/l*0.22, my+oy/l*0.22
	a, b = sharp.At(mx, my).Wall, round.At(mx, my).Wall
	// Under a hundredth of the rounding radius, against the σ·ln2/2 it
	// takes off at the corner.
	if math.Abs(a-b) > 0.01*0.03 {
		t.Errorf("mid-wall the rounding moved the wall from %.6f to %.6f", a, b)
	}
}

// TestAHeavierSiteClaimsMoreGround is why the metric is additively weighted
// at all: weight is the only lever on relative cell size, and it is what
// makes the walls curve. Equal weights give a crystalline Voronoi.
func TestAHeavierSiteClaimsMoreGround(t *testing.T) {
	sites := []Site{{X: 0.3, Y: 0.5, W: 0.12}, {X: 0.7, Y: 0.5, W: 0.01}}
	f := New(sites, Identity(2), 1, DefaultParams())
	heavy, light := f.Cells()[0].Area, f.Cells()[1].Area
	if heavy <= light {
		t.Errorf("heavy cell holds %.3f of the sheet against the light cell's %.3f", heavy, light)
	}
	// And the wall between them bulges toward the light site rather than
	// sitting on the midline.
	if h := f.At(0.5, 0.5); h.Cell != 0 {
		t.Error("the midpoint belongs to the light site — the weight is not being applied")
	}
}

// TestMergedSitesAreOneRegionWithNoWallInside is what makes a lobe a lobe.
// Merging must remove the shared wall entirely, not hide it: a fill, a rim
// or a band pattern that still sees an interior boundary would draw the
// seam the merge was supposed to erase.
func TestMergedSitesAreOneRegionWithNoWallInside(t *testing.T) {
	sites := []Site{
		{X: 0.3, Y: 0.5, W: 0.03},
		{X: 0.45, Y: 0.5, W: 0.03},
		{X: 0.85, Y: 0.5, W: 0.03},
		{X: 0.6, Y: 0.15, W: 0.03},
	}
	group := []int{0, 0, 1, 2}
	f := New(sites, group, 1, DefaultParams())

	if got := f.Cells()[0].Sites; got != 2 {
		t.Fatalf("the lobe reports %d sites, want 2", got)
	}
	minWall := math.Inf(1)
	for x := 0.3; x <= 0.45; x += 0.001 {
		h := f.At(x, 0.5)
		if h.Cell != 0 {
			t.Fatalf("x=%.3f falls in cell %d — the merged sites are not one region", x, h.Cell)
		}
		minWall = math.Min(minWall, h.Wall)
	}
	// Between the two merged sites the nearest *other* cell is far away, so
	// the wall distance stays large the whole way across.
	if minWall < 0.1 {
		t.Errorf("wall distance drops to %.4f between the merged sites — an interior seam survived", minWall)
	}
}

// TestMergeJoinsNeighboursAndNothingElse guards the shape of a lobe. Merging
// two distant sites would leave a cell in two disconnected pieces, which the
// metric cannot express and which a fill cannot draw.
func TestMergeJoinsNeighboursAndNothingElse(t *testing.T) {
	rng := rngFor(11)
	sites := scatter(rng, 60, 1)
	group := Merge(rngFor(11), sites, 1, 2)

	members := map[int][]int{}
	for i, g := range group {
		members[g] = append(members[g], i)
	}
	// The mean distance between merged partners, against the mean distance
	// between arbitrary pairs. Neighbours must be dramatically closer.
	pairs, sum := 0, 0.0
	for _, m := range members {
		if len(m) != 2 {
			continue
		}
		sum += math.Hypot(sites[m[0]].X-sites[m[1]].X, sites[m[0]].Y-sites[m[1]].Y)
		pairs++
	}
	if pairs < 5 {
		t.Fatalf("only %d lobes out of 60 sites at share 1", pairs)
	}
	if mean := sum / float64(pairs); mean > 0.1 {
		t.Errorf("merged partners average %.3f apart — that is not a neighbour", mean)
	}
	for g, m := range members {
		if len(m) > 2 {
			t.Errorf("cell %d absorbed %d sites against a cap of 2", g, len(m))
		}
	}
}

// TestMergeIsDeterministic — invariant 1. Merging walks sites in index
// order for exactly this reason; ranging over a map would make the same
// seed draw a different picture on every run.
func TestMergeIsDeterministic(t *testing.T) {
	sites := scatter(rngFor(3), 40, 1)
	a := Merge(rngFor(9), sites, 0.5, 3)
	b := Merge(rngFor(9), sites, 0.5, 3)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("site %d landed in cell %d then %d", i, a[i], b[i])
		}
	}
}

// TestEveryCellIsMeasured — a cell with no area is one a neighbour swallowed
// or one that fell off the frame, and a sketch has to be able to tell those
// from the rest. On an evenly packed sheet nearly all of them should have
// real area, and the areas should account for the whole canvas.
func TestEveryCellIsMeasured(t *testing.T) {
	p := DefaultParams()
	sites := scatter(rngFor(5), 50, 1.2)
	f := New(sites, Identity(50), 1.2, p)
	total, empty := 0.0, 0
	for _, c := range f.Cells() {
		total += c.Area
		if c.Area == 0 {
			empty++
			continue
		}
		// A cell can hold area and still have no interior: the rounding is
		// taken off the wall distance, so a cell smaller than the rounding
		// radius lies entirely inside its own corners. Anything comfortably
		// larger than that has to have somewhere to put a fill.
		if c.Inradius <= 0 && c.Area > 4*p.Round*p.Round {
			t.Errorf("cell %d has area %.4f but no inradius", c.ID, c.Area)
		}
		if c.CX < 0 || c.CX > 1.2 || c.CY < 0 || c.CY > 1 {
			t.Errorf("cell %d centroid (%.3f,%.3f) is off the canvas", c.ID, c.CX, c.CY)
		}
	}
	if math.Abs(total-1) > 1e-9 {
		t.Errorf("areas sum to %.6f, want 1", total)
	}
	if empty > 5 {
		t.Errorf("%d of 50 cells were swallowed", empty)
	}
}
