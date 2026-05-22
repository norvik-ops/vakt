#!/usr/bin/env bash
# rotate-key.sh — Vakt master key rotation
#
# Usage: ./scripts/rotate-key.sh [--db-url <url>] [--old-key <hex>] [--new-key <hex>]
#
# If --old-key / --new-key are omitted, OLD_KEY is read from VAKT_SECRET_KEY
# and a fresh 32-byte NEW_KEY is generated via openssl.
#
# Steps:
#   1. Re-encrypt all so_secrets (Vault secrets) with the new key
#   2. Re-encrypt all totp_secrets.secret (Auth TOTP secrets) with the new key
#   3. Print the new VAKT_SECRET_KEY value to stdout
#
# The script is idempotent: if interrupted, re-run with the same --new-key
# to resume (already-rotated rows are skipped because their decrypt with
# the old key will fail and they will be treated as already migrated).
#
# Requires: psql, openssl (for key generation)

set -euo pipefail

DB_URL="${VAKT_DB_URL:-}"
OLD_KEY="${VAKT_SECRET_KEY:-}"
NEW_KEY=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --db-url)  DB_URL="$2";  shift 2 ;;
    --old-key) OLD_KEY="$2"; shift 2 ;;
    --new-key) NEW_KEY="$2"; shift 2 ;;
    *) echo "Unknown argument: $1"; exit 1 ;;
  esac
done

if [[ -z "$DB_URL" ]]; then
  echo "ERROR: --db-url or VAKT_DB_URL is required" >&2
  exit 1
fi
if [[ -z "$OLD_KEY" ]]; then
  echo "ERROR: --old-key or VAKT_SECRET_KEY is required" >&2
  exit 1
fi
if [[ -z "$NEW_KEY" ]]; then
  NEW_KEY=$(openssl rand -hex 32)
  echo "Generated new key: $NEW_KEY"
fi

if [[ ${#OLD_KEY} -ne 64 ]] || [[ ${#NEW_KEY} -ne 64 ]]; then
  echo "ERROR: keys must be 64 hex characters (32 bytes)" >&2
  exit 1
fi

echo "=== Vakt Key Rotation ==="
echo "DB: $DB_URL"
echo "Old key: ${OLD_KEY:0:8}...(redacted)"
echo "New key: ${NEW_KEY:0:8}...(redacted)"
echo ""

# Run the Go rotation binary
# The backend/cmd/rotate-key tool handles the actual crypto operations.
cd "$(dirname "$0")/../backend"

VAKT_DB_URL="$DB_URL" \
VAKT_OLD_SECRET_KEY="$OLD_KEY" \
VAKT_NEW_SECRET_KEY="$NEW_KEY" \
  go run ./cmd/rotate-key

echo ""
echo "=== Rotation complete ==="
echo ""
echo "Update your .env / docker-compose.yml / Kubernetes secret:"
echo "  VAKT_SECRET_KEY=$NEW_KEY"
echo ""
echo "Restart all Vakt containers after updating the key."
