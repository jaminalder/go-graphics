# Scree Gold Review Set Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce and visually verify 15 distinct 2000x2000 scree-gold renders across five palettes and three stone sizes.

**Architecture:** Use the existing `staticart render` command directly on `master`; no source change is expected. Render each matrix cell into one output directory, prefix files in matrix order, write a manifest, and use the existing sheet tooling through `staticart sweep` only where it preserves the exact matrix; otherwise build the review sheet with the repository's existing sheet package via a small temporary output-side workflow that is not committed.

**Tech Stack:** Go CLI `cmd/staticart`, existing scree options and render profiles, PNG output under gitignored `out/`.

---

## File Structure

- Create outputs: `out/scree-gold-set/01_*.png` through `15_*.png`.
- Create output: `out/scree-gold-set/manifest.txt` listing the exact matrix.
- Create output: `out/scree-gold-set/sheet.png` for one-look review.
- Modify source: none unless rendering exposes a confirmed defect.

### Task 1: Render the Matrix

**Files:**
- Create: `out/scree-gold-set/*.png`

- [ ] **Step 1: Verify the output parent and create the set directory**

Run:

```bash
ls out
mkdir -p out/scree-gold-set
```

Expected: `out/` exists and `out/scree-gold-set/` is available.

- [ ] **Step 2: Render Avery at three scales**

Run these exact commands, then rename each output with its matrix prefix:

```bash
go run ./cmd/staticart render scree --profile web --out out/scree-gold-set --seed 11 --gold --colourway avery-bicycle-rider --palette kandinsky-soft-pressure --count 240 --base 0.0215 --facet 0.34 --bed gravel --stones worn --facets cut --light noon --wet wet --scheme passage
go run ./cmd/staticart render scree --profile web --out out/scree-gold-set --seed 12 --gold --colourway avery-bicycle-rider --palette kandinsky-soft-pressure --count 340 --base 0.017 --facet 0.34 --bed gravel --stones worn --facets cut --light noon --wet wet --scheme passage
go run ./cmd/staticart render scree --profile web --out out/scree-gold-set --seed 13 --gold --colourway avery-bicycle-rider --palette kandinsky-soft-pressure --count 500 --base 0.013 --facet 0.34 --bed gravel --stones worn --facets cut --light noon --wet wet --scheme passage
```

Expected: three 2000x2000 PNGs, renamed `01_`, `02_`, and `03_` respectively.

- [ ] **Step 3: Render Rothko at three scales**

Use the same fixed flags with `--colourway from-flag --palette rothko-white-black-rust`, seeds 14, 15, 16, and the three `(count, base)` pairs `(240, 0.0215)`, `(340, 0.017)`, `(500, 0.013)`. Rename outputs `04_`, `05_`, `06_`.

- [ ] **Step 4: Render Picasso at three scales**

Use `--colourway from-flag --palette picasso-demoiselles`, seeds 17, 18, 19, and the same three size pairs. Rename outputs `07_`, `08_`, `09_`.

- [ ] **Step 5: Render Hokusai at three scales**

Use `--colourway from-flag --palette hokusai-great-wave`, seeds 20, 21, 22, and the same three size pairs. Rename outputs `10_`, `11_`, `12_`.

- [ ] **Step 6: Render Magritte at three scales**

Use `--colourway from-flag --palette magritte-menaced-assassin`, seeds 23, 24, 25, and the same three size pairs. Rename outputs `13_`, `14_`, `15_`.

- [ ] **Step 7: Verify count and dimensions**

Run a shell loop using `sips -g pixelWidth -g pixelHeight` over `out/scree-gold-set/[0-9][0-9]_*.png`.

Expected: exactly 15 files; every file reports width 2000 and height 2000.

### Task 2: Build Review Metadata

**Files:**
- Create: `out/scree-gold-set/manifest.txt`
- Create: `out/scree-gold-set/sheet.png`

- [ ] **Step 1: Write the manifest**

Write 15 tab-separated lines with columns:

```text
number	palette	seed	base	count	filename
```

Rows follow the matrix order specified in Task 1. Use the actual renamed filename from disk, not an inferred name.

- [ ] **Step 2: Build a five-by-three sheet**

Use the existing `internal/render` sheet behavior through a temporary Go program under `/var/folders/zp/66m3p9ks56vdcf54dd68kwgh0000gn/T/opencode`, reading the 15 numbered PNGs in lexical order. Scale each tile to 300x300 and write a 900x1500 `sheet.png` with three columns and five rows. The temporary program is not part of the repository.

- [ ] **Step 3: Verify sheet and manifest**

Run `sips -g pixelWidth -g pixelHeight out/scree-gold-set/sheet.png` and inspect `manifest.txt`.

Expected: sheet is 900x1500; manifest has one header plus 15 data rows.

### Task 3: Visual Review and Replacement

**Files:**
- Read: `out/scree-gold-set/sheet.png`
- Read selected full-size PNGs as needed.

- [ ] **Step 1: Inspect the contact sheet**

Read `sheet.png`. Check all 15 against the spec: distinct compositions, clear row palette families, clear scale progression by column, full Voronoi faceting, visible gold, no ordinary yellow, and no broken packing.

- [ ] **Step 2: Inspect ambiguous tiles at full resolution**

Read every tile where facets, nugget count, border clipping, or fine-scale readability cannot be judged from the contact sheet.

- [ ] **Step 3: Replace failed seeds only**

For any failed cell, increment to an unused seed above 25, rerender with that cell's palette and size, update its numbered filename and manifest row, rebuild the sheet, and repeat visual review. Do not alter fixed treatment or matrix sizes.

### Task 4: Final Verification

**Files:**
- Verify: `out/scree-gold-set/`
- Verify repository state; no source changes expected.

- [ ] **Step 1: Confirm final artifacts**

Verify 15 numbered 2000x2000 PNGs, `sheet.png`, and `manifest.txt` exist.

- [ ] **Step 2: Confirm repository state**

Run: `git status --short` and `git diff --stat`.

Expected: no tracked source changes from rendering; the previously declined `.superpowers/` directory may remain untracked and untouched.
