#!/usr/bin/env bash
# ==============================================================================
# Manual deploy of the Inox backend + admin portal to the Oracle Cloud VM.
#
# Pushing to main normally deploys via .github/workflows/ci.yml, which builds
# the image on a native arm64 runner. This script is the fallback for when CI
# is unavailable or you need to ship an uncommitted change; it builds on the
# host instead, which takes minutes on its single core.
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
# Pin BACKEND_IMAGE to a local tag and record it the same way CI does, so the
# host's .env.prod always states which image is actually running.
ssh "$SSH_TARGET" "cd ${REMOTE_DIR}/deploy/oracle && \
    if grep -q '^BACKEND_IMAGE=' .env.prod; then \
        sed -i 's|^BACKEND_IMAGE=.*|BACKEND_IMAGE=inox-backend:local|' .env.prod; \
    else \
        echo 'BACKEND_IMAGE=inox-backend:local' >> .env.prod; \
    fi && \
    docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --build && \
    docker compose --env-file .env.prod -f docker-compose.prod.yml ps"

echo "==> Deployed. Verifying..."
curl -fsS "https://${PUBLIC_HOST}/healthz" && echo
echo "    API   https://${PUBLIC_HOST}/api/v1"
echo "    Admin https://${PUBLIC_HOST}/admin/"
