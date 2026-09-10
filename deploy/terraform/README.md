# Cloud objects and state

Terraform owns the server, primary IPs, firewall and SSH public key. Docker
Compose owns application processes; release scripts never run through a
Terraform provisioner. `environment` is staging by default.

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
