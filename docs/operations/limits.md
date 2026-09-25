# Operational limit profiles

Limits are validated at startup in [internal/limits](../../internal/limits/policy.go). No HTTP header or k6 option selects server policy. Binaries/Compose default to production; local helpers explicitly default to `local-capacity`.

## Defaults

Rates are tokens per minute plus an immediate burst. Browsers sharing an IP share IP buckets.

| Limit | Production | Local capacity |
| --- | --- | --- |
| Ordinary requests per IP (non-asset GET/POST) | 1200/min, burst 120 | 60000/min, burst 4000 |
| Assets per IP | 3000/min, burst 300 | 60000/min, burst 4000 |
| Starts/imports per IP | 60/min, burst 12 | 12000/min, burst 400 |
| Generation actions per IP | 120/min, burst 24 | 12000/min, burst 400 |
| Generation actions per visitor | 30/min, burst 6 | 600/min, burst 30 |
| Global outstanding jobs | 16 | 64 |
| Active interests per visitor | 4 | 4 |
| Maximum age at first start | 120 seconds | 120 seconds |
| Image responses per web | 32 | 32 |
| Encoded image buffer per web | 64 MiB | 64 MiB |
| Wait for image capacity | 2 seconds | 2 seconds |

Production values are shared-network-friendly starting policies, not a proven public-service SLO. Local settings let a single-IP generator keep renderers busy without disabling bounded work/resources.

Unchanged: one child per renderer, 15/30-second child deadlines, 90-second job timeout, three attempts, five-minute retry lifetime, 16 MiB PNG maximum, container CPU/memory, 128 HTTP slots, 32 SSE streams and existing workspace/storage caps. Increasing backlog does not guarantee every job starts before expiry: tune against execution time and renderer count.

## Configuration

```sh
# Local capacity (default)
bash deploy/scripts/browser-server.sh
# Production-policy test on disposable local stack
ART_LIMITS_PROFILE=production bash deploy/scripts/browser-server.sh
# Optional validated overrides
ART_LIMITS_JSON='{"outstanding_jobs":32,"queue_age_seconds":120}' \
  bash deploy/scripts/browser-server.sh
```

On Hetzner set `ART_LIMITS_PROFILE=production` and optional `ART_LIMITS_JSON` in `/etc/art/storage.env`. Compose passes the same policy to web/renderer. Recreate/redeploy to apply changes; restarting an old container does not change its environment. Effective web policy appears in `artctl status --json`, `make watch` and load `config.json`.

JSON overrides named fields such as `read_ip.per_minute`, `start_ip.burst`, `outstanding_jobs` and `image_buffer_mib`. Invalid fields/profiles/ranges and inconsistent bounds fail startup. Configure all replicas consistently: counts are global, maxima are startup configuration. Renderer evaluates first-start age. Limits do not automatically multiply with replicas.

Ad-hoc local Compose scale commands must preserve `ART_LIMITS_PROFILE=local-capacity` and any `ART_LIMITS_JSON` alongside secrets/network variables. Omitting them defaults to production and can recreate a local renderer with a different policy. Hosted release wrappers read the stored environment.

## Enforcement

- Web entry: per-process read/asset IP buckets and request/SSE bounds.
- Actions: PostgreSQL start/generation IP/visitor buckets, keys namespaced by profile.
- `QueueTx.Admit`: transactional global/per-visitor outstanding checks, coalescing and admission/build controls.
- Renderer: first-start age and execution bounds.
- Images: actual declared bytes and reader slot reserved before fetch, held through client writes and released on all exits. Capacity waits are bounded; zero wait is immediate/nonblocking.

Temporal action limits still count attempts before queue admission; replay/cached-download charging has not been redesigned. Read rates are per web; expensive-operation rates are shared. Neither is account authorization.

`X-Art-Limit` on relevant rejections reports `read-ip`, `asset-ip`, `start`, `generate-ip`, `generate-workspace`, `image-readers` or `image-bytes`. Load reports retain them with HTTP status/operation. Generic queue/capacity `ErrBusy` paths still share their message.

## Capacity verification

`make check-capacity` owns a disposable stack and runs 20 users for 30 seconds with one then two renderers. Initial runs completed 228 images/57 batches with one, and 376 images/94 batches with two (188 per renderer). No throttles, render failures or observation errors occurred. Queue-busy responses were 12 then 9: demand reached the backlog bound.

Mean server queue wait fell from 7.85s to 4.00s; client batch p95 from 12.14s to 6.36s. All jobs completed by the final snapshot. These are short local observations including drain tails and varying generated recipes, not production guarantees or CPU measurements.
