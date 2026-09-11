# Security boundaries

The public service exposes a curated finite artwork space through forms. It does not accept arbitrary CLI arguments, executables, output paths, arbitrary dimensions or private numeric artwork overrides. [Publication validation](../../internal/publish/catalog.go) is repeated in the web admission path, renderer protocol and child execution.

## Network and browser boundary

The web listener defaults to loopback. A non-loopback binding requires exactly one trusted proxy IP; only requests from that peer may supply the client identity header used for rate limiting. Caddy overwrites `X-Art-Client` and removes forwarded host/client headers. The canonical Host and POST Origin must match configuration. The private admin listener remains loopback-only and Caddy blocks its paths. The renderer uses HTTP over a mode-0660 Unix socket and has no network in Compose.

Responses set a same-origin Content Security Policy, deny framing, disable content sniffing, constrain referrers and browser permissions. Existing-session writes require constant-time CSRF token comparison. Cookies are HttpOnly and SameSite Lax; HTTPS adds Secure and the `__Host-` name. Public art pages/GETs do not create workspace state or jobs.

## Resource admission

The application bounds concurrent HTTP handling to 128, request targets to 4096 bytes and form bodies to 64 KiB. Token buckets separately limit reads/assets, new starts, generation per IP and generation per workspace. Default read rate is 240/minute burst 30, assets 1200/minute burst 60, starts 12/minute burst 2, generation IP 12/minute burst 3 and workspace 6/minute burst 3. Limiter keys are bounded to 10000 with idle pruning.

The queue has eight waiting positions and one serial worker; each workspace has up to four active interests. A supervisor admits only one child, with deadline/output bounds. Cache capacity, file count/age, job records, subscriber count, open image readers and recipe memory are separately bounded. See [data limits](../reference/data.md) and [private protocol](../reference/http-api.md).

## Execution and storage boundary

The child executable path is fixed by the supervisor, and the only child argument is `--child`; a recipe never becomes shell text. The renderer container has no network, no web cache mount, read-only root and a separate user. Docker imposes memory/CPU/process bounds. The web process alone publishes validated PNG files. Requests carry a release identity that both sides must match, so a mixed release fails closed.

Workspaces and capability tokens are private transient state. Artifact URLs are intentionally usable without a session while cached. Recovery JSON contains artistic recipes; it does not contain cookies, CSRF tokens or navigation capabilities. There are no user accounts or cloud favourites. Protect exported files according to the user's needs.

## Operational limits

The host operator has privileged infrastructure control. Terraform's local state, cloud credentials, SSH trust records, operator environment and release archives belong to that boundary. SSH provisioning trusts a first observed key (`accept-new`), and empty-hostname deployments use unencrypted HTTP. The code does not implement an external monitoring/alerting service or application-level account authentication. These are descriptions of the current system's boundaries, not assertions of a third-party security audit.

Source: [web guards](../../internal/web/web.go), [proxy tests](../../internal/web/proxy_test.go), [renderer protocol](../../internal/renderjob/protocol.go), [Compose](../../deploy/compose.yaml), [provisioning](../../deploy/scripts/provision-app.py). [Tests](../development/testing.md) describes the checks that exercise these controls.
