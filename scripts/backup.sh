#!/usr/bin/env bash
set -euo pipefail

# Vakt backup script — exports PostgreSQL dump + encryption key.
# Usage: ./scripts/backup.sh [output-dir]
# Requires: pg_dump, openssl, docker (if using compose)

OUTPUT_DIR="${1:-.}"
DATE=$(date +%Y-%m-%d_%H-%M-%S)
BACKUP_NAME="vakt-backup-${DATE}"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# Load env if .env exists
if [ -f .env ]; then
  set -a; source .env; set +a
fi

DB_URL="${VAKT_DB_URL:-}"
SECRET_KEY="${VAKT_SECRET_KEY:-}"

if [ -z "$DB_URL" ]; then
  echo "ERROR: VAKT_DB_URL not set" >&2
  exit 1
fi

if [ -z "$SECRET_KEY" ]; then
  echo "ERROR: VAKT_SECRET_KEY not set" >&2
  exit 1
fi

echo "→ Dumping PostgreSQL..."
pg_dump "$DB_URL" --format=custom --compress=9 -f "$WORK_DIR/db.pgdump"

echo "→ Encrypting encryption key..."
while true; do
  read -r -s -p "   Enter passphrase (min. 12 characters): " PASSPHRASE; echo
  if [ "${#PASSPHRASE}" -lt 12 ]; then
    echo "   ERROR: Passphrase must be at least 12 characters." >&2; continue
  fi
  read -r -s -p "   Confirm passphrase: " PASSPHRASE2; echo
  if [ "$PASSPHRASE" != "$PASSPHRASE2" ]; then
    echo "   ERROR: Passphrases do not match." >&2; continue
  fi
  break
done
echo "$SECRET_KEY" | openssl enc -aes-256-cbc -pbkdf2 -pass "pass:$PASSPHRASE" -out "$WORK_DIR/secret.key.enc"
unset PASSPHRASE PASSPHRASE2

# Write manifest
cat > "$WORK_DIR/manifest.json" <<EOF
{
  "backup_date": "${DATE}",
  "schema_version": "$(date +%Y%m%d)",
  "tool": "vakt-backup"
}
EOF

echo "→ Creating archive..."
tar -czf "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" -C "$WORK_DIR" .

echo "→ Signing archive..."
openssl dgst -sha256 -hmac "$SECRET_KEY" "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" \
  | awk '{print $NF}' > "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig"

echo "✓ Backup saved:    ${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz"
echo "✓ Signature saved: ${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz.sig"
