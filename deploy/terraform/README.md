# Cloud objects and state

Terraform owns the server, primary IPs, firewall and SSH public key. Docker
Compose owns application processes; release scripts never run through a
Terraform provisioner. `environment` is staging by default. The selected server is **CX23 (x86-64)**
in **nbg1 (Nuremberg)**; release images and host bootstrap target Linux x86-64.

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

Before a real plan, create the private, versioned Hetzner state bucket with
[the separate bootstrap configuration](../terraform-state/README.md). It also
outputs the backend settings for this directory. Keep backend configuration
and credentials outside Git. Supply the separate S3 credentials and the saved
SSE-C encryption key through the environment, then initialize this backend.
Prove lock contention from two clients, recovery after interruption, and prior
state-version restoration in a disposable state key before provisioning.
Do not disable locking to accommodate a backend. Provider credentials stay on
the operator machine, never the app host or in cloud-init.

Bootstrap is immutable: changes to `user_data` may require replacement. We do
not ignore those changes. Read the saved plan; `prevent_destroy` and provider
deletion protections intentionally stop casual replacement. Rebuild only as an
explicit recovery/migration operation with backups and IP lifecycle reviewed.
Updating a release uses Compose and does not change cloud-init.

The state backend live verification, DNS ownership, administrator CIDRs, spending ceiling and
apply remain owner decisions. The owner has created the SSH key, firewall and primary IPs. Server creation
failed for CAX11 in nbg1; CX23 is the replacement target. Generate a fresh plan
after this configuration change; do not reuse the partially applied plan. See [the operating runbook](../README.md).

The 2026-09-10 staging run proceeds at the owner’s request without the extra
lock-contention and restore exercises. These remain unverified, not passed.

## First-boot SSH failure (2026-09-11)

The CX23 was created, but both operator and root SSH logins were rejected.
The supplied public key matches the key offered by SSH and the fingerprint
shown in Hetzner. Local reproduction with Ubuntu 24.04 and real cloud-init
found that the existing `operator` group makes `useradd operator` fail unless
the primary group is explicit. The template now sets `primary_group: operator`.
The regression runs cloud-init's user/file modules and authenticates over SSH
inside a disposable Ubuntu container: `deploy/scripts/verify-cloud-init.sh`.

This fixes future provisioning; it does not repair the running server.
Use Hetzner's rescue system with `macbook-key` to access the installed disk
over SSH and inspect its cloud-init logs. Root SSH being disabled in the
installed OS does not disable root SSH in the separate rescue OS. The browser
console and new SSH keys are not required for this route.

Changing user_data triggers server replacement in this configuration. Keep
destruction protections in place while diagnosing; do not apply a replacement
plan merely to repair login. If rebuilding is chosen, replace only the server
through a deliberate protection/plan workflow, preserving the IPs and bucket.
