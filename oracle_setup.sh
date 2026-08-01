#!/bin/bash

# ==============================================================================
# Oracle Cloud VM Setup Script for Inox Backend Infrastructure
# ==============================================================================

# Exit immediately if a command exits with a non-zero status
set -e

echo "Starting Inox Oracle Cloud Setup..."

# 1. Update and Install Prerequisites
echo "Updating system and installing dependencies..."
sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get update
sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get upgrade -y -q -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"
sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get install -y -q curl wget git jq ufw build-essential

# 2. Setup Swap (Crucial for 1GB RAM instance)
# We will create a 4GB swap file to prevent Out-Of-Memory (OOM) errors when 
# running Postgres, Redis, Minio, and the Go backend simultaneously.
if [ -f /swapfile ]; then
    echo "Swapfile already exists. Skipping."
else
    echo "Setting up 4GB swap space..."
    sudo fallocate -l 4G /swapfile
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile
    echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
    
    # Adjust swappiness to prefer RAM over swap when possible, but allow swap
    sudo sysctl vm.swappiness=10
    echo 'vm.swappiness=10' | sudo tee -a /etc/sysctl.conf
fi

# 3. Install Docker and Docker Compose
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    rm get-docker.sh
    
    # Add ubuntu user to docker group so you don't need sudo for docker commands
    sudo usermod -aG docker ubuntu
    echo "Docker installed successfully."
else
    echo "Docker is already installed."
fi

# Configure Docker logging to prevent disk space exhaustion
echo "Configuring Docker log rotation..."
sudo mkdir -p /etc/docker
cat <<EOF | sudo tee /etc/docker/daemon.json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "3"
  }
}
EOF
sudo systemctl restart docker

# 4. Install Go (For compiling/running the backend natively if not dockerized)
GO_VERSION="1.22.5"
if ! command -v go &> /dev/null; then
    echo "Installing Go ${GO_VERSION}..."
    wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
    rm go${GO_VERSION}.linux-amd64.tar.gz
    
    # Add to path for the ubuntu user
    if ! grep -q "/usr/local/go/bin" /home/ubuntu/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /home/ubuntu/.profile
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /home/ubuntu/.bashrc
    fi
fi

# 5. Firewall / Ports configuration (Ubuntu UFW)
# NOTE: You MUST also open these ports in the Oracle Cloud Console (VCN Security Lists)
echo "Configuring UFW (Uncomplicated Firewall)..."
sudo ufw allow OpenSSH

# Only open these if you are NOT using a VPN (like Tailscale)
# It's highly recommended to use Tailscale and restrict these ports to the tailscale interface (tailscale0)
sudo ufw allow 5432/tcp  # Postgres (for local transcoding worker)
sudo ufw allow 6379/tcp  # Redis (for local transcoding worker)
sudo ufw allow 9000/tcp  # Minio API (for local transcoding worker and frontend)
sudo ufw allow 8080/tcp  # Go Backend Server API
sudo ufw allow 5050/tcp  # PGAdmin (if used, consider disabling in production)
sudo ufw allow 9001/tcp  # Minio Console
sudo ufw allow 80/tcp    # HTTP (for Caddy/SSL setup)
sudo ufw allow 443/tcp   # HTTPS (for Caddy/SSL setup)

echo "y" | sudo ufw enable

# 6. Setup Project Directory and Environment Template
echo "Setting up environment template..."
PROJECT_DIR="/home/ubuntu/inox"

cat << 'EOF' > /home/ubuntu/inox-env.template
# Inox Backend Environment Variables
# Copy this to /home/ubuntu/inox/.env and change passwords!

# Postgres
POSTGRES_USER=postgres
POSTGRES_PASSWORD=CHANGE_ME_STRONG_PASSWORD
POSTGRES_DB=inox
DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/${POSTGRES_DB}?sslmode=disable

# Redis
# IMPORTANT: Use a password if exposed to the internet!
REDIS_PASSWORD=CHANGE_ME_STRONG_PASSWORD
REDIS_URL=redis://:${REDIS_PASSWORD}@localhost:6379/0

# Minio
MINIO_ROOT_USER=admin
MINIO_ROOT_PASSWORD=CHANGE_ME_STRONG_PASSWORD
MINIO_ENDPOINT=YOUR_ORACLE_PUBLIC_IP:9000
MINIO_USE_SSL=false

# Backend Server
PORT=8080
GIN_MODE=release
EOF

echo "======================================================================"
echo "SETUP COMPLETED SUCCESSFULLY!"
echo "======================================================================"
echo "IMPORTANT NEXT STEPS:"
echo "1. Log out and log back in so the Docker group permissions take effect."
echo "2. Oracle Cloud Firewall: You MUST open ports (80, 443, 5432, 6379, 9000, 8080) in the Oracle Cloud VCN Security List!"
echo "3. Copy /home/ubuntu/inox-env.template to $PROJECT_DIR/.env and SET SECURE PASSWORDS."
echo "4. In your docker-compose.yml, add the redis password to the redis command if exposing it."
echo "5. CD into $PROJECT_DIR and start infrastructure: docker compose up -d"
echo "6. Start Go server: cd $PROJECT_DIR && export PATH=\$PATH:/usr/local/go/bin && go build -o server . && ./server"
echo "======================================================================"
