#!/usr/bin/env bash
# ==============================================================================
# Inox End-to-End Cloudflare Tunnel Orchestrator (cloud_dev.sh)
# 
# What this automation script does:
#   1. Cleans up any existing cloudflared tunnels
#   2. Starts Docker infrastructure (Postgres, Redis, MinIO)
#   3. Launches background Cloudflare Quick Tunnels using HTTP/2 (for zero packet drop / QUIC stability)
#   4. Captures live public URLs from cloudflared logs dynamically
#   5. Updates `tunnel.env` and executes `./toggle_tunnel.sh --on` to inject URLs across all `.env` files
#   6. Displays a prominent ASCII banner with the Public URLs + saves to `urls.txt`
#   7. Launches `./dev.sh` (which boots the tmux session running Go Backend + Vite Frontends)
# ==============================================================================

set -e

BACKEND_LOG="/tmp/inox_tunnel_backend.log"
FRONTEND_LOG="/tmp/inox_tunnel_frontend.log"
URLS_FILE="urls.txt"

show_help() {
    echo "Usage: ./cloud_dev.sh [COMMAND]"
    echo "Commands:"
    echo "  (no args)  Start Cloudflare tunnels, update envs, and launch dev.sh"
    echo "  --stop     Stop all running tunnels, revert envs to localhost, and stop tmux dev session"
    echo "  --help     Show this help message"
}

stop_all() {
    echo "========================================================================"
    echo "🛑 Stopping Inox Cloudflare Tunnels & Dev Services..."
    echo "========================================================================"
    
    # 1. Kill background cloudflared quick tunnels
    if pgrep -f "cloudflared tunnel.*http://localhost:" > /dev/null 2>&1; then
        echo "Killing background cloudflared tunnel processes..."
        pkill -f "cloudflared tunnel.*http://localhost:" || true
    fi
    
    # 2. Revert environment files to localhost
    if [ -x "./toggle_tunnel.sh" ]; then
        ./toggle_tunnel.sh --off
    fi

    # 3. Kill tmux dev session if running
    if tmux has-session -t inox 2>/dev/null; then
        echo "Shutting down tmux session 'inox'..."
        tmux kill-session -t inox || true
    fi

    echo "✅ All cloud tunnels and dev sessions stopped cleanly."
    exit 0
}

if [ "$1" = "--stop" ] || [ "$1" = "stop" ]; then
    stop_all
fi

if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    show_help
    exit 0
fi

echo "========================================================================"
echo "🚀 INOX CLOUD DEV ORCHESTRATOR"
echo "========================================================================"

# Check dependencies
for CMD in cloudflared docker tmux; do
    if ! command -v $CMD >/dev/null 2>&1; then
        echo "❌ Error: Required command '$CMD' not found. Please install it first."
        exit 1
    fi
done

# 1. Clean up lingering old quick tunnels
echo "🧹 Cleaning up any old tunnel logs & processes..."
pkill -f "cloudflared tunnel.*http://localhost:" >/dev/null 2>&1 || true
rm -f "$BACKEND_LOG" "$FRONTEND_LOG" "$URLS_FILE"

# 2. Ensure Docker infrastructure is running
echo "🐳 Checking Docker infrastructure (Postgres, Redis, MinIO)..."
docker compose up -d

# 3. Start Cloudflare Quick Tunnels in the background using HTTP/2 protocol
#    Why --protocol http2? Because QUIC (UDP 443) frequently times out on ISPs. HTTP/2 TCP is 100% reliable.
echo "☁️  Launching background Cloudflare Tunnels (--protocol http2)..."
cloudflared tunnel --protocol http2 --url http://localhost:8080 > "$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!

cloudflared tunnel --protocol http2 --url http://localhost:5173 > "$FRONTEND_LOG" 2>&1 &
FRONTEND_PID=$!

echo "⏳ Waiting for Cloudflare edge servers to assign public URLs..."

