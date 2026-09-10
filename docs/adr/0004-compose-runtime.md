---
status: accepted
---

# Package and operate the studio with Docker Compose

On 2026-09-10 the owner chose Docker Compose on the same single Hetzner VPS,
replacing ADR 0003's host systemd application services. Standard image packaging,
a locally reproducible topology and container operations are now explicit
learning goals. The cost is maintaining a container runtime, image lifecycle,
volume ownership and Docker-aware firewall policy.

Keep three containers: Caddy, artweb and artrender. The web process still owns
the sole bounded queue; the renderer still supervises a fixed child over a Unix
socket. Separate users/cgroups, explicit CPU/memory limits, read-only roots and
minimal mounts preserve the failure boundary. Only Caddy publishes ports; an
internal bridge and one explicit trusted proxy IP connect it to web. Renderer
networking is disabled. No application receives the Docker control socket.

Terraform owns cloud objects, cloud-init the host baseline, Compose process
policy, and scripts checksummed image archives plus smoke/drain/activation/
rollback. Production references immutable image IDs loaded from retained
archives, requiring no registry. Images contain binaries and Caddy configuration;
operator configuration and persistent TLS volumes remain external. A separate
staging-approval record permits rehearsal without claiming public launch approval.

A brief restart remains acceptable and destroys in-memory explorations. Compose
adds neither shared admission nor host failover. Kubernetes remains a later
learning lab. Actual host OOM, reboot, TLS, firewall and restore evidence is
required before launch; successful container startup is not that evidence.
