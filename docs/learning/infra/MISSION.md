# Mission: run this studio on a VPS you can explain

## Why

You want to ship the public art studio on a cheap Hetzner VPS, keep the
infrastructure configuration small, and automate as much of the boring path as
possible. The underlying outcome is not “know Kubernetes”. It is: **be able to
stand up, operate, recover, and argue for a single-server production runtime**
without renting a hyperscaler control plane you do not need.

That argument is only honest if you have felt the limits of the small setup:
what a firewall actually blocks, what a reverse proxy actually terminates, what
a cgroup actually kills, what a restart actually loses, and which problems
really require a second machine or a scheduler.

## Success looks like

- You can draw the live request path from the internet to a renderer child and
  name the control at each hop (cloud firewall, host firewall, Caddy, `artweb`
  admission, Unix socket, `artrender` cgroup).
- You can recreate the host from a second machine in a measured time, from
  versioned Terraform state and a checksummed release, without copying secrets
  out of Git.
- You can explain, with this application's constraints, how far systemd +
  Caddy + Terraform go for zero-downtime and scaling, and what would have to
  change in the *application* before Kubernetes would help.
- The infrastructure source of truth stays small: cloud objects in Terraform,
  first-boot in cloud-init, process policy in unit files, releases in scripts.
  A new tool earns its keep by deleting a class of manual steps, not by adding
  a parallel universe.

## Constraints

- The product already chose one Hetzner VPS, Terraform, Caddy, systemd, no
  database, no Kubernetes, and a brief planned restart. Learn that stack
  first; do not “improve” it by adding Docker or a cluster before it has
  served real traffic.
- Do not edit `docs/web/*` or the worktree `deploy/` from this track. Capture
  desired implementation changes only in [ADDITIONS.md](ADDITIONS.md).
- Cloud create, DNS, and credentials remain owner actions. Learning includes
  reading plans and rehearsing locally; it does not include an unapproved
  `terraform apply`.
- Budget intent is a small VPS (on the order of the documented €15/month
  operating target), not an always-on managed Kubernetes bill.

## Out of scope until a later mission

- Multi-cloud abstractions, AWS landing zones, Terraform modules for their own
  sake.
- Service meshes, GitOps operators, and Helm-for-everything.
- Turning the art studio into microservices so that Kubernetes has more to
  schedule.
