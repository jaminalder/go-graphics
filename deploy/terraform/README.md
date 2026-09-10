# Cloud objects and state

Terraform owns the server, primary IPs, firewall and SSH public key. Docker
Compose owns application processes; release scripts never run through a
Terraform provisioner. `environment` is staging by default. The selected server is **CAX11 (ARM64)**
in **nbg1 (Nuremberg)**; release images and host bootstrap target Linux ARM64.

## Access configuration

- `hcloud_token` is a sensitive Terraform input. Supply `TF_VAR_hcloud_token`
  through your local environment or credential manager. If another workflow
  already supplies `HCLOUD_TOKEN`, map it locally with
  `export TF_VAR_hcloud_token="$HCLOUD_TOKEN"`; the provider now uses the explicit
  Terraform variable. Do not put a token in the example, Git or cloud-init.
- `ssh_public_key_path` defaults to `~/.ssh/id_ed25519.pub`. Terraform expands
  the home path and reads the public key on the operator machine. The same
  value registers `macbook-key` and configures the cloud-init operator user.
  The private key is never read or uploaded.
- `admin_cidrs` is still required: set the public IPv4/IPv6 ranges allowed to
  reach SSH. The example documentation addresses must be replaced.

Hetzner SSH keys are scoped to a project. In a separate Hetzner project, the
same public key can be registered again. If `macbook-key` is already managed
by another Terraform configuration in the **same** project, reference it as
existing data rather than managing it twice. An unmanaged existing key can be
imported into `hcloud_ssh_key.admin` after reviewing ownership.

## State and host lifecycle

Before a real plan, independently bootstrap a private encrypted, versioned S3
state bucket. Keep backend configuration and credentials outside Git. Use
`backend.hcl.example`, then `terraform init -backend-config=/private/path/backend.hcl`.
Prove lock contention from two clients, recovery after interruption, and prior
state-version restoration in a disposable state key before provisioning.
Do not disable locking to accommodate a backend. Provider credentials stay on
the operator machine, never the app host or in cloud-init.

Bootstrap is immutable: changes to `user_data` may require replacement. We do
not ignore those changes. Read the saved plan; `prevent_destroy` and provider
deletion protections intentionally stop casual replacement. Rebuild only as an
explicit recovery/migration operation with backups and IP lifecycle reviewed.
Updating a release uses Compose and does not change cloud-init.

The state backend, DNS ownership, administrator CIDRs, spending ceiling and
apply remain owner decisions. No infrastructure has been provisioned by these
files. See [the operating runbook](../README.md).
