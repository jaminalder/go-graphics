> Historical starting point, 2026-09-09. The owner superseded the systemd
> runtime choice on 2026-09-10; see [record 0002](0002-compose-before-provisioning.md).
> The original observation below is retained as decision history.

# The v1 runtime is already a complete small-shop stack

The public studio is designed as Terraform-managed Hetzner resources, cloud-init
bootstrap, Caddy at the edge, and two systemd services with separate cgroups.
Containers, Ansible, Redis, a database, and Kubernetes are deferred on purpose,
not missing features. Future sessions should teach *that* stack to fluency
before adding a second operating model.

This matters because stage 1 of the learning track is proving the designed
path, not replacing it with Docker Compose or a cluster so it “looks like
production”.
