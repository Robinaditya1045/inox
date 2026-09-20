#!/usr/bin/env bash
# ==============================================================================
# One-time host preparation for the Inox Oracle Cloud VM (Oracle Linux 9, arm64).
# Idempotent — safe to re-run.
# ==============================================================================
set -euo pipefail

echo "==> Installing Docker Engine + Compose plugin"
if ! command -v docker >/dev/null 2>&1; then
    sudo dnf -y install dnf-plugins-core
    sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    # --allowerasing: Oracle Linux ships podman's runc, which conflicts with containerd.io
    sudo dnf -y install --allowerasing docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    sudo systemctl enable --now docker
    sudo usermod -aG docker "$USER"
else
    echo "    docker already present: $(docker --version 2>/dev/null || sudo docker --version)"
fi

echo "==> Capping container log growth (30 GB root disk)"
sudo mkdir -p /etc/docker
echo '{"log-driver":"json-file","log-opts":{"max-size":"20m","max-file":"3"}}' | sudo tee /etc/docker/daemon.json >/dev/null
sudo systemctl restart docker

echo "==> Opening host firewall"
# Only 80/443 (Caddy) and the SFU's UDP media range face the internet. Postgres,
# Redis and MinIO stay on 127.0.0.1 and are deliberately absent from this list.
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --permanent --add-port=50000-50100/udp
sudo firewall-cmd --reload
sudo firewall-cmd --list-all

cat <<'NOTE'

==> Host is ready.

REMAINING MANUAL STEP — the host firewall is only half of it. Oracle Cloud also
filters at the VCN level, and that can only be changed from the console:

  Networking > Virtual Cloud Networks > <your VCN> > Security Lists > Default
  Add ingress rules (Source CIDR 0.0.0.0/0, stateless: No):
    TCP  80          HTTP / ACME challenge
    TCP  443         HTTPS + WSS
    UDP  50000-50100 WebRTC media (voice + screen share)

Without the TCP rules Let's Encrypt cannot validate and Caddy will never get a
certificate. Without the UDP rule everything works except voice chat.
NOTE
