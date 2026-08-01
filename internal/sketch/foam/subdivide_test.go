package foam

import (
	"math"
	"testing"
)

// TestNoSeedIsSubdividedOrLit. Both new dimensions carry weight 0
// throughout, so the sheet a seed draws is exactly the sheet it drew before
// the mosaic existed. If this fails, every finished piece of sketch 009 has
// silently become a different picture.
func TestNoSeedIsSubdividedOrLit(t *testing.T) {
	s := configured(t)
	for seed := uint64(1); seed <= 80; seed++ {
		tr := s.Traits(testCtx(t, seed))
		if got := tr.Get(dimMosaic); got != "plain" {
			t.Fatalf("seed %d drew mosaic %q, which carries weight 0", seed, got)
		}
		if got := tr.Get(dimRelief); got != "flat" {
			t.Fatalf("seed %d drew relief %q, which carries weight 0", seed, got)
		}
	}
	if sh := planned(t, s, 7); sh.tiles != nil {
		t.Error("a plain sheet built a mosaic layer")
	}
}

// TestEveryCellIsDividedIntoComparablyManyTiles is the claim the
// variable-radius pack exists to defend. One global site spacing would give
// a quarter-canvas lobe fifty tiles and a sliver one; sizing each dart from
// the inradius of the outer cell it lands in is what makes the subdivision
// a property of the cell rather than of the page.
func TestEveryCellIsDividedIntoComparablyManyTiles(t *testing.T) {
	s := configured(t, "--mosaic", "tonal", "--density", "medium", "--tiled", "1")
	sh := planned(t, s, 5)
	if sh.tiles == nil {
		t.Fatal("no mosaic was built")
	}

	// Count the distinct tiles each cell contains, over a sample grid.
	seen := make([]map[int]bool, sh.foam.Len())
	for i := range seen {
		seen[i] = map[int]bool{}
	}
	const res = 220
	for j := range res {
		for i := range res {
			u, v := (float64(i)+0.5)/res, (float64(j)+0.5)/res
			wu, wv := s.warp(sh.field, sh.level, u, v)
			seen[sh.foam.At(wu, wv).Cell][sh.tiles.foam.At(wu, wv).Cell] = true
		}
	}

	// Only cells with a real presence on the paper: a cell showing a dozen
	// pixels at the frame's edge cannot hold a comparable number of
	// anything, and is not what the claim is about.
	small, large := math.Inf(1), 0.0
	lo, hi := math.Inf(1), 0.0
	for i, c := range sh.foam.Cells() {
		if c.Area < 0.004 {
			continue
		}
		n := float64(len(seen[i]))
		if c.Area < small {
			small = c.Area
		}
		if c.Area > large {
			large = c.Area
		}
		lo, hi = math.Min(lo, n), math.Max(hi, n)
	}
	if large/small < 6 {
		t.Fatalf("the cells only span %.1fx in area — this seed cannot test the claim", large/small)
	}
	if hi/lo > 5 {
		t.Errorf("cells spanning %.0fx in area got tile counts spanning %.0fx (%.0f..%.0f)",
			large/small, hi/lo, lo, hi)
	}
}

// TestTheLightHasOneDirectionForTheWholeSheet. Relief is a lie the eye
// forgives only while it is consistent: two shadows pointing different ways
// is the single thing that makes fake depth read as fake. The direction is
// drawn once per sheet, it is a unit vector, and it comes from above — a
// sheet lit from below reads as holes rather than bumps.
func TestTheLightHasOneDirectionForTheWholeSheet(t *testing.T) {
	for _, mode := range []string{"bevel", "cushion", "terrace", "glass", "occlude"} {
		for seed := uint64(1); seed <= 8; seed++ {
			r := planned(t, configured(t, "--relief", mode), seed).level.rel
			if n := math.Sqrt(r.lx*r.lx + r.ly*r.ly + r.lz*r.lz); math.Abs(n-1) > 1e-9 {
				t.Errorf("%s seed %d: the light is %.4f long, not a unit vector", mode, seed, n)
			}
			// v grows downward, so a light from above has ly < 0.
			if r.ly >= 0 || r.lx >= 0 {
				t.Errorf("%s seed %d: the light bears (%.2f, %.2f), not up and to the left", mode, seed, r.lx, r.ly)
			}
			if r.lz <= 0 {
				t.Errorf("%s seed %d: the light is at or below the page", mode, seed)
			}
		}
	}
}

