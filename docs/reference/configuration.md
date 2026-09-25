# Configuration reference

## Application environment

| Variable | Default | Consumer and meaning |
| --- | --- | --- |
| `ART_ADDR` | `127.0.0.1:8080` | artweb public listener; non-loopback requires one explicit trusted proxy IP |
| `ART_ORIGIN` | `http://` + public address | artweb canonical scheme/host; exact Host and POST Origin checks |
| `ART_ADMIN_ADDR` | `127.0.0.1:8081` | artweb private controls, loopback-only |
| `ART_TRUSTED_PROXY` | empty | One unicast IP allowed to supply `X-Art-Client` |
| `ART_DATABASE_URL_FILE` | required | Protected per-service PostgreSQL connection string file |
| `ART_S3_CREDENTIALS_FILE` | required | Protected JSON access_key/secret_key file |
| `ART_S3_ENDPOINT`, `ART_S3_REGION`, `ART_S3_BUCKET` | required | Private S3 location; HTTPS required in production |
| `ART_S3_PREFIX` | `artifacts` | Application-owned object prefix, explicitly bound to database |
| `ART_S3_LOCAL` | `false` | Explicit disposable development allowance for HTTP S3 |
| `ART_LOG_LEVEL` | `info` | Structured text logging: debug, info, warn or error |
| `ART_INSTANCE_NAME` | OS/Docker hostname | Optional explicit process label; boot ID remains unique. Default Compose display names are resolved by the host-side monitor |
| `ART_LIMITS_PROFILE` | `production`; local helper selects `local-capacity` | Validated finite request/admission/image policy; same setting for all replicas |
| `ART_LIMITS_JSON` | empty | Partial bounded JSON overrides, e.g. `{"outstanding_jobs":24}`; unknown/invalid fields fail startup |
| `GOMAXPROCS` | Go runtime default | Runtime CPU parallelism; Compose sets renderer to 2 |

Both service pgx pools currently allow eight connections; web's result listener uses another dedicated session. PostgreSQL Compose caps connections at 40. These are code/Compose limits, not environment knobs. Expensive-operation quotas are shared in SQL; read/asset quotas remain per web process. Generation admission is changed through CLI controls rather than environment initialization.

Build identity is linker-injected `main.build`, default `development`, shared by web, renderer and artdb. It selects a build-specific River queue and publication epoch. Admission is stored in SQL, not `ART_GENERATION`; `ART_SOCKET`/`ART_CACHE` no longer configure production runtime.

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
| `ART_DATABASE_NETWORK` | `singular-seed-data_default` | External private network owned by data project |
| `ART_SECRETS_DIR` | `/etc/art/secrets` | Host-side protected Compose secret files |
| `ART_LOG_LEVEL` | `info` | Web/renderer log threshold |

[operator.env.example](../../deploy/operator.env.example) describes origin settings; [storage.env.example](../../deploy/storage.env.example) describes separate bucket settings. Compose fixes web `:8080`, web-admin loopback `:8081`, renderer-health loopback `:8082`; no socket/cache volume. Release wrappers combine operator, storage and immutable-image configuration. Use the [local helper](../operations/persistence.md), not `deploy/local.env` alone.

## Limits and artwork configuration

Queue/runtime limits are explicit in [River worker configuration](../../internal/persistence/worker.go), [admission](../../internal/persistence/queue.go) and Compose. The public catalogue still fixes preview/download policy in `publish.Tier`. [Data limits](data.md) and [HTTP limits](http-api.md) record the bounds. Local sketch flags/profiles do not alter the public 600/1200-pixel policy.

[Operational profiles](../operations/limits.md) define production/local defaults and override ranges. Private monitoring and load reports expose effective settings; render deadlines, process limits and storage caps remain separate. Local ad-hoc scale commands must preserve profile/overrides, not fall back accidentally to production defaults.

Terraform inputs are documented in [provisioning](../operations/provisioning.md), with exact validators in [main.tf](../../deploy/terraform/main.tf).
