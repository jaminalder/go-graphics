terraform {
  required_version = "= 1.14.9"
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "= 1.68.0"
    }
  }
  backend "s3" {}
}
variable "hcloud_token" {
  description = "Hetzner project API token; supply through TF_VAR_hcloud_token."
  type        = string
  sensitive   = true
}
provider "hcloud" {
  token = var.hcloud_token
}
variable "name" { default = "singular-seed" }
variable "location" { default = "nbg1" }
variable "admin_cidrs" {
  type = list(string)
  validation {
    condition     = length(var.admin_cidrs) > 0 && alltrue([for cidr in var.admin_cidrs : can(cidrhost(cidr, 0))])
    error_message = "Provide at least one valid administrator IPv4 or IPv6 CIDR."
  }
}
variable "ssh_public_key_path" {
  description = "Public key file on the machine running Terraform."
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}
variable "ssh_identity_path" {
  description = "Local SSH identity path, passed to ssh; Terraform never reads the private key."
  type        = string
  default     = "~/.ssh/id_ed25519"
}
variable "release_directory" {
  description = "Retained release directory from build-release.sh; relative to deploy/terraform or absolute."
  type        = string
}
variable "hostname" {
  description = "Optional DNS hostname for HTTPS. Empty uses HTTP at the assigned IPv4."
  type        = string
  default     = ""
  validation {
    condition     = var.hostname == "" || can(regex("^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$", var.hostname))
    error_message = "Use a lowercase DNS hostname without a scheme, port or path."
  }
}
variable "protect_server" {
  description = "Hetzner delete/rebuild protection. False permits an explicit destroy/rebuild."
  type        = bool
  default     = false
}
locals {
  ssh_public_key = trimspace(file(pathexpand(var.ssh_public_key_path)))
  release_path   = abspath(pathexpand(var.release_directory))
  release_digest = filesha256("${local.release_path}/SHA256SUMS")
  site_origin    = var.hostname == "" ? "http://${hcloud_primary_ip.v4.ip_address}" : "https://${var.hostname}"
}
resource "hcloud_ssh_key" "admin" {
  name       = "macbook-key"
  public_key = local.ssh_public_key
}
resource "hcloud_firewall" "web" {
  name = var.name
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.admin_cidrs
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}
resource "hcloud_primary_ip" "v4" {
  name        = "${var.name}-v4"
  type        = "ipv4"
  auto_delete = false
  location    = var.location
}
resource "hcloud_primary_ip" "v6" {
  name        = "${var.name}-v6"
  type        = "ipv6"
  auto_delete = false
  location    = var.location
}
resource "hcloud_server" "web" {
  name         = var.name
  server_type  = "cx23"
  image        = "ubuntu-24.04"
  location     = var.location
  ssh_keys     = [hcloud_ssh_key.admin.id]
  firewall_ids = [hcloud_firewall.web.id]
  user_data = templatefile("${path.module}/../cloud-init/user-data.yaml", {
    ssh_public_key  = local.ssh_public_key
    bootstrap_host  = filebase64("${path.module}/../scripts/bootstrap-host.sh")
    install_release = filebase64("${path.module}/../scripts/install-release.sh")
    admin_cidrs     = jsonencode(var.admin_cidrs)
  })
  delete_protection  = var.protect_server
  rebuild_protection = var.protect_server
  backups            = true
  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
    ipv4         = hcloud_primary_ip.v4.id
    ipv6         = hcloud_primary_ip.v6.id
  }
  labels = { service = "singular-seed", environment = "production" }
}
output "ipv4" { value = hcloud_primary_ip.v4.ip_address }
output "ipv6" { value = hcloud_server.web.ipv6_address }

# Failure taints only this deployment task, not the server. The next apply
# retries the upload/activation against the existing host.
resource "terraform_data" "application" {
  triggers_replace = {
    server_id      = hcloud_server.web.id
    release_digest = local.release_digest
    origin         = local.site_origin
    deploy_script  = filesha256("${path.module}/../scripts/provision-app.py")
  }
  lifecycle {
    precondition {
      condition     = startswith(local.site_origin, "https://") || local.site_origin == "http://${hcloud_primary_ip.v4.ip_address}"
      error_message = "Use HTTPS with a hostname or HTTP at the assigned IPv4."
    }
  }
  provisioner "local-exec" {
    working_dir = abspath("${path.module}/../..")
    command     = "python3 deploy/scripts/provision-app.py"
    environment = {
      ART_SERVER_IP      = hcloud_primary_ip.v4.ip_address
      ART_SERVER_ID      = tostring(hcloud_server.web.id)
      ART_SSH_IDENTITY   = pathexpand(var.ssh_identity_path)
      ART_RELEASE_PATH   = local.release_path
      ART_RELEASE_DIGEST = local.release_digest
      ART_SITE_ORIGIN    = local.site_origin
    }
  }
}
output "site_url" {
  value      = local.site_origin
  depends_on = [terraform_data.application]
}
