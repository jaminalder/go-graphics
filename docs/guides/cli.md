# Local command-line guide

`staticart` operates directly on local artwork libraries. It does not use the web service, public recipe allowlist or render queue. Source: [main](../../cmd/staticart/main.go), [sweep](../../cmd/staticart/sweep.go), [flock](../../cmd/staticart/flock.go).

## Commands

| Command | Purpose |
| --- | --- |
| `list` | List all fresh registered sketch definitions |
| `palettes` | List palette slugs with artist/artwork provenance |
| `render <sketch> [flags]` | Write one image |
| `traits <sketch> [flags]` | Resolve weighted traits without rendering; Traited sketches only |
| `sweep <sketch> [flags]` | Render seed and/or parameter combinations and a sheet |
| `flock <sketch> [flags]` | Explore or breed a Traited population and record its ancestry |
| `help` | Show command synopsis |

Options follow the sketch name. `render <sketch> --help` prints common and sketch-owned options. This help path prints flags and exits with the Go flag package's help error. The [sketch pages](../reference/sketches.md) include captured current help for browsing.

## Rendering and file names

| Flag | Default | Meaning |
| --- | --- | --- |
| `--profile` | `preview` | Named output dimensions |
| `--width`, `--height` | `0`, `0` | Override dimensions; supply both together |
| `--seed` | `42` | Unsigned 64-bit composition seed |
| `--palette` | `kandinsky-soft-pressure` | Local palette slug |
| `--aa` | `2` | Sampler supersampling per axis; flame orbit multiplier; painted sketches may ignore it |
| `--deep` | `false` | Request 16-bit PNG; supported by sampler/flame paths, not all painted sketches |
| `--format` | `png` | PNG or JPEG (`jpg` and `jpeg` accepted); deep requires PNG |
| `--out` | `out` | Output directory |

| Profile | Width × height |
| --- | --- |
| `preview` | 600 × 600 |
| `web` | 2000 × 2000 |
| `print` | 6000 × 6000 |
| `preview-tall` | 480 × 600 |
| `web-tall` | 1600 × 2000 |
| `print-tall` | 4800 × 6000 |

QQL is composed for a 4:5 frame; the CLI does not select it automatically. [Material behavior](../reference/materials.md) explains differences in AA/deep handling. Profiles are defined in [profile.go](../../internal/render/profile.go).

Files follow `<sketch><option-and-trait-suffix>_<palette>_<seed>_<width>x<height>.<format>`. PNG/JPEG metadata records software revision, recipe and traits, with 300 DPI for CLI output. Reusing the same path overwrites the prior file. Trait-selected colourways can supersede `--palette`: Pools, Foam, Scree and Riffle use `--colourway from-flag` to select the supplied palette; Flame uses `--cast from-flag`. See each sketch's help.

## Inspect traits

```sh
go run ./cmd/staticart traits iris --seed 42
go run ./cmd/staticart traits foam --seed 42 --fills watercolour
```

Trait output lists the resolved value and its weight relative to the dimension total; a zero-weight explicit choice is marked pinned. The command accepts `--seed` and sketch-owned flags, not the global render profile/palette options. Numeric overrides remain separate from a trait set. `contour`, for example, has no trait schema and rejects this command.

## Sweep a space

```sh
go run ./cmd/staticart sweep pools --seeds 1-12 --vary fill=busy,packed --out out/pools-sweep
go run ./cmd/staticart sweep warp --seeds 3,7 --vary detail=uniform,varied --profile web --out out/warp-sweep
```

Supply `--seeds` or at least one `--vary`. Seed lists accept comma-separated values and ranges. Repeat `--vary flag=v1,v2` to form a Cartesian product; the total is capped at 240 renders. Other flags pass to `render`.

Sweep owns `--out` (default `out/sweep`), `--cols` (automatic near-square), `--cell` (300 pixels) and `--jobs` (half the CPU count, bounded to 1–4 by default). Outputs include numbered images, `manifest.txt` and `sheet.png`. The manifest maps tile numbers to filenames and swept values. Full render commands are embedded in image metadata. Parallelism changes throughput, not artistic random streams. Larger painted images can require substantial memory per worker.

## Explore and breed flocks

```sh
go run ./cmd/staticart flock iris --count 24 --seed-base 1 --out out/iris-1
go run ./cmd/staticart flock iris --from out/iris-1 --likes 3,11 --count 20 --out out/iris-2
```

`--count` is required (1–120). Without parents, seeds begin at `--seed-base` (default 1). Breeding requires `--from` (directory or `flock.jsonl`) and liked **seeds**, supplied by `--likes`. `--boost` defaults to 3. Half the population derives from boosted trait weights; the rest keeps parent traits with neighbor seeds using stride 100003. This is trait/seed exploration, not geometry/genome crossover.

Flock owns `--out` (default `out/flock`), `--cols`, `--cell` (280), and `--jobs`. A sketch option with the same name, such as `--count`, is consumed by flock. The output includes numbered images, `sheet.png`, `manifest.txt`, and `flock.jsonl` records containing seed, traits, file, mode and parent. Keep the JSONL with the images to breed again. Source: [candidate planning](../../internal/explore/explore.go).
