#!/bin/bash
# CI deploy gate — restricted SSH ForceCommand for GitHub Actions runner
# Commands: migrate | deploy | rollback | logs
#
# Installation (norvikserver):
#   cp scripts/deploy-gate.sh /usr/local/bin/vakt-deploy-gate.sh
#   chmod 700 /usr/local/bin/vakt-deploy-gate.sh
#
# In /root/.ssh/authorized_keys, add the deploy key line:
#   command="/usr/local/bin/vakt-deploy-gate.sh",no-pty,no-X11-forwarding,no-agent-forwarding <pubkey>

set -euo pipefail

COMPOSE_DIR=/root/sechealth-server
COMPOSE="docker compose -p sechealth-server --profile demo -f $COMPOSE_DIR/docker-compose.yml"
APP_VERSION=$(grep '^APP_VERSION=' $COMPOSE_DIR/.env | cut -d= -f2 | tr -d '"')
REGISTRY=ghcr.io/matharnica
NETWORK=sechealth-server_default

log() { echo "[$(date -u +%H:%M:%S)] $*"; }

case "${SSH_ORIGINAL_COMMAND:-}" in
  migrate)
    log 'Pulling api image for migration...'
    docker pull "${REGISTRY}/vakt-api:${APP_VERSION}"
    log 'Running migrations...'
    docker run --rm \
      --network "${NETWORK}" \
      --env-file "${COMPOSE_DIR}/.env" \
      --entrypoint /migrate \
      "${REGISTRY}/vakt-api:${APP_VERSION}"
    log 'Migrations done.'
    ;;
  deploy)
    log 'Pulling new demo images...'
    docker pull "${REGISTRY}/vakt-api:${APP_VERSION}"
    docker pull "${REGISTRY}/vakt-frontend:${APP_VERSION}"
    docker pull "${REGISTRY}/vakt-scanners:${APP_VERSION}" 2>/dev/null || true
    log 'Restarting containers...'
    ${COMPOSE} up -d --no-deps --pull never vakt-worker
    ${COMPOSE} up -d --no-deps --pull never vakt-api
    ${COMPOSE} up -d --no-deps --pull never vakt-frontend
    log 'Deploy done.'
    ;;
  rollback)
    log 'Rolling back to :demo-stable...'
    docker pull "${REGISTRY}/vakt-api:demo-stable" 2>/dev/null && \
      APP_VERSION=demo-stable ${COMPOSE} up -d --no-deps --pull never vakt-worker vakt-api || \
      log 'WARNING: demo-stable not found, rollback skipped'
    docker pull "${REGISTRY}/vakt-frontend:demo-stable" 2>/dev/null && \
      APP_VERSION=demo-stable ${COMPOSE} up -d --no-deps --pull never vakt-frontend || true
    log 'Rollback done.'
    ;;
  logs)
    ${COMPOSE} logs --tail=80 vakt-api vakt-worker vakt-frontend 2>&1 | tail -80
    ;;
  *)
    echo "Unknown command: ${SSH_ORIGINAL_COMMAND:-<empty>}" >&2
    exit 1
    ;;
esac
