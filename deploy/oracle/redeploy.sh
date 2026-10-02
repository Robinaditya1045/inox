#!/usr/bin/env bash
# ==============================================================================
# Redeploy the stack in place, on the Oracle VM itself.
#
# Recreates every service whose configuration changed -- typically after an edit
# to .env.prod, which a plain `docker compose restart` does not pick up -- on the
# image already pinned in BACKEND_IMAGE, then waits for the backend to report
# healthy. Shipping new code is still a push to main; this never builds or pulls.
#
#   ~/inox/deploy/oracle/redeploy.sh           # recreate what changed
#   ~/inox/deploy/oracle/redeploy.sh --force   # recreate every container
# ==============================================================================
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -f .env.prod ]; then
  echo "error: .env.prod not found in $(pwd)" >&2
  exit 1
fi

compose=(docker compose --env-file .env.prod -f docker-compose.prod.yml)
up_args=(up -d --no-build)
case "${1:-}" in
  "") ;;
  --force) up_args+=(--force-recreate) ;;
  *)
    echo "usage: $0 [--force]" >&2
    exit 2
    ;;
esac

# --no-build: the host must never fall back to compiling on its single core.
"${compose[@]}" "${up_args[@]}"
"${compose[@]}" ps

public_host=$(grep -m1 '^PUBLIC_HOST=' .env.prod | cut -d= -f2-)
for attempt in $(seq 1 10); do
  body=$(curl -fsS --max-time 15 "https://${public_host}/healthz" || true)
  echo "attempt ${attempt}: ${body:-<no response>}"
  case "$body" in
    *'"status":"ok"'*)
      echo "Redeployed and healthy: https://${public_host}"
      exit 0
      ;;
  esac
  sleep 10
done
echo "error: backend did not report healthy after the redeploy" >&2
exit 1
