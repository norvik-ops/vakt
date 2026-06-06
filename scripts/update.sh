#!/usr/bin/env bash
# update.sh — Safely update a self-hosted Vakt instance.
# Usage: ./scripts/update.sh [--no-backup] [--tag <version>]
set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
SERVICE_API="api"
SERVICE_WORKER="worker"
HEALTH_URL="${VAKT_HEALTH_URL:-http://localhost:8080/health/ready}"
HEALTH_RETRIES=30
HEALTH_WAIT=2
SKIP_BACKUP=false
TAG="latest"

# ── Dependency checks ─────────────────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
  echo "ERROR: Docker is not installed. See https://docs.docker.com/get-docker/"
  exit 1
fi

if docker compose version &>/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
elif docker-compose version &>/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
else
  echo "ERROR: docker compose (v2) or docker-compose (v1) is required."
  exit 1
fi

# ── Parse flags ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-backup) SKIP_BACKUP=true; shift ;;
    --tag) TAG="$2"; shift 2 ;;
    *) echo "WARNING: Unknown argument: $1"; shift ;;
  esac
done

# Load .env if present
if [ -f .env ]; then
  set -a; source .env; set +a
fi

echo "==> Vakt Update — $(date '+%Y-%m-%d %H:%M:%S')"
echo "    Target tag:   ${TAG}"
echo "    Compose file: ${COMPOSE_FILE}"

# ── Step 1: Backup ────────────────────────────────────────────────────────────
echo ""
if [[ "$SKIP_BACKUP" == "false" ]]; then
  echo "==> Step 1/5: Creating backup before update..."
  if [[ -f ./scripts/backup.sh ]]; then
    bash ./scripts/backup.sh
    echo "    Backup complete."
  else
    echo "    WARNING: backup.sh not found. Proceeding without backup."
    echo "    Use --no-backup to suppress this warning."
  fi
else
  echo "==> Step 1/5: Skipping backup (--no-backup)"
fi

# ── Step 2: Pull new images ───────────────────────────────────────────────────
echo ""
echo "==> Step 2/5: Pulling new images (tag: ${TAG})..."
if [[ "$TAG" != "latest" ]]; then
  VAKT_TAG="$TAG" $COMPOSE_CMD -f "$COMPOSE_FILE" pull "$SERVICE_API" "$SERVICE_WORKER"
else
  $COMPOSE_CMD -f "$COMPOSE_FILE" pull "$SERVICE_API" "$SERVICE_WORKER"
fi
echo "    Images pulled."

# ── Step 3: Run migrations ────────────────────────────────────────────────────
echo ""
echo "==> Step 3/5: Running database migrations..."
$COMPOSE_CMD -f "$COMPOSE_FILE" run --rm migrate
echo "    Migrations complete."

# ── Step 4: Restart services ──────────────────────────────────────────────────
echo ""
echo "==> Step 4/5: Restarting services..."
$COMPOSE_CMD -f "$COMPOSE_FILE" up -d --no-deps "$SERVICE_API" "$SERVICE_WORKER"
echo "    Services restarting..."

# ── Step 5: Health check ──────────────────────────────────────────────────────
echo ""
echo "==> Step 5/5: Waiting for health check (${HEALTH_URL})..."
for i in $(seq 1 "$HEALTH_RETRIES"); do
  if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
    echo "    Health check passed after $((i * HEALTH_WAIT))s."
    echo ""
    echo "Update complete! Vakt is running with the new version."
    exit 0
  fi
  sleep "$HEALTH_WAIT"
done

echo ""
echo "ERROR: Health check failed after $((HEALTH_RETRIES * HEALTH_WAIT))s."
echo "  Check logs: $COMPOSE_CMD logs $SERVICE_API"
echo ""
echo "  To rollback: restore from the backup created in Step 1."
echo "  Run: ./scripts/restore.sh <backup-file>"
exit 1