# Loop up to 30 seconds waiting for URLs to appear in the log files
TIMEOUT=30
ELAPSED=0
BACKEND_URL=""
FRONTEND_URL=""

while [ $ELAPSED -lt $TIMEOUT ]; do
    if [ -z "$BACKEND_URL" ] && [ -f "$BACKEND_LOG" ]; then
        BACKEND_URL=$(grep -oE "https://[a-zA-Z0-9.-]+\.trycloudflare\.com" "$BACKEND_LOG" | head -1 || true)
    fi
    if [ -z "$FRONTEND_URL" ] && [ -f "$FRONTEND_LOG" ]; then
        FRONTEND_URL=$(grep -oE "https://[a-zA-Z0-9.-]+\.trycloudflare\.com" "$FRONTEND_LOG" | head -1 || true)
    fi

    if [ -n "$BACKEND_URL" ] && [ -n "$FRONTEND_URL" ]; then
        break
    fi

    sleep 1
    ELAPSED=$((ELAPSED + 1))
    echo -n "."
done
echo ""

if [ -z "$BACKEND_URL" ] || [ -z "$FRONTEND_URL" ]; then
    echo "❌ Error: Timed out waiting for Cloudflare URLs after ${TIMEOUT}s."
    if [ -z "$BACKEND_URL" ]; then
        echo "--- Backend Tunnel Log Snippet ($BACKEND_LOG) ---"
        tail -n 10 "$BACKEND_LOG" || true
    fi
    if [ -z "$FRONTEND_URL" ]; then
        echo "--- Frontend Tunnel Log Snippet ($FRONTEND_LOG) ---"
        tail -n 10 "$FRONTEND_LOG" || true
    fi
    pkill -f "cloudflared tunnel.*http://localhost:" || true
    exit 1
fi

# Strip trailing slashes just in case
BACKEND_URL=$(echo "$BACKEND_URL" | sed 's/*$//')
FRONTEND_URL=$(echo "$FRONTEND_URL" | sed 's/*$//')

# Derive WSS URL
BACKEND_WS_URL="wss://${BACKEND_URL#https://}"

# 4. Write new live URLs into tunnel.env
echo "📝 Saving captured URLs to 'tunnel.env'..."
cat << EOF > tunnel.env
# ==============================================================================
# Cloudflare Tunnel Configuration for Inox (Auto-Generated by cloud_dev.sh)
# ==============================================================================
CF_BACKEND_URL="$BACKEND_URL"
CF_FRONTEND_URL="$FRONTEND_URL"
CF_ADMIN_URL="http://localhost:3000"
CF_MINIO_URL="http://localhost:9001"
EOF

# 5. Run toggle_tunnel.sh to update all .env files (frontend, admin-portal, backend)
echo "⚡ Updating environment configuration files (`toggle_tunnel.sh --on`)..."
./toggle_tunnel.sh --on

# 6. Save public URLs to urls.txt and print prominent ASCII banner
cat << EOF > "$URLS_FILE"
========================================================================
🌟 INOX PUBLIC CLOUDFLARE URLS (Generated: $(date '+%Y-%m-%d %H:%M:%S'))
========================================================================
🎬 Watch Party Frontend : $FRONTEND_URL
⚙️  Backend API Base     : $BACKEND_URL/api/v1
🔌 Backend WebSocket    : $BACKEND_WS_URL/api/v1
📦 Local Admin Portal   : http://localhost:3000
========================================================================
EOF

echo "========================================================================"
echo "🎯 INOX IS READY FOR PUBLIC INTERNET ACCESS!"
echo "========================================================================"
cat "$URLS_FILE"
echo "💡 (Note: These URLs are also saved in '$URLS_FILE')"
echo "------------------------------------------------------------------------"
echo "🚀 Launching background dev services via './start_services.sh'..."
echo "💡 To stop everything later, run: ./cloud_dev.sh --stop"
echo "========================================================================"
sleep 2

# 7. Launch start_services.sh cleanly without attaching to tmux
./start_services.sh

