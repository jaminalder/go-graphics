# Security boundaries

The public service exposes a curated finite artwork space through forms. It does not accept arbitrary CLI arguments, executables, output paths, arbitrary dimensions or private numeric artwork overrides. [Publication validation](../../internal/publish/catalog.go) is repeated in the web admission path, renderer protocol and child execution.

## Network and browser boundary

The web listener defaults to loopback. Non-loopback requires one trusted proxy IP; Caddy overwrites `X-Art-Client` and removes forwarded host/client headers. Canonical Host and POST Origin must match. Private control listeners are loopback-only and Caddy blocks their paths. Web/renderer communicate through PostgreSQL, not a render API. Renderer has database and bucket networking; children share that namespace by design.

Responses set a same-origin Content Security Policy, deny framing, disable content sniffing, constrain referrers and browser permissions. Existing-session writes require constant-time CSRF token comparison. Cookies are HttpOnly and SameSite Lax; HTTPS adds Secure and the `__Host-` name. Public art pages/GETs do not create workspace state or jobs.

## Resource admission

HTTP concurrency is bounded to 128, targets to 4096 bytes and forms to 64 KiB. Validated [profiles](limits.md) configure read/asset/start/generation buckets. Production rates are 1200/3000/60/120/30 per minute respectively; local-capacity increases rates for renderer tests. Keys remain bounded to 10000 with pruning; no client header selects policy.

Default queue caps are 16 production/64 local-capacity outstanding jobs and four active interests per workspace. River runs one child per renderer. Images reserve actual bytes (64 MiB total) and one of 32 readers, waiting at most two seconds. SSE retains 32 streams and 30-second reconciliation; streams do not renew identity lifetime. See [data limits](../reference/data.md).

## Execution and storage boundary

The child executable is fixed and its only argument is `--child`; recipes never become shell text. Children receive sanitized environment/stdin/stdout, but share the container network and can access same-UID mounted files: this is trusted artwork process isolation, not a hostile-code sandbox. Renderer validates and uploads PNGs, then publishes pointers with attempt/epoch fencing and River completion. Runtime database roles lack DDL; migrations use separate owner credentials. Web has read-only production bucket credentials; renderer has scoped upload/cleanup access. All production bucket requests verify TLS.

Cookie tokens are hashed for SQL lookup; workspace/CSRF data persists for 90-day idle retention. Artifact URLs intentionally work without a session when the key is known and retained. Export JSON contains recipes, not capabilities. There are no accounts/login or automatic cross-device recovery. Favourite previews persist with the anonymous owner. Image cleanup is bound to the configured endpoint/bucket/prefix and refuses an unrelated location.

## Operational limits

The host operator has privileged infrastructure control. Terraform's local state, cloud credentials, SSH trust records, operator environment and release archives belong to that boundary. SSH provisioning trusts a first observed key (`accept-new`), and empty-hostname deployments use unencrypted HTTP. The code does not implement an external monitoring/alerting service or application-level account authentication. These are descriptions of the current system's boundaries, not assertions of a third-party security audit.

Source: [web guards](../../internal/web/web.go), [proxy tests](../../internal/web/proxy_test.go), [renderer protocol](../../internal/renderjob/protocol.go), [Compose](../../deploy/compose.yaml), [provisioning](../../deploy/scripts/provision-app.py). [Tests](../development/testing.md) describes the checks that exercise these controls.