// TestTheSurfaceIsTheSameAtEverySize. Invariant 2, for the relief: the
// height is built from canvas lengths — the rise, the run and the
// difference step are all fractions of the smallest cell — so the same seed
// has the same surface at preview and at print. Anything measured in pixels
// would give a chamfer that hardened as the render grew.
func TestTheSurfaceIsTheSameAtEverySize(t *testing.T) {
	s := configured(t, "--mosaic", "family", "--relief", "cushion")
	small, err := s.plan(testCtx(t, 9))
	if err != nil {
		t.Fatal(err)
	}
	big := testCtx(t, 9)
	big.Width, big.Height = 512, 512
	large, err := s.plan(big)
	if err != nil {
		t.Fatal(err)
	}
	for j := range 12 {
		for i := range 12 {
			u, v := (float64(i)+0.5)/12, (float64(j)+0.5)/12
			a := s.surface(small, u, v, 9)
			b := s.surface(large, u, v, 9)
			if math.Abs(a-b) > 1e-12 {
				t.Fatalf("at (%.2f, %.2f) the surface is %.6f at 96px and %.6f at 512px", u, v, a, b)
			}
		}
	}
}

// TestTheSoloistDrainsEverythingButOneCell. The scheme is a composition
// device, not a palette: one cell carries the colour and the rest of the
// sheet has to fall back far enough to let it. A "near-neutral" that still
// reads as coloured gives a sheet with one slightly louder cell in it,
// which is not the same picture at all.
func TestTheSoloistDrainsEverythingButOneCell(t *testing.T) {
	s := configured(t, "--mosaic", "soloist", "--density", "medium")
	sh := planned(t, s, 3)
	if sh.tiles == nil || sh.tiles.hero < 0 {
		t.Fatal("the soloist scheme chose no cell to carry the colour")
	}
	hero, rest, n := 0.0, 0.0, 0
	for id := range sh.tiles.foam.Len() {
		if _, sat, _ := s.tileColour(sh, sh.tiles, sh.tiles.hero, id, 3).HSL(); sat > hero {
			hero = sat
		}
	}
	for cell := range sh.foam.Len() {
		if cell == sh.tiles.hero || !sh.tiles.piece[cell].on {
			continue
		}
		_, sat, _ := s.tileColour(sh, sh.tiles, cell, cell, 3).HSL()
		rest += sat
		n++
	}
	if n == 0 {
		t.Fatal("only one cell was subdivided; the scheme has nothing to be quiet against")
	}
	if mean := rest / float64(n); hero < 3*mean {
		t.Errorf("the hero reaches saturation %.2f against a mean of %.2f elsewhere — it does not stand out", hero, mean)
	}
}

// TestTilesWearTheNeighbourTheyTouch. The neighbour scheme's whole content
// is that a cell's rim belongs to the cell it is touching, so a tile at a
// wall must actually take the pigment from the other side and a tile in the
// core must actually keep its own. Measured against the cell's own size,
// which is what the first version got wrong: a fixed reach is longer than a
// small cell is wide, and then every tile is a half-and-half mix.
func TestTilesWearTheNeighbourTheyTouch(t *testing.T) {
	s := configured(t, "--mosaic", "neighbour", "--density", "medium", "--tiled", "1")
	sh := planned(t, s, 6)
	if sh.tiles == nil {
		t.Fatal("no mosaic was built")
	}
	core, rim := 0, 0
	for id := range sh.tiles.foam.Len() {
		if sh.tiles.area[id] <= 0 {
			continue
		}
		switch e := sh.tiles.edge[id]; {
		case e < 0.05:
			core++
		case e > 0.9:
			rim++
		}
	}
	if core == 0 {
		t.Error("no tile is deep enough inside its cell to keep the cell's own colour")
	}
	if rim == 0 {
		t.Error("no tile is near enough to a wall to take its neighbour's colour")
	}
}
