# Scree Gold Nuggets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in scree mode that removes yellow-like colours from ordinary stones and turns two or three randomly selected small-to-medium stones into saturated Avery-gold nuggets.

**Architecture:** Keep the feature inside `internal/sketch/scree`: palette filtering happens before ink and scheme construction, while nugget selection and recolouring happen after ordinary dressing. A dedicated context RNG stream makes the selection deterministic without perturbing existing layout, colour, facet, or trait streams; the existing render path remains unchanged when the option is off.

**Tech Stack:** Go 1.24, stdlib `flag`, `math/rand/v2`, existing `palette`, `cells`, `opt`, `scheme`, and sketch test helpers.

---

## File Structure

- Modify `internal/sketch/scree/scree_test.go`: add behavioral tests for filtering, nugget count/size, determinism, resolution independence, and option naming.
- Modify `internal/sketch/scree/colour.go`: define yellow detection/filtering, Avery gold treatment, candidate ranking, and deterministic selection without replacement.
- Modify `internal/sketch/scree/scree.go`: add the `Gold` setting, dedicated RNG stream, filtered-palette planning, and nugget dressing call.
- Modify `internal/sketch/scree/options.go`: declare `--gold` through `internal/opt` with filename tag `gold`.
- Modify `docs/sketches/010-scree.md`: document the opt-in visual contract and tunable.

### Task 1: Specify Gold-Mode Behaviour

**Files:**
- Modify: `internal/sketch/scree/scree_test.go`

- [ ] **Step 1: Add a palette helper and tests for yellow filtering**

Add a helper that plans with an explicitly supplied palette, then add tests using Avery's palette. Assert that `yellowLike` recognizes `#F3C937`, rejects the reddish and neutral Avery colours, and that `withoutYellow` returns four colours in original order.

```go
func plannedWithPalette(t *testing.T, s *Sketch, seed uint64, pal palette.Palette, width, height int) *sheet {
	t.Helper()
	sh, err := s.plan(sketch.Context{Width: width, Height: height, Seed: seed, Palette: pal})
	if err != nil {
		t.Fatal(err)
	}
	return sh
}

func TestGoldModeReservesYellowForNuggets(t *testing.T) {
	pal, ok := palette.ByName("avery-bicycle-rider")
	if !ok {
		t.Fatal("palette missing")
	}
	if !yellowLike(palette.MustHex("#F3C937")) {
		t.Fatal("Avery gold was not recognized as yellow")
	}
	for _, hex := range []string{"#7B533E", "#BFA588", "#604847", "#552723"} {
		if yellowLike(palette.MustHex(hex)) {
			t.Errorf("%s was classified as yellow", hex)
		}
	}
	filtered, err := withoutYellow(pal)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Colors) != 4 {
		t.Fatalf("filtered palette has %d colours, want 4", len(filtered.Colors))
	}
}
```

- [ ] **Step 2: Add tests for count, eligibility, saturation, and deterministic selection**

Plan seeds 1 through 12 with `--gold --colourway from-flag --bed gravel`. For each sheet, collect `stone.nugget`; assert count is 2 or 3, every selected cell is in the lower two-thirds of visible cells ranked by area, each nugget has HSL saturation 1, and each ordinary pigment is not yellow-like. Plan the same seed twice and compare nugget IDs.

```go
func TestGoldModeChoosesTwoOrThreeSmallToMediumNuggets(t *testing.T) {
	pal, _ := palette.ByName("avery-bicycle-rider")
	for seed := uint64(1); seed <= 12; seed++ {
		sh := plannedWithPalette(t, configured(t, "--gold", "--colourway", "avery-bicycle-rider", "--bed", "gravel"), seed, pal, 96, 96)
		eligible := nuggetCandidates(sh.stones)
		allowed := make(map[int]bool, len(eligible))
		for _, id := range eligible {
			allowed[id] = true
		}
		count := 0
		for id, d := range sh.skin {
			if !d.nugget {
				if yellowLike(d.pigment) {
					t.Errorf("seed %d: ordinary stone %d is yellow", seed, id)
				}
				continue
			}
			count++
			if !allowed[id] {
				t.Errorf("seed %d: nugget %d is outside the candidate set", seed, id)
			}
			_, sat, _ := d.pigment.HSL()
			if math.Abs(sat-1) > 1e-12 {
				t.Errorf("seed %d: nugget %d saturation is %.3f, want 1", seed, id, sat)
			}
		}
		if count != 2 && count != 3 {
			t.Errorf("seed %d: got %d nuggets, want 2 or 3", seed, count)
		}
	}
}
```

