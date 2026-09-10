# Mission: run a containerized studio on a VPS you can explain

Ship and operate Singular Seed on a cost-efficient Hetzner VPS, using small,
versioned configuration and repeatable releases. Container images and Docker
Compose are explicit learning goals, chosen by the owner on 2026-09-10.

Success means you can:

- Draw the live path from the internet through firewall rules, Caddy's port
  mappings, the internal bridge, web admission, a Unix socket and renderer child.
- Explain which files belong to an image, a volume, the host and Terraform
  state; recreate containers without accidentally deleting persistent state.
- Build a release, identify exactly which image runs, deploy, inspect logs and
  limits, roll back and explain what a restart loses.
- Recreate the host from another machine using independent state backups,
  retained image archives, TLS volumes and operator configuration; measure it.
- Explain how far one-host Compose can go and what application changes would
  be needed for multiple web replicas or Kubernetes.

The runtime is Terraform + cloud-init + Docker Engine/Compose + Caddy on one
Ubuntu VPS. Web and renderer remain separate containers and resource groups.
The renderer executes a child inside its container, never controls Docker.
There is one in-memory admission queue, no application database, no Redis and
no Kubernetes. A brief planned restart is acceptable.

Keep ownership clear: Terraform declares cloud objects; cloud-init and the
bootstrap script establish the host; Dockerfiles package software; Compose
configures containers/networks/volumes; release scripts coordinate activation.
Host systemd still supervises Docker. Containers share the Linux kernel, so
Linux permissions, cgroups and networking remain part of the lesson.

Cloud creation, credentials, DNS and public launch remain owner actions.
The approximate operating-cost target in the web docs is not an approved
spending ceiling. Include state storage, backups, domain and monitoring before
buying anything. Hyperscaler services, clusters and multi-cloud abstractions
are outside stage 1.
