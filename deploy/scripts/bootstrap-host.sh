#!/usr/bin/env bash
# Ubuntu 24.04 amd64 only. Run as root on an owner-approved staging/production host.
# Run by cloud-init. Installs Docker and configures host INPUT filtering.
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Run as root on the target host' >&2; exit 1; }
. /etc/os-release
[[ "$ID" == ubuntu && "$VERSION_ID" == 24.04 && $(dpkg --print-architecture) == amd64 ]]
apt-get update
apt-get install -y ca-certificates curl
install -d -m 0755 /etc/apt/keyrings /etc/docker /opt/art/releases /etc/art
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod 0644 /etc/apt/keyrings/docker.asc
cat > /etc/apt/sources.list.d/docker.sources <<'EOF'
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: noble
Components: stable
Architectures: amd64
Signed-By: /etc/apt/keyrings/docker.asc
EOF
apt-get update
apt-get install -y \
 docker-ce=5:29.8.0-1~ubuntu.24.04~noble \
 docker-ce-cli=5:29.8.0-1~ubuntu.24.04~noble \
 containerd.io=2.3.5-1~ubuntu.24.04~noble \
 docker-buildx-plugin=0.37.0-1~ubuntu.24.04~noble \
 docker-compose-plugin=5.4.0-1~ubuntu.24.04~noble
systemctl enable --now docker
docker version
docker compose version
# Hetzner's cloud firewall is the public perimeter for Docker-published ports.
# UFW protects host INPUT; Docker forwarding does not pass through those rules.
python3 - <<'PY'
import ipaddress
import json
import subprocess
from pathlib import Path

cidrs = json.loads(Path('/etc/art/admin-cidrs.json').read_text())
if not cidrs:
    raise SystemExit('At least one administrator CIDR is required')
for cidr in cidrs:
    ipaddress.ip_network(cidr, strict=False)
subprocess.run(['ufw', 'default', 'deny', 'incoming'], check=True)
for cidr in cidrs:
    subprocess.run(['ufw', 'allow', 'from', cidr, 'to', 'any', 'port', '22', 'proto', 'tcp'], check=True)
for port in ('80/tcp', '443/tcp'):
    subprocess.run(['ufw', 'allow', port], check=True)
subprocess.run(['ufw', '--force', 'enable'], check=True)
PY
echo 'Docker and host firewall ready. Terraform can now upload the application.'