- [ ] **Step 3: Add resolution and opt-in regression tests**

Assert a gold plan at `96x96` and `600x600` selects the same nugget IDs. Assert an unconfigured plan has no nuggets. Assert configuring `--gold` returns `-gold` so output files identify the treatment.

- [ ] **Step 4: Run tests to verify the new claims fail**

Run: `go test ./internal/sketch/scree -run 'TestGold|TestWithoutGold'`

Expected: build failure because `yellowLike`, `withoutYellow`, `nuggetCandidates`, `stone.nugget`, and `Sketch.Gold` do not exist yet.

### Task 2: Implement Palette Filtering and Nugget Dressing

**Files:**
- Modify: `internal/sketch/scree/colour.go`
- Modify: `internal/sketch/scree/scree.go`
- Modify: `internal/sketch/scree/options.go`

- [ ] **Step 1: Add the option and isolated RNG stream**

Add `Gold bool` to `Sketch`, add `streamGold = 6`, and declare:

```go
o.Bool("gold", "reserve yellow for two or three rare gold nuggets", "gold", &s.Gold)
```

- [ ] **Step 2: Implement yellow filtering**

Add the following focused helpers to `colour.go`. Preserve palette metadata and colour order.

```go
func yellowLike(c palette.Color) bool {
	h, sat, _ := c.HSL()
	return h >= 35 && h <= 75 && sat >= 0.20
}

func withoutYellow(p palette.Palette) (palette.Palette, error) {
	colors := make([]palette.Color, 0, len(p.Colors))
	for _, c := range p.Colors {
		if !yellowLike(c) {
			colors = append(colors, c)
		}
	}
	if len(colors) == 0 {
		return palette.Palette{}, fmt.Errorf("scree: palette %q has no non-yellow colours for --gold", p.Slug)
	}
	p.Colors = colors
	return p, nil
}
```

- [ ] **Step 3: Extract wet pigment treatment and define saturated gold**

Move the existing shade/depth operations in `dress` into `wetPigment`, call it for ordinary stones, and define gold from the provenance-backed literal. Force final pigment saturation to 1 while retaining its post-water hue/lightness.

```go
var nuggetGold = palette.MustHex("#F3C937")

func wetPigment(c palette.Color, l levels, ink inks) palette.Color {
	p := shade(c, 1-l.wat.soak*0.30)
	return palette.Lerp(p, ink.deep, l.wat.deep)
}

func goldPigment(l levels, ink inks) palette.Color {
	p := wetPigment(nuggetGold, l, ink)
	h, _, light := p.HSL()
	return palette.FromHSL(h, 1, light)
}
```

- [ ] **Step 4: Implement area-ranked candidates and random selection**

Collect visible cells, sort their IDs by ascending area with ID as a deterministic tie-break, retain the first `ceil(2*n/3)`, and choose two or three IDs without replacement by partial Fisher-Yates using the dedicated RNG. Add `nugget bool` to `stone` and recolour selected stones after ordinary dressing.

```go
func nuggetCandidates(st *cells.Foam) []int {
	var ids []int
	for i, c := range st.Cells() {
		if c.Area > 0 {
			ids = append(ids, i)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := st.Cells()[ids[i]], st.Cells()[ids[j]]
		if a.Area == b.Area {
			return ids[i] < ids[j]
		}
		return a.Area < b.Area
	})
	return ids[:min(len(ids), (2*len(ids)+2)/3)]
}

func addNuggets(skin []stone, st *cells.Foam, l levels, ink inks, rng *rand.Rand) {
	ids := nuggetCandidates(st)
	want := min(len(ids), 2+rng.IntN(2))
	for i := range want {
		j := i + rng.IntN(len(ids)-i)
		ids[i], ids[j] = ids[j], ids[i]
		id := ids[i]
		skin[id].pigment = goldPigment(l, ink)
		skin[id].nugget = true
	}
}
```

