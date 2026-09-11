# Provision the production server

This root manages a CX23 (x86-64), Ubuntu 24.04, in nbg1, its IPs,
firewall and SSH key, then deploys a retained Linux amd64 release. There is
one production environment and one VPS. An empty `hostname` uses **HTTP at the
assigned IPv4**; a configured hostname uses HTTPS and requires external DNS.
Keep Terraform running:
its local deployment task waits for cloud-init, uploads over SSH and activates
Compose. A successful apply includes a working site check from your machine.

## Inputs and prerequisites

Use Terraform 1.14.9, Python 3.9+, OpenSSH and Docker on your machine.
`terraform.tfvars.example` documents the nonsecret inputs. Set `admin_cidrs`
to your current public address ranges; they control SSH in Hetzner and UFW.
`ssh_public_key_path` defaults to `~/.ssh/id_ed25519.pub`; the same key creates
`macbook-key` and the cloud-init operator account. `ssh_identity_path` defaults
to the matching private-key path. Native SSH uses it locally; Terraform does
not read the private key. For a passphrase-protected key, run
`ssh-add ~/.ssh/id_ed25519` first because deployment uses noninteractive SSH.

Supply `TF_VAR_hcloud_token` through your environment. No object storage,
S3 credentials or encryption key is needed; the app has no database or cloud
storage dependency. The cloud token is not uploaded to the server.

Terraform keeps its resource inventory in `deploy/terraform/terraform.tfstate`,
ignored by Git. Keep this file while infrastructure exists and back it up
outside this checkout and the VPS after applies. Use one operator checkout:
do not apply from another worktree or machine with fresh state against existing
resources. To move machines, transfer the latest local state securely first.
After a complete destroy, a fresh checkout can rebuild from the repo, credentials,
SSH key and retained release alone.

From the repository root, build from a clean committed checkout, then select
that retained artifact (reuse it for subsequent rebuilds):

```sh
deploy/scripts/build-release.sh
export TF_VAR_release_directory="$PWD/out/releases/$(git rev-parse HEAD)"
terraform -chdir=deploy/terraform init
terraform -chdir=deploy/terraform plan -out=infra.tfplan
terraform -chdir=deploy/terraform apply infra.tfplan
```

The builder refuses to overwrite an existing release. If it already exists,
select its directory without rebuilding. Alternatively persist its absolute
path as `release_directory` in your ignored `terraform.tfvars`. A saved plan
pins the checksum manifest; changing the artifact causes deployment to fail.
The release contains images and configuration, so the VPS needs no registry
login or Git checkout. `site_url` is the resulting address.

## What runs during apply

1. `main.tf` creates the cloud resources and renders cloud-init.
2. `cloud-init/user-data.yaml` creates `operator` with its explicit existing
   primary group, disables root/password SSH and runs `bootstrap-host.sh`.
3. Bootstrap installs the pinned Docker stack and enables UFW for host INPUT
   traffic. The Hetzner firewall protects public traffic, including Docker
   published ports; UFW alone does not filter Docker forwarding. Compose
   publishes only Caddy's 80/443.
4. `terraform_data.application` runs `provision-app.py` locally. It verifies
   the release, waits for SSH and cloud-init, then uploads a temporary archive.
5. The cloud-init-installed `install-release.sh` writes `/etc/art/operator.env`
   and records the deployment selected by this apply in `deployment-approved`.
   No separate manual approval file is needed. It invokes the
   release's existing smoke/drain/activate/rollback scripts and checks the site.

SSH uses trust on first connection with a host alias per Hetzner server ID,
in ignored `out/provision/known_hosts`. Reusing an IP after replacement does
not clash with the previous server's host key. This is first-use trust, not
out-of-band host-key verification. Manual SSH uses your normal known-hosts file.

If upload or activation fails, make a **fresh plan**, then apply it. The failed
application task retries against the existing server. Cloud-init changes
replace the server; selecting a different release only redeploys the app.
A failed cloud-init bootstrap needs diagnosis or explicit server replacement;
retrying an upload does not rerun first boot.
A complete destroy/apply rehearsal of this production-only configuration is
still pending; local tests cover SSH account creation and deployment failures.

## Replace or destroy

`protect_server` defaults to false and this root has no `prevent_destroy`.
To rebuild the server while keeping its IPs:

```sh
terraform -chdir=deploy/terraform plan -replace=hcloud_server.web -out=infra.tfplan
terraform -chdir=deploy/terraform apply infra.tfplan
```

To remove all resources in **this root**, then recreate them:

```sh
terraform -chdir=deploy/terraform plan -destroy -out=destroy.tfplan
terraform -chdir=deploy/terraform apply destroy.tfplan
terraform -chdir=deploy/terraform plan -out=infra.tfplan
terraform -chdir=deploy/terraform apply infra.tfplan
```

Keep `release_directory` and the credentials available for these commands.
Full destruction removes the server, IPs, firewall and SSH-key resource. New
IPs may differ. There is no state bucket in this configuration. Server replacement
and destruction lose local Docker volumes: cached artwork and Caddy data.
Visitor state is already in memory. This rebuild does not recover server-local
data; retain required artwork/release archives outside the VPS.

For the complete [destroy/recreate and Porkbun DNS procedure](../../docs/operations/provisioning.md#destroy-and-recreate-the-deployment),
including retained release inputs, backup loss and deployment retries, see the
operations guide. With `hostname = "singularseed.art"`, a new IP requires a
manual update to the domain's A record (and AAAA record if present) in Porkbun.
Terraform does not change DNS. A stale record can cause HTTPS setup or the final
website check to fail; correct DNS, let caches expire, then run a fresh apply.

### Existing installation: transition before the rebuild

This is a fresh setup after destruction, not a state migration. If the old
infrastructure still exists, destroy it using revision `cabf554` with its
existing backend configuration and credentials **before** initializing this
configuration. Keep the old state bucket until that destroy finishes; removing
these source files does not delete any existing bucket or cloud resource.

After the old infrastructure has been destroyed, an already initialized checkout
uses `terraform -chdir=deploy/terraform init -reconfigure` to forget the previous
backend. A fresh checkout uses plain `init`. Do not pass `backend.hcl` or use
`-migrate-state`. Existing ignored backend/cache files are not removed by this
cleanup.

Remove `environment` from your ignored `terraform.tfvars`. During the planned
full rebuild, set `name = "singular-seed"`; the old `singular-seed-staging` name
was a resource label, not another environment. Leave `hostname = ""` until DNS
is ready. Build a fresh release from the committed production-only changes and
select it with `release_directory` before planning destruction or creation.
Keep that artifact available through both operations.

Use the full destroy/apply sequence above for this transition. The host's
cloud-init-installed installer and release activation scripts change together;
old release archives expect the previous environment/approval contract. Do not
mix the new provisioner with the old host installer or treat pre-cleanup
archives as compatible manual rollback releases. After the rebuild, retain the
first working production-only release for subsequent rollback.

The current server has delete/rebuild protection disabled. If a later server
has protection enabled, disable it deliberately before a planned destruction;
changing a default does not remove protection on a server being deleted.

## Add the domain and HTTPS later

After the IP-only rebuild passes, point the domain's A record at the assigned
IPv4. Publish AAAA only after IPv6 access is verified. Set `hostname` to the
domain and make a fresh plan/apply. The same production server is redeployed
with an HTTPS origin; Caddy manages certificates. DNS is not managed here.
There is no environment switch or separate public-launch approval file.
