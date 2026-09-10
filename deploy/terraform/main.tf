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
provider "hcloud" {}
variable "name" { default = "singular-seed" }
variable "location" { default = "nbg1" }
variable "admin_cidrs" { type = list(string) }
variable "ssh_public_key" { type = string }
resource "hcloud_ssh_key" "admin" {
  name       = "${var.name}-admin"
  public_key = var.ssh_public_key
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
  lifecycle { prevent_destroy = true }
}
resource "hcloud_primary_ip" "v6" {
  name        = "${var.name}-v6"
  type        = "ipv6"
  auto_delete = false
  location    = var.location
  lifecycle { prevent_destroy = true }
}
resource "hcloud_server" "web" {
  name               = var.name
  server_type        = "cx23"
  image              = "ubuntu-24.04"
  location           = var.location
  ssh_keys           = [hcloud_ssh_key.admin.id]
  firewall_ids       = [hcloud_firewall.web.id]
  user_data          = templatefile("${path.module}/../cloud-init/user-data.yaml", { ssh_public_key = var.ssh_public_key })
  delete_protection  = true
  rebuild_protection = true
  backups            = true
  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
    ipv4         = hcloud_primary_ip.v4.id
    ipv6         = hcloud_primary_ip.v6.id
  }
  lifecycle { prevent_destroy = true }
  labels = { service = "singular-seed", environment = "staging" }
}
output "ipv4" { value = hcloud_primary_ip.v4.ip_address }
output "ipv6" { value = hcloud_primary_ip.v6.ip_address }
