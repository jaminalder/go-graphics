# Experiment result — flame-wash

- Branch: `exp/flame-wash`
- Worktree: `../worktrees/flame-wash`
- Stage: stain Develop shipped for review

## What landed

- `Hist.Measure` shares log-density + optional DE between ember and wash
- `--medium wash` (weight 0) + `--manner stain` (appended schema)
- Wash Develop: load → FlatWash strength, mean colour → pigment, forced paper, no gleam
- Ember goldens / determinism unchanged (`make check` green)

## Review sheet

1000×1000 / q80 / spindle / tint split / cast zander-spindle:

| Seed   | File |
|--------|------|
| 26     | `out/flame-wash-review/…_26_1000x1000.png` |
| 100029 | `out/flame-wash-review/…_100029_1000x1000.png` |
| 5      | `out/flame-wash-review/…_5_1000x1000.png` |

```sh
go run ./cmd/staticart render flame --medium wash --manner stain \
  --cast zander-spindle --structure spindle --tint split \
  --width 1000 --height 1000 --quality 80 --palette zander-spindle \
  --seed 26 --out out/flame-wash-review
```

## Notes for the human

- Reads as pigment on paper, not a nebula with a cream fill.
- Cool nest / warm mass still structural; crossings glaze rather than overwrite.
- Tooth and blotch are visible; not a soft blur of ember.
- Seed 26 is the sparsest hero — if stain feels too ghostly there, lift Body/strength before inventing rimmed.

## Recommendation

Ship stain if the three heroes pass the brief's acceptance list. Defer rimmed/pooled until stain is wanted as the default wash look.
