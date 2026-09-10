terraform {
  required_version = "= 1.14.9"
  required_providers {
    minio = {
      source  = "aminueza/minio"
      version = "= 3.33.1"
    }
  }
  # Bootstrap state stays local, independently backed up. The bucket must
  # exist before the separate VPS configuration can initialize its backend.
  backend "local" {}
}

variable "access_key" {
  description = "Hetzner S3 access key from the selected project's Security tab."
  type        = string
  sensitive   = true
  ephemeral   = true
}

variable "secret_key" {
  description = "Hetzner S3 secret key; supply with TF_VAR_secret_key."
  type        = string
  sensitive   = true
  ephemeral   = true
}

provider "minio" {
  minio_server        = "nbg1.your-objectstorage.com"
  minio_region        = "nbg1"
  minio_ssl           = true
  minio_user          = var.access_key
  minio_password      = var.secret_key
  skip_bucket_tagging = true # Hetzner does not implement bucket tagging.
}

resource "minio_s3_bucket" "state" {
  bucket_prefix  = "singular-seed-tfstate-"
  acl            = "private"
  force_destroy  = false
  object_locking = false # Retention locks are unrelated to Terraform locking.
  lifecycle { prevent_destroy = true }
}

resource "minio_s3_bucket_versioning" "state" {
  bucket = minio_s3_bucket.state.bucket
  versioning_configuration {
    status = "Enabled"
  }
  lifecycle { prevent_destroy = true }
}

output "bucket_name" {
  value = minio_s3_bucket.state.bucket
}

output "backend_hcl" {
  description = "Non-secret backend settings for deploy/terraform. Export AWS_SSE_CUSTOMER_KEY before use."
  value = replace(
    file("${path.module}/../terraform/backend.hcl.example"),
    "BUCKET-NAME-FROM-BOOTSTRAP", minio_s3_bucket.state.bucket
  )
  depends_on = [minio_s3_bucket_versioning.state]
}
