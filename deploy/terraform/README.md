# Provision a usable staging server

This root manages a CX23 (x86-64), Ubuntu 24.04, in nbg1, its IPs,
firewall and SSH key, then deploys a retained Linux amd64 release. Staging
opens over **HTTP at the assigned IPv4**, without DNS. Keep Terraform running:
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

Supply `TF_VAR_hcloud_token`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and
the **saved** `AWS_SSE_CUSTOMER_KEY` through your environment. The latter must
remain the same to read existing state; do not generate a replacement key.
The [separate state root](../terraform-state/README.md) creates the bucket once.
Keep its bootstrap state and the encryption key backed up outside the VPS.
Neither cloud nor S3 credentials are uploaded to the server.

From the repository root, build from a clean committed checkout, then select
that retained artifact (reuse it for subsequent rebuilds):

```sh
deploy/scripts/build-release.sh
export TF_VAR_release_directory="$PWD/out/releases/$(git rev-parse HEAD)"
terraform -chdir=deploy/terraform init -backend-config=backend.hcl
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
   and records the staging deployment selected by this apply. It invokes the
   release's existing smoke/drain/activate/rollback scripts and checks the site.

SSH uses trust on first connection with a host alias per Hetzner server ID,
in ignored `out/provision/known_hosts`. Reusing an IP after replacement does
not clash with the previous server's host key. This is first-use trust, not
out-of-band host-key verification. Manual SSH uses your normal known-hosts file.

If upload or activation fails, make a **fresh plan**, then apply it. The failed
application task retries against the existing server. Cloud-init changes
replace the server; selecting a different release only redeploys the app.
A failed cloud-init bootstrap needs diagnosis or explicit server replacement;
retrying an upload does not rerun first boot. The complete remote rebuild has
not yet been verified; local tests cover SSH account creation and deployment
failure handling.

## Replace or destroy

`protect_server` defaults to false and this root has no `prevent_destroy`.
To rebuild the server while keeping its IPs:

```sh
terraform -chdir=deploy/terraform plan -replace=hcloud_server.web -out=infra.tfplan
terraform -chdir=deploy/terraform apply infra.tfplan
```

To remove all resources in **this root**, then recreate them:

```sh
terraform -chdir=deploy/terraform destroy
terraform -chdir=deploy/terraform apply
```

Keep `release_directory` and the credentials available for these commands.
Full destruction removes the server, IPs, firewall and SSH-key resource. New
IPs may differ. The separately managed state bucket remains. Server replacement
and destruction lose local Docker volumes: cached artwork and Caddy data.
Visitor state is already in memory. This is disposable staging, not persistent
data recovery; retain required artwork/release archives outside the VPS.

### Existing protected server: one-time transition

The current server (165431120) was created with deletion/rebuild protection.
Disable both protections in Hetzner before applying its replacement (with the
CLI: `hcloud server disable-protection 165431120 delete rebuild`); setting
new defaults cannot remove protection from an old server being deleted.
The original SSH failure was reproduced locally: Ubuntu already has the
`operator` group, so cloud-init needs `primary_group: operator`. That fix is
included. Replacing this server preserves its current IP resources.

## Optional HTTPS and production

Set `hostname` to a DNS hostname to use HTTPS; point its DNS at the assigned
server addresses yourself. DNS is not managed here. Production requires a
hostname and a separately supplied `/etc/art/launch-approved` record; this
staging automation does not authorize or automate public launch approval.
The extra state lock/recovery exercises were deferred by the owner for staging
and remain unverified.
