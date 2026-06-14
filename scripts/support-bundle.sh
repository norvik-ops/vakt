#!/usr/bin/env bash
set -euo pipefail

# Vakt support bundle — collects diagnostics for a support case into one archive.
# Usage: ./scripts/support-bundle.sh [output-dir]
# Env:
#   TAIL=2000    number of recent log lines per service (default 2000)
#   SINCE=       only logs newer than this (e.g. "30m", "2026-06-14T08:00:00")
#
# NOTHING leaves your machine — this only writes a local archive. Logs are
# PII-redacted (emails appear as ***@domain) but still contain domains, IPs and
# request URLs, so review the archive before sending it. See docs/wiki/support.md.

OUTPUT_DIR="${1:-.}"
TAIL="${TAIL:-2000}"
SINCE="${SINCE:-}"
DATE=$(date +%Y%m%d-%H%M%S)
BUNDLE_NAME="vakt-support-${DATE}"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT
OUT="$WORK_DIR/$BUNDLE_NAME"
mkdir -p "$OUT"

# Resolve docker compose v2 ("docker compose") vs legacy v1 ("docker-compose").
if docker compose version >/dev/null 2>&1; then
	COMPOSE=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
	COMPOSE=(docker-compose)
else
	echo "ERROR: neither 'docker compose' nor 'docker-compose' found." >&2
	echo "Run this script in the directory containing your docker-compose.yml." >&2
	exit 1
fi

LOG_ARGS=(--tail="$TAIL" --no-color)
[ -n "$SINCE" ] && LOG_ARGS+=(--since="$SINCE")

echo "Collecting Vakt support bundle..."

# 1. Meta / versions
{
	echo "Generated:   $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	echo "VAKT_TAG:    ${VAKT_TAG:-latest}"
	echo "Host:        $(uname -srm)"
	echo "Docker:      $(docker --version 2>/dev/null || echo 'n/a')"
	echo "Compose:     $("${COMPOSE[@]}" version --short 2>/dev/null || echo 'n/a')"
	echo "Tail/svc:    $TAIL lines${SINCE:+ (since $SINCE)}"
} > "$OUT/meta.txt"

# 2. Container status
"${COMPOSE[@]}" ps > "$OUT/compose-ps.txt" 2>&1 || true

# 3. Health endpoints (best effort — instance may not be on localhost:80)
{
	echo "# GET /health"
	curl -fsS http://localhost/health 2>&1 || echo "(unreachable)"
	echo; echo "# GET /health/ready"
	curl -fsS http://localhost/health/ready 2>&1 || echo "(unreachable)"
} > "$OUT/health.txt"

# 4. Per-service logs. Discover services dynamically so the bundle stays correct
# even if the compose file changes.
mkdir -p "$OUT/logs"
SERVICES=$("${COMPOSE[@]}" config --services 2>/dev/null || echo "api worker nginx postgres pgbouncer redis ollama")
for svc in $SERVICES; do
	"${COMPOSE[@]}" logs "${LOG_ARGS[@]}" "$svc" > "$OUT/logs/${svc}.log" 2>&1 || true
done

# 5. Package
ARCHIVE="$(cd "$OUTPUT_DIR" && pwd)/${BUNDLE_NAME}.tar.gz"
tar -czf "$ARCHIVE" -C "$WORK_DIR" "$BUNDLE_NAME"

echo "✓ Support bundle written to: $ARCHIVE"
echo
echo "Before sending: review the contents — logs contain domains, IPs and URLs."
echo "  tar -tzf \"$ARCHIVE\"        # list files"
echo "  tar -xzf \"$ARCHIVE\"        # extract to inspect"
