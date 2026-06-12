#!/usr/bin/env bash
set -euo pipefail

BACKUP_FILE="${1:-}"
if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
  echo "ERROR: Usage: $0 <backup-file.tar.gz>" >&2
  exit 1
fi

SIG_FILE="${BACKUP_FILE}.sig"
if [ ! -f "$SIG_FILE" ]; then
  echo "ERROR: Signature file not found: $SIG_FILE" >&2
  exit 1
fi

if [ -f .env ]; then
  set -a; source .env; set +a
fi

SECRET_KEY="${VAKT_SECRET_KEY:-}"
if [ -z "$SECRET_KEY" ]; then
  echo "ERROR: VAKT_SECRET_KEY not set" >&2
  exit 1
fi

echo "→ Verifying HMAC-SHA256 signature..."
EXPECTED=$(cat "$SIG_FILE")
# Use the same derived HMAC key as backup.sh.
HMAC_KEY=$(printf 'vakt-backup-hmac:%s' "$SECRET_KEY" | sha256sum | cut -d' ' -f1)
ACTUAL=$(openssl dgst -sha256 -hmac "$HMAC_KEY" "$BACKUP_FILE" | awk '{print $NF}')
unset HMAC_KEY
if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "ERROR: HMAC signature mismatch — archive may be corrupted or tampered with" >&2
  exit 1
fi
echo "✓ HMAC signature valid"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

echo "→ Extracting..."
tar -xzf "$BACKUP_FILE" -C "$WORK_DIR"

echo "→ Verifying manifest..."
cat "$WORK_DIR/manifest.json"

echo "→ Checking dump integrity..."
pg_restore --list "$WORK_DIR/db.pgdump" > /dev/null && echo "✓ Dump is valid"

echo "✓ Backup verification passed: $BACKUP_FILE"
