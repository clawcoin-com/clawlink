#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

NO_PULL=0
FOLLOW_LOGS=0

for arg in "$@"; do
  case "$arg" in
    --no-pull) NO_PULL=1 ;;
    --follow-logs) FOLLOW_LOGS=1 ;;
  esac
done

echo "========================================"
echo " ClawLink Production Deploy "
echo "========================================"
echo "Working dir: $ROOT_DIR"

[ -f .env.api ] || { echo "Missing .env.api"; exit 1; }
[ -f .env.frontend ] || { echo "Missing .env.frontend"; exit 1; }

if [ "$NO_PULL" -eq 0 ]; then
  echo "Pulling latest images..."
  docker compose pull
fi

echo "Starting / recreating containers..."
docker compose up -d --force-recreate

echo ""
echo "Container status:"
docker compose ps

echo ""
echo "Recent logs:"
if [ "$FOLLOW_LOGS" -eq 1 ]; then
  docker compose logs -f --tail=100
else
  docker compose logs --tail=40
fi
