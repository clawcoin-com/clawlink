#!/usr/bin/env sh
set -eu

if [ "$#" -lt 2 ]; then
  echo "Usage: ./scripts/rollback.sh <api-image> <frontend-image>"
  exit 1
fi

API_IMAGE="$1"
FRONTEND_IMAGE="$2"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

cat > .env.images <<EOF
API_IMAGE=$API_IMAGE
FRONTEND_IMAGE=$FRONTEND_IMAGE
EOF

echo "========================================"
echo " ClawLink Production Rollback "
echo "========================================"
echo "API_IMAGE=$API_IMAGE"
echo "FRONTEND_IMAGE=$FRONTEND_IMAGE"

docker compose --env-file .env.images up -d --force-recreate

echo ""
echo "Container status:"
docker compose --env-file .env.images ps

echo ""
echo "Recent logs:"
docker compose --env-file .env.images logs --tail=40
