# Scree Gold Nuggets Design

## Goal

Add an explicit `--gold` mode to `scree`. In this mode, ordinary stones cannot
receive yellow-like pigments, while two or three small-to-medium stones become
rare, saturated gold nuggets. The treatment must be deterministic and reusable
for every scree seed and colourway.

Ordinary renders remain byte-identical when `--gold` is absent.

## Colour Treatment

Gold mode filters the selected colourway before `inks` and `scheme.New` use it.
A swatch is yellow-like when its HSL hue is in the yellow/amber interval of
35-75 degrees and its saturation is at least 0.20. Filtering before the scheme
runs prevents yellow from reaching ordinary stones through any arrangement,
and filtering before `inks` are derived prevents a yellow lamp tint from
reintroducing it through lighting.

The filter applies to any selected scree colourway. It removes only yellow-like
swatches; all remaining hues and the scheme's independent value structure stay
unchanged. If a colourway has no yellow-like swatches, its ordinary-stone ramp
is unchanged. If filtering would leave no colours, planning returns a clear
error rather than silently restoring yellow.

Every nugget uses Milton Avery's `#F3C937` from `avery-bicycle-rider`. This is a
fixed, provenance-backed gold rather than an invented colour or the warmest
member of the active colourway. The gold pigment is assigned after the scheme
has dressed the bed, so the scheme cannot spread it to other stones. Nuggets
retain their existing tone/load and use the same wash, facets, water, and lamp
as every other stone; only their pigment changes. After the normal wet-stone
darkening and water-depth mix, its HSL saturation is lifted toward 1 so the
accent remains richer than the ordinary bed without exposing another slider.

## Nugget Selection

Gold mode uses a dedicated seed-derived RNG stream. This keeps nugget count and
selection deterministic while isolating them from layout, traits, facets, and
ordinary colour arrangement.

Candidates are visible stones in the lower two-thirds of the bed's area
ranking. This includes small and medium stones while excluding the prominent
largest stones. The stream chooses two or three candidates with equal
probability, without replacement. Selection has no spacing rule: nuggets may
land near each other or far apart according to the seed. If an unusually sparse
bed has fewer candidates than the drawn count, all candidates become nuggets.

## Interface

Add one boolean sketch option:

```text
--gold    reserve yellow for two or three rare gold nuggets
```

The option participates in the normal `internal/opt` filename suffix and
metadata path. No trait or scheme is added because the treatment is deliberate
and opt-in, not part of the seed's normal output space.

## Implementation Boundaries

- `internal/sketch/scree/options.go` declares the boolean option.
- `internal/sketch/scree/colour.go` owns yellow filtering, the Avery gold
  constant, candidate selection, and final nugget pigment assignment.
- `internal/sketch/scree/scree.go` adds the isolated RNG stream and invokes the
  gold-mode colour path while planning the sheet.
- `docs/sketches/010-scree.md` documents the option and its visual contract.

No generic palette or scheme API changes are needed. This behavior depends on
scree stone size and belongs to the sketch.

## Verification

Tests defend these claims:

- without `--gold`, the existing golden and deterministic render are unchanged;
- with `--gold`, no ordinary stone has a yellow-like pigment;
- with `--gold`, exactly two or three nuggets are selected when enough
  candidates exist;
- every nugget is selected from the small-to-medium candidate set;
- nugget selection and rendering are deterministic;
- preview and print plans select the same nugget stone IDs.

Visual verification uses application output only. Render several `600x600`
seeds in gold mode and inspect a contact sheet, then render the requested seed-8
composition at `600x600` for approval. After approval, render that composition
at `2000x2000`.
