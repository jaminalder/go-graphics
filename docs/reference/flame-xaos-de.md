# Flam3 xaos (relative weights) and density estimation

Primary-source notes for porting two flam3 / Apophysis features into
`internal/flame`. Not a genome format, not a plugin registry, not
bit-exact flam3.

Companion: [fractal-flames.md](fractal-flames.md) (the three flame
innovations). This file is only the chaos-game *picker* and the
*variable-width blur* that flam3 applies after the histogram exists.

## Sources

| Source | What it owns |
|---|---|
| Draves & Reckase, *The Fractal Flame Algorithm*, 2003, rev. Nov 2008, [flam3.com/flame_draves.pdf](https://flam3.com/flame_draves.pdf) | Chaos-game weights (§2), final xform (§3.2), colour-inert symmetry (§7), density estimation as a dynamic filter after log-density (§8), motion-blur interaction (§9). Cites Silverman 1986 [5] and Suykens & Willems WSCG 2000 [6]. |
| [github.com/scottdraves/flam3](https://github.com/scottdraves/flam3) (`master`) | The implementation. Xaos lives as `chaos[][]` plus a precomputed inverse-CDF table. DE lives in `filters.c` / `rect.c`. |
| Suykens & Willems, *Adaptive filtering for progressive Monte Carlo image rendering*, WSCG 2000, Journal of WSCG vol. 8. Record: [dspace.zcu.cz/items/233c198e-8ca1-4cd3-9372-09b56227564a](https://dspace.zcu.cz/items/233c198e-8ca1-4cd3-9372-09b56227564a); stable handle [hdl.handle.net/11025/15459](http://hdl.handle.net/11025/15459) | The paper flam3 cites as [6]. Variable-kernel density estimation on Monte Carlo samples; kernel width from local density; energy-preserving splat. |
| flam3 `README.txt` changelog | When DE and Apophysis chaos landed, and the XML rename `estimator` → `estimator_radius`. |
| flam3 `docstring.c` / `flam3-render.man` | CLI env-vars. Estimator knobs are **genome XML attributes**, not render env-vars. |

Apophysis is the UI name **xaos**. flam3's 2009 changelog records the
port: “Apophysis chaos and solo xform/plotmode features have been
implemented … opacity and chaos are both interpolatable”
([README.txt](https://github.com/scottdraves/flam3/blob/master/README.txt),
18 Mar 2009). The on-disk attribute is `chaos="…"`, not `xaos`.

---

# 1. Xaos / relative weights

Textbook IFS picks `F_i` independently each step from weights `w_i`
(paper §2). Xaos replaces that with a **Markov chain**: the probability
of picking `j` next depends on which xform just fired.

Effective weight from `i` to `j`:

```
w̃ᵢⱼ = densityⱼ × chaos[i][j]
```

Independent picks are the special case `chaos[i][j] = 1` for all `i, j`.

## 1.1 Data structure in flam3

`flam3_genome` holds a dense square matrix of standard (non-final)
xforms only:

```c
double **chaos;   /* [num_std][num_std] */
int chaos_enable;
```

Declared in [`flam3.h`](https://github.com/scottdraves/flam3/blob/master/flam3.h)
on the genome struct (comment “Xaos implementation”). Each row is “from
this xform, relative weights **to** every standard xform.”

Default is **all 1s**, filled when xforms are added
([`flam3_add_xforms`](https://github.com/scottdraves/flam3/blob/master/flam3.c)
around the “Handle the chaos array” block, lines 1065–1081 of `flam3.c`):
existing rows are `realloc`'d and new columns set to `1.0`; new rows
are `malloc`'d and filled with `1.0`. Interpolation clamps negatives to
0 but **does not clamp above 1**
([`interpolation.c`](https://github.com/scottdraves/flam3/blob/master/interpolation.c),
`INTERP(chaos[i][j])` with comment “chaos can be > 1”).

XML: each `<xform>` may carry `chaos="a b c …"`, a space-separated row
of scalars to xforms 0, 1, 2, … in order
([`parse_xform_xml`](https://github.com/scottdraves/flam3/blob/master/parser.c)
lines 919–956). Entries equal to `1.0` are **not stored** (sparse
override list). After all xforms parse, the list is written onto the
all-1s matrix
([`parse_flame_element`](https://github.com/scottdraves/flam3/blob/master/parser.c)
lines 799–801: `cp->chaos[xaos[i].from][xaos[i].to] = xaos[i].scalar`).
A missing `chaos` attribute therefore means the row stays all 1s.

Writing trims trailing 1s
([`flam3_print_xform`](https://github.com/scottdraves/flam3/blob/master/flam3.c)
lines 1987–1998). Final xforms do not get a chaos row.

## 1.2 How the chaos-game picker uses it

Build once per interpolated genome, then O(1) per iterate.

### Build: `flam3_create_xform_distrib` (`flam3.c` lines 73–108)

```
numrows = num_xforms - (final present ? 1 : 0) + 1
xform_distrib = calloc(numrows * 16384, unsigned short)
```

`CHOOSE_XFORM_GRAIN` is 16384, `CHOOSE_XFORM_GRAIN_M1` is 16383
(`flam3.c` lines 67–68).

- **Row 0** is the unconditional distribution: `flam3_create_chaos_distrib(cp, -1, row0)`
  uses raw `density[j]` only.
- `chaos_enable = 1 - flam3_check_unity_chaos(cp)` (`flam3.c` lines 143–158).
  Unity means every `chaos[i][j]` is within `EPS` of 1. If so, **only
  row 0 is used** and the rest of the table is left unused.
- If any entry is not 1, one extra row is built per standard xform:
  `flam3_create_chaos_distrib(cp, i, row_{k})` with `d = density[j] * chaos[i][j]`.
  The final xform is skipped (`continue` when `i == final_xform_index`).
  Rows are packed, so with the final forced to the end, row `k+1`
  belongs to standard xform `k`.

### Inverse-CDF table: `flam3_create_chaos_distrib` (`flam3.c` lines 160–219)

Not a scan at pick time. For a row `xi` (`-1` = unconditional):

1. Sum `dr = Σ density[j] * (xi>=0 ? chaos[xi][j] : 1)`. Negative
   density is an error. **`dr == 0` is fatal**: `"cannot iterate empty
   flame"` and the whole table is discarded.
2. `dr /= 16384`. Walk `j` as a running cumulative `t`. For each of
   16384 bins, store `xform_distrib[i] = j` while `r >= t`, then
   `r += dr`.

This is a 16384-entry **inverse CDF**: a uniform integer indexes the
next xform in O(1). Zero weights are skipped because `t` does not
advance, so `j` jumps over them. Weights smaller than `1/16384` of the
row total can vanish (quantization).

### Pick: `flam3_iterate` (`flam3.c` lines 226–293)

`lastxf` starts at **0**. Each iteration:

```c
if (cp->chaos_enable)
    fn = xform_distrib[ lastxf*16384 + (irand(rc) & 16383) ];
else
    fn = xform_distrib[ irand(rc) & 16383 ];

apply_xform(cp, fn, p, q, rc);
lastxf = fn + 1;          /* 1-based index into the extra rows */
```

Code path, last xform → next:

1. First pick (`lastxf == 0`) **always** uses row 0, the independent
   weights. Even with xaos on, the chain has no predecessor yet.
2. After applying `fn`, `lastxf = fn+1`. The next pick uses the row
   built from `chaos[fn][*]`.
3. `apply_xform` (`variations.c` lines 2126–2138) updates colour with
   `color_speed` and writes `q[3] = vis_adjusted` (opacity). Xaos does
   not touch colour.
4. If a final xform is enabled, it is applied **after** the chosen
   `F_i`, with a possible opacity lottery. The result is **not** fed
   back (`q` is plotted; `p` for the next step is the pre-final point
   in the non-opacity-1 path — see the copy `p ← q` *before* the final
   block, then final writes `q` for plotting only). `lastxf` is **not**
   updated from the final. The final is not a chaos row.

Commented-out line 245 used `% 16384`; the live path uses `& 16383`.

## 1.3 Default when xaos is absent

| Situation | Behaviour |
|---|---|
| No `chaos` XML, new xforms | Matrix is all `1.0`. |
| All entries ≈ 1 | `chaos_enable = 0`. Picker uses row 0 only (raw densities). Equivalent to independent weighted picks. |
| Some zeros in a row | Those destinations are never chosen from that source (`t` does not advance). Other entries are renormalised by the row sum. |
| Entire row sums to 0, or all densities 0 | **Hard error** at table build. The genome will not iterate. |

Zeros are skipped, not treated as “almost never.” There is no epsilon
floor.

## 1.4 Symmetry xforms

Paper §7: rotational / dihedral symmetry is extra maps, colour-inert,
with weight equal to the **sum of the others** so branches have equal
density. Colours wash out if symmetry maps blend the colour coordinate
(Figure 5d).

flam3:

- [`flam3_add_symmetry`](https://github.com/scottdraves/flam3/blob/master/flam3.c)
  (`flam3.c` lines 2098–2176) appends linear maps with
  `color_speed = 0.0` (colour-inert) and `density = 1.0` — **not** the
  paper's sum-of-others. (This repo's `System` already uses the paper
  weight; keep that.)
- Adding xforms grows `chaos` with **1.0 fill**
  (`flam3_add_xforms`), so a symmetry map is a new row and column of
  ones unless the genome later overwrites them. From a symmetry map,
  the next pick uses raw densities (row of 1s). Toward a symmetry map,
  `w̃ = 1.0 × density_sym`.
- Xaos and symmetry compose: a zero in `chaos[i][sym]` forbids
  jumping *to* that symmetry map from `i`; a zero in the symmetry
  row forbids leaving it for that destination. Nothing in the picker
  special-cases `Symmetry`. Colour-inert is `color_speed`, not xaos.

## 1.5 Efficient implementation: CDF per row vs scanning

flam3 does **not** scan weights every pick. It builds inverse-CDF
lookup tables once (`16384 × (n_std+1)` `unsigned short`s, 32 KB per
row) and indexes with a masked random integer.

A linear scan of `n` densities each pick is also correct. For the
`n` of a flame (typically 3–12) it is lost in the noise of variation
eval. The 16384-grain table exists for O(1) and for historical
quantization, not because `n` is large.

Walker’s alias method is unnecessary at this `n`.

## 1.6 Gotchas

**Zero rows / all-zero.** Any chaos row (or the unconditional row)
with `dr == 0` aborts the render at setup, including “this xform can
be reached but has nowhere to go.” A single dead-end row poisons the
genome.

**First pick.** Always independent weights (`lastxf = 0` → row 0).
Do not start the chain at a random row.

**Off-by-one encoding.** flam3 stores extra rows at `fn+1` so row 0
can mean “no predecessor.” A 0-based matrix plus `last = -1` on the
first pick is the same idea with less confusion. Do not copy `fn+1`.

**Quantization.** Density × xaos is snapped to 16384 bins. Tiny
weights can disappear. A float CDF does not have this bug; do not
copy the grain unless matching a flam3 genome byte-for-byte (out of
scope).

**Final xform.** Not in the matrix, not in `lastxf`, not in the loop
state. Opacity on the final is a plot lottery; flam3 copies
`q[3] = p[3]` so plot weight stays the IFS xform's
(`flam3_iterate` lines 274–280).

**Colour.** `apply_xform` blends `q[2]` with `color_speed` regardless
of xaos. Symmetry still needs `color_speed = 0`. Xaos cannot be used
as a substitute for colour-inert maps.

**Opacity / plotmode.** Independent of xaos. `vis_adjusted` from
`adjust_percentage(opacity)` (`interpolation.c` lines 28–34) is the
plot weight. Opacity 0 still iterates (the point stays in the loop);
it is skipped at bucket time (`rect.c` `if (p[3]==0) continue`).
README 3.0.1: opacity also scales DE filter width via the 5th bucket
channel (see §2.4).

**Bad-value reset.** flam3 does **not** reset `lastxf` on NaN/Inf
retries (`flam3_iterate` lines 251–264). This repo's `orbit` sets
`last = -1` on `bad()`. Prefer the reset: a divergent pick should
not lock the Markov chain.

**Interpolation.** Chaos is interpolated linearly per entry, negatives
clamped to 0, values `> 1` allowed. Motion xforms cannot carry
`chaos` (parser error).

---

# 2. Density estimation (DE)

## 2.1 What the paper claims

Paper §8: after log-density and gamma, low-density regions look
dotted. A wider spatial filter would blur the cores. “We have
implemented a dynamic filter where a blur kernel of width inversely
proportional to the density of points in the histogram is applied to
the samples [6]. The blur kernel is scaled based on the supersampling
level, and is applied **after the log density scaling**.” Figure 6
shows the filament-smoothing / core-sharp effect.

Paper §9: the “proper” motion-blur path logs each temporal sample and
runs DE into a second buffer; that can **double** render time, so the
default is a single buffer.

Reference [6] is Suykens & Willems WSCG 2000, not the EGWR 2000
“Density Control for Photon Maps” paper (that one is about *storing
fewer photons*).

Erik Reckase's code landed 24 Sep 2005 (`README.txt`: “new density
estimator and temporal jitter code”). XML names as of 11 Jan 2006:
`de_max_filter` → `estimator` → (15 Apr 2006)
`estimator_radius`; `de_min_filter` → `estimator_minimum`;
`de_alpha` → `estimator_curve`.

## 2.2 Suykens & Willems (WSCG 2000)

*Adaptive filtering for progressive Monte Carlo image rendering*,
Journal of WSCG 8, 2000. Full PDF on DSpace is login-walled; the
text below is from the stable record's extracted body
([hdl.handle.net/11025/15459](http://hdl.handle.net/11025/15459)).

They reconstruct an image-plane function by **variable-kernel density
estimation** (Silverman 1986). Three DE families: histogram, nearest
neighbour, kernel; they use kernel DE.

- Kernel shape: **Epanechnikov** (compact quadratic). They explicitly
  consider Gaussian and reject it for discontinuous radiance.
- Kernel width: smaller where sample density (or the reconstructed
  function) is large; wider in sparse / “bad” regions. Width also
  shrinks as sample count grows (progressivity). Heuristic of the
  form `h ∝ n^{-α} × (weight / reference)^{1/2}` with a user scale
  `y` (their eq. 8). Minimum width: **one pixel**, so a kernel cannot
  miss the buffer.
- Application: splat each **sample** with its kernel during rendering
  (not a post-process on pixels). Energy preserving. They store the
  first batch of hits, estimate a reference image, then iterate.
- They note Heckbert (histogram radiosity textures), Shirley /
  Myszkowski / Walter (kernel DE on surfaces), Jensen (k-NN photon
  map). Adaptive width from local density is credited to Myszkowski
  and Walter.

Relationship to flam3: the **idea** (blur sparse Monte Carlo hits more
than dense ones, after the samples exist) is taken from this paper.
The **algorithm** is not. flam3:

| Suykens & Willems | flam3 |
|---|---|
| Splat each path sample | Histogram first, then blur **bins** |
| Epanechnikov | Truncated Gaussian (`flam3_gaussian_kernel`) |
| Width from sample “badness” and a running reference image | Width from that bin's hit count (and a small OS neighbourhood) |
| Progressive, kernels shrink with `n` | One shot after the chaos game; `quality` already chose `n` |
| Image-space samples in a path tracer | IFS histogram buckets |

Calling flam3 “the Suykens filter” is a citation, not a port.

## 2.3 The flam3 algorithm exactly

Pipeline comment at the top of
[`rect.c`](https://github.com/scottdraves/flam3/blob/master/rect.c)
(lines 21–32):

```
for batch
  generate de filters
  for temporal_sample
    iterate → buckets += cmap[samples]
  accum += time_filter * log[buckets] * de_filter
image = filter(accum)
```

DE is **not** applied to individual chaos-game points. Points have
already been spatially binned. That is how they avoid O(n²) over
samples: the histogram **is** the spatial binning.

### Kernel creation: `flam3_create_de_filters` (`filters.c` lines 268–401)

Arguments: `max_rad` (`estimator` / `estimator_radius`), `min_rad`
(`estimator_minimum`), `curve` (`estimator_curve`), `ss` (spatial
oversample).

Rejects `curve <= 0` or `max_rad < min_rad`. Very small `curve` can
request millions of kernels; they abort if
`(max/min)^(1/curve) > 1e7` (`README.txt` 20 Dec 2009: die instead of
segfault).

Radii in **bucket pixels**:

```
comp_max = estimator_radius * ss + 1
comp_min = estimator_minimum * ss + 1
```

The `+1` is “assumed distance to the first pixel.” Header comment on
the genome (`flam3.h`): `estimator` is “filter width for bin with one
hit”; `estimator_curve` is “exponent on decay function
`( MAX / a^(k-1) )`” — the live formula is `MAX / (k+1)^curve`, not
`a^(k-1)`.

For filter index `k = 0, 1, …`:

```
h(k) = comp_max / (k+1)^curve          // k < 100
```

Once `h` would fall to `comp_min`, they stop adding kernels
(`max_filter_index = k`) and remaining indices share the minimum
width.

If the theoretical number of kernels exceeds `keep_thresh = 100`
(`DE_THRESH` in [`filters.h`](https://github.com/scottdraves/flam3/blob/master/filters.h)),
indices `k ≥ 100` are condensed:

```
adj    = (k - 100)^(1/curve) + 100     // virtual hit count
h(k)   = comp_max / (adj+1)^curve
```

`max_filtered_counts` is the hit count at which they clamp to the
smallest kernel.

**Shape.** Isotropic 2D Gaussian, truncated at `r = h`:

```
d = hypot(dj, dk) / h
if d > 1: coef = 0
else:     coef = flam3_gaussian_filter(1.5 * d) / sum
```

`flam3_gaussian_filter` (`filters.c` lines 155–157):
`exp(-2 x²) * sqrt(2/π)`, support 1.5
(`flam3_spatial_support[0]`). Epanechnikov is present but
**commented out**. Coefficients are stored for **one octant**
(`dej ≥ 0`, `dek ≤ dej`); `kernel_size = (half+1)*(half+2)/2`.
Normalisation uses the full disc sum, then the octant is scaled.

Defaults when `flam3_defaults_on` (`flam3.c` lines 1293–1296):
`estimator = 9.0`, `estimator_minimum = 0.0`, `estimator_curve = 0.4`.
DE is **on** by default in flam3. `bits <= 32` integer buffers force
`estimator = 0` with a warning (`rect.c` lines 729–733) — float
buckets (`bits == 33`) or 64-bit are required.

### Application: `de_thread` (`rect.c` lines 55–249)

For each bucket `(i,j)` in the interior (a one-oversample gutter is
skipped):

1. Skip if `b[4] == 0` or `b[3] == 0` (no hits / no alpha).
2. **Local density** `f_select`: sum `b[4]/255` over a
   `(2⌊ss/2⌋+1)²` neighbourhood. If `ss` is even, multiply by
   `(ss/(ss+1))²`. Channel 4 is “hit weight”: each plotted point
   adds `255` (or `opacity*255`). So `f_select` ≈ neighbourhood hit
   count. At `ss = 1` it is this pixel's hit count.
3. Map density to a kernel index:

   ```
   if f_select > max_filtered_counts: idx = max_filter_index
   else if f_select <= 100:           idx = ceil(f_select) - 1    // 1 hit → 0
   else:                              idx = 100 + floor((f_select-100)^curve)
   ```

   Then clamp `idx` to `max_filter_index`. This inverts the
   condensation used at build time, so radius falls as
   `~ 1 / (hits)^curve`.
4. **Log-density on the source pixel**, then scatter. With
   `c[0..3]` the RGBA bucket (palette-weighted sums, not the hit
   channel):

   ```
   ls = filter_coefs[k] * (k1 * log(1 + c[3]*k2) / c[3])
   c[0..3] *= ls
   ```

   `k1` / `k2` fold in brightness, contrast, sample density, area,
   oversample, batch filter (`rect.c` lines 927–931). Then
   `add_c_to_accum` writes the scaled 4-tuple into neighbours,
   replicating the octant 1/4/8 ways.

Empty bins contribute nothing. Dense bins pick a ~1-pixel kernel.
Sparse bins pick a kernel up to `estimator_radius * ss + 1` buckets.

Without DE (`max_filter_index == 0`), the same `k1*log(1+α*k2)/α`
scale is applied **in place** with no spatial spread (`rect.c`
lines 941–968).

### Gutter

`rect.c` lines 648–681: histogram is oversized by
`max(spatial_filter_gutter, ceil(estimator)*ss + (ss-1))` so the DE
kernel does not run off the edge. After DE, the spatial down-filter
(Gaussian / Mitchell / box / … — a **separate** kernel from
`flam3_create_spatial_filter`) reduces oversampled buckets to output
pixels. That downsample is not DE.

## 2.4 Log-density: before or after the blur?

**After log, then blur.** Confirmed three ways:

1. Paper §8: “applied after the log density scaling.”
2. `de_thread` computes `log(1 + c[3]*k2)/c[3]` on the source bucket
   **before** scattering into `accumulate`.
3. §9's expensive path: “take the logarithm of this buffer and
   accumulate it into the second one, applying the density estimation
   filter in the process.”

They are **not** doing statistical KDE on linear hit counts and then
taking log. They tone-map each bin, then use a density-dependent
blur as a denoiser on the HDR preview. Filtering after a nonlinearity
is not energy-preserving in Suykens' sense; flam3 accepted that.

Radius uses hit-count channel **4**; the log uses palette-alpha
channel **3**. Opacity therefore thins both the plot and the DE
radius (README 3.0.1). This repo's four-channel `Hist` uses `a` for
both; that is consistent with full-opacity hits.

## 2.5 Spatial-binning / box / separable Gaussian / k-d tree

What flam3 **actually** does:

| Technique | Used? |
|---|---|
| Spatial binning (histogram) | **Yes.** Chaos-game points are already in buckets. DE never sees individual samples. |
| k-d tree / photon map | **No.** |
| Box filter as DE kernel | **No.** Box exists only as a *spatial downsample* kernel (`flam3_box_kernel`). |
| Separable Gaussian | **No.** DE kernel is a 2D isotropic disc, stored as an octant, applied with 8-way scatter. The spatial downsample *is* a separable product `f(ii)*f(jj)` (`filters.c` `flam3_create_spatial_filter`), but that is the oversample filter, not DE. |
| Per-sample splat (Suykens) | **No.** |
| Nearest-neighbour / k-NN | **No.** |

The O(n²) they avoid is “for each of N samples, search nearby
samples.” Binning reduces that to “for each nonempty bucket, splat a
kernel of radius R(density).”

## 2.6 Parameters

XML on the `<flame>` tag
([`parser.c`](https://github.com/scottdraves/flam3/blob/master/parser.c)
lines 481–486). Not in `docstring.c`'s env-var table.

| XML | Genome field | Default | Role |
|---|---|---|---|
| `estimator_radius` | `estimator` | 9 | `R`. Width in **output pixels** for a bin with one hit, before `*ss+1`. 0 disables DE. |
| `estimator_minimum` | `estimator_minimum` | 0 | Floor on kernel width (same units). 0 → `comp_min = 1` bucket. Never sharper than this, even in the core. |
| `estimator_curve` | `estimator_curve` | 0.4 | Exponent `γ`. `h ∝ R / n^γ`. Smaller γ → widths fall slowly → more blur into mid-densities. Must be `> 0`. |

`flam3.h` comments on the three fields (genome struct, “Density
estimation parameters for blurring low density hits”).

## 2.7 Performance

Chaos-game cost: `O(samples × cost(F_i))`. Samples = `quality ×
width × height` (paper §2). This dominates at print quality.

DE cost: `O(nonempty_buckets × R(pixel)²)`, worst case
`O(W_hist × H_hist × (R·ss)²)` if every pixel used the max kernel
(they don't: empties skip, cores use ~1 tap). Octant storage cuts
kernel **memory** by ~8, not the scatter writes (those still hit
8 neighbours).

Paper §9: applying DE once per temporal sample can double wall time.
`rect.c` progress meter treats DE as its own phase (`"density
estimation: %d/%d"`).

No k-d tree, so there is no `O(N log N)` rebuild. Threads in
`de_thread` write a **shared** `accumulate` with **no mutex** (the
bucket mutex is only around iteration). That is a data race; do not
copy it.

---

# 3. Recommended port (`internal/flame`)

Constraints: `uint64` 4-channel histogram already at **output**
resolution; 8 parallel orbits with a **fixed** worker count;
determinism (no `GOMAXPROCS`-shaped partitions); stdlib only.

`System.Xaos`, per-row `xcdf`, `pick(last)`, and
`Hist.developEstimated` already exist. Align them with the sources
above; do not grow a flam3 clone.

## 3.1 Xaos

**Algorithm**

1. Store `Xaos [][]float64` of size `n×n` for the IFS xforms only.
   `nil` = all ones = independent picks (`xaosActive` already does
   this).
2. `InitXaos` fills 1s. `SetXaos(from, to, w)` punches holes. Clamp
   negatives to 0 at prepare time (flam3 interpolation does).
3. At `prepare()`, build:
   - `cdf[j] = Σ_{i≤j} Weight[i]` (unconditional).
   - if xaos active, `xcdf[i][j] = Σ_{k≤j} Weight[k] * Xaos[i][k]`.
4. `pick(rng, last)`: `last < 0` → `cdf`; else `xcdf[last]`. Uniform
   `u * total`, linear scan of the CDF. Skip zeros for free (the
   cumulative does not move).
5. Orbit: `last = -1` at start and after a bad-value reset. After a
   successful `F_i`, `last = i`. Apply `Final` only for plotting.

Linear CDF scan is the right complexity. `n` is tiny next to a
variation. Do **not** copy 16384 inverse-CDF tables (quantization +
memory + no benefit). Do **not** scan the raw matrix every pick
without a cached row CDF — that is the one thing flam3 got right.

**Zero rows.** flam3 aborts. Current `prepare` falls back to `cdf` if
a row sums to 0, and `pick` falls back to `IntN` if `total <= 0`.
Keep a defined behaviour: treat an all-zero row as “this xform is a
dead end; next pick uses unconditional weights,” and do not fail the
render. Document it. A genome builder that *wants* flam3's refusal
can check row sums at plan time.

**Symmetry.** `AddSymmetry` already uses paper weights and
`ColorSpeed = 0`. After appending symmetry xforms, xaos must grow:
new row and column of 1s (same as `flam3_add_xforms`). If `Xaos` was
`nil`, leave it `nil`. If it was set, pad it.

**Colour / final.** Unchanged. Xaos is picker-only.

**Complexity / memory / determinism**

- Prepare: `O(n²)`. Memory: `n²` floats plus `n` CDFs of length `n`.
  For `n = 16`, negligible.
- Per iterate: `O(n)` scan, deterministic given the RNG stream.
- Each of the 8 orbits holds its own `last`. The matrix is immutable
  after `prepare`. No atomics on xaos.

## 3.2 Density estimation

**Algorithm** (histogram variable-kernel splat, flam3's actual method,
not Suykens' per-sample method)

After `Accumulate` fills `Hist` (uint64 RGBA at output size):

1. If `Estimate.Radius <= 0`, develop as today (per-pixel log-density).
2. Allocate four `float64` accumulate buffers, same `w×h` (one extra
   32 B/pixel — at 6000² another 1.15 GB peak on the 32 GB machine;
   free after develop).
3. For each nonempty bin `i`, in **row-major order** (fixed, not
   threaded):
   - `hits = a[i] / colorScale`
   - `scale = log(1 + hits * k2) / hits` with the same `k1/k2`
     brightness convention already used in `develop`, or the current
     `log(1+hits*bright)/logMax` — either is log-then-blur.
   - `rad = Radius / max(hits,1)^Curve`, clamp to `[Min, Radius]`.
     Default `Curve = 0.4`, `Min = 0`, `Radius` off unless the sketch
     asks. Match flam3's `h = R / n^γ`, skipping the `*ss+1` until
     we supersample the histogram (we don't).
   - Scatter `meanRGB * scale * kernel(dx,dy)` into the float
     buffers with a **normalised truncated Gaussian**
     `exp(-2 (r/R)²)` on the disc `r ≤ R` (`discKernel` already).
4. Run gamma / vibrancy / gleam / ground on the accumulated floats
   (already in `developEstimated`).

**Do not** parallelise this scatter across `GOMAXPROCS` or the 8
orbit workers. Float add is order-dependent at ulp; flam3's unlocked
threaded scatter is a race. Sequential scatter is deterministic and,
at preview, cheaper than the chaos game. At print, nonempty sparse
pixels with `Radius = 9` are a few hundred taps; empty pixels skip.
If it ever shows up in a profile: still sequential, but precompute
kernels per integer radius (already cached in `developEstimated`).

**Not recommended here**

- k-d tree: the histogram already binned the points.
- Separable variable-width Gaussian: not flam3, and variable `R` per
  pixel makes true separation inexact.
- Box filter: flam3 rejected it for DE (commented Epanechnikov, live
  Gaussian).
- Per-sample Suykens splat: would need to keep every hit, O(samples ×
  R²), wrecks the “uint64 histogram is the picture” design.
- flam3 oversample gutter / 5th channel / `DE_THRESH` condensation:
  those exist because flam3 histograms at `ss ×` resolution with
  integer buckets. We splat at output resolution with uint64. A
  neighbourhood density at `ss = 1` **is** the pixel's own `hits`.
- Copying `k1/k2`'s `PREFILTER_WHITE * 268 / 256` numerology.

**Complexity / memory / determinism**

- Time: `O(pixels + nonempty × R²)` after iteration.
  Iteration remains `O(samples)`. DE is a develop-time tax, not a
  per-orbit tax.
- Memory: one float64 RGBA scratch, same shape as `Hist`. No 8×
  private buffers.
- Determinism: single-threaded row-major scatter. Kernel tables
  depend only on `Radius/Min/Curve`, not on `GOMAXPROCS`.

## 3.3 What not to copy from flam3

- XML genomes, `chaos=""` parsing, `estimator_*` attributes as a
  public CLI surface (sketch traits / `Tone.Estimate` are enough).
- Plugin variation registries, motion xforms, Electric Sheep,
  ISAAC, `CHOOSE_XFORM_GRAIN`.
- GPU paths (not in flam3; later ports).
- `bits` 32/33/64 bucket types and “DE disabled on 32-bit.”
- Pthread DE races, temporal-sample multi-buffer DE, field rendering,
  `nstrips`.
- Spatial-filter kernel menagerie on the DE path (keep Gaussian).
- flam3's `density = 1` symmetry weights (paper §7 is the spec;
  this repo already follows it).
- Final-xform opacity lottery unless the sketch grows an opacity
  trait.
- Bit-exact images. Same as QQL / the existing flame note: vocabulary
  and look, our PCG streams, our canvas units.

## 3.4 Suggested defaults for this sketch

DE off until a sweep shows filaments looking dotted at the chosen
quality. When on: `Radius = 7…9`, `Min = 0`, `Curve = 0.4` (flam3
defaults). Judge on a `sweep`, not one seed. Xaos stays `nil` unless
a trait actually needs a forbidden transition (a spindle does not).
