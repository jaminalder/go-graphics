#!/usr/bin/env bash
# Ubuntu 24.04 arm64 only. Run as root on an owner-approved staging/production host.
# Installs the runtime; does not launch the app, write credentials, DNS, or enable UFW.
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Run as root on the target host' >&2; exit 1; }
. /etc/os-release
[[ "$ID" == ubuntu && "$VERSION_ID" == 24.04 && $(dpkg --print-architecture) == arm64 ]]
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
Architectures: arm64
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
echo 'Runtime installed. Configure operator.env, firewall recovery, and staging approval next.'
