# HTTP interfaces

The public interface is server-rendered HTML and form submissions. It is not a general JSON render API. Paths below follow [internal/web/web.go](../../internal/web/web.go); private interfaces follow [artweb](../../cmd/artweb/main.go), [artctl](../../cmd/artctl/main.go) and [render protocol](../../internal/renderjob/protocol.go).

## Public reads

GET and HEAD use the same route selection. Every request must use the configured canonical Host. Reading never creates a workspace or render job, though session reads refresh its idle timestamp.

| Path | Result |
| --- | --- |
| `/` | Gallery with published entries |
| `/about` | About page |
| `/art/<artwork>` | Artwork and visual choices |
| `/favourites` | Kept samples; empty page without a session |
| `/recover` | 303 redirect to favourites |
| `/explorations/<id>` | Owned exploration, batch and sample status |
| `/explorations/<id>/samples/<sample>` | Owned sample detail |
| `/fragments/explorations/<id>` | Main HTML fragment for polling |
| `/fragments/explorations/<id>/samples/<sample>` | Sample main fragment |
| `/recovery` | Existing workspace's `{version:1, recipes:[...]}` JSON |
| `/export` | Same recovery records as `art-favourites.json` attachment |
| `/images/<64-character-key>` | Available PNG, public immutable cache headers and ETag |
| `/downloads/<64-character-key>` | PNG with attachment filename `singular-seed-<key-prefix>.png` |
| `/assets/<hash>/<name>` | Embedded static asset only when its hash matches |
| `/health/live`, `/health/ready` | 204 from web route handling; neither probes the renderer |

Exploration, sample and recovery reads require a valid workspace cookie. Image/download reads do not. A missing artifact returns 410 and never starts work. Assets and ready images use `public, max-age=31536000, immutable`; ordinary HTML defaults to `private, no-store`. Caddy hides health/private paths from the public origin.

## Public form writes

POST requires an exact configured `Origin`, exact `Content-Type: application/x-www-form-urlencoded`, no query string, a body of at most 65536 bytes and allowed form keys only. Existing workspaces must submit their `csrf` token. Fresh sessions may be created only by exploration entry or restore. Single fields reject duplicates; the parser permits at most four values but currently exposed actions select only one similarity parent.

| Path | Fields in addition to `csrf` | Operation |
| --- | --- | --- |
| `/explorations` | `artwork`, `style`, `colour`, `action` | Create/reuse artwork exploration and atomically request four samples |
| `/restore` | `recovery` | Validate versioned JSON text and restore into fresh navigation |
| `/clear` | none | Remove server workspace and cancel its interests |
| `/explorations/<id>/choices` | `revision`, `style`, `colour` | Change current pins |
| `/explorations/<id>/batches` | `revision`, `action`, `batch` | Retry the unavailable latest batch using its original direction |
| `/explorations/<id>/similar` | `revision`, `action`, `sample` | Generate four images in the selected sample's family |
| `/explorations/<id>/favourites` | `revision`, `sample`, `on`, `return` | Set favourite when `on=yes`; optional return is `sample` or `favourites` |
| `/explorations/<id>/download` | `sample` | Request/reuse larger rendition; no revision required |
| `/explorations/<id>/cancel` | `revision`, `batch` | Cancel unfinished interests in latest batch |

Success redirects with 303. Revisions are nonnegative integers; commands validate current ownership and revision. Action identifiers are 48 hexadecimal characters. Exact replay returns the original admitted result within retained history; reuse with a different request conflicts. Errors use HTML pages: 400 invalid input, 403 origin/CSRF, 409 conflict, 410 expiry, 413 oversized form, 415 unsupported content type, 421 unknown host, 429 rate limit, 503 finite capacity. Busy/rate replies set `Retry-After: 10`.

## Private administrator interface

`ART_ADMIN_ADDR` must be a literal loopback IP and port; default `127.0.0.1:8081`. This listener is distinct from public route security and is not authenticated for remote use. It must remain local to the process/container network namespace.

| Method and path | Behavior | `artctl` command |
| --- | --- | --- |
| `GET /ready` | 204 only if renderer health succeeds with the matching build; otherwise 503 | `ready` |
| `GET /metrics` | Four queue/cache numeric counters | `metrics` |
| `POST /generation/off` | Disable new admission; 204 | `generation-off` |
| `POST /generation/on` | Enable admission; 204 | `generation-on` |

`artctl live` calls public loopback `:8080/health/live` with Host derived from `ART_ORIGIN`. The command's ports are hard-coded, so changing admin/public ports also requires using another HTTP client. `renderer-ready` checks the Unix socket from `ART_SOCKET`; it needs the matching build. Private commands accept one operation and use a three-second deadline.

## Renderer socket protocol

`artrender` listens on `ART_SOCKET` with mode 0660. `GET /health` returns 204 and `X-Renderer-Build`. `POST /render` accepts strict JSON up to 32768 bytes:

```json
{"version":1,"build":"<matching-release>","recipe":{},"tier":"preview"}
```

The empty recipe above is a structural placeholder, not an executable request; the `recipe` value must be a complete validated [edition record](data.md). `tier` is `preview` or `download`. Unknown fields, duplicate JSON keys, unsupported editions and release mismatches fail closed. A successful response is `image/png` with `X-Renderer-Build`; invalid input returns 400 and execution failure 503.

There is at most one active child. The supervisor starts only its fixed executable with `--child`, passes JSON over stdin and accepts PNG on stdout. Preview deadline is 15 seconds, download 30 seconds; stdout is limited to 16 MiB and stderr to 8192 bytes. The client has a 35-second overall timeout and rejects oversized or mismatched-build responses. The manager validates the PNG and dimensions before publication. This protocol is private, not routed by Caddy.