- [ ] **Step 5: Wire filtering and nugget assignment into planning**

In `plan`, after `colours` resolves the palette, conditionally call `withoutYellow` before luminance sorting and `inks`. After `dress`, conditionally call `addNuggets` with `ctx.RNG(streamGold)`, then calculate the joint darkness from the final skin.

- [ ] **Step 6: Format and run focused tests**

Run: `make fmt && go test ./internal/sketch/scree`

Expected: PASS, including the unchanged existing golden when `--gold` is absent.

- [ ] **Step 7: Commit the tested implementation**

```bash
git add internal/sketch/scree/colour.go internal/sketch/scree/options.go internal/sketch/scree/scree.go internal/sketch/scree/scree_test.go
git commit -m "scree: hide rare gold among ordinary stones"
```

### Task 3: Document and Visually Calibrate Gold Mode

**Files:**
- Modify: `docs/sketches/010-scree.md`
- Create output only: `out/scree-gold-review/` (gitignored)

- [ ] **Step 1: Document the feature**

Add a `Gold nuggets` subsection under `Colour` explaining that `--gold` removes yellow-like palette members from the ordinary ramp, chooses two or three random small-to-medium stones, and paints those with saturated Avery gold. Add `--gold` to the tunables table and add an acceptance item requiring rarity and clear separation from the muted bed.

- [ ] **Step 2: Render seed variations at compact resolution**

Run:

```bash
go run ./cmd/staticart sweep scree --seeds 1-12 --profile preview --out out/scree-gold-review/sweep --gold --colourway avery-bicycle-rider --palette kandinsky-soft-pressure --bed gravel --stones worn --facets cut --light noon --wet wet --scheme passage
```

Expected: `out/scree-gold-review/sweep/sheet.png` shows two or three saturated gold stones per tile, with no yellow spread through ordinary stones.

- [ ] **Step 3: Render the requested seed-8 composition at 600x600**

Run:

```bash
go run ./cmd/staticart render scree --profile preview --out out/scree-gold-review --seed 8 --gold --colourway avery-bicycle-rider --palette kandinsky-soft-pressure --count 240 --base 0.0215 --faceted 0.34 --bed gravel --stones worn --facets cut --light noon --wet wet --scheme passage
```

Expected: a `600x600` PNG matching the named composition, now with a gray/reddish bed and two or three rare gold nuggets.

- [ ] **Step 4: Inspect both generated PNGs**

Read `out/scree-gold-review/sweep/sheet.png` and the seed-8 PNG. Confirm nuggets are rare, small-to-medium, clearly more saturated, and ordinary stones contain no yellow passages. If visual calibration is needed, change only the documented yellow interval or gold saturation treatment, add/update the corresponding focused test first, and rerun Tasks 2 Step 6 and 3 Steps 2-4.

- [ ] **Step 5: Render the requested final at 2000x2000**

Run the same seed-8 command with `--profile web` into `out/scree-gold-final`.

Expected: a `2000x2000` PNG with the approved compact render's exact composition.

- [ ] **Step 6: Commit documentation**

```bash
git add docs/sketches/010-scree.md
git commit -m "docs: describe scree gold mode"
```

### Task 4: Full Verification

**Files:**
- Verify all modified files; no new source changes expected.

- [ ] **Step 1: Run the repository gate**

Run: `make check`

Expected: formatting, vet, lint, and all tests PASS.

- [ ] **Step 2: Inspect repository state and committed diff**

Run: `git status --short` and `git diff HEAD~3..HEAD -- internal/sketch/scree docs/sketches/010-scree.md docs/superpowers`

Expected: only `.superpowers/` remains untracked from the declined browser companion; `out/` stays ignored; all intended source and documentation changes are committed.
