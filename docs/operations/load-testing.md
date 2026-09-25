# k6 load testing

Three HTTP scenarios run with pinned k6 1.6.1 in a disposable Docker container. They target an **already running** application; they do not change replica counts, application limits or database state directly. Only Docker and Python 3 are needed to run load, not a local k6/Node installation.

## Quick start

Local helpers now default to the bounded **local-capacity** profile, so shared-IP throttling does not dominate renderer tests. Production stays the default outside local helpers. See [profiles](limits.md) for values and production-policy tests. Recreate an older running environment to apply new limits.

From `master/`:

```sh
# Terminal 1: disposable application
bash deploy/scripts/browser-server.sh
# Terminal 2: monitor jobs and instances
make watch
# Terminal 3: one scenario at a time
make load SCENARIO=browse USERS=5 DURATION=2m
make load SCENARIO=studio USERS=3 DURATION=5m
make load SCENARIO=burst USERS=12 DURATION=2m
```

The launcher reads `out/browser-runtime.json`, discovers web's canonical origin and joins Caddy's edge network. It overrides k6's **dial address**, preserving URL/Host, cookies and POST Origin. This works on Linux/macOS/Colima without host networking or a database port. It traverses Caddy but bypasses host-port forwarding/public DNS, so local runs are application tests, not public-network benchmarks.

```sh
python3 deploy/scripts/load.py --project art-persistent-79509 --scenario studio --users 3 --duration 5m
python3 deploy/scripts/load.py --project art-persistent-79509 --dry-run
```

Use the current project name; the launcher does not generate replacement credentials. Ctrl-C stops only k6. Accepted jobs may still finish, and application records/images remain subject to normal retention. Favourites are normally added/removed in the same journey; interrupted/failed journeys can leave pins. Use a disposable environment for repeatable cleanup. No bulk cleanup of a hosted application occurs.

## Scenarios

| Scenario | Load shape | Work |
| --- | --- | --- |
| `browse` | Constant concurrent VUs | Gallery + artwork page + up to eight distinct catalogue image assets per page; optional existing bucket image; think time |
| `studio` | Constant concurrent VUs | Initial four-preview batch, wait/read PNGs, optional favourite/unfavourite, detail, download, similar batch, think time |
| `burst` | One-user baseline, quick ramp to USERS, peak, ramp to zero | Initial preview batches only, observe accepted results; no follow-up downloads/similarity during drain |

Burst stages are approximately 15% baseline, 5% ramp, 45% peak, one second ramp-down, remainder zero-user observation. k6 may finish a zero-user stage early; teardown waits remaining nominal time up to 60 seconds without requests. In-flight iterations continue observing accepted work during graceful ramp-down. It does not query global queue emptiness; use `make watch` to observe global drain.

These are **closed-loop user** tests: each VU waits for its accepted work before another iteration. USERS is not requests/second; slower rendering reduces offered demand. Fixed-arrival-rate testing is not included.

Each VU retains its own cookie jar and extracts current CSRF, revision, action and sample IDs from HTML. Pools/Foam/Iris and direction selection are seeded/reproducible, but application render seeds remain fresh: this is not exact artwork replay or guaranteed cold-cache benchmarking.

This executes HTTP, **not JavaScript/SSE or a browser**. Fragment polling observes accepted work every three seconds, then validates PNG signature/dimensions (600 previews, 1200 downloads). Completion latency starts before POST and ends after image retrieval; it includes polling resolution and transfer time. Browse does not simulate browser caching, CSS/JS execution or concurrent resource scheduling.

## Workload controls

```sh
# Preview-only steady pressure
make load SCENARIO=studio USERS=5 DURATION=10m \
  LOAD_ARGS='--download-every 0 --similar-every 0 --favourite-every 0 --think 0'

# More downloads, weighted artwork mix
make load SCENARIO=studio USERS=3 DURATION=10m \
  LOAD_ARGS='--download-every 1 --artworks iris,iris,pools,foam --seed 42'

# Existing bucket images, no new rendering
make load SCENARIO=browse USERS=5 DURATION=2m \
  LOAD_ARGS='--image-keys <64-character-rendition-key>'

# Finite full journey
make load SCENARIO=studio USERS=1 DURATION=3m \
  LOAD_ARGS='--iterations 1 --artworks iris --download-every 1 --similar-every 1 --favourite-every 1'
```

| Option | Default | Meaning |
| --- | --- | --- |
| `--users` | Make: 3; direct launcher browse: 5, others: 3 | Constant VUs or burst peak, 1–2000 |
| `--duration` | `2m` | Load phase/max-duration for finite iterations; graceful completion extends total time |
| `--iterations` | disabled | Iterations **per VU**, replaces constant/ramp profile |
| `--completion-timeout` | 120 seconds | Per batch/download observation deadline, 5–600 |
| `--poll` | 3 seconds | Observation interval, 2–30 |
| `--think` | 2 seconds | Successful iteration-end pause, 0–60 |
| `--artworks` | `pools,foam,iris` | Comma-separated mix; repeat names for weights |
| `--download-every` | 5 | Studio VU/iteration cadence; 1 always, 0 never |
| `--similar-every` | 2 | Similarity cadence; 0 disables |
| `--favourite-every` | 3 | Add then remove favourite; 0 disables |
| `--seed` | 1 | Workload-choice seed, not a render seed override |
| `--image-keys` | empty | Browse-only existing rendition keys; missing images count as errors |

