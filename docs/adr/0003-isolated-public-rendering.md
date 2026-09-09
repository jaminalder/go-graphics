---
status: proposed
---

# Bound and isolate public rendering on one VPS

Use one Hetzner VPS with Caddy and separate systemd web/renderer services. The
web application owns a bounded in-memory job queue and disposable image cache;
the private renderer supervisor executes one fixed child process per admitted
job with hard deadline and cgroup resource limits.

The existing sampling, painting and histogram loops have no cooperative
cancellation. In-process HTTP timeouts would leave computation running, and
one shared memory limit could take down browsing. A private local process seam
provides enforceable isolation without a full engine cancellation migration,
Redis, a database, or a distributed queue. The cost is supervisor/protocol
testing and temporary job loss on restart.

Terraform provisions cloud resources; release scripts deploy versioned
binaries. Public release introduces CI/security/recovery gates, superseding
architecture decision 3's local-only policy when implemented. See
[security and operations](../web/security-and-operations.md).
