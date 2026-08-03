#!/bin/bash

# toggle_ports.sh
# Toggles the public exposure of Postgres (5432) and Redis (6379)
# by changing their port bindings in docker-compose.yml and restarting them.
# Docker bypasses UFW by default, so binding to 127.0.0.1 is the safest way to restrict access.

COMPOSE_FILE="docker-compose.yml"

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "Error: $COMPOSE_FILE not found in the current directory."
    exit 1
fi

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Determine whether to use 'docker compose' or 'docker-compose'
if command_exists docker && docker compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
elif command_exists docker-compose; then
    DOCKER_COMPOSE="docker-compose"
else
    echo "Error: docker compose is not installed."
    exit 1
fi

case "$1" in
    open)
        echo "[+] Exposing Postgres and Redis to the public internet..."
        # Replace 127.0.0.1 bindings with public bindings
        sed -i 's/- "127.0.0.1:5432:5432"/- "5432:5432"/' "$COMPOSE_FILE"
        sed -i 's/- "127.0.0.1:6379:6379"/- "6379:6379"/' "$COMPOSE_FILE"
        
        # Also update UFW (for completeness, even though Docker bypasses it)
        if command_exists ufw; then
            sudo ufw allow 5432/tcp >/dev/null 2>&1
            sudo ufw allow 6379/tcp >/dev/null 2>&1
            echo "[+] UFW rules added."
        fi
        ;;
    close)
        echo "[-] Restricting Postgres and Redis to localhost only..."
        # Replace public bindings with 127.0.0.1 bindings
        sed -i 's/- "5432:5432"/- "127.0.0.1:5432:5432"/' "$COMPOSE_FILE"
        sed -i 's/- "6379:6379"/- "127.0.0.1:6379:6379"/' "$COMPOSE_FILE"
        
        # Update UFW
        if command_exists ufw; then
            sudo ufw delete allow 5432/tcp >/dev/null 2>&1
            sudo ufw delete allow 6379/tcp >/dev/null 2>&1
            echo "[-] UFW rules removed."
        fi
        ;;
    status)
        echo "Current bindings in $COMPOSE_FILE:"
        grep -E "5432:5432|6379:6379" "$COMPOSE_FILE"
        exit 0
        ;;
    *)
        echo "Usage: $0 {open|close|status}"
        echo "  open   - Bind ports to 0.0.0.0 (Expose to public internet)"
        echo "  close  - Bind ports to 127.0.0.1 (Restrict to localhost)"
        echo "  status - Check current port bindings"
        exit 1
        ;;
esac

echo "[*] Applying changes and restarting containers..."
$DOCKER_COMPOSE up -d postgres redis

echo "[*] Done. Status of Postgres and Redis:"
$DOCKER_COMPOSE ps postgres redis
