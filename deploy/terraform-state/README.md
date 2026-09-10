# Terraform state bucket

This directory creates one private, versioned Hetzner Object Storage bucket in
Nuremberg (`nbg1`). Its name starts with `singular-seed-tfstate-`; the provider
adds a suffix. It uses the MinIO provider to call Hetzner's S3 API. No MinIO
server or container is installed. The VPS remains in `../terraform`.

The configurations are separate because Terraform initializes its backend
before it can create resources. This configuration uses **local state**;
the VPS configuration uses the resulting bucket. Keep an encrypted backup of
this directory's `terraform.tfstate` outside this checkout and outside the VPS
after every apply. State and provider caches are ignored by Git.

## Credentials and creation

In the selected Hetzner project, open **Security → S3 Credentials → Generate
credentials**. Save both values in your password manager; the secret is shown
only once. These are separate from the cloud API token used by `hcloud`.
Load them into the terminal environment as `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY`, without pasting literal secrets into shell history.
Do not send credentials through chat.

From the repository root, in that same terminal:

```sh
export TF_VAR_access_key="$AWS_ACCESS_KEY_ID"
export TF_VAR_secret_key="$AWS_SECRET_ACCESS_KEY"
terraform -chdir=deploy/terraform-state init
terraform -chdir=deploy/terraform-state plan -out=bucket.tfplan
terraform -chdir=deploy/terraform-state apply bucket.tfplan
terraform -chdir=deploy/terraform-state output -raw backend_hcl > deploy/terraform/backend.hcl
```

Review the plan before applying: it should create only a bucket and its
versioning configuration. Keep the credential environment available for apply.
Creating the first bucket on the account starts Hetzner's Object Storage base
charge, even when empty; see [current pricing](https://www.hetzner.com/storage/object-storage/).
No bucket has been created or live backend checks performed by adding these files.

## Encryption and connecting the VPS configuration

Hetzner supports **SSE-C**, where you supply the encryption key with each
request. Generate a 32-byte key once (`openssl rand -base64 32`), save it in your
password manager with an independent recovery copy, and load that saved value
as `AWS_SSE_CUSTOMER_KEY`. Reuse this exact key for every state operation on
every operator machine. Losing it loses access to the encrypted state; do not
regenerate it on each login. Never put it in `backend.hcl`, Git or the VPS.

The generated backend file contains only bucket/endpoint settings. It enables
encryption and Terraform's `.tflock` locking. Bucket Object Lock is deliberately
off: retention locks are a different feature. Versioning retains earlier state
versions; it is not an independent backup.

Before provisioning, verify lock contention from two clients, interrupted-run
recovery and restoration of a previous encrypted version using a **disposable
state key**, not `singular-seed/staging.tfstate`. Actual compatibility with our
Terraform version remains unverified until these live checks pass. Never work
around a failure with `-lock=false`.

Then initialize the VPS configuration:

```sh
terraform -chdir=deploy/terraform init -backend-config=backend.hcl
```

If real VPS state already exists locally, back it up first and use
`init -migrate-state -backend-config=backend.hcl` to migrate it. Do not create a
fresh empty state over resources that already exist. For production, use a
separate backend file with `key = "singular-seed/production.tfstate"`.

Continue with [VPS access and lifecycle](../terraform/README.md). Keep the local
bootstrap state separate; running the VPS configuration cannot delete the
bucket. `prevent_destroy` protects both bucket and versioning within this
configuration, and `force_destroy = false` prevents emptying it during deletion.
These protections do not prevent deletion through the Hetzner console.

References: [Hetzner's Terraform bucket setup](https://docs.hetzner.com/storage/object-storage/getting-started/creating-a-bucket-minio-terraform/),
[S3 credentials](https://docs.hetzner.com/storage/object-storage/getting-started/generating-s3-keys/),
[supported encryption](https://docs.hetzner.com/storage/object-storage/supported-actions/),
[Terraform backend settings](https://developer.hashicorp.com/terraform/language/backend/s3).
