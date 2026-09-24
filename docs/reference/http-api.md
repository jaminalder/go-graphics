# HTTP interfaces

The public interface is server-rendered HTML and form submissions. It is not a general JSON render API. Paths below follow [internal/web/web.go](../../internal/web/web.go); private interfaces follow [artweb](../../cmd/artweb/main.go), [artctl](../../cmd/artctl/main.go) and [render protocol](../../internal/renderjob/protocol.go).

## Public reads

Every request must use the canonical Host. Ordinary GET/HEAD share route selection, but SSE is GET-only. Reads never create an identity/job. Meaningful page GETs renew an existing identity and cookie; HEAD, image, asset, fragment, health and SSE requests do not. `Session` lookup alone does not renew expiry.

| Path | Result |
| --- | --- |
| `/` | Gallery with published entries |
| `/about` | About page |
| `/art/<artwork>` | Artwork and visual choices |
| `/favourites` | Kept samples; empty page without a session |
| `/recover` | 303 redirect to favourites |
| `/explorations/<id>` | Owned exploration, batch and sample status |
| `/explorations/<id>/samples/<sample>` | Owned sample detail |
| `/fragments/explorations/<id>` | Main HTML fragment refreshed after result events |
| `/events/<id>` | Authorized SSE snapshot/change/heartbeat hints; no work creation or session refresh |
| `/fragments/explorations/<id>/samples/<sample>` | Sample main fragment |
| `/recovery` | Existing workspace's `{version:1, recipes:[...]}` JSON |
| `/export` | Same recovery records as `art-favourites.json` attachment |
| `/images/<64-character-key>` | Available PNG, public immutable cache headers and ETag |
| `/downloads/<64-character-key>` | PNG with attachment filename `singular-seed-<key-prefix>.png` |
| `/assets/<hash>/<name>` | Embedded static asset only when its hash matches |
| `/health/live` | Web-process liveness; 204 without database access |
| `/health/ready` | Application/River schema check; 503 on database incompatibility/unavailability, no renderer probe |

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
| `/explorations/<id>/favourites` | `revision`, `sample`, `on`, `return` | Set/pin favourite when `on=yes`; explicitly admits an unavailable preview; optional return is `sample` or `favourites` |
| `/explorations/<id>/download` | `sample` | Request/reuse larger rendition; no revision required |
| `/explorations/<id>/cancel` | `revision`, `batch` | Cancel unfinished interests in latest batch |

Success redirects with 303. Revisions are nonnegative; commands validate ownership/revision. Action IDs are 48 hexadecimal characters; retained replay returns the original result, and conflicting reuse is rejected. Errors: 400 invalid input, 403 origin/CSRF, 409 conflict, 410 expiry/confirmed missing object, 413 oversized form, 415 content type, 421 Host, 429 rate, 503 capacity or database/bucket failure. Busy/rate replies set `Retry-After: 10`; storage failure never clears the visitor cookie.

## Private administrator interface

`ART_ADMIN_ADDR` must be a literal loopback IP and port; default `127.0.0.1:8081`. This listener is distinct from public route security and is not authenticated for remote use. It must remain local to the process/container network namespace.

| Method and path | Behavior | `artctl` command |
| --- | --- | --- |
| `GET /ready` | 204 only if renderer health succeeds with the matching build; otherwise 503 | `ready` |
| `GET /metrics` | Four queue/cache numeric counters | `metrics` |
| `POST /generation/off` | Disable new admission; 204 | `generation-off` |
| `POST /generation/on` | Enable admission; 204 | `generation-on` |

`artctl live` calls loopback `:8080/health/live` with canonical Host. `ready` queries both schemas and recent matching renderer/storage status through PostgreSQL. `renderer-live`/`renderer-ready` call renderer-local HTTP `:8082`. Ports are fixed in artctl. Commands use a three-second deadline. Admission switches are durable in SQL; process restart does not reset them.

## Renderer child protocol

There is no production renderer HTTP/Unix socket API. River invokes a handler which starts its fixed executable with `--child`, passing strict JSON up to 32768 bytes over stdin:

```json
{"version":1,"build":"<matching-release>","recipe":{},"tier":"preview"}
```

The empty recipe is a placeholder; the recipe must be a complete validated [edition record](data.md), tier preview/download. Unknown fields, duplicate keys, unsupported editions and build mismatches fail closed. Child stdout is complete PNG; failure exits nonzero. This pipe interface is not publicly routable.

One child is active per renderer. Preview deadline is 15 seconds, download 30; stdout is at most 16 MiB and stderr 8192 bytes. River jobs have a 90-second total bound including upload/publication. Parent validates PNG/dimensions and commits a fenced pointer after upload. Children share container networking but receive a sanitized environment.
