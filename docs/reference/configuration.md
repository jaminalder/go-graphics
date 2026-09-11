# Configuration reference

## Application environment

| Variable | Default | Consumer and meaning |
| --- | --- | --- |
| `ART_ADDR` | `127.0.0.1:8080` | artweb public listener; non-loopback requires one explicit trusted proxy IP |
| `ART_ORIGIN` | `http://` + public address | artweb canonical scheme/host; exact Host and POST Origin checks |
| `ART_ADMIN_ADDR` | `127.0.0.1:8081` | artweb private controls, loopback-only |
| `ART_TRUSTED_PROXY` | empty | One unicast IP allowed to supply `X-Art-Client` |
| `ART_SOCKET` | `out/artrender.sock` in artweb/artrender | Private renderer socket; artctl renderer probe reads the variable directly |
| `ART_CACHE` | `out/cache` | Web-owned disposable image directory |
| `ART_GENERATION` | enabled unless exactly `off` | Startup job admission state |
| `ART_LOG_LEVEL` | `info` | Structured text logging: debug, info, warn or error |
| `GOMAXPROCS` | Go runtime default | Runtime CPU parallelism; Compose sets renderer to 2 |

Build identity is a linker-injected `main.build` value, default `development`, shared by web, renderer and artctl. It is not an environment variable. The Docker build supplies its source revision. Mixing builds fails renderer health/render validation. Source: [artweb](../../cmd/artweb/main.go), [artrender](../../cmd/artrender/main.go), [logging](../../internal/logging/logging.go), [Dockerfile](../../deploy/Dockerfile).

## Compose configuration

| Variable | Checked-in default / required | Meaning |
| --- | --- | --- |
| `ART_DOMAIN` | required | Caddy canonical site address (hostname/HTTPS or explicit HTTP address) |
| `ART_ORIGIN` | required | Exact browser origin including local port where used |
| `ART_APP_IMAGE`, `ART_EDGE_IMAGE` | development image tags | Release wrapper overrides with checked immutable image IDs |
| `ART_REVISION` | `development` | Build argument for development image creation |
| `ART_BIND`, `ART_BIND6` | `127.0.0.1`, `[::1]` | Host IPv4/IPv6 publishing addresses |
| `ART_HTTP_PORT`, `ART_HTTPS_PORT` | `8088`, `8443` | Published edge ports |
| `ART_PROXY_NET` | `172.30.80` | Prefix of fixed `/29` internal proxy subnet |
| `ART_GENERATION` | `on` | Web startup admission state |
| `ART_LOG_LEVEL` | `info` | Web/renderer log threshold |

[deploy/local.env](../../deploy/local.env) supplies a local HTTP origin. [operator.env.example](../../deploy/operator.env.example) illustrates host settings; the installer writes `/etc/art/operator.env`. Compose fixes public web `:8080`, admin loopback `:8081`, socket `/run/art/renderer.sock` and cache `/var/cache/art`. The release wrapper fixes project name `singular-seed` and combines operator settings with release image identities. See [operations](../operations/running.md).

## Limits and artwork configuration

Queue/cache defaults are concrete `renderjob.Config` fields, not exposed environment knobs. The public catalogue's preview/download policy is fixed in `publish.Tier`. [Data limits](data.md) and [HTTP limits](http-api.md) record those values. Local artwork controls use Go's `flag` plus sketch-specific declarations, documented on [individual sketch pages](sketches.md). Global render profiles do not alter the public catalogue's 600/1200-pixel policy.

Terraform inputs are documented in [provisioning](../operations/provisioning.md), with exact validators in [main.tf](../../deploy/terraform/main.tf).
