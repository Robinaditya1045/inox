#!/bin/bash

# enable_https.sh
# Installs Caddy Server and configures automatic HTTPS for the backend
# using sslip.io (a free DNS service that maps IP addresses to domains).

if [ -z "$1" ]; then
    echo "Usage: $0 <YOUR_ORACLE_PUBLIC_IP>"
    echo "Example: $0 80.225.246.216"
    exit 1
fi

PUBLIC_IP=$1
DOMAIN="${PUBLIC_IP}.sslip.io"

echo "Setting up HTTPS for $DOMAIN..."

# 1. Install Caddy (Official Debian/Ubuntu instructions)
echo "Installing Caddy..."
sudo apt-get update
sudo apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt-get update
sudo apt-get install caddy -y

# 2. Configure Caddyfile
echo "Configuring Caddyfile..."
cat <<EOF | sudo tee /etc/caddy/Caddyfile
$DOMAIN {
    # Enable gzip compression
    encode gzip

    # Proxy all traffic to the Go backend running on port 8080
    reverse_proxy localhost:8080
}
EOF

# 3. Open Firewall Ports for HTTP (80) and HTTPS (443)
echo "Configuring UFW firewall..."
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# 4. Restart Caddy to apply changes
echo "Restarting Caddy to acquire SSL certificate..."
sudo systemctl restart caddy

echo "======================================================================"
echo "HTTPS SETUP COMPLETE!"
echo "Your backend is now available at: https://$DOMAIN"
echo ""
echo "IMPORTANT NEXT STEPS:"
echo "1. Oracle Cloud Firewall: You MUST open ports 80 and 443 in the Oracle Cloud VCN Security List!"
echo "2. Update your Vercel Environment Variables in the Vercel Dashboard:"
echo "     VITE_API_BASE_URL=https://$DOMAIN/api/v1"
echo "     VITE_WS_BASE_URL=wss://$DOMAIN/api/v1"
echo "3. Redeploy your frontend on Vercel so the new environment variables take effect."
echo "======================================================================"