Graceful stop/ramp-down allows `3 × completion-timeout + 90s` for studio/burst, 30 seconds for browse. A two-minute test can run longer. Ctrl-C can interrupt observation; check interrupted iterations and queue state rather than assuming every accepted job has a final measured outcome.

## Results

The 2000-user ceiling is our tool guardrail, not a k6 engine limit. `make load SCENARIO=browse USERS=500 DURATION=60s LOAD_ARGS='--think 0'` is supported. More VUs consume generator CPU/memory and do not bypass server IP/asset quotas. Above 1000 studio users, retained-identity capacity may also bind. Increase gradually and inspect per-operation limit counts before interpreting throughput.

Each run writes `out/load/<UTC-time>-<scenario>-<unique-id>/`:

- `config.json`: target, controls and pinned image, without app credentials.
- `console.log`: k6 progress, errors, interrupted iterations and summary.
- `summary.json`: aggregate k6 metrics/thresholds and preview/download latency.
- `summary.txt`: totals, throughput, latency and observation retries.
- `operations.json`: per-operation outcome, HTTP status and `X-Art-Limit` counts, explicitly retained as tagged submetrics.
- `server.json` / `server.txt`: server-clock window timings plus two-second queue/instance/policy samples through private Docker-local diagnostics. Missing/remote evidence is labelled unavailable, not zero work.
- `result.json`: exit/interruption status and summary availability; startup failures can have no summary.

Counters distinguish accepted actions, throttles, explicit queue-busy responses, conflicts, completed previews/downloads and render outcomes (ready/failed/timeout/observation error). `render_completion_ms` covers successful outcomes only: always read it alongside completion success/errors. HTTP latency measures individual requests, not async rendering.

The custom unexpected-response metric treats 429 as throttling and only a POST 503 with `generation is busy; try again shortly` as queue/admission busy. Other 503s remain unexpected storage/application errors. Conflicts are separate. Built-in `http_req_failed` still treats 409/503 conservatively; it is not identical to the custom metric.

Default thresholds fail on script/protocol errors, 1% or more unexpected responses, or 5% or more unsuccessful observed render completions. Studio/burst also require an accepted action **and** completed batch, so all-throttled runs cannot masquerade as renderer benchmarks. Browse requires a completed journey. Failure returns nonzero but preserves reports. No latency SLO is asserted.

## Scaling experiments and limits

Run the same mix for several minutes; keep `make watch` and `docker stats` open; change renderer counts with the existing Compose command. Compare completion latency/throughput, queue age, rejection rate and per-renderer distribution. Scale down gracefully and observe accepted work settle; forced termination is a separate failure experiment. k6 never scales services automatically.

Deployment limits stay active: default 16 production/64 local-capacity outstanding jobs and four active interests per visitor. Local rates are deliberately high but finite; production-policy tests can still be IP-throttled. No forged headers/direct enqueue bypass exists. The generator competes with local resources; later capacity tests should use a separate host.

Observation GETs retry network errors, 429 and 502/503/504 within the same deadline; POSTs are not automatically replayed. Retry-After is a minimum, with added jitter. Confirmed missing images are not retried. All errors remain counted even after successful retry; image/status retry and exhaustion counters separate delivery trouble from rendering. No extra rendering is requested to recover an image read.

Client completion includes queueing, render/upload, polling and transfer. Server evidence groups by tier/state/last renderer: enqueue-to-last-attempt mean, last-attempt-to-final mean and enqueue-to-final p95. Its half-open enqueue window uses database timestamps and includes **all clients**, so avoid unrelated activity for clean comparisons. Retry timing describes the last attempt, not total CPU; unfinished jobs stay unfinished, not falsely failed. Sampled running counts are not CPU utilization. Configuration and samples record effective policy and instances during scaling.

Remote generators cannot access private diagnostics. Collect `artctl load-stats --since <RFC3339> --until <RFC3339>` through web on the server, or run the launcher there with an explicit project. The endpoint remains private and no database credential goes to k6.

## Explicit remote target

```sh
python3 deploy/scripts/load.py --scenario studio --users 2 --duration 2m \
  --target https://your-studio.example --allow-remote
```

Non-loopback origins require explicit opt-in; certificate verification stays enabled and scripts do not follow off-origin redirects. This creates real sessions/jobs/images and consumes resources. No remote test is part of local validation. A remote target uses public DNS/HTTPS from k6. Local targets require an existing Compose environment; arbitrary non-Compose localhost servers are not supported by this launcher.

## Verification

```sh
node --test web/load/config.test.js
python3 deploy/tests/test_load.py
make check-load
make check-capacity
```

`check-load` owns an isolated disposable stack and runs browse, a full studio journey, existing-bucket reads and a timed burst. Unit tests cover bounded inputs, target selection, seeded choices and stage configuration. Actual k6 runs cover HTML parsing, cookies/CSRF and image completion. This is behavioral evidence, not a capacity benchmark.

Sources: [scenarios](../../web/load/scenarios.js), [configuration](../../web/load/config.js), [launcher](../../deploy/scripts/load.py), [verification](../../deploy/scripts/verify-load.sh).
