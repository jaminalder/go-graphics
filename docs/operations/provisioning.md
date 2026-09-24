# Provisioning the supplied host

The supplied target is Ubuntu 24.04 amd64 on a Hetzner `cx23` in `nbg1`. Terraform provisions infrastructure and, when `deploy_application=true` (default), deploys a selected local release after storage bootstrap. These pages describe configuration, not the actual state of a remote host.

The application now needs [persistent data bootstrap](persistence.md#fresh-vps-bootstrap) and a private managed image bucket. On a fresh VPS set `deploy_application=false` until credentials/bootstrap are prepared, then enable deployment. Credentials never enter Terraform state or cloud-init. Installer can stage a verified release and stop with bootstrap instructions. Database backup work is excluded; destroying the VPS loses PostgreSQL metadata while the separate bucket survives.

## Inputs and local tools

[Terraform](../../deploy/terraform/main.tf) pins Terraform 1.14.9 and the `hetznercloud/hcloud` provider 1.68.0. Provisioning also uses Python 3, local SSH/SCP, and a previously built [release](releases.md). The private key is passed to the SSH client by path; the Python task does not read or upload it.

| Input | Meaning |
| --- | --- |
| `hcloud_token` | Sensitive project API token, supplied using `TF_VAR_hcloud_token` |
| `name` | Resource name; default `singular-seed` |
| `location` | Default `nbg1` |
| `admin_cidrs` | Required nonempty administrator IPv4/IPv6 CIDR list for SSH |
| `ssh_public_key_path` | Default `~/.ssh/id_ed25519.pub` |
| `ssh_identity_path` | Default `~/.ssh/id_ed25519`; consumed by local SSH |
| `release_directory` | Required retained release directory, relative to Terraform directory or absolute |
| `hostname` | Optional lowercase hostname; empty uses HTTP at assigned IPv4 |
| `protect_server` | Delete/rebuild protection, default false |
| `deploy_application` | Default true; false omits application upload/activation for fresh-host preparation; a retained release path/checksum is still required |

Start from [terraform.tfvars.example](../../deploy/terraform/terraform.tfvars.example), supply the intended values, and inspect the plan before an operator applies it:

```sh
terraform -chdir=deploy/terraform init
terraform -chdir=deploy/terraform plan
terraform -chdir=deploy/terraform apply
```

The token is an environment secret, not an example value to commit. State is local `deploy/terraform/terraform.tfstate`; retain and protect it as infrastructure state. A supplied hostname needs DNS pointing to the assigned server for canonical access/TLS; Terraform here does not create a DNS record.

## What apply creates and runs

Terraform declares a public SSH key, cloud firewall, separately retained IPv4/IPv6 resources and the VM with backups enabled. Inbound TCP 80/443 are public; TCP 22 is limited to administrator CIDRs. Cloud-init creates the operator account, installs a root release helper, applies host hardening and runs the pinned Docker bootstrap.

`bootstrap-host.sh` installs Docker Engine/CLI 29.8.0, containerd 2.3.5, buildx 0.37.0 and Compose plugin 5.4.0 from the Docker Ubuntu repository. It configures UFW for host INPUT traffic. Docker-published traffic is controlled at the cloud firewall perimeter; UFW INPUT alone is not its boundary.

When enabled, `terraform_data.application[0]` verifies the release digest, waits for SSH/cloud-init, uploads and calls the installer. A `moved` block preserves the prior unindexed resource address on upgrade. Failed application tasks retry independently of VM creation. Triggers include server, release digest, origin and provisioning-script digest. SSH uses a per-server alias and `StrictHostKeyChecking=accept-new`, with known hosts in `out/provision/known_hosts`; this is first-observed trust, not out-of-band key verification.

The enabled task checks the origin without operator HTTP proxies. `site_url` is also available with deployment disabled and then denotes the intended URL, not a running application. HTTPS-hostname and HTTP-by-IPv4 modes differ; the latter does not encrypt traffic.

## Destroy and recreate the deployment

A full destroy/apply recreates infrastructure, but a new host needs explicit persistent-data/credential bootstrap before application activation. Use `deploy_application=false` during preparation. DNS is separate: update Porkbun when addresses change.

Keep the local Terraform state and input configuration, API token, SSH key pair, and the complete `release_directory` outside the VPS. The release normally lives under `out/releases/`, which is ignored by Git; a fresh checkout alone does not restore it. These local files must remain available when planning destruction and recreation because Terraform reads the public key and release checksum manifest. Applying deploys the selected retained release; it does not build the latest `master`. To deploy a newer version, [build and select a new release](releases.md) first. Before recreating, check that `admin_cidrs` still includes your public IP and unlock a passphrase-protected SSH key in your local agent.

From the repository root, using the existing operator checkout and state:

```sh
terraform -chdir=deploy/terraform destroy

# Later, recreate the host before persistent-data bootstrap:
terraform -chdir=deploy/terraform apply -var='deploy_application=false'
# Follow persistence.md bootstrap, then deploy the retained release:
terraform -chdir=deploy/terraform apply -var='deploy_application=true'
```

Both commands show a plan and request confirmation. If server deletion protection is enabled, disable it with a normal apply before destroying; merely changing `protect_server` in the file does not change the existing server during destroy.

Destruction removes the resources managed by this Terraform root: the VM, both primary IPs, cloud firewall and cloud SSH-key registration. `auto_delete = false` retains the IPs when replacing only the VM, but does not retain them during a full Terraform destroy. New addresses may differ. Domain registration, Porkbun DNS records and resources outside this state are unaffected.

The server's local volumes disappear, including PostgreSQL identities/favourites/jobs and Caddy certificate state. Managed image objects survive separately but cannot reconstruct the database. Export wanted recipes/images. Database backup/restore is outside this task. Existing provider VM backups are not an application restore mechanism; see [Hetzner's lifecycle](https://docs.hetzner.com/cloud/servers/backups-snapshots/faq/).

### Reconnect the domain in Porkbun

For the `singularseed.art` deployment, keep `hostname = "singularseed.art"`. Terraform configures the application's HTTPS origin from this value, but has no Porkbun provider or DNS resources.

1. Once the new server exists, obtain its assigned addresses from the Hetzner Console, or from Terraform after the apply finishes or fails:

   ```sh
   terraform -chdir=deploy/terraform output -raw ipv4
   terraform -chdir=deploy/terraform output -raw ipv6
   ```

2. If an address changed, open Porkbun **Domain Management → singularseed.art → Details → DNS Records**. Edit the website's existing **A** record to use the new IPv4. Update any existing **AAAA** record to the new IPv6, or remove that website AAAA record if IPv6 will not be used. Do not leave it pointing at the old server. For the root domain, Porkbun uses a blank Host field. See [Porkbun's record editing guide](https://kb.porkbun.com/article/68-how-to-edit-dns-records) and [record creation guide](https://kb.porkbun.com/article/231-how-to-add-dns-records-on-porkbun).
3. Save the records and allow cached DNS answers to expire. The domain must reach the new server for HTTPS certificate issuance and the deployment's website checks. If the first apply fails while DNS still points at the old IP, wait for DNS to resolve correctly and run a fresh `terraform -chdir=deploy/terraform apply`. The failed application task retries against the existing server; another destroy is unnecessary.
4. After apply succeeds, open `https://singularseed.art/` and check the studio. If it still fails, inspect the activation output and Caddy logs using the [running guide](running.md).

An empty `hostname` uses HTTP at the assigned IPv4 and needs no DNS update; use the new `site_url` output after recreation.

## Troubleshooting and retained state

For SSH failures, check the chosen identity/agent, administrator CIDR and server host key. For bootstrap failures, inspect `cloud-init status --long` and cloud-init logs on the host. For upload/activation failures, inspect the application task output and [release procedure](releases.md). Keep `/opt/art/releases`, `/etc/art/operator.env`, Terraform state and SSH known-host records available to the operator. Artist favourites are not stored in Terraform or host provisioning state.

Source: [Terraform resources](../../deploy/terraform/main.tf), [cloud-init template](../../deploy/cloud-init/user-data.yaml), [bootstrap](../../deploy/scripts/bootstrap-host.sh), [provisioning task](../../deploy/scripts/provision-app.py), [installer](../../deploy/scripts/install-release.sh).
