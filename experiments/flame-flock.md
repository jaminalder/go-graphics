# Flame flock — first curation pass

QQL output space + Electric Sheep likes→breed (decision 60).

## Gen 1 — explore

```sh
staticart flock flame --count 48 --seed-base 1 --out out/flame-flock-1 \
  --width 400 --height 400 --quality 24 --estimator 3 --aa 1 --oversample 2
```

Liked seeds (Zander spindle motif: void, split, pointed mass, cool nest):
`26, 30, 17, 23, 14, 37`.

## Gen 2 — breed

```sh
staticart flock flame --from out/flame-flock-1 --likes 26,30,17,23,14,37 \
  --count 36 --seed-base 100 --out out/flame-flock-2 \
  --width 400 --height 400 --quality 24 --estimator 3 --aa 1 --oversample 2
```

Structure frequency shifted: filament 18→7, spindle 13→25 (of 36).

## Heroes (1000² / q80 / estimator 3)

| seed   | notes |
|--------|--------|
| 26     | liked parent — gold spindle, cyan lobes, void |
| 100029 | neighbor of 26 — denser amber nest in cyan shell |
| 30     | liked parent — open silk almond, gold core |
| 200036 | neighbor of 30 — wilder reach, shredded core |
| 111    | boosted draw — chorus arcs, icy ring around gold |
| 17     | liked diebenkorn cast — frost-blue petals |

Files under `out/flame-heroes/` (gitignored). Re-render with the
commands in the flock-2 `manifest.txt` pins plus
`--width 1000 --height 1000 --quality 80`.
