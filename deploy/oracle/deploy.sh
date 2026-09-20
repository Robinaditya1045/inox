#!/usr/bin/env bash
# ==============================================================================
# Ship the Inox backend + admin portal to the Oracle Cloud VM.
# Run from a workstation; the VM only has one core, so the admin bundle is built
# here rather than there.
#
#   ./deploy/oracle/deploy.sh            # build, sync, rebuild image, restart
#   SKIP_ADMIN=1 ./deploy/oracle/deploy.sh   # backend only
# ==============================================================================
set -euo pipefail

SSH_TARGET="${SSH_TARGET:-opc@140.238.248.49}"
REMOTE_DIR="${REMOTE_DIR:-/home/opc/inox}"
PUBLIC_HOST="${PUBLIC_HOST:-140.238.248.49.sslip.io}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

if [[ "${SKIP_ADMIN:-0}" != "1" ]]; then
    echo "==> Building admin portal for https://${PUBLIC_HOST}/admin/"
    (
        cd admin-portal
        [[ -d node_modules ]] || npm ci
        ADMIN_BASE_PATH=/admin/ \
        VITE_API_BASE_URL="https://${PUBLIC_HOST}/api/v1" \
        VITE_TELEMETRY_WS_URL="wss://${PUBLIC_HOST}/api/v1/admin/telemetry/ws" \
        VITE_ENABLE_DEMO_MODE=false \
        VITE_LOG_LEVEL=info \
        npm run build
    )
    rm -rf deploy/oracle/admin-dist
    cp -r admin-portal/dist deploy/oracle/admin-dist
fi

echo "==> Syncing sources to ${SSH_TARGET}:${REMOTE_DIR}"
ssh "$SSH_TARGET" "mkdir -p ${REMOTE_DIR}/backend ${REMOTE_DIR}/deploy/oracle"

rsync -az --delete \
    --exclude 'storage_data/' \
    --exclude '.env' \
    backend/ "${SSH_TARGET}:${REMOTE_DIR}/backend/"

# .env.prod lives only on the server — never overwrite the live secrets from here.
rsync -az --delete --exclude '.env.prod' \
    deploy/oracle/ "${SSH_TARGET}:${REMOTE_DIR}/deploy/oracle/"

echo "==> Rebuilding and restarting the stack"
ssh "$SSH_TARGET" "cd ${REMOTE_DIR}/deploy/oracle && \
    docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build && \
    docker compose --env-file .env.prod -f docker-compose.prod.yml ps"

echo "==> Deployed. Verifying..."
curl -fsS "https://${PUBLIC_HOST}/healthz" && echo
echo "    API   https://${PUBLIC_HOST}/api/v1"
echo "    Admin https://${PUBLIC_HOST}/admin/"
