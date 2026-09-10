# Glossary for this track

Use these words as the web docs use them. Do not rename a designed mechanism
to sound more “cloud native”.

**Infrastructure as code (IaC)**:
The cloud objects (server, firewall, IPs, SSH key, later DNS) declared so that
`plan` shows the next change. Not a synonym for “all configuration on the
host”.

**Bootstrap**:
First-boot host facts: users, packages, directories, sshd, journal bounds.
This project's vehicle is cloud-init. It is not the application release.

**Release**:
A checksummed, immutable directory of Linux binaries plus the Caddyfile and
unit files that belong to that commit. Activation points `/opt/art/current` at
it. Distinct from an artwork **edition**.

**Admission**:
The application decision to accept expensive render work. The primary rate
limit that matters for this product. Distinct from TCP connection throttling.

**Edge**:
The process that speaks TLS to the internet. Here: Caddy, bound to :80/:443.
`artweb` listens on loopback only.

**Isolation**:
Making a renderer OOM, hang, or panic fail the job without taking down
browsing. Here: separate users, a Unix socket, and systemd cgroups.
Container runtimes are one way to get isolation, not the definition of it.

**Drain**:
Stop admitting new generation, wait for running jobs, then restart. The
current activation script already does this. It is not zero-downtime.

**Vertical scale**:
A larger VPS (more RAM/CPU). The first scaling move for a CPU-bound renderer.

**Horizontal scale**:
More machines. For this app that means a second renderer or a second web
process, which immediately raises the question of *who owns the only queue*.

**Scheduler**:
A system that places processes on machines and restarts them (Kubernetes,
Nomad, even systemd on one box). You already have a one-node scheduler.

**State**:
Anything that must survive a process restart. Terraform remote state, Caddy
certificates, `/etc/art/*`, and release tarballs are operator state.
Workspaces and the image cache are disposable by design.

**Hyperscaler**:
AWS/GCP/Azure-style platforms with a large managed-service catalogue. Hetzner
Cloud is still “cloud”; it is not that catalogue. The claim under test is that
you do not *need* the catalogue for this product.
